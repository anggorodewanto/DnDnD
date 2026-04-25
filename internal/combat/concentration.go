package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ConcentrationCheckDC computes the DC for a concentration saving throw
// when a concentrating caster takes damage: max(10, floor(damage / 2)).
func ConcentrationCheckDC(damage int) int {
	half := damage / 2
	if half > 10 {
		return half
	}
	return 10
}

// ConcentrationCheckResult describes whether a concentration save is needed
// after a concentrating caster takes damage.
type ConcentrationCheckResult struct {
	NeedsSave bool
	DC        int
	SpellName string
}

// CheckConcentrationOnDamage determines if a concentration save is required
// when a caster takes damage. No save is needed if not concentrating or damage is 0.
func CheckConcentrationOnDamage(currentConcentration string, damage int) ConcentrationCheckResult {
	if currentConcentration == "" || damage <= 0 {
		return ConcentrationCheckResult{}
	}
	return ConcentrationCheckResult{
		NeedsSave: true,
		DC:        ConcentrationCheckDC(damage),
		SpellName: currentConcentration,
	}
}

// ConcentrationBreakResult describes the outcome of a concentration break check.
type ConcentrationBreakResult struct {
	Broken    bool
	SpellName string
	Reason    string
}

// CheckConcentrationOnIncapacitation checks if an incapacitating condition
// should auto-break concentration (no save allowed).
func CheckConcentrationOnIncapacitation(currentConcentration string, conditions []CombatCondition) ConcentrationBreakResult {
	if currentConcentration == "" {
		return ConcentrationBreakResult{}
	}
	for _, c := range conditions {
		if incapacitatingConditions[c.Condition] {
			return ConcentrationBreakResult{
				Broken:    true,
				SpellName: currentConcentration,
				Reason:    c.Condition,
			}
		}
	}
	return ConcentrationBreakResult{}
}

// CheckConcentrationOnIncapacitationRaw checks incapacitation from raw JSON conditions.
func CheckConcentrationOnIncapacitationRaw(currentConcentration string, conditions json.RawMessage) ConcentrationBreakResult {
	conds, err := parseConditions(conditions)
	if err != nil {
		return ConcentrationBreakResult{}
	}
	return CheckConcentrationOnIncapacitation(currentConcentration, conds)
}

// HasVerbalOrSomaticComponent returns true if the spell has V or S components.
func HasVerbalOrSomaticComponent(spell refdata.Spell) bool {
	for _, c := range spell.Components {
		if c == "V" || c == "S" {
			return true
		}
	}
	return false
}

// ValidateSilenceZone checks whether a spell can be cast in a silence zone.
// Spells with V or S components cannot be cast in silence.
func ValidateSilenceZone(inSilence bool, spell refdata.Spell) error {
	if !inSilence {
		return nil
	}
	if HasVerbalOrSomaticComponent(spell) {
		return fmt.Errorf("You cannot cast %s — you are inside a zone of Silence (requires verbal/somatic components).", spell.Name)
	}
	return nil
}

// CheckConcentrationInSilence checks if being in a silence zone breaks
// concentration on a spell with V or S components.
func CheckConcentrationInSilence(currentConcentration string, inSilence bool, spell refdata.Spell) ConcentrationBreakResult {
	if currentConcentration == "" || !inSilence || !HasVerbalOrSomaticComponent(spell) {
		return ConcentrationBreakResult{}
	}
	return ConcentrationBreakResult{
		Broken:    true,
		SpellName: currentConcentration,
		Reason:    "silence",
	}
}

// ConcentrationCleanupFunc is a callback invoked when concentration breaks
// to clean up active spell effects (zones, conditions applied by the spell, etc.).
type ConcentrationCleanupFunc func(spellName string)

// FullConcentrationBreakResult includes the break result plus a formatted log message.
type FullConcentrationBreakResult struct {
	Broken    bool
	SpellName string
	Reason    string
	Message   string
}

// BreakConcentration breaks concentration on a spell, invokes the cleanup callback,
// and returns the formatted log message.
func BreakConcentration(casterName, spellName, reason string, cleanup ConcentrationCleanupFunc) FullConcentrationBreakResult {
	if cleanup != nil {
		cleanup(spellName)
	}
	msg := FormatConcentrationBreakLog(casterName, spellName, reason)
	return FullConcentrationBreakResult{
		Broken:    true,
		SpellName: spellName,
		Reason:    reason,
		Message:   msg,
	}
}

// RemoveSpellSourcedConditions strips every condition whose
// (SourceCombatantID, SourceSpell) matches (casterID, spellID) from every
// combatant in the encounter and persists the changes. Returns the total
// number of conditions removed across all combatants. This generalizes the
// per-combatant Phase 113 invisibility break to cover any concentration spell
// that applies a condition (Hold Person, Hex, Bless, Bane, Web, etc.).
func (s *Service) RemoveSpellSourcedConditions(ctx context.Context, encounterID, casterID uuid.UUID, spellID string) (int, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return 0, fmt.Errorf("listing combatants for spell-sourced cleanup: %w", err)
	}

	casterIDStr := casterID.String()
	totalRemoved := 0
	for _, c := range combatants {
		conds, perr := parseConditions(c.Conditions)
		if perr != nil {
			return totalRemoved, fmt.Errorf("parsing conditions for %s: %w", c.DisplayName, perr)
		}
		filtered := make([]CombatCondition, 0, len(conds))
		removed := 0
		for _, cond := range conds {
			if cond.SourceCombatantID == casterIDStr && cond.SourceSpell == spellID {
				removed++
				continue
			}
			filtered = append(filtered, cond)
		}
		if removed == 0 {
			continue
		}
		updated, merr := json.Marshal(filtered)
		if merr != nil {
			return totalRemoved, fmt.Errorf("marshalling conditions for %s: %w", c.DisplayName, merr)
		}
		if _, uerr := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
			ID:              c.ID,
			Conditions:      updated,
			ExhaustionLevel: c.ExhaustionLevel,
		}); uerr != nil {
			return totalRemoved, fmt.Errorf("updating conditions for %s: %w", c.DisplayName, uerr)
		}
		totalRemoved += removed
	}
	return totalRemoved, nil
}

// BreakConcentrationAndDismissSummons breaks concentration, invokes cleanup, and also
// dismisses all summoned creatures belonging to the caster. Returns the break result
// and the number of dismissed summons.
func (s *Service) BreakConcentrationAndDismissSummons(ctx context.Context, encounterID, casterID uuid.UUID, casterName, spellName, reason string, cleanup ConcentrationCleanupFunc) (FullConcentrationBreakResult, int, error) {
	result := BreakConcentration(casterName, spellName, reason, cleanup)

	dismissed, err := s.DismissSummonsByConcentration(ctx, encounterID, casterID)
	if err != nil {
		return result, 0, fmt.Errorf("dismissing summons on concentration break: %w", err)
	}

	return result, dismissed, nil
}

// ConcentrationSaveSource is the canonical pending_saves.source value used for
// damage-induced CON saves on concentrating combatants.
const ConcentrationSaveSource = "concentration"

// CheckSilenceBreaksConcentration is the Phase 118 entry point for the
// Silence-zone trigger. It looks up the combatant's authoritative
// concentration spell, fetches the spell's components, and breaks
// concentration if the spell has V or S components AND the combatant's
// current tile is inside an active "silence"-type zone in the encounter.
//
// Callers: zone-creation hook (after a Silence zone is inserted, run for
// every concentrating combatant in the encounter); position-update hook
// (Service.UpdateCombatantPosition runs this for the moving combatant
// only). Returns nil when the combatant is not concentrating, the spell
// has no V/S, or no Silence zone covers their tile.
func (s *Service) CheckSilenceBreaksConcentration(ctx context.Context, combatantID uuid.UUID) (*BreakConcentrationFullyResult, error) {
	row, err := s.store.GetCombatantConcentration(ctx, combatantID)
	if err != nil {
		return nil, fmt.Errorf("looking up concentration: %w", err)
	}
	if !row.ConcentrationSpellID.Valid || !row.ConcentrationSpellName.Valid {
		return nil, nil
	}
	combatant, err := s.store.GetCombatant(ctx, combatantID)
	if err != nil {
		return nil, fmt.Errorf("getting combatant: %w", err)
	}
	spell, err := s.store.GetSpell(ctx, row.ConcentrationSpellID.String)
	if err != nil {
		return nil, fmt.Errorf("looking up spell %q: %w", row.ConcentrationSpellID.String, err)
	}
	if !HasVerbalOrSomaticComponent(spell) {
		return nil, nil
	}
	zones, err := s.store.ListEncounterZonesByEncounterID(ctx, combatant.EncounterID)
	if err != nil {
		return nil, fmt.Errorf("listing zones: %w", err)
	}
	col := colToIndex(combatant.PositionCol)
	row0 := int(combatant.PositionRow) - 1
	for _, z := range zones {
		if z.ZoneType != "silence" {
			continue
		}
		if !tileInSet(col, row0, zoneAffectedTiles(z)) {
			continue
		}
		cleanup, err := s.BreakConcentrationFully(ctx, BreakConcentrationFullyInput{
			EncounterID: combatant.EncounterID,
			CasterID:    combatant.ID,
			CasterName:  combatant.DisplayName,
			SpellID:     row.ConcentrationSpellID.String,
			SpellName:   row.ConcentrationSpellName.String,
			Reason:      "silence",
		})
		if err != nil {
			return nil, err
		}
		return &cleanup, nil
	}
	return nil, nil
}

// applyDamageHP is the centralized HP-update helper for damage paths. It
// updates the combatant's HP, then performs two Phase 118 hooks driven by
// `damage = max(0, prevHP - newHP)`:
//
//   - When damage > 0 AND the target is concentrating, enqueue a pending CON
//     save via MaybeCreateConcentrationSaveOnDamage. The pending-save
//     resolution path will later call ResolveConcentrationSave on failure.
//   - When the target transitions from `prevHP > 0` to `newHP <= 0` and is
//     still alive (dying, not corpse), apply the `unconscious` condition.
//     ApplyCondition's Phase 118 incapacitation hook will then auto-break
//     the target's own concentration if any.
//
// Healing (newHP > prevHP) is a no-op for both hooks. The helper is the
// single seam every damage call site funnels through, so callers do not
// need to repeat the prevHP/newHP math.
func (s *Service) applyDamageHP(ctx context.Context, encounterID, combatantID uuid.UUID, prevHP, newHP, tempHP int32, isAlive bool) (refdata.Combatant, error) {
	updated, err := s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID:        combatantID,
		HpCurrent: newHP,
		TempHp:    tempHP,
		IsAlive:   isAlive,
	})
	if err != nil {
		return refdata.Combatant{}, fmt.Errorf("updating HP: %w", err)
	}

	damage := int(prevHP) - int(newHP)
	if damage <= 0 {
		return updated, nil
	}

	if _, err := s.MaybeCreateConcentrationSaveOnDamage(ctx, encounterID, combatantID, damage); err != nil {
		return updated, fmt.Errorf("enqueuing concentration save: %w", err)
	}

	// Apply unconscious when alive but newly at 0 HP. Avoid the redundant
	// store call if the condition is already present (e.g. another damage
	// tick already flipped the target to unconscious in this turn).
	if !isAlive || prevHP <= 0 || newHP > 0 {
		return updated, nil
	}
	if HasCondition(updated.Conditions, "unconscious") {
		return updated, nil
	}
	if _, _, err := s.ApplyCondition(ctx, combatantID, CombatCondition{Condition: "unconscious"}); err != nil {
		return updated, fmt.Errorf("applying unconscious at 0 HP: %w", err)
	}
	return updated, nil
}

// MaybeCreateConcentrationSaveOnDamage checks whether the combatant is
// concentrating and, if so, enqueues a pending CON save with DC =
// max(10, damage/2) and source = "concentration". Returns the created
// pending-save row (or nil when no save is needed). Damage paths
// (`attack.go`, `aoe.go`, `dm_dashboard_handler.go`, ...) call this after
// applying damage so the existing pending-save resolution flow can later
// trigger BreakConcentrationFully on failure.
func (s *Service) MaybeCreateConcentrationSaveOnDamage(ctx context.Context, encounterID, combatantID uuid.UUID, damage int) (*refdata.PendingSafe, error) {
	if damage <= 0 {
		return nil, nil
	}
	row, err := s.store.GetCombatantConcentration(ctx, combatantID)
	if err != nil {
		return nil, fmt.Errorf("looking up concentration: %w", err)
	}
	if !row.ConcentrationSpellName.Valid || row.ConcentrationSpellName.String == "" {
		return nil, nil
	}
	check := CheckConcentrationOnDamage(row.ConcentrationSpellName.String, damage)
	if !check.NeedsSave {
		return nil, nil
	}
	created, err := s.store.CreatePendingSave(ctx, refdata.CreatePendingSaveParams{
		EncounterID: encounterID,
		CombatantID: combatantID,
		Ability:     "con",
		Dc:          int32(check.DC),
		Source:      ConcentrationSaveSource,
	})
	if err != nil {
		return nil, fmt.Errorf("creating concentration save: %w", err)
	}
	return &created, nil
}

// ResolveConcentrationSave inspects a freshly-resolved pending save row and,
// when it represents a failed concentration save (`source =
// "concentration"`, `success = false`), fires the full concentration break
// pipeline. Returns the cleanup result (or nil for non-concentration sources
// or successful saves). The pending-save resolver invokes this after
// persisting the save's outcome.
func (s *Service) ResolveConcentrationSave(ctx context.Context, ps refdata.PendingSafe) (*BreakConcentrationFullyResult, error) {
	if ps.Source != ConcentrationSaveSource {
		return nil, nil
	}
	if !ps.Success.Valid || ps.Success.Bool {
		return nil, nil
	}
	target, err := s.store.GetCombatant(ctx, ps.CombatantID)
	if err != nil {
		return nil, fmt.Errorf("getting combatant for concentration save resolution: %w", err)
	}
	row, err := s.store.GetCombatantConcentration(ctx, ps.CombatantID)
	if err != nil {
		return nil, fmt.Errorf("looking up concentration on save resolution: %w", err)
	}
	if !row.ConcentrationSpellID.Valid || !row.ConcentrationSpellName.Valid {
		// Caster is no longer concentrating (e.g. a parallel auto-break already
		// ran); the save outcome is moot.
		return nil, nil
	}
	cleanup, err := s.BreakConcentrationFully(ctx, BreakConcentrationFullyInput{
		EncounterID: ps.EncounterID,
		CasterID:    ps.CombatantID,
		CasterName:  target.DisplayName,
		SpellID:     row.ConcentrationSpellID.String,
		SpellName:   row.ConcentrationSpellName.String,
		Reason:      "failed CON save",
	})
	if err != nil {
		return nil, err
	}
	return &cleanup, nil
}

// BreakConcentrationFullyInput holds the parameters required to fully tear
// down a concentration spell's lingering effects.
type BreakConcentrationFullyInput struct {
	EncounterID uuid.UUID
	CasterID    uuid.UUID
	CasterName  string
	SpellID     string // e.g. "invisibility", "bless"; used for spell-sourced condition lookup
	SpellName   string // human-readable name for log lines
	Reason      string // e.g. "failed CON save", "stunned", "silence", "voluntary drop"
}

// BreakConcentrationFullyResult is the outcome of a concentration break,
// including counts of side-effect cleanups and combat log lines.
type BreakConcentrationFullyResult struct {
	Broken              bool
	SpellName           string
	Reason              string
	ConditionsRemoved   int    // total spell-sourced conditions stripped across all combatants
	SummonsDismissed    int    // count of summoned creatures removed
	ZonesRemoved        bool   // whether the zone-cleanup query was issued (count not reported by the store)
	PerSourceMessage    string // legacy 🔮 helper output (FormatConcentrationBreakLog)
	ConsolidatedMessage string // new 💨 line: "<Caster> lost concentration on <Spell> — effects ended on N targets."
}

// BreakConcentrationFully orchestrates the complete concentration break
// pipeline used by every Phase 118 trigger point (damage CON-save failure,
// incapacitation, Silence entry, replacing a concentration spell, voluntary
// drop). It strips spell-sourced conditions across the encounter, removes
// concentration-tagged zones, dismisses summons, clears the caster's
// concentration columns, and returns both legacy and consolidated combat log
// lines so the caller can choose what to surface.
//
// The "N targets" in the consolidated line is defined as
// (conditions removed) + (summons dismissed). Zone removal is signalled via
// `ZonesRemoved` because the underlying SQL is :exec and does not return a
// row count; counting them precisely would require a separate query.
func (s *Service) BreakConcentrationFully(ctx context.Context, in BreakConcentrationFullyInput) (BreakConcentrationFullyResult, error) {
	result := BreakConcentrationFullyResult{
		Broken:    true,
		SpellName: in.SpellName,
		Reason:    in.Reason,
	}

	// 1. Strip spell-sourced conditions from every combatant in the encounter.
	if in.SpellID != "" {
		n, err := s.RemoveSpellSourcedConditions(ctx, in.EncounterID, in.CasterID, in.SpellID)
		if err != nil {
			return result, fmt.Errorf("removing spell-sourced conditions: %w", err)
		}
		result.ConditionsRemoved = n
	}

	// 2. Delete concentration-tagged zones (Silence, Web, Hunger of Hadar, ...).
	if err := s.store.DeleteConcentrationZonesByCombatant(ctx, in.CasterID); err != nil {
		return result, fmt.Errorf("deleting concentration zones: %w", err)
	}
	result.ZonesRemoved = true

	// 3. Dismiss any concentration-linked summons.
	dismissed, err := s.DismissSummonsByConcentration(ctx, in.EncounterID, in.CasterID)
	if err != nil {
		return result, fmt.Errorf("dismissing summons: %w", err)
	}
	result.SummonsDismissed = dismissed

	// 4. Clear the caster's concentration columns. Best-effort: the rows may
	// already have been NULL (e.g. tests not exercising the columns); errors
	// must still surface to the caller.
	if err := s.store.ClearCombatantConcentration(ctx, in.CasterID); err != nil {
		return result, fmt.Errorf("clearing concentration: %w", err)
	}

	// 5. Compose the single consolidated cleanup log line. Per the iter-2
	// user clarification, the cleanup path emits ONLY the 💨 line with the
	// trigger reason in parentheses; the legacy 🔮 helper output is no
	// longer surfaced here. `FormatConcentrationBreakLog` remains available
	// for non-cleanup callers (e.g. a future "save succeeded — concentration
	// held" message).
	result.ConsolidatedMessage = FormatConcentrationCleanupLog(in.CasterName, in.SpellName, in.Reason, result.ConditionsRemoved+result.SummonsDismissed)

	return result, nil
}

// FormatConcentrationCleanupLog produces the consolidated 💨 cleanup line:
// "💨 <Caster> lost concentration on <Spell> (<reason>) — effects ended on N
// targets.". This is the ONLY line emitted from the cleanup path.
func FormatConcentrationCleanupLog(casterName, spellName, reason string, n int) string {
	return fmt.Sprintf("💨 %s lost concentration on %s (%s) — effects ended on %d targets.", casterName, spellName, reason, n)
}

// FormatConcentrationBreakLog produces the combat log message for concentration loss.
// Uses "drops" for voluntary changes (new spell), "loses" for forced breaks.
func FormatConcentrationBreakLog(casterName, spellName, reason string) string {
	verb := "loses"
	if strings.HasPrefix(reason, "cast new concentration spell") {
		verb = "drops"
	}
	return fmt.Sprintf("🔮 %s %s concentration on %s (%s)", casterName, verb, spellName, reason)
}
