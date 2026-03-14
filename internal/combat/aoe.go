package combat

import (
	"context"
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
	if err := ValidateBonusActionSpellRestriction(cmd.Turn, spell); err != nil {
		return AoECastResult{}, err
	}

	// 4. Look up caster combatant
	caster, err := s.store.GetCombatant(ctx, cmd.CasterID)
	if err != nil {
		return AoECastResult{}, fmt.Errorf("getting caster: %w", err)
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

	// 13. Resolve concentration
	concentration := ResolveConcentration(cmd.CurrentConcentration, spell)

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

	return AoECastResult{
		CasterName:     caster.DisplayName,
		SpellName:      spell.Name,
		SpellLevel:     spellLevel,
		IsBonusAction:  isBonusAction,
		SaveDC:         saveDC,
		SaveAbility:    saveAbility,
		AffectedNames:  affectedNames,
		PendingSaves:   pendingSaves,
		Concentration:  concentration,
		SlotUsed:       deduction.SlotUsed,
		SlotsRemaining: deduction.SlotsRemaining,
		OriginCol:      originCol,
		OriginRow:      originRow,
	}, nil
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

		// 4. Apply damage to HP
		hpBefore := int(combatant.HpCurrent)
		newHP := int(combatant.HpCurrent) - damage
		if newHP < 0 {
			newHP = 0
		}
		isAlive := newHP > 0

		_, err = s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
			ID:        combatant.ID,
			HpCurrent: int32(newHP),
			TempHp:    combatant.TempHp,
			IsAlive:   isAlive,
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
			DamageDealt: damage,
			HPBefore:    hpBefore,
			HPAfter:     newHP,
		})
		totalDamage += damage
	}

	return AoEDamageResult{
		Targets:     targets,
		TotalDamage: totalDamage,
	}, nil
}
