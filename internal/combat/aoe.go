package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// AreaOfEffect holds parsed AoE data from a spell's area_of_effect JSONB field.
type AreaOfEffect struct {
	Shape    string `json:"shape"`
	RadiusFt int    `json:"radius_ft,omitempty"`
	LengthFt int    `json:"length_ft,omitempty"`
	SideFt   int    `json:"side_ft,omitempty"`
	WidthFt  int    `json:"width_ft,omitempty"`
}

// ParseAreaOfEffect parses the area_of_effect JSONB into an AreaOfEffect struct.
func ParseAreaOfEffect(raw []byte) (AreaOfEffect, error) {
	if len(raw) == 0 {
		return AreaOfEffect{}, errors.New("area_of_effect data is empty")
	}
	var aoe AreaOfEffect
	if err := json.Unmarshal(raw, &aoe); err != nil {
		return AreaOfEffect{}, err
	}
	return aoe, nil
}

// GridPos represents a 0-based grid position (col, row).
type GridPos struct {
	Col int
	Row int
}

// SphereAffectedTiles returns all grid tiles whose center is within radiusFt
// of the origin tile's center. Each grid square = 5ft.
func SphereAffectedTiles(originCol, originRow, radiusFt int) []GridPos {
	radiusSquares := radiusFt / 5
	radiusFtF := float64(radiusFt)
	var tiles []GridPos

	for dc := -radiusSquares; dc <= radiusSquares; dc++ {
		for dr := -radiusSquares; dr <= radiusSquares; dr++ {
			distFt := math.Sqrt(float64(dc*dc+dr*dr)) * 5.0
			if distFt <= radiusFtF {
				tiles = append(tiles, GridPos{originCol + dc, originRow + dr})
			}
		}
	}
	return tiles
}

// directedAffectedTiles returns grid tiles along a direction from the caster,
// filtered by a halfWidthFn that returns the allowed perpendicular half-width
// at a given projection distance (in feet). The caster's own tile is excluded.
func directedAffectedTiles(casterCol, casterRow, targetCol, targetRow, lengthFt int, halfWidthFn func(projFt float64) float64) []GridPos {
	if lengthFt <= 0 {
		return nil
	}

	dx := float64(targetCol - casterCol)
	dy := float64(targetRow - casterRow)
	dirLen := math.Sqrt(dx*dx + dy*dy)
	if dirLen == 0 {
		return nil
	}
	dx /= dirLen
	dy /= dirLen

	lengthSquares := lengthFt / 5
	lengthFtF := float64(lengthFt)
	var tiles []GridPos

	for dc := -lengthSquares; dc <= lengthSquares; dc++ {
		for dr := -lengthSquares; dr <= lengthSquares; dr++ {
			if dc == 0 && dr == 0 {
				continue
			}
			fc := float64(dc)
			fr := float64(dr)

			projFt := (fc*dx + fr*dy) * 5.0
			if projFt <= 0 || projFt > lengthFtF {
				continue
			}

			perpFt := math.Abs(fc*(-dy)+fr*dx) * 5.0
			if perpFt <= halfWidthFn(projFt) {
				tiles = append(tiles, GridPos{casterCol + dc, casterRow + dr})
			}
		}
	}
	return tiles
}

// ConeAffectedTiles returns all grid tiles within a 53-degree cone.
// The cone emanates from the caster's tile edge toward the target direction.
// In 5e, at distance d along the cone axis, the cone's width equals d.
// The caster's own tile is excluded.
func ConeAffectedTiles(casterCol, casterRow, targetCol, targetRow, lengthFt int) []GridPos {
	return directedAffectedTiles(casterCol, casterRow, targetCol, targetRow, lengthFt, func(projFt float64) float64 {
		return projFt / 2.0
	})
}

// LineAffectedTiles returns all grid tiles within a rectangular line from the caster
// toward the target with the given length and width (in feet).
// The caster's own tile is excluded.
func LineAffectedTiles(casterCol, casterRow, targetCol, targetRow, lengthFt, widthFt int) []GridPos {
	halfWidthFt := float64(widthFt) / 2.0
	return directedAffectedTiles(casterCol, casterRow, targetCol, targetRow, lengthFt, func(_ float64) float64 {
		return halfWidthFt
	})
}

// SquareAffectedTiles returns all grid tiles within a square area.
// The origin is the corner of the square. sideFt is the side length in feet.
func SquareAffectedTiles(originCol, originRow, sideFt int) []GridPos {
	sideSquares := sideFt / 5
	if sideSquares <= 0 {
		return nil
	}
	tiles := make([]GridPos, 0, sideSquares*sideSquares)
	for dc := 0; dc < sideSquares; dc++ {
		for dr := 0; dr < sideSquares; dr++ {
			tiles = append(tiles, GridPos{originCol + dc, originRow + dr})
		}
	}
	return tiles
}

// FindAffectedCombatants returns combatants whose grid position falls within the affected tiles.
// Combatant positions use PositionCol (letter, 0-based via colToIndex) and PositionRow (1-based).
func FindAffectedCombatants(affectedTiles []GridPos, combatants []refdata.Combatant) []refdata.Combatant {
	tileSet := make(map[GridPos]bool, len(affectedTiles))
	for _, t := range affectedTiles {
		tileSet[t] = true
	}

	var result []refdata.Combatant
	for _, c := range combatants {
		if !c.IsAlive {
			continue
		}
		pos := GridPos{Col: colToIndex(c.PositionCol), Row: int(c.PositionRow) - 1}
		if tileSet[pos] {
			result = append(result, c)
		}
	}
	return result
}

// PendingSave represents a save that needs to be resolved for an AoE spell.
type PendingSave struct {
	CombatantID uuid.UUID
	SaveAbility string
	DC          int
	CoverBonus  int
	IsNPC       bool
}

// SaveResult holds the outcome of a saving throw.
type SaveResult struct {
	CombatantID uuid.UUID
	Rolled      int
	Total       int
	Success     bool
	CoverBonus  int
}

// CalculateAoECover computes the PendingSave for a combatant affected by an AoE spell.
// If the save ability is "dex", the cover bonus from the spell origin is applied.
func CalculateAoECover(originCol, originRow int, combatant refdata.Combatant, saveAbility string, dc int, walls []renderer.WallSegment) PendingSave {
	coverBonus := 0
	if saveAbility == "dex" {
		targetCol := colToIndex(combatant.PositionCol)
		targetRow := int(combatant.PositionRow) - 1
		cover := CalculateCoverFromOrigin(originCol, originRow, targetCol, targetRow, walls)
		coverBonus = cover.DEXSaveBonus()
	}
	return PendingSave{
		CombatantID: combatant.ID,
		SaveAbility: saveAbility,
		DC:          dc,
		CoverBonus:  coverBonus,
		IsNPC:       combatant.IsNpc,
	}
}

// ApplySaveResult returns the damage multiplier based on the save outcome and spell's save effect.
// Returns 0.5 for half damage on save, 0.0 for no damage on save, 1.0 for full damage on failure,
// and -1.0 for "special" (DM resolution needed).
func ApplySaveResult(saveSuccess bool, saveEffect string) float64 {
	switch saveEffect {
	case "half_damage":
		if saveSuccess {
			return 0.5
		}
		return 1.0
	case "no_effect":
		if saveSuccess {
			return 0.0
		}
		return 1.0
	case "special":
		return -1.0
	default:
		return 1.0
	}
}

// GetAffectedTiles dispatches to the correct shape function based on the AoE shape.
// For sphere/square, the origin is the target point.
// For cone/line, the caster position and target direction are used.
func GetAffectedTiles(aoe AreaOfEffect, casterCol, casterRow, targetCol, targetRow int) ([]GridPos, error) {
	switch aoe.Shape {
	case "sphere":
		return SphereAffectedTiles(targetCol, targetRow, aoe.RadiusFt), nil
	case "cone":
		return ConeAffectedTiles(casterCol, casterRow, targetCol, targetRow, aoe.LengthFt), nil
	case "line":
		return LineAffectedTiles(casterCol, casterRow, targetCol, targetRow, aoe.LengthFt, aoe.WidthFt), nil
	case "square":
		return SquareAffectedTiles(targetCol, targetRow, aoe.SideFt), nil
	default:
		return nil, fmt.Errorf("unsupported AoE shape: %s", aoe.Shape)
	}
}

// AoECastResult holds the outcome of an AoE spell cast.
type AoECastResult struct {
	CasterName     string
	SpellName      string
	SpellLevel     int
	IsBonusAction  bool
	SaveDC         int
	SaveAbility    string
	AffectedNames  []string
	PendingSaves   []PendingSave
	Concentration  ConcentrationResult
	SlotUsed       int
	SlotsRemaining int
	OriginCol      int
	OriginRow      int
	// Phase 118: when this cast replaced an existing concentration spell, the
	// dropped spell's effects were cleaned up across the encounter. The
	// result is surfaced here so the handler can post the consolidated 💨 line.
	ConcentrationCleanup BreakConcentrationFullyResult
}

// FormatAoECastLog produces the combat log output for an AoE spell cast.
func FormatAoECastLog(result AoECastResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\u2728 %s casts %s", result.CasterName, result.SpellName)
	if result.IsBonusAction {
		b.WriteString(" (bonus action)")
	}
	b.WriteString("\n")

	// Slot usage
	if result.SpellLevel > 0 {
		fmt.Fprintf(&b, "\U0001f4a0 Used %s-level slot (%d remaining)\n", ordinal(result.SlotUsed), result.SlotsRemaining)
	}

	// Save DC
	if result.SaveDC > 0 {
		fmt.Fprintf(&b, "\U0001f6e1\ufe0f DC %d %s save\n", result.SaveDC, strings.ToUpper(result.SaveAbility))
	}

	// Affected creatures
	if len(result.AffectedNames) == 0 {
		b.WriteString("\U0001f4ad No creatures affected\n")
	} else {
		fmt.Fprintf(&b, "\U0001f4a5 Affected: %s\n", strings.Join(result.AffectedNames, ", "))
	}

	// Concentration
	if result.Concentration.DroppedPrevious {
		fmt.Fprintf(&b, "\u26a0\ufe0f Dropped concentration on %s\n", result.Concentration.PreviousSpell)
	}
	if result.Concentration.NewConcentration == result.SpellName && result.Concentration.NewConcentration != "" {
		fmt.Fprintf(&b, "\U0001f9e0 Concentrating on %s\n", result.Concentration.NewConcentration)
	}

	return strings.TrimRight(b.String(), "\n")
}

// AoECastCommand holds the inputs for an AoE /cast command.
type AoECastCommand struct {
	SpellID              string
	CasterID             uuid.UUID
	EncounterID          uuid.UUID
	TargetCol            string // target grid coordinate column letter
	TargetRow            int32  // target grid coordinate row (1-based)
	Turn                 refdata.Turn
	CurrentConcentration string
	Walls                []renderer.WallSegment
	SlotLevel            int // explicit slot choice; 0 = auto-select lowest available
}

// CastAoE orchestrates the AoE spell casting flow:
// lookup spell, validate resources, parse AoE, calculate affected tiles,
// find affected combatants, compute cover/saves, deduct slot, persist turn.
func (s *Service) CastAoE(ctx context.Context, cmd AoECastCommand) (AoECastResult, error) {
	// 1. Look up the spell
	spell, err := s.store.GetSpell(ctx, cmd.SpellID)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("looking up spell %q: %w", cmd.SpellID, err)
	}

	isBonusAction := IsBonusActionSpell(spell)

	// 2. Validate action/bonus action resource
	resource := ResourceAction
	if isBonusAction {
		resource = ResourceBonusAction
	}
	if err := ValidateResource(cmd.Turn, resource); err != nil {
		return AoECastResult{}, err
	}

	// 3. Validate bonus action spell restriction
	if err := ValidateBonusActionSpellRestriction(cmd.Turn, spell, isBonusAction); err != nil {
		return AoECastResult{}, err
	}

	// 4. Look up caster combatant
	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("getting caster: %w", err)
	}

	// 4a. med-25 / Phase 61: pre-validate Silence zones for AoE casts too.
	// Same rationale as Cast — slot must not be deducted if the cast is
	// silenced.
	inSilence, err := s.combatantInSilenceZone(ctx, caster)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("checking silence zone: %w", err)
	}
	if err := ValidateSilenceZone(inSilence, spell); err != nil {
		return AoECastResult{}, err
	}

	// 5. Look up character for spell slots and ability scores
	if !caster.CharacterID.Valid {
		return AoECastResult{}, errors.New("only player characters can cast spells via /cast")
	}

	char, err := s.store.GetCharacter(ctx, caster.CharacterID.UUID)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("getting character: %w", err)
	}

	// 6. Parse spell slots and select slot level
	spellLevel := int(spell.Level)
	slots, err := parseIntKeyedSlots(char.SpellSlots.RawMessage)
	if err != nil {
		return AoECastResult{}, err
	}
	effectiveSlotLevel := 0
	if spellLevel > 0 {
		effectiveSlotLevel, err = SelectSpellSlot(slots, spellLevel, cmd.SlotLevel)
		if err != nil {
			return AoECastResult{}, err
		}
	}

	// 7. Validate range to target coordinate
	targetColIdx := colToIndex(cmd.TargetCol)
	targetRowIdx := int(cmd.TargetRow) - 1
	casterColIdx := colToIndex(caster.PositionCol)
	casterRowIdx := int(caster.PositionRow) - 1
	distFt := Distance3D(casterColIdx, casterRowIdx, int(caster.AltitudeFt), targetColIdx, targetRowIdx, 0)
	if err := ValidateSpellRange(spell, distFt); err != nil {
		return AoECastResult{}, err
	}

	// 8. Parse AoE data
	if !spell.AreaOfEffect.Valid {
		return AoECastResult{}, errors.New("spell does not have area_of_effect data")
	}
	aoe, err := ParseAreaOfEffect(spell.AreaOfEffect.RawMessage)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("parsing area_of_effect: %w", err)
	}

	// 9. Calculate affected tiles
	tiles, err := GetAffectedTiles(aoe, casterColIdx, casterRowIdx, targetColIdx, targetRowIdx)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("calculating affected tiles: %w", err)
	}

	// 10. Find affected combatants
	allCombatants, err := s.store.ListCombatantsByEncounterID(ctx, caster.EncounterID)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("listing combatants: %w", err)
	}
	affected := FindAffectedCombatants(tiles, allCombatants)

	// 11. Determine spellcasting ability and save DC
	var classes []CharacterClass
	if err := json.Unmarshal(char.Classes, &classes); err != nil {
		return AoECastResult{}, fmt.Errorf("parsing classes: %w", err)
	}
	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}
	spellAbilityScore := resolveSpellcastingAbilityScore(classes, scores)

	saveDC := 0
	saveAbility := ""
	if spell.SaveAbility.Valid && spell.SaveAbility.String != "" {
		saveDC = SpellSaveDC(int(char.ProficiencyBonus), spellAbilityScore)
		saveAbility = spell.SaveAbility.String
	}

	// 12. Calculate pending saves with cover
	// Determine AoE origin for cover calculation
	originCol := targetColIdx
	originRow := targetRowIdx
	// For cone/line, origin is the caster
	if aoe.Shape == "cone" || aoe.Shape == "line" {
		originCol = casterColIdx
		originRow = casterRowIdx
	}

	var pendingSaves []PendingSave
	var affectedNames []string
	for _, c := range affected {
		affectedNames = append(affectedNames, c.DisplayName)
		if saveAbility != "" {
			ps := CalculateAoECover(originCol, originRow, c, saveAbility, saveDC, cmd.Walls)
			pendingSaves = append(pendingSaves, ps)
		}
	}

	// E-59: persist one pending_saves row per affected combatant so the
	// /save (or DM dashboard) flow can resolve each one and trigger AoE
	// damage once they are all rolled. Source uses the "aoe:<spell-id>"
	// tag so the resolver can find every row tied to this cast without
	// touching unrelated concentration/DM-prompted saves.
	source := AoEPendingSaveSource(spell.ID)
	for _, ps := range pendingSaves {
		if _, err := s.store.CreatePendingSave(ctx, refdata.CreatePendingSaveParams{
			EncounterID: cmd.EncounterID,
			CombatantID: ps.CombatantID,
			Ability:     ps.SaveAbility,
			Dc:          int32(ps.DC),
			Source:      source,
		}); err != nil {
			return AoECastResult{}, fmt.Errorf("creating pending AoE save for %s: %w", ps.CombatantID, err)
		}
	}

	// 13. Resolve concentration: clean up any dropped spell, persist the new
	// concentration to the authoritative columns when applicable.
	concentration := ResolveConcentration(cmd.CurrentConcentration, spell)
	cleanupResult, err := s.applyConcentrationOnCast(ctx, caster, spell, concentration)
	if err != nil {
		return AoECastResult{}, err
	}

	// 14. Use action/bonus action resource
	turn := cmd.Turn
	turn, err = UseResource(turn, resource)
	if err != nil {
		return AoECastResult{}, err
	}
	if isBonusAction {
		turn.BonusActionSpellCast = true
	} else if spellLevel > 0 {
		turn.ActionSpellCast = true
	}

	// 15. Persist turn state
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return AoECastResult{}, fmt.Errorf("updating turn: %w", err)
	}

	// 16. Deduct spell slot and persist
	deduction, err := s.deductAndPersistSlot(ctx, char.ID, slots, effectiveSlotLevel)
	if err != nil {
		return AoECastResult{}, err
	}

	// 17. med-26 / Phase 67: auto-create persistent zones at the targeted
	// tile for known AoE spells. Different from Cast because AoE origin is
	// the targeted coordinate, not the caster's position. Errors are
	// surfaced so the DM can investigate.
	if def, ok := LookupZoneDefinition(spell.Name); ok {
		anchorID := uuid.NullUUID{}
		if def.AnchorMode == "combatant" {
			anchorID = uuid.NullUUID{UUID: caster.ID, Valid: true}
		}
		// E-67-zone-cleanup: same duration-based expiry as Cast's
		// maybeCreateSpellZone path so AoE-created zones (Fog Cloud,
		// Darkness, Wall of Fire, etc.) also tick down on the round
		// advance hook.
		expiresAt := s.computeZoneExpiry(ctx, caster.EncounterID, spell)
		_, zoneErr := s.CreateZone(ctx, CreateZoneInput{
			EncounterID:           caster.EncounterID,
			SourceCombatantID:     caster.ID,
			SourceSpell:           spell.Name,
			Shape:                 def.Shape,
			OriginCol:             cmd.TargetCol,
			OriginRow:             cmd.TargetRow,
			Dimensions:            zoneDimensionsForDefinition(def, spell),
			AnchorMode:            zoneAnchorOrDefault(def.AnchorMode),
			AnchorCombatantID:     anchorID,
			ZoneType:              def.ZoneType,
			OverlayColor:          def.OverlayColor,
			MarkerIcon:            def.MarkerIcon,
			RequiresConcentration: def.RequiresConcentration,
			ExpiresAtRound:        expiresAt,
			Triggers:              def.Triggers,
		})
		if zoneErr != nil {
			return AoECastResult{}, fmt.Errorf("creating zone for %s: %w", spell.Name, zoneErr)
		}
	}

	return AoECastResult{
		CasterName:           caster.DisplayName,
		SpellName:            spell.Name,
		SpellLevel:           spellLevel,
		IsBonusAction:        isBonusAction,
		SaveDC:               saveDC,
		SaveAbility:          saveAbility,
		AffectedNames:        affectedNames,
		PendingSaves:         pendingSaves,
		Concentration:        concentration,
		SlotUsed:             deduction.SlotUsed,
		SlotsRemaining:       deduction.SlotsRemaining,
		OriginCol:            originCol,
		OriginRow:            originRow,
		ConcentrationCleanup: cleanupResult,
	}, nil
}

// AoEPendingSaveSourcePrefix is the source-column prefix used for pending_saves
// rows persisted by CastAoE. Encoded as "aoe:<spell-id>" so the resolver can
// distinguish AoE-cast rows from concentration / DM-prompted saves and look
// the spell up for damage application without a side-table.
const AoEPendingSaveSourcePrefix = "aoe:"

// AoEPendingSaveSource returns the canonical pending_saves.source value for
// a pending AoE save originating from the given spell.
func AoEPendingSaveSource(spellID string) string {
	return AoEPendingSaveSourcePrefix + spellID
}

// IsAoEPendingSaveSource reports whether the given pending_saves.source value
// was produced by CastAoE. The /save handler uses this to detect AoE-tagged
// rows on the rolling player's combatant.
func IsAoEPendingSaveSource(source string) bool {
	return strings.HasPrefix(source, AoEPendingSaveSourcePrefix)
}

// SpellIDFromAoEPendingSaveSource extracts the spell ID from an
// "aoe:<spell-id>" source tag. Returns "" when the source is not AoE-tagged.
func SpellIDFromAoEPendingSaveSource(source string) string {
	if !IsAoEPendingSaveSource(source) {
		return ""
	}
	return strings.TrimPrefix(source, AoEPendingSaveSourcePrefix)
}

// ResolveAoEPendingSaves checks every pending_saves row in the encounter
// tagged for the given spell. When all are resolved (rolled or forfeited),
// it dispatches to ResolveAoESaves to apply damage and returns the result.
// While any row remains pending the function returns (nil, nil) — callers
// (the /save handler, the DM dashboard) invoke it after each individual
// resolution to drive the eventual damage application without polling.
//
// This is the post-resolution hook required by E-59: damage / effects must
// land once every affected combatant's save has come back, regardless of
// whether it was rolled by a player (/save) or by the DM (dashboard).
func (s *Service) ResolveAoEPendingSaves(ctx context.Context, encounterID uuid.UUID, spellID string, roller *dice.Roller) (*AoEDamageResult, error) {
	rows, err := s.store.ListPendingSavesByEncounter(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing pending saves: %w", err)
	}
	source := AoEPendingSaveSource(spellID)
	var spellRows []refdata.PendingSafe
	for _, r := range rows {
		if r.Source == source {
			spellRows = append(spellRows, r)
		}
	}
	if len(spellRows) == 0 {
		return nil, nil
	}
	for _, r := range spellRows {
		if r.Status == "pending" {
			return nil, nil
		}
	}

	// All resolved — derive damage parameters from the spell and run the
	// existing ResolveAoESaves pipeline. Forfeited rows (DM cancelled, etc.)
	// are treated as failed saves: that matches the player-side default of
	// "no roll means no benefit of the save".
	spell, err := s.store.GetSpell(ctx, spellID)
	if err != nil {
		return nil, fmt.Errorf("looking up spell %q for AoE damage: %w", spellID, err)
	}
	if !spell.Damage.Valid {
		// Non-damaging AoE (e.g. condition-only). Nothing to apply at this
		// hook; condition/effect work happens elsewhere.
		return &AoEDamageResult{}, nil
	}
	dmgInfo, err := ParseSpellDamage(spell.Damage.RawMessage)
	if err != nil {
		return nil, fmt.Errorf("parsing AoE damage: %w", err)
	}
	saveEffect := ""
	if spell.SaveEffect.Valid {
		saveEffect = spell.SaveEffect.String
	}
	saveResults := make([]SaveResult, 0, len(spellRows))
	for _, r := range spellRows {
		success := r.Success.Valid && r.Success.Bool
		total := 0
		if r.RollResult.Valid {
			total = int(r.RollResult.Int32)
		}
		saveResults = append(saveResults, SaveResult{
			CombatantID: r.CombatantID,
			Rolled:      total,
			Total:       total,
			Success:     success,
		})
	}
	input := AoEDamageInput{
		EncounterID: encounterID,
		SpellName:   spell.Name,
		DamageDice:  dmgInfo.Dice,
		DamageType:  dmgInfo.DamageType,
		SaveEffect:  saveEffect,
		SaveResults: saveResults,
	}
	res, err := s.ResolveAoESaves(ctx, input, roller)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// RecordAoEPendingSaveRoll resolves a single AoE pending_saves row for a
// player's /save command. Looks for the oldest pending row on this combatant
// tagged with any AoE source and writes the rolled value. Success is
// computed canonically as (total >= row.Dc) — auto-fail is signalled by the
// caller passing autoFail=true (e.g. natural 1, paralyzed). Returns the
// resolved row's spell ID (for the caller to drive
// ResolveAoEPendingSavesForSpell) and a bool indicating whether anything
// was resolved.
//
// When the combatant has no pending AoE save the function is a no-op so the
// /save handler can call it unconditionally.
func (s *Service) RecordAoEPendingSaveRoll(ctx context.Context, combatantID uuid.UUID, ability string, total int, autoFail bool) (string, bool, error) {
	rows, err := s.store.ListPendingSavesByCombatant(ctx, combatantID)
	if err != nil {
		return "", false, fmt.Errorf("listing pending saves: %w", err)
	}
	for _, r := range rows {
		if !IsAoEPendingSaveSource(r.Source) {
			continue
		}
		if r.Status != "pending" {
			continue
		}
		if r.Ability != ability {
			continue
		}
		success := !autoFail && total >= int(r.Dc)
		updated, err := s.store.UpdatePendingSaveResult(ctx, refdata.UpdatePendingSaveResultParams{
			ID:         r.ID,
			RollResult: sql.NullInt32{Int32: int32(total), Valid: true},
			Success:    sql.NullBool{Bool: success, Valid: true},
		})
		if err != nil {
			return "", false, fmt.Errorf("updating pending save: %w", err)
		}
		return SpellIDFromAoEPendingSaveSource(updated.Source), true, nil
	}
	return "", false, nil
}


// AoEDamageInput holds the inputs for resolving AoE save results and applying damage.
type AoEDamageInput struct {
	EncounterID uuid.UUID
	SpellName   string
	DamageDice  string // e.g., "8d6"
	DamageType  string // e.g., "fire"
	SaveEffect  string // "half_damage", "no_effect", "special"
	SaveResults []SaveResult
}

// AoEDamageResult holds the outcomes of resolving AoE saves and applying damage.
type AoEDamageResult struct {
	Targets    []AoETargetOutcome
	TotalDamage int
}

// AoETargetOutcome holds the outcome for a single target of an AoE spell.
type AoETargetOutcome struct {
	CombatantID uuid.UUID
	DisplayName string
	SaveSuccess bool
	SaveTotal   int
	CoverBonus  int
	DamageDealt int
	HPBefore    int
	HPAfter     int
}

// ResolveAoESaves rolls damage once, applies save multipliers, and updates combatant HP.
func (s *Service) ResolveAoESaves(ctx context.Context, input AoEDamageInput, roller *dice.Roller) (AoEDamageResult, error) {
	// 1. Roll damage once for all targets
	rollResult, err := roller.Roll(input.DamageDice)
	if err != nil {
		return AoEDamageResult{}, fmt.Errorf("rolling damage %q: %w", input.DamageDice, err)
	}
	baseDamage := rollResult.Total

	var targets []AoETargetOutcome
	totalDamage := 0

	for _, sr := range input.SaveResults {
		// 2. Look up combatant
		combatant, err := s.store.GetCombatant(ctx, sr.CombatantID)
		if err != nil {
			return AoEDamageResult{}, fmt.Errorf("getting combatant %s: %w", sr.CombatantID, err)
		}

		// 3. Apply save multiplier
		multiplier := ApplySaveResult(sr.Success, input.SaveEffect)
		damage := int(float64(baseDamage) * multiplier)
		if damage < 0 {
			damage = 0 // special case: DM resolution needed
		}

		// 4. Route through ApplyDamage so Phase 42 (R/I/V, temp HP,
		// exhaustion HP-halving / level-6 death) applies before the
		// underlying applyDamageHP write. ApplyDamage in turn fires the
		// Phase 118 concentration save / unconscious-at-0-HP hooks.
		hpBefore := int(combatant.HpCurrent)
		dmgRes, err := s.ApplyDamage(ctx, ApplyDamageInput{
			EncounterID: combatant.EncounterID,
			Target:      combatant,
			RawDamage:   damage,
			DamageType:  input.DamageType,
		})
		if err != nil {
			return AoEDamageResult{}, fmt.Errorf("updating HP for %s: %w", combatant.DisplayName, err)
		}

		targets = append(targets, AoETargetOutcome{
			CombatantID: combatant.ID,
			DisplayName: combatant.DisplayName,
			SaveSuccess: sr.Success,
			SaveTotal:   sr.Total,
			CoverBonus:  sr.CoverBonus,
			DamageDealt: dmgRes.FinalDamage,
			HPBefore:    hpBefore,
			HPAfter:     int(dmgRes.NewHP),
		})
		totalDamage += dmgRes.FinalDamage
	}

	return AoEDamageResult{
		Targets:     targets,
		TotalDamage: totalDamage,
	}, nil
}
