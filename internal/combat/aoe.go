package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/character"
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
	for dc := range sideSquares {
		for dr := range sideSquares {
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
//
// AutoSuccess flips a row to a guaranteed save (Careful Spell metamagic). The
// pending_saves row is still inserted and resolved as a "success" so the
// existing all-rolls-resolved gate in ResolveAoEPendingSaves continues to
// fire damage application for the non-spared targets.
//
// Disadvantage marks the first save target for Heightened Spell metamagic so
// the rolling surface (/save) can apply disadvantage. The DB pending_saves
// table has no advantage column today, so this flag is informational on the
// in-memory result returned by CastAoE; wiring it end-to-end through the
// /save UX is SR-025 territory.
type PendingSave struct {
	CombatantID  uuid.UUID
	SaveAbility  string
	DC           int
	CoverBonus   int
	IsNPC        bool
	AutoSuccess  bool
	Disadvantage bool
	FullCover    bool
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
// If the target has full cover from the origin, FullCover is set to true (target should be excluded).
func CalculateAoECover(originCol, originRow int, combatant refdata.Combatant, saveAbility string, dc int, walls []renderer.WallSegment) PendingSave {
	coverBonus := 0
	targetCol := colToIndex(combatant.PositionCol)
	targetRow := int(combatant.PositionRow) - 1
	cover := CalculateCoverFromOrigin(originCol, originRow, targetCol, targetRow, walls)
	if cover == CoverFull {
		return PendingSave{
			CombatantID: combatant.ID,
			SaveAbility: saveAbility,
			DC:          dc,
			IsNPC:       combatant.IsNpc,
			FullCover:   true,
		}
	}
	if saveAbility == "dex" {
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
// For sphere/cylinder/square, the origin is the target point.
// For cone/line, the caster position and target direction are used.
//
// Cylinder geometry note: a cylinder of radius R and height H projected onto a
// top-down 2D grid covers exactly the same disc of tiles as a sphere of radius
// R — height_ft is decorative for tile selection and only matters for vertical
// clearance / fog / line-of-effect, which we do not model here. SR-008/SR-014
// covers the 3D side of this if/when needed.
func GetAffectedTiles(aoe AreaOfEffect, casterCol, casterRow, targetCol, targetRow int) ([]GridPos, error) {
	switch aoe.Shape {
	case "sphere":
		return SphereAffectedTiles(targetCol, targetRow, aoe.RadiusFt), nil
	case "cylinder":
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
	CasterName         string
	SpellName          string
	SpellLevel         int
	IsBonusAction      bool
	SaveDC             int
	SaveAbility        string
	AffectedNames      []string
	PendingSaves       []PendingSave
	Concentration      ConcentrationResult
	SlotUsed           int
	SlotsRemaining     int
	UsedPactSlot       bool
	PactSlotsRemaining int
	OriginCol          int
	OriginRow          int
	// SR-013: AoE pipeline now consumes Metamagic. Mirrors CastResult's fields.
	CarefulSpellCreatures  int    // CHA mod cap on auto-success allies (Careful Spell)
	IsEmpowered            bool   // Empowered Spell active — may reroll damage dice
	EmpoweredRerolls       int    // CHA mod re-roll allowance
	IsHeightened           bool   // first save target gets disadvantage (Heightened Spell)
	IsSubtle               bool   // Subtle Spell active
	DistantRange           string // Distant Spell new range description
	ExtendedDuration       string // Extended Spell new duration
	MetamagicCost          int    // sorcery points spent on metamagic
	SorceryPointsRemaining int    // sorcery points after metamagic spend
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

	// Slot usage. Warlocks spend a pact slot, not a leveled slot — report the
	// pact-slot remaining count, mirroring FormatCastLog (single-target path).
	if result.SpellLevel > 0 {
		if result.UsedPactSlot {
			fmt.Fprintf(&b, "\U0001f4a0 Used pact slot (%d remaining)\n", result.PactSlotsRemaining)
		} else {
			fmt.Fprintf(&b, "\U0001f4a0 Used %s-level slot (%d remaining)\n", ordinal(result.SlotUsed), result.SlotsRemaining)
		}
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
	// SR-013: AoE pipeline accepts Metamagic. Mirrors CastCommand naming.
	Metamagic []string // normalized option names ("careful", "heightened", ...)
	// CarefulTargetIDs lists allies who auto-succeed on the save when
	// Metamagic includes "careful". Capped at CHA mod (CarefulSpellCreatureCount).
	// Discord prompt UX (SR-025) populates this; CastAoE just honours it.
	CarefulTargetIDs []uuid.UUID
	// HeightenedTargetID names the affected combatant whose save gains
	// Disadvantage when Metamagic includes "heightened". When unset
	// (uuid.Nil) the existing behaviour falls back to "first affected
	// combatant" so Heightened still works from non-interactive code paths
	// (DM dashboard, replays). SR-025.
	HeightenedTargetID uuid.UUID
	UseSpellSlot       bool // true = force regular spell slots instead of pact slots
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
	pactSlots, _ := parsePactMagicSlots(char.PactMagicSlots.RawMessage)

	effectiveSlotLevel := 0
	usePactSlot := false
	if spellLevel > 0 {
		if !cmd.UseSpellSlot && pactSlots.Current > 0 && spellLevel <= pactSlots.SlotLevel {
			if cmd.SlotLevel > 0 && cmd.SlotLevel != pactSlots.SlotLevel {
				return AoECastResult{}, fmt.Errorf("Pact slots always cast at level %d; cannot use --slot %d", pactSlots.SlotLevel, cmd.SlotLevel)
			}
			effectiveSlotLevel = pactSlots.SlotLevel
			usePactSlot = true
		} else {
			effectiveSlotLevel, err = SelectSpellSlot(slots, spellLevel, cmd.SlotLevel)
			if err != nil {
				return AoECastResult{}, err
			}
		}
	}

	// 6b. SR-013: Metamagic validation. Must run BEFORE slot deduction (and
	// before pending_saves rows are written) so a rejected metamagic cast
	// doesn't burn a slot. ValidateMetamagicOptions reads the spell shape —
	// Twinned Spell rejects AoE here via validateTwinnedSpell.
	var metamagicCost int
	var metamagicFeatureUses map[string]character.FeatureUse
	var metamagicCurrentPoints int
	if len(cmd.Metamagic) > 0 {
		// COV-15: gate each option on the sorcerer having actually learned it
		// (builder-captured picks), before slot/sorcery-point deduction.
		if err := validateKnownMetamagic(char.Features, cmd.Metamagic); err != nil {
			return AoECastResult{}, err
		}
		metamagicFeatureUses, metamagicCurrentPoints, err = ParseFeatureUses(char, FeatureKeySorceryPoints)
		if err != nil {
			return AoECastResult{}, err
		}
		if err := ValidateMetamagic(cmd.Metamagic, spellLevel, metamagicCurrentPoints); err != nil {
			return AoECastResult{}, err
		}
		if err := ValidateMetamagicOptions(cmd.Metamagic, spell); err != nil {
			return AoECastResult{}, err
		}
		metamagicCost, err = MetamagicTotalCost(cmd.Metamagic, spellLevel)
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
	spellAbilityScore := resolveSpellcastingAbilityScore(classes, scores, spell.Classes)

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

	// SR-013: precompute Careful Spell allow-list (capped at CHA mod). The
	// command's CarefulTargetIDs may exceed the cap if the prompt UX over-
	// fills; we truncate canonically by first-N-input-order.
	carefulSet := make(map[uuid.UUID]bool)
	carefulCap := 0
	if hasMetamagic(cmd.Metamagic, "careful") {
		carefulCap = CarefulSpellCreatureCount(scores.Cha)
		for i, id := range cmd.CarefulTargetIDs {
			if i >= carefulCap {
				break
			}
			carefulSet[id] = true
		}
	}
	heightened := hasMetamagic(cmd.Metamagic, "heightened")
	// SR-025: explicit HeightenedTargetID (from the Discord prompt) wins
	// over the "first affected" default. uuid.Nil falls back to first-
	// affected so existing call sites (DM dashboard, replay) keep working.
	heightenedExplicit := cmd.HeightenedTargetID != uuid.Nil

	var pendingSaves []PendingSave
	var affectedNames []string
	heightenedAssigned := false
	for _, c := range affected {
		if saveAbility != "" {
			ps := CalculateAoECover(originCol, originRow, c, saveAbility, saveDC, cmd.Walls)
			// F-19: Full cover from AoE origin excludes the target entirely.
			if ps.FullCover {
				continue
			}
			affectedNames = append(affectedNames, c.DisplayName)
			if carefulSet[c.ID] {
				ps.AutoSuccess = true
			}
			if heightened && !ps.AutoSuccess {
				if heightenedExplicit && c.ID == cmd.HeightenedTargetID {
					ps.Disadvantage = true
					heightenedAssigned = true
				} else if !heightenedExplicit && !heightenedAssigned {
					ps.Disadvantage = true
					heightenedAssigned = true
				}
			}
			pendingSaves = append(pendingSaves, ps)
		} else {
			affectedNames = append(affectedNames, c.DisplayName)
		}
	}

	// E-59: persist one pending_saves row per affected combatant so the
	// /save (or DM dashboard) flow can resolve each one and trigger AoE
	// damage once they are all rolled. Source uses the "aoe:<spell-id>"
	// tag so the resolver can find every row tied to this cast without
	// touching unrelated concentration/DM-prompted saves.
	//
	// SR-013: Careful Spell auto-success rows are immediately resolved (rolled
	// = DC, success = true) so the all-rolls-resolved gate in
	// ResolveAoEPendingSaves advances without waiting on a player roll the
	// ally would never make.
	// SR-025: when Empowered is in the metamagic list, bake the reroll
	// count (CHA-mod, min 1) into the source tag so ResolveAoEPendingSaves
	// can echo it into ResolveAoESaves(EmpoweredRerolls: N).
	empoweredRerolls := 0
	if hasMetamagic(cmd.Metamagic, "empowered") {
		empoweredRerolls = EmpoweredRerollCount(scores.Cha)
	}
	source := AoEPendingSaveSourceFull(spell.ID, effectiveSlotLevel, int(char.Level), empoweredRerolls)
	for _, ps := range pendingSaves {
		created, err := s.store.CreatePendingSave(ctx, refdata.CreatePendingSaveParams{
			EncounterID: cmd.EncounterID,
			CombatantID: ps.CombatantID,
			Ability:     ps.SaveAbility,
			Dc:          int32(ps.DC),
			Source:      source,
			CoverBonus:  int32(ps.CoverBonus),
		})
		if err != nil {
			return AoECastResult{}, fmt.Errorf("creating pending AoE save for %s: %w", ps.CombatantID, err)
		}
		if ps.AutoSuccess {
			if _, err := s.store.UpdatePendingSaveResult(ctx, refdata.UpdatePendingSaveResultParams{
				ID:         created.ID,
				RollResult: sql.NullInt32{Int32: int32(ps.DC), Valid: true},
				Success:    sql.NullBool{Bool: true, Valid: true},
			}); err != nil {
				return AoECastResult{}, fmt.Errorf("auto-resolving Careful Spell save for %s: %w", ps.CombatantID, err)
			}
		}
	}

	// 2024 Rage sustain (b): forcing enemies to make saving throws keeps a rage
	// alive even on a turn with no attack roll. Best-effort, no-op unless the
	// caster is raging.
	if len(pendingSaves) > 0 {
		s.markRageForcedSave(ctx, caster)
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
	} else {
		// Casting a spell with your action is the Cast-a-Spell action, not the
		// Attack action — zero the seeded attack count so /done and the resource
		// summary don't report a phantom attack (mirrors Service.Cast).
		turn.AttacksRemaining = 0
		if spellLevel > 0 {
			turn.ActionSpellCast = true
		}
	}

	// 15. Persist turn state
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(turn)); err != nil {
		return AoECastResult{}, fmt.Errorf("updating turn: %w", err)
	}

	// 16. Deduct spell slot and persist
	var slotUsed, slotsRemaining int
	var usedPactSlotResult bool
	var pactSlotsRemainingResult int
	if usePactSlot {
		deduction, err := s.deductAndPersistPactSlot(ctx, char.ID, pactSlots)
		if err != nil {
			return AoECastResult{}, err
		}
		slotUsed = effectiveSlotLevel
		usedPactSlotResult = true
		pactSlotsRemainingResult = deduction.SlotsRemaining
	} else {
		deduction, err := s.deductAndPersistSlot(ctx, char.ID, slots, effectiveSlotLevel)
		if err != nil {
			return AoECastResult{}, err
		}
		slotUsed = deduction.SlotUsed
		slotsRemaining = deduction.SlotsRemaining
	}

	// 16a. SR-013: deduct sorcery points for metamagic.
	sorceryPointsRemaining := metamagicCurrentPoints
	if metamagicCost > 0 {
		sorceryPointsRemaining, err = s.DeductFeaturePool(ctx, char, FeatureKeySorceryPoints, metamagicFeatureUses, metamagicCurrentPoints, metamagicCost)
		if err != nil {
			return AoECastResult{}, err
		}
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

	// SR-013: surface metamagic-derived fields. Empowered/Heightened/Subtle/
	// Extended/Distant/Careful are populated whenever the matching flag is in
	// cmd.Metamagic, mirroring CastResult's contract.
	result := AoECastResult{
		CasterName:           caster.DisplayName,
		SpellName:            spell.Name,
		SpellLevel:           spellLevel,
		IsBonusAction:        isBonusAction,
		SaveDC:               saveDC,
		SaveAbility:          saveAbility,
		AffectedNames:        affectedNames,
		PendingSaves:         pendingSaves,
		Concentration:        concentration,
		SlotUsed:             slotUsed,
		SlotsRemaining:       slotsRemaining,
		UsedPactSlot:         usedPactSlotResult,
		PactSlotsRemaining:   pactSlotsRemainingResult,
		OriginCol:            originCol,
		OriginRow:            originRow,
		ConcentrationCleanup: cleanupResult,
	}
	if len(cmd.Metamagic) > 0 {
		applyAoEMetamagicEffects(&result, cmd.Metamagic, spell, scores.Cha)
		result.MetamagicCost = metamagicCost
		result.SorceryPointsRemaining = sorceryPointsRemaining
	}

	// ISSUE-014: persist the resolved area cast to action_log so it surfaces in
	// the DM Console timeline. Best-effort; no single target id for an AoE.
	// Use the loaded caster's encounter (authoritative here, like the rest of
	// CastAoE) rather than cmd.EncounterID, which partial callers may leave nil.
	s.recordCombatAction(ctx, cmd.Turn.ID, caster.EncounterID, cmd.CasterID,
		uuid.NullUUID{}, actionTypeCast,
		describeAoECast(result.CasterName, result.SpellName, result.AffectedNames))

	return result, nil
}

// applyAoEMetamagicEffects mirrors applyMetamagicEffects (CastResult flavor)
// for AoE casts. Kept separate because AoECastResult / CastResult are distinct
// types; the behavior is identical for the AoE-eligible options.
func applyAoEMetamagicEffects(result *AoECastResult, metamagics []string, spell refdata.Spell, chaScore int) {
	for _, m := range metamagics {
		switch strings.ToLower(m) {
		case "careful":
			result.CarefulSpellCreatures = CarefulSpellCreatureCount(chaScore)
		case "distant":
			result.DistantRange = ApplyDistantSpell(spell)
		case "empowered":
			result.IsEmpowered = true
			result.EmpoweredRerolls = EmpoweredRerollCount(chaScore)
		case "extended":
			result.ExtendedDuration = ApplyExtendedSpell(spell.Duration)
		case "heightened":
			result.IsHeightened = true
		case "subtle":
			result.IsSubtle = true
		}
	}
}

// AoEPendingSaveSourcePrefix is the source-column prefix used for pending_saves
// rows persisted by CastAoE and by single-target Cast (COV-1). The "aoe:" name
// is historical — the tag really means "a cast-originated spell save this
// pipeline owns", of which a single-target save is an "AoE of one". Encoded as
// "aoe:<spell-id>" (or "aoe:<spell-id>:e<N>" when Empowered metamagic is active)
// so the resolver can distinguish these rows from concentration / DM-prompted
// saves and look the spell up for damage application without a side-table.
const AoEPendingSaveSourcePrefix = "aoe:"

// AoEPendingSaveSource returns the canonical pending_saves.source value for
// a pending AoE save originating from the given spell.
func AoEPendingSaveSource(spellID string) string {
	return AoEPendingSaveSourcePrefix + spellID
}

// AoEPendingSaveSourceEmpowered is the SR-025 variant that bakes the
// Empowered reroll count into the source string. ResolveAoEPendingSaves
// reads this back to drive ResolveAoESaves(EmpoweredRerolls: N) so the
// damage roll re-rolls the N lowest dice once.
func AoEPendingSaveSourceEmpowered(spellID string, rerolls int) string {
	if rerolls <= 0 {
		return AoEPendingSaveSource(spellID)
	}
	return fmt.Sprintf("%s%s:e%d", AoEPendingSaveSourcePrefix, spellID, rerolls)
}

// AoEPendingSaveSourceFull encodes spell ID, effective slot level, character
// level, and empowered rerolls into the source tag so ResolveAoEPendingSaves
// can scale damage dice on resolution. Format:
//
//	"aoe:<spell-id>:s<slotLevel>c<charLevel>"          (no empowered)
//	"aoe:<spell-id>:s<slotLevel>c<charLevel>:e<N>"     (with empowered)
func AoEPendingSaveSourceFull(spellID string, slotLevel, charLevel, rerolls int) string {
	base := fmt.Sprintf("%s%s:s%dc%d", AoEPendingSaveSourcePrefix, spellID, slotLevel, charLevel)
	if rerolls > 0 {
		return fmt.Sprintf("%s:e%d", base, rerolls)
	}
	return base
}

// IsAoEPendingSaveSource reports whether the given pending_saves.source value
// was produced by CastAoE. The /save handler uses this to detect AoE-tagged
// rows on the rolling player's combatant.
func IsAoEPendingSaveSource(source string) bool {
	return strings.HasPrefix(source, AoEPendingSaveSourcePrefix)
}

// SpellIDFromAoEPendingSaveSource extracts the spell ID from an
// "aoe:<spell-id>" or "aoe:<spell-id>:s<N>c<N>" or "aoe:<spell-id>:e<N>"
// or "aoe:<spell-id>:s<N>c<N>:e<N>" source tag. Returns "" when the
// source is not AoE-tagged.
func SpellIDFromAoEPendingSaveSource(source string) string {
	if !IsAoEPendingSaveSource(source) {
		return ""
	}
	rest := strings.TrimPrefix(source, AoEPendingSaveSourcePrefix)
	// Strip trailing :e<N>
	if idx := strings.LastIndex(rest, ":e"); idx >= 0 {
		rest = rest[:idx]
	}
	// Strip trailing :s<N>c<N>
	if idx := strings.LastIndex(rest, ":s"); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}

// EmpoweredRerollsFromAoEPendingSaveSource extracts the Empowered reroll
// count from an "aoe:<spell-id>:e<N>" source. Returns 0 when no suffix is
// present. SR-025.
func EmpoweredRerollsFromAoEPendingSaveSource(source string) int {
	if !IsAoEPendingSaveSource(source) {
		return 0
	}
	rest := strings.TrimPrefix(source, AoEPendingSaveSourcePrefix)
	idx := strings.LastIndex(rest, ":e")
	if idx < 0 {
		return 0
	}
	n, err := strconv.Atoi(rest[idx+2:])
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// SlotLevelFromAoEPendingSaveSource extracts the effective slot level from
// an "aoe:<spell-id>:s<slotLevel>c<charLevel>" source tag. Returns 0 when
// no scaling info is present (legacy tags).
func SlotLevelFromAoEPendingSaveSource(source string) int {
	if !IsAoEPendingSaveSource(source) {
		return 0
	}
	rest := strings.TrimPrefix(source, AoEPendingSaveSourcePrefix)
	// Strip trailing :e<N>
	if idx := strings.LastIndex(rest, ":e"); idx >= 0 {
		rest = rest[:idx]
	}
	idx := strings.LastIndex(rest, ":s")
	if idx < 0 {
		return 0
	}
	seg := rest[idx+2:] // e.g. "5c10"
	before, _, ok := strings.Cut(seg, "c")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(before)
	if err != nil {
		return 0
	}
	return n
}

// CharLevelFromAoEPendingSaveSource extracts the caster's character level from
// an "aoe:<spell-id>:s<slotLevel>c<charLevel>" source tag. Returns 0 when
// no scaling info is present (legacy tags).
func CharLevelFromAoEPendingSaveSource(source string) int {
	if !IsAoEPendingSaveSource(source) {
		return 0
	}
	rest := strings.TrimPrefix(source, AoEPendingSaveSourcePrefix)
	// Strip trailing :e<N>
	if idx := strings.LastIndex(rest, ":e"); idx >= 0 {
		rest = rest[:idx]
	}
	idx := strings.LastIndex(rest, ":s")
	if idx < 0 {
		return 0
	}
	seg := rest[idx+2:] // e.g. "5c10"
	_, after, ok := strings.Cut(seg, "c")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(after)
	if err != nil {
		return 0
	}
	return n
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
// whether it was rolled by a player (/save) or by the DM (dashboard). Despite
// the name it is target-agnostic: single-target save spells (COV-1) enqueue one
// row with the same tag and resolve through this identical path (an "AoE of one").
func (s *Service) ResolveAoEPendingSaves(ctx context.Context, encounterID uuid.UUID, spellID string, roller *dice.Roller) (*AoEDamageResult, error) {
	// ISSUE-044: list EVERY save row for the encounter regardless of status. The
	// apply step is driven AFTER a save flips pending→rolled, so the row being
	// gated on is already resolved; the pending-only ListPendingSavesByEncounter
	// would hide it and the gate would never fire (damage never applied).
	rows, err := s.store.ListSavesByEncounter(ctx, encounterID)
	if err != nil {
		return nil, fmt.Errorf("listing saves: %w", err)
	}
	// SR-025: rows for the same spell may use either the plain source
	// ("aoe:<spell-id>") or the Empowered variant ("aoe:<spell-id>:eN").
	// Match by SpellIDFromAoEPendingSaveSource so both shapes resolve.
	var spellRows []refdata.PendingSafe
	for _, r := range rows {
		if SpellIDFromAoEPendingSaveSource(r.Source) != spellID {
			continue
		}
		spellRows = append(spellRows, r)
	}
	// Gate (a): nothing tagged for this spell.
	if len(spellRows) == 0 {
		return nil, nil
	}
	// Gate (b): still waiting on at least one target's save.
	for _, r := range spellRows {
		if r.Status == pendingSaveStatusPending {
			return nil, nil
		}
	}
	// Apply only to rows that have rolled but not yet been applied. This makes
	// the drive idempotent and multi-cast-safe: a row marked 'applied' is never
	// damaged again, so repeated drives (player /save + DM resolver both call
	// this) and a second cast of the same spell in one encounter cannot
	// double-hit. SR-025: the highest empowered-reroll suffix among the rows
	// being applied wins.
	empoweredRerolls := 0
	var toApply []refdata.PendingSafe
	for _, r := range spellRows {
		if r.Status != pendingSaveStatusRolled {
			continue
		}
		toApply = append(toApply, r)
		if n := EmpoweredRerollsFromAoEPendingSaveSource(r.Source); n > empoweredRerolls {
			empoweredRerolls = n
		}
	}
	// Gate (c): every matched row is already 'applied' → idempotent no-op.
	if len(toApply) == 0 {
		return nil, nil
	}

	// Gate (d): all resolved with at least one freshly rolled row — derive
	// damage parameters from the spell and run the ResolveAoESaves pipeline.
	// Forfeited rows (DM cancelled, etc.) are treated as failed saves: that
	// matches the player-side default of "no roll means no benefit of the save".
	spell, err := s.store.GetSpell(ctx, spellID)
	if err != nil {
		return nil, fmt.Errorf("looking up spell %q for AoE damage: %w", spellID, err)
	}
	// Resolve save-for-half/none damage (COV-1) when the spell deals damage. A
	// condition-only save spell (Hold Person, Web) has no damage and leaves res
	// zero-valued — it lands only its conditions in the shared tail below.
	var res AoEDamageResult
	if spell.Damage.Valid {
		dmgInfo, err := ParseSpellDamage(spell.Damage.RawMessage)
		if err != nil {
			return nil, fmt.Errorf("parsing AoE damage: %w", err)
		}

		// E-C02: scale damage dice using slot level and char level encoded in
		// the source tag. Legacy tags without scaling info fall back to base dice.
		slotLevel := SlotLevelFromAoEPendingSaveSource(toApply[0].Source)
		charLevel := CharLevelFromAoEPendingSaveSource(toApply[0].Source)
		scaledDice := ScaleSpellDice(dmgInfo, int(spell.Level), slotLevel, charLevel)

		saveEffect := ""
		if spell.SaveEffect.Valid {
			saveEffect = spell.SaveEffect.String
		}
		saveAbility := ""
		if spell.SaveAbility.Valid {
			saveAbility = spell.SaveAbility.String
		}
		saveResults := make([]SaveResult, 0, len(toApply))
		for _, r := range toApply {
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
			EncounterID:      encounterID,
			SpellName:        spell.Name,
			DamageDice:       scaledDice,
			DamageType:       dmgInfo.DamageType,
			SaveEffect:       saveEffect,
			SaveAbility:      saveAbility,
			SaveResults:      saveResults,
			EmpoweredRerolls: empoweredRerolls, // SR-025
		}
		res, err = s.ResolveAoESaves(ctx, input, roller)
		if err != nil {
			return nil, err
		}
	}

	// COV-2: land conditions_applied on every target that failed its save (in
	// addition to any save-for-half/none damage above), then close the save
	// lifecycle so the next drive is an idempotent no-op. Shared by the damage
	// and condition-only paths.
	condMsgs, err := s.applyOnFailConditions(ctx, encounterID, spell, toApply)
	if err != nil {
		return nil, err
	}
	res.ConditionMessages = condMsgs

	// ISSUE-066: a single-target concentration save spell (Hold Person, Tasha's
	// Hideous Laughter, ...) that lands NOTHING — its sole target made the save,
	// so no condition took hold — leaves the caster concentrating on an inert
	// spell. Nothing sustains it, so drop the concentration. Gated on
	// !hasAreaOfEffect so a zone concentration spell (Web, Moonbeam), whose area
	// persists regardless of any save, keeps concentration.
	if spell.Concentration.Valid && spell.Concentration.Bool && !hasAreaOfEffect(spell) {
		drop, derr := s.dropConcentrationIfAllSaved(ctx, encounterID, spell, spellRows)
		if derr != nil {
			return nil, derr
		}
		if drop != nil && drop.Broken {
			res.ConditionMessages = append(res.ConditionMessages, drop.ConsolidatedMessage)
		}
	}

	if err := s.markSavesApplied(ctx, toApply); err != nil {
		return nil, err
	}
	return &res, nil
}

// dropConcentrationIfAllSaved ends the caster's concentration when a single-
// target concentration save spell landed nothing: every resolved row made its
// save, so no condition/effect took hold and there is nothing left to sustain
// (ISSUE-066). Returns nil when any row failed or was forfeited (an effect
// landed → concentration holds) or when no caster is tracked. A row counts as
// "saved" only when Success is explicitly true, mirroring applyOnFailConditions'
// "no benefit of the save" default so a forfeited (never-rolled) row keeps
// concentration alive.
func (s *Service) dropConcentrationIfAllSaved(ctx context.Context, encounterID uuid.UUID, spell refdata.Spell, rows []refdata.PendingSafe) (*BreakConcentrationFullyResult, error) {
	for _, r := range rows {
		if !(r.Success.Valid && r.Success.Bool) {
			return nil, nil // a target failed/forfeited → effect landed → hold
		}
	}
	casterID, err := s.casterConcentratingOn(ctx, encounterID, spell.ID)
	if err != nil {
		return nil, err
	}
	if casterID == "" {
		return nil, nil
	}
	id, err := uuid.Parse(casterID)
	if err != nil {
		return nil, fmt.Errorf("parsing caster id %q for concentration drop: %w", casterID, err)
	}
	caster, err := s.store.GetCombatant(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting caster for concentration drop: %w", err)
	}
	return s.breakStoredConcentration(ctx, caster, "target saved")
}

// applyOnFailConditions lands each condition in spell.ConditionsApplied on
// every target in rows whose save FAILED (COV-2). This is the resolution-time
// counterpart to COV-1's save-for-half damage: the condition is applied only
// after the save comes back failed, never at cast time.
//
// Each condition is scoped to the spell (SourceSpell) and, for concentration
// spells, to its caster (SourceCombatantID, looked up from the encounter's
// concentration columns) so BreakConcentrationFully / RemoveSpellSourcedConditions
// strip it when the caster drops concentration. Duration is indefinite
// (DurationRounds=0): concentration spells clear via teardown, non-concentration
// ones via combat-end cleanup or the DM editor — per-turn re-saves and timed
// expiry are a follow-up. Condition-immune targets are skipped inside
// ApplyCondition (it emits a 🛡️ line, no error). Returns the combat-log lines.
//
// A no-op (returns nil, nil) for spells with no conditions_applied, so the
// damage-only AoE/single-target path never touches the condition machinery.
func (s *Service) applyOnFailConditions(ctx context.Context, encounterID uuid.UUID, spell refdata.Spell, rows []refdata.PendingSafe) ([]string, error) {
	if !hasConditions(spell) {
		return nil, nil
	}
	casterID, err := s.casterConcentratingOn(ctx, encounterID, spell.ID)
	if err != nil {
		return nil, err
	}
	var msgs []string
	for _, r := range rows {
		if r.Success.Valid && r.Success.Bool {
			continue // made the save — no condition
		}
		for _, name := range spell.ConditionsApplied {
			cond := CombatCondition{
				Condition:         name,
				SourceSpell:       spell.ID,
				SourceCombatantID: casterID,
			}
			// COV-19: stamp the end-of-turn repeat save (save ends) so the turn
			// engine can re-roll it. The DC is frozen from the pending-save row
			// (the caster's spell save DC at cast time).
			if spellResavesAtEndOfTurn(spell) {
				cond.SaveEndsAbility = spell.SaveAbility.String
				cond.SaveEndsDC = int(r.Dc)
			}
			_, applied, aerr := s.ApplyCondition(ctx, r.CombatantID, cond)
			if aerr != nil {
				return msgs, fmt.Errorf("applying %q from %s: %w", name, spell.ID, aerr)
			}
			msgs = append(msgs, applied...)
		}
	}
	return msgs, nil
}

// casterConcentratingOn returns the string ID of the combatant in the encounter
// currently concentrating on spellID, or "" when none is (a non-concentration
// spell, or the caster is not tracked). The value scopes an applied condition's
// SourceCombatantID so concentration teardown can match and strip it.
//
// Known limitation (mirrors COV-1's multi-cast note): if two casters concentrate
// on the same spell ID simultaneously, the first match wins — narrow window.
func (s *Service) casterConcentratingOn(ctx context.Context, encounterID uuid.UUID, spellID string) (string, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return "", fmt.Errorf("listing combatants for condition source: %w", err)
	}
	for _, c := range combatants {
		if c.ConcentrationSpellID.Valid && c.ConcentrationSpellID.String == spellID {
			return c.ID.String(), nil
		}
	}
	return "", nil
}

// markSavesApplied closes the lifecycle on every freshly-applied save so a
// repeated ResolveAoEPendingSaves drive becomes an idempotent no-op (ISSUE-044).
func (s *Service) markSavesApplied(ctx context.Context, rows []refdata.PendingSafe) error {
	for _, r := range rows {
		if err := s.store.MarkPendingSaveApplied(ctx, r.ID); err != nil {
			return fmt.Errorf("marking save %s applied: %w", r.ID, err)
		}
	}
	return nil
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
		success := !autoFail && total+int(r.CoverBonus) >= int(r.Dc)
		updated, err := s.store.UpdatePendingSaveResult(ctx, refdata.UpdatePendingSaveResultParams{
			ID:         r.ID,
			RollResult: sql.NullInt32{Int32: int32(total + int(r.CoverBonus)), Valid: true},
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
	// SaveAbility is the save ability slug (e.g. "dex"). Used to gate Evasion
	// (Rogue 7+), which upgrades only DEX save-for-half outcomes. COV-3.
	SaveAbility string
	SaveResults []SaveResult
	// SR-025: when > 0, ResolveAoESaves re-rolls the N lowest damage dice
	// once after the initial roll (Empowered Spell metamagic). The reroll
	// always targets the lowest values because the interactive prompt's
	// "pick which dice" UI is forfeit-friendly: the canonical default is
	// "reroll the worst", which is what an empowered cast resolves to
	// when the player does not click before the AoE save-resolution gate
	// fires (every per-target save has already returned).
	EmpoweredRerolls int
}

// AoEDamageResult holds the outcomes of resolving AoE saves and applying damage.
type AoEDamageResult struct {
	Targets     []AoETargetOutcome
	TotalDamage int
	// ConditionMessages holds the combat-log lines for conditions applied to
	// targets that FAILED their save (COV-2), e.g. "🧟 Goblin is paralyzed".
	// Empty for damage-only spells; populated from spell.ConditionsApplied by
	// applyOnFailConditions. Immune targets contribute a "🛡️ …immune…" line.
	ConditionMessages []string
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

	// 1a. SR-025: when Empowered metamagic is active, re-roll the N lowest
	// damage dice once and recompute the total. Walks each group in the
	// roll result (so multi-group expressions like "8d6+1d4" reroll within
	// each group separately by face count). The original rolls aren't
	// mutated — RerollLowestDice returns a new slice.
	if input.EmpoweredRerolls > 0 {
		remaining := input.EmpoweredRerolls
		newTotal := 0
		for _, g := range rollResult.Groups {
			if remaining <= 0 || g.Die == 0 {
				newTotal += g.Total
				continue
			}
			rerolledGroup := RerollLowestDice(g.Results, g.Die, remaining, func(faces int) int {
				one, rerollErr := roller.Roll(fmt.Sprintf("1d%d", faces))
				if rerollErr != nil {
					return 1
				}
				return one.Total
			})
			groupTotal := 0
			for _, v := range rerolledGroup {
				groupTotal += v
			}
			newTotal += groupTotal
			// Burn the rerolls used in this group against the remaining
			// budget. RerollLowestDice clamps count to len internally so
			// `min(remaining, len(g.Results))` is what was spent.
			used := min(remaining, len(g.Results))
			remaining -= used
		}
		baseDamage = newTotal + rollResult.Modifier
	}

	var targets []AoETargetOutcome
	totalDamage := 0

	for _, sr := range input.SaveResults {
		// 2. Look up combatant
		combatant, err := s.store.GetCombatant(ctx, sr.CombatantID)
		if err != nil {
			return AoEDamageResult{}, fmt.Errorf("getting combatant %s: %w", sr.CombatantID, err)
		}

		// 3. Apply save multiplier, then upgrade DEX save-for-half outcomes for
		// targets with a damage-reduction feature.
		//   - Evasion (Rogue 7+, COV-3) is a PASSIVE: no damage on a made save,
		//     half on a failed one.
		//   - Shield Master's Interpose Shield (COV-9) is RAW a REACTION: no damage
		//     on a made save (with a shield), full on a failed one.
		// Evasion is checked first because it strictly dominates — both zero a made
		// save, but Evasion halves a failed one where Interpose gives full. The
		// Interpose lookup is gated on sr.Success so its shield check only runs for
		// the made saves where it can matter. Every other target keeps the multiplier.
		//
		// SIMPLIFICATION (deferred): Interpose is auto-applied for free here, like
		// the passive Evasion beside it — its RAW reaction COST, the one-per-round
		// economy, and a pre-declare prompt are NOT charged. The save-resolution path
		// has no reaction surface (unlike the enemy-turn Turn Builder where Uncanny
		// Dodge, COV-16, does pay), and per the pre-declare rule a reaction must be
		// declared BEFORE the roll, not auto-resolved after it. Charging it here would
		// be a retroactive spend, so it waits for a real save-path reaction lane (the
		// same lane COV-1's PC-auto-prompt and COV-16's /attack defender-prompt await);
		// when built, Interpose moves OUT of this switch into that reaction machinery.
		multiplier := ApplySaveResult(sr.Success, input.SaveEffect)
		damage := max(int(float64(baseDamage)*multiplier),
			// special case: DM resolution needed
			0)
		if input.SaveEffect == "half_damage" && strings.EqualFold(input.SaveAbility, "dex") {
			switch {
			case s.combatantHasEvasion(ctx, combatant):
				damage = ApplyEvasion(baseDamage, sr.Success)
			case sr.Success && s.combatantHasInterposeShield(ctx, combatant):
				damage = ApplyInterposeShield(baseDamage, sr.Success)
			}
		}

		// 4. Route through ApplyDamage so Phase 42 (R/I/V, temp HP,
		// exhaustion level-6 death) applies before the
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

// combatantHasEvasion reports whether the target combatant is backed by a PC
// that has the Evasion class feature (Rogue 7+). Best-effort: a missing/invalid
// character row or a features-JSON parse error degrades to false, matching the
// collectFESResistances convention of "drop the bonus rather than fail the
// application". COV-3.
func (s *Service) combatantHasEvasion(ctx context.Context, target refdata.Combatant) bool {
	if !target.CharacterID.Valid {
		return false
	}
	char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
	if err != nil {
		return false
	}
	return hasFeatureEffect(char.Features, "evasion")
}

// combatantHasInterposeShield reports whether the target combatant is a PC with
// the Shield Master feat AND a shield equipped — the two prerequisites for
// Interpose Shield (take no damage on a successful DEX save-for-half). Best-effort:
// a missing/invalid character row degrades to false, matching combatantHasEvasion.
// COV-9.
func (s *Service) combatantHasInterposeShield(ctx context.Context, target refdata.Combatant) bool {
	if !target.CharacterID.Valid {
		return false
	}
	char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
	if err != nil {
		return false
	}
	return HasFeatureByName(char.Features.RawMessage, "Shield Master") && s.hasEquippedShield(ctx, char)
}
