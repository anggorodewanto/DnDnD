package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// CreatureScoresLookup is the minimal slice of the combat Store needed to
// resolve a beast's ability scores by ID. The /check and /save handlers
// (which live in package discord) inject an implementation so they can
// pass beast scores to EffectiveAbilityScores while the player is Wild
// Shaped. Returning the parsed character.AbilityScores keeps the handler
// side free of the combat.AbilityScores ↔ character.AbilityScores
// conversion noise. (SR-022)
type CreatureScoresLookup interface {
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
}

// LookupBeastScores resolves the beast's parsed character.AbilityScores for
// a Wild Shaped combatant, returning ok=false (and no error) when the
// combatant is not wild-shaped, has no beast ref, the lookup fails, or
// the beast's ability_scores column can't be parsed. The silent
// degradation matches the SR-006 pattern: a lookup gap must never block a
// player's roll — better to fall back to druid scores than fail the
// /check or /save. (SR-022)
func LookupBeastScores(ctx context.Context, lookup CreatureScoresLookup, c refdata.Combatant) (character.AbilityScores, bool) {
	if lookup == nil {
		return character.AbilityScores{}, false
	}
	if !c.IsWildShaped {
		return character.AbilityScores{}, false
	}
	if !c.WildShapeCreatureRef.Valid || c.WildShapeCreatureRef.String == "" {
		return character.AbilityScores{}, false
	}
	beast, err := lookup.GetCreature(ctx, c.WildShapeCreatureRef.String)
	if err != nil {
		return character.AbilityScores{}, false
	}
	var scores character.AbilityScores
	if err := json.Unmarshal(beast.AbilityScores, &scores); err != nil {
		return character.AbilityScores{}, false
	}
	return scores, true
}

// WildShapeCRLimit returns the maximum CR beast a druid can Wild Shape into.
// Standard druids: CR 1/4 at level 2, CR 1/2 at level 4, CR 1 at level 8.
// Circle of the Moon: CR 1 at level 2, CR = level/3 (rounded down) at level 6+.
func WildShapeCRLimit(druidLevel int, isCircleOfMoon bool) float64 {
	if isCircleOfMoon {
		if druidLevel >= 6 {
			return float64(druidLevel / 3)
		}
		return 1
	}
	if druidLevel >= 8 {
		return 1
	}
	if druidLevel >= 4 {
		return 0.5
	}
	return 0.25
}

// WildShapeSnapshot stores the original combatant stats before Wild Shape transformation.
type WildShapeSnapshot struct {
	HpMax         int32          `json:"hp_max"`
	HpCurrent     int32          `json:"hp_current"`
	Ac            int32          `json:"ac"`
	SpeedFt       int32          `json:"speed_ft"`
	AbilityScores map[string]int `json:"ability_scores"`
}

// SnapshotCombatantState creates a JSON snapshot of the combatant's current stats
// so they can be restored when Wild Shape ends.
func SnapshotCombatantState(c refdata.Combatant, speedFt int32, abilityScores json.RawMessage) (json.RawMessage, error) {
	var scores map[string]int
	if err := json.Unmarshal(abilityScores, &scores); err != nil {
		return nil, fmt.Errorf("parsing ability scores: %w", err)
	}
	snap := WildShapeSnapshot{
		HpMax:         c.HpMax,
		HpCurrent:     c.HpCurrent,
		Ac:            c.Ac,
		SpeedFt:       speedFt,
		AbilityScores: scores,
	}
	return json.Marshal(snap)
}

// EffectiveAbilityScores returns the ability scores in effect for a
// combatant right now. When the combatant is Wild Shaped, the beast's
// physical scores (STR/DEX/CON) replace the druid's while INT/WIS/CHA are
// retained from the druid (PHB Druid: "your game statistics are replaced
// by the statistics of the beast, but you retain your alignment,
// personality, and Intelligence, Wisdom, and Charisma scores"). When the
// combatant is not Wild Shaped, the druid's scores pass through unchanged.
//
// The druid's pre-transform scores are persisted in the existing
// WildShapeOriginal snapshot, so revert is implicit: once
// RevertWildShape clears the snapshot and flips IsWildShaped to false,
// this helper falls back to druidScores automatically — no extra restore
// step required at the call site. (SR-022)
func EffectiveAbilityScores(c refdata.Combatant, druidScores, beastScores character.AbilityScores) character.AbilityScores {
	if !c.IsWildShaped {
		return druidScores
	}
	// Merge: beast physicals + druid mentals.
	return character.AbilityScores{
		STR: beastScores.STR,
		DEX: beastScores.DEX,
		CON: beastScores.CON,
		INT: druidScores.INT,
		WIS: druidScores.WIS,
		CHA: druidScores.CHA,
	}
}

// characterScoresFromCombat converts a combat.AbilityScores (lowercase
// field names, matches the JSONB tag layout) into a character.AbilityScores
// (uppercase field names, the canonical type used by check/save/character
// packages). The two structs predate SR-022 and are kept independent.
func characterScoresFromCombat(s AbilityScores) character.AbilityScores {
	return character.AbilityScores{
		STR: s.Str, DEX: s.Dex, CON: s.Con,
		INT: s.Int, WIS: s.Wis, CHA: s.Cha,
	}
}

// combatScoresFromCharacter is the inverse of characterScoresFromCombat.
// Used by Service.Attack / OffhandAttack to feed merged Wild Shape scores
// back into the combat.AbilityScores-typed buildAttackInput pipeline.
func combatScoresFromCharacter(s character.AbilityScores) AbilityScores {
	return AbilityScores{
		Str: s.STR, Dex: s.DEX, Con: s.CON,
		Int: s.INT, Wis: s.WIS, Cha: s.CHA,
	}
}

// ResolveAttackerScores returns the combat.AbilityScores in effect for the
// given attacker right now. When the attacker is Wild Shaped and the
// beast can be resolved via lookup, the beast's STR/DEX/CON replace the
// druid's while INT/WIS/CHA stay with the druid. On any lookup gap
// (no lookup wired, beast not found, malformed beast row, attacker not
// wild-shaped) the druid's scores pass through unchanged — the SR-006
// silent-degradation pattern. (SR-022)
func ResolveAttackerScores(ctx context.Context, lookup CreatureScoresLookup, attacker refdata.Combatant, druidScores AbilityScores) AbilityScores {
	beastScores, ok := LookupBeastScores(ctx, lookup, attacker)
	if !ok {
		return druidScores
	}
	merged := EffectiveAbilityScores(attacker, characterScoresFromCombat(druidScores), beastScores)
	return combatScoresFromCharacter(merged)
}

// ApplyBeastFormToCombatant overwrites the combatant's stats with the beast's stats.
// Sets IsWildShaped to true and records the beast reference.
func ApplyBeastFormToCombatant(c refdata.Combatant, beast refdata.Creature) refdata.Combatant {
	c.HpMax = beast.HpAverage
	c.HpCurrent = beast.HpAverage
	c.Ac = beast.Ac
	c.IsWildShaped = true
	c.WildShapeCreatureRef = sql.NullString{String: beast.ID, Valid: true}
	return c
}

// RevertWildShape restores the combatant from the wild shape snapshot.
// overflowDamage is the damage that carries over from beast form (beast HP went below 0).
// Returns the reverted combatant, the overflow damage applied, and any error.
func RevertWildShape(c refdata.Combatant, overflowDamage int32) (refdata.Combatant, int32, error) {
	if !c.IsWildShaped {
		return c, 0, fmt.Errorf("not in Wild Shape")
	}
	if !c.WildShapeOriginal.Valid {
		return c, 0, fmt.Errorf("no Wild Shape snapshot found")
	}

	var snap WildShapeSnapshot
	if err := json.Unmarshal(c.WildShapeOriginal.RawMessage, &snap); err != nil {
		return c, 0, fmt.Errorf("parsing wild shape snapshot: %w", err)
	}

	c.HpMax = snap.HpMax
	c.HpCurrent = snap.HpCurrent - overflowDamage
	if c.HpCurrent < 0 {
		c.HpCurrent = 0
	}
	c.Ac = snap.Ac
	c.IsWildShaped = false
	c.WildShapeCreatureRef = sql.NullString{}
	c.WildShapeOriginal = pqtype.NullRawMessage{}

	return c, overflowDamage, nil
}

// ValidateWildShapeActivation checks all preconditions for Wild Shape activation.
func ValidateWildShapeActivation(isWildShaped bool, beastType, beastCR string, druidLevel int, isCircleOfMoon bool, beastSpeed json.RawMessage) error {
	if isWildShaped {
		return fmt.Errorf("already in Wild Shape")
	}
	if beastType != "beast" {
		return fmt.Errorf("creature type %q is not a beast", beastType)
	}
	crVal := ParseCR(beastCR)
	crLimit := WildShapeCRLimit(druidLevel, isCircleOfMoon)
	if crVal > crLimit {
		return fmt.Errorf("CR %s exceeds limit of %v for Druid level %d", beastCR, crLimit, druidLevel)
	}
	if CreatureHasSwimSpeed(beastSpeed) && druidLevel < 4 {
		return fmt.Errorf("beast with swim speed requires Druid level 4+, have %d", druidLevel)
	}
	if CreatureHasFlySpeed(beastSpeed) && druidLevel < 8 {
		return fmt.Errorf("beast with fly speed requires Druid level 8+, have %d", druidLevel)
	}
	return nil
}

// CreatureHasSwimSpeed returns true if the creature's speed JSON contains a swim speed > 0.
func CreatureHasSwimSpeed(speed json.RawMessage) bool {
	return creatureHasSpeed(speed, "swim")
}

// CreatureHasFlySpeed returns true if the creature's speed JSON contains a fly speed > 0.
func CreatureHasFlySpeed(speed json.RawMessage) bool {
	return creatureHasSpeed(speed, "fly")
}

func creatureHasSpeed(speed json.RawMessage, key string) bool {
	if len(speed) == 0 {
		return false
	}
	var speeds map[string]int
	if err := json.Unmarshal(speed, &speeds); err != nil {
		return false
	}
	return speeds[key] > 0
}

// CanWildShapeSpellcast returns true if the druid level is 18+ (Beast Spells feature).
func CanWildShapeSpellcast(druidLevel int) bool {
	return druidLevel >= 18
}

// FormatWildShapeActivation returns the combat log for Wild Shape transformation.
func FormatWildShapeActivation(name, beastName string, usesRemaining int, hp, ac, speed int32, attacksDesc string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\U0001f43a  %s Wild Shapes into a %s! (%d use remaining)\n", name, beastName, usesRemaining)
	fmt.Fprintf(&b, "     \u2764\ufe0f  HP: %d | \U0001f6e1\ufe0f AC: %d | \U0001f3c3 %dft", hp, ac, speed)
	if attacksDesc != "" {
		fmt.Fprintf(&b, "\n     \u2694\ufe0f  Attacks: %s", attacksDesc)
	}
	return b.String()
}

// FormatWildShapeRevert returns the combat log for voluntary Wild Shape revert.
func FormatWildShapeRevert(name string) string {
	return fmt.Sprintf("\U0001f43a  %s reverts from Wild Shape", name)
}

// FormatWildShapeAutoRevert returns the combat log for auto-revert when beast HP reaches 0.
func FormatWildShapeAutoRevert(name, beastName string, overflowDmg, hpCurrent, hpMax int32) string {
	return fmt.Sprintf("\U0001f43a  %s's %s form drops to 0 HP! Reverts to Druid form (%d overflow damage \u2192 %d/%d HP)",
		name, beastName, overflowDmg, hpCurrent, hpMax)
}

// AutoRevertWildShape handles auto-revert when beast form HP reaches 0.
// overflowDamage is the excess damage beyond 0 HP in beast form.
func AutoRevertWildShape(c refdata.Combatant, overflowDamage int32) (refdata.Combatant, int32, error) {
	return RevertWildShape(c, overflowDamage)
}

// ParseCR converts a CR string like "1/4", "1/2", "1", "0" into a float64.
func ParseCR(cr string) float64 {
	if strings.Contains(cr, "/") {
		parts := strings.SplitN(cr, "/", 2)
		num, _ := strconv.ParseFloat(parts[0], 64)
		den, _ := strconv.ParseFloat(parts[1], 64)
		if den == 0 {
			return 0
		}
		return num / den
	}
	val, _ := strconv.ParseFloat(cr, 64)
	return val
}

// HasDruidClass checks whether a character's classes JSON includes a Druid entry.
func HasDruidClass(classesJSON json.RawMessage) bool {
	return ClassLevelFromJSON(classesJSON, "Druid") > 0
}

// isCircleOfMoon checks if the character has the Circle of the Moon subclass.
func isCircleOfMoon(features pqtype.NullRawMessage) bool {
	return hasFeatureEffect(features, "circle_of_the_moon")
}


// getBeastWalkSpeed extracts the walk speed from a beast's speed JSON.
func getBeastWalkSpeed(speed json.RawMessage) int32 {
	var speeds map[string]int32
	if err := json.Unmarshal(speed, &speeds); err != nil {
		return 0
	}
	return speeds["walk"]
}

// WildShapeCommand holds the service-level inputs for activating Wild Shape.
type WildShapeCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	BeastName string
}

// WildShapeResult holds the result of activating Wild Shape.
type WildShapeResult struct {
	Combatant     refdata.Combatant
	Turn          refdata.Turn
	CombatLog     string
	Remaining     string
	UsesRemaining int
}

// ActivateWildShape handles the /bonus wild-shape command.
func (s *Service) ActivateWildShape(ctx context.Context, cmd WildShapeCommand) (WildShapeResult, error) {
	// Validate bonus action
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return WildShapeResult{}, err
	}

	// Must be a PC
	if !cmd.Combatant.CharacterID.Valid {
		return WildShapeResult{}, fmt.Errorf("Wild Shape requires a character (not NPC)")
	}

	// Get character
	char, err := s.store.GetCharacter(ctx, cmd.Combatant.CharacterID.UUID)
	if err != nil {
		return WildShapeResult{}, fmt.Errorf("getting character: %w", err)
	}

	// Must be druid
	if !HasDruidClass(char.Classes) {
		return WildShapeResult{}, fmt.Errorf("Wild Shape requires Druid class")
	}

	// Check Wild Shape uses
	featureUses, wsRemaining, err := ParseFeatureUses(char, FeatureKeyWildShape)
	if err != nil {
		return WildShapeResult{}, err
	}
	if wsRemaining <= 0 {
		return WildShapeResult{}, fmt.Errorf("no Wild Shape uses remaining (0/2)")
	}

	// Get the beast creature
	beast, err := s.store.GetCreature(ctx, cmd.BeastName)
	if err != nil {
		return WildShapeResult{}, fmt.Errorf("getting beast %q: %w", cmd.BeastName, err)
	}

	// Validate Wild Shape preconditions
	dLevel := ClassLevelFromJSON(char.Classes, "Druid")
	moon := isCircleOfMoon(char.Features)
	if err := ValidateWildShapeActivation(cmd.Combatant.IsWildShaped, beast.Type, beast.Cr, dLevel, moon, beast.Speed); err != nil {
		return WildShapeResult{}, err
	}

	// Deduct Wild Shape use
	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyWildShape, featureUses, wsRemaining)
	if err != nil {
		return WildShapeResult{}, err
	}

	// Use bonus action
	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return WildShapeResult{}, fmt.Errorf("using bonus action: %w", err)
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return WildShapeResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Create snapshot of original state
	snapshot, err := SnapshotCombatantState(cmd.Combatant, char.SpeedFt, char.AbilityScores)
	if err != nil {
		return WildShapeResult{}, fmt.Errorf("creating snapshot: %w", err)
	}

	// Apply beast form to combatant
	transformed := ApplyBeastFormToCombatant(cmd.Combatant, beast)
	transformed.WildShapeOriginal = pqtype.NullRawMessage{RawMessage: snapshot, Valid: true}

	// Persist wild shape state
	persisted, err := s.store.UpdateCombatantWildShape(ctx, refdata.UpdateCombatantWildShapeParams{
		ID:                   transformed.ID,
		IsWildShaped:         transformed.IsWildShaped,
		WildShapeCreatureRef: transformed.WildShapeCreatureRef,
		WildShapeOriginal:    transformed.WildShapeOriginal,
		HpMax:                transformed.HpMax,
		HpCurrent:            transformed.HpCurrent,
		Ac:                   transformed.Ac,
	})
	if err != nil {
		return WildShapeResult{}, fmt.Errorf("updating combatant wild shape: %w", err)
	}

	walkSpeed := getBeastWalkSpeed(beast.Speed)
	combatLog := FormatWildShapeActivation(cmd.Combatant.DisplayName, beast.Name, newUsesRemaining,
		beast.HpAverage, beast.Ac, walkSpeed, "")
	remaining := FormatRemainingResources(updatedTurn, nil)

	return WildShapeResult{
		Combatant:     persisted,
		Turn:          updatedTurn,
		CombatLog:     combatLog,
		Remaining:     remaining,
		UsesRemaining: newUsesRemaining,
	}, nil
}

// RevertWildShapeCommand holds the service-level inputs for reverting Wild Shape.
type RevertWildShapeCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
}

// RevertWildShapeResult holds the result of reverting Wild Shape.
type RevertWildShapeResult struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	CombatLog string
	Overflow  int32
}

// RevertWildShapeService handles the /bonus revert command.
func (s *Service) RevertWildShapeService(ctx context.Context, cmd RevertWildShapeCommand) (RevertWildShapeResult, error) {
	if !cmd.Combatant.IsWildShaped {
		return RevertWildShapeResult{}, fmt.Errorf("not in Wild Shape")
	}

	// Voluntary revert costs bonus action
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return RevertWildShapeResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return RevertWildShapeResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	// Restore movement to druid's original speed from snapshot.
	if cmd.Combatant.WildShapeOriginal.Valid {
		var snap WildShapeSnapshot
		if err := json.Unmarshal(cmd.Combatant.WildShapeOriginal.RawMessage, &snap); err == nil && snap.SpeedFt > 0 {
			updatedTurn.MovementRemainingFt = snap.SpeedFt
		}
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return RevertWildShapeResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Revert with no overflow (voluntary)
	reverted, _, err := RevertWildShape(cmd.Combatant, 0)
	if err != nil {
		return RevertWildShapeResult{}, err
	}

	// Persist
	persisted, err := s.store.UpdateCombatantWildShape(ctx, refdata.UpdateCombatantWildShapeParams{
		ID:                   reverted.ID,
		IsWildShaped:         reverted.IsWildShaped,
		WildShapeCreatureRef: reverted.WildShapeCreatureRef,
		WildShapeOriginal:    reverted.WildShapeOriginal,
		HpMax:                reverted.HpMax,
		HpCurrent:            reverted.HpCurrent,
		Ac:                   reverted.Ac,
	})
	if err != nil {
		return RevertWildShapeResult{}, fmt.Errorf("updating combatant wild shape: %w", err)
	}

	combatLog := FormatWildShapeRevert(cmd.Combatant.DisplayName)

	return RevertWildShapeResult{
		Combatant: persisted,
		Turn:      updatedTurn,
		CombatLog: combatLog,
	}, nil
}
