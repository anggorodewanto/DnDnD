package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: ParseTeleportInfo parses teleport JSONB into typed struct
func TestParseTeleportInfo_SelfWithSight(t *testing.T) {
	raw := json.RawMessage(`{"target": "self", "range_ft": 30, "requires_sight": true}`)
	info, err := ParseTeleportInfo(raw)
	require.NoError(t, err)
	assert.Equal(t, "self", info.Target)
	assert.Equal(t, 30, info.RangeFt)
	assert.True(t, info.RequiresSight)
	assert.Equal(t, 0, info.CompanionRangeFt)
	assert.Equal(t, "", info.AdditionalEffects)
}

func TestParseTeleportInfo_SelfPlusCreature(t *testing.T) {
	raw := json.RawMessage(`{"target": "self+creature", "range_ft": 500, "requires_sight": false, "companion_range_ft": 5}`)
	info, err := ParseTeleportInfo(raw)
	require.NoError(t, err)
	assert.Equal(t, "self+creature", info.Target)
	assert.Equal(t, 500, info.RangeFt)
	assert.False(t, info.RequiresSight)
	assert.Equal(t, 5, info.CompanionRangeFt)
}

func TestParseTeleportInfo_WithAdditionalEffects(t *testing.T) {
	raw := json.RawMessage(`{"target": "self+creature", "range_ft": 90, "requires_sight": true, "companion_range_ft": 5, "additional_effects": "3d10 thunder to creatures within 10ft of departure"}`)
	info, err := ParseTeleportInfo(raw)
	require.NoError(t, err)
	assert.Equal(t, "3d10 thunder to creatures within 10ft of departure", info.AdditionalEffects)
}

func TestParseTeleportInfo_GroupTarget(t *testing.T) {
	raw := json.RawMessage(`{"target": "group"}`)
	info, err := ParseTeleportInfo(raw)
	require.NoError(t, err)
	assert.Equal(t, "group", info.Target)
}

func TestParseTeleportInfo_EmptyReturnsError(t *testing.T) {
	_, err := ParseTeleportInfo(nil)
	require.Error(t, err)
}

func TestParseTeleportInfo_InvalidJSON(t *testing.T) {
	_, err := ParseTeleportInfo(json.RawMessage(`{invalid`))
	require.Error(t, err)
}

// TDD Cycle 2: IsDMQueueTeleport identifies narrative teleports
func TestIsDMQueueTeleport(t *testing.T) {
	tests := []struct {
		target string
		want   bool
	}{
		{"self", false},
		{"self+creature", false},
		{"creature", false},
		{"portal", true},
		{"party", true},
		{"creatures", true},
		{"group", true},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			assert.Equal(t, tc.want, IsDMQueueTeleport(tc.target))
		})
	}
}

// TDD Cycle 3: ValidateTeleportDestination rejects occupied destination
func TestValidateTeleportDestination_OccupiedDestination(t *testing.T) {
	info := TeleportInfo{Target: "self", RangeFt: 30, RequiresSight: true}
	caster := refdata.Combatant{PositionCol: "A", PositionRow: 1}
	occupants := []refdata.Combatant{
		{PositionCol: "C", PositionRow: 3},
	}
	err := ValidateTeleportDestination(info, caster, "C", int32(3), occupants, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "occupied")
}

// TDD Cycle 3b: ValidateTeleportDestination allows unoccupied destination
func TestValidateTeleportDestination_UnoccupiedDestination(t *testing.T) {
	info := TeleportInfo{Target: "self", RangeFt: 30, RequiresSight: true}
	caster := refdata.Combatant{PositionCol: "A", PositionRow: 1}
	occupants := []refdata.Combatant{
		{PositionCol: "C", PositionRow: 5},
	}
	err := ValidateTeleportDestination(info, caster, "C", int32(3), occupants, nil)
	require.NoError(t, err)
}

// TDD Cycle 4: ValidateTeleportDestination rejects out-of-range destination
func TestValidateTeleportDestination_OutOfRange(t *testing.T) {
	info := TeleportInfo{Target: "self", RangeFt: 30, RequiresSight: true}
	// Caster at A1, destination at A20 = 19 squares = 95ft > 30ft
	caster := refdata.Combatant{PositionCol: "A", PositionRow: 1}
	err := ValidateTeleportDestination(info, caster, "A", int32(20), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "teleport range")
}

// TDD Cycle 5: ValidateTeleportDestination rejects companion too far
func TestValidateTeleportDestination_CompanionTooFar(t *testing.T) {
	info := TeleportInfo{Target: "self+creature", RangeFt: 500, RequiresSight: false, CompanionRangeFt: 5}
	caster := refdata.Combatant{PositionCol: "A", PositionRow: 1}
	companion := refdata.Combatant{PositionCol: "A", PositionRow: 10} // 9 squares = 45ft > 5ft
	err := ValidateTeleportDestination(info, caster, "C", int32(3), nil, &companion)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "companion")
}

// TDD Cycle 5b: ValidateTeleportDestination allows companion in range
func TestValidateTeleportDestination_CompanionInRange(t *testing.T) {
	info := TeleportInfo{Target: "self+creature", RangeFt: 500, RequiresSight: false, CompanionRangeFt: 5}
	caster := refdata.Combatant{PositionCol: "A", PositionRow: 1}
	companion := refdata.Combatant{PositionCol: "A", PositionRow: 2} // 1 square = 5ft
	err := ValidateTeleportDestination(info, caster, "C", int32(3), nil, &companion)
	require.NoError(t, err)
}

// TDD Cycle 6: ValidateTeleportDestination with zero range_ft (unlimited)
func TestValidateTeleportDestination_ZeroRangeUnlimited(t *testing.T) {
	info := TeleportInfo{Target: "self", RangeFt: 0, RequiresSight: false}
	caster := refdata.Combatant{PositionCol: "A", PositionRow: 1}
	// Very far destination - should pass since range_ft is 0 (unlimited)
	err := ValidateTeleportDestination(info, caster, "Z", int32(50), nil, nil)
	require.NoError(t, err)
}

// TDD Cycle 7: TeleportResult and CastResult integration
func TestCastResult_TeleportFields(t *testing.T) {
	result := CastResult{
		CasterName: "Gandalf",
		SpellName:  "Misty Step",
		SpellLevel: 2,
		Teleport: &TeleportResult{
			CasterMoved:   true,
			CasterDestCol: "D",
			CasterDestRow: 5,
			DMQueueRouted: false,
		},
	}
	assert.NotNil(t, result.Teleport)
	assert.True(t, result.Teleport.CasterMoved)
	assert.Equal(t, "D", result.Teleport.CasterDestCol)
	assert.Equal(t, int32(5), result.Teleport.CasterDestRow)
}

func TestCastResult_TeleportDMQueue(t *testing.T) {
	result := CastResult{
		CasterName: "Gandalf",
		SpellName:  "Teleport",
		SpellLevel: 7,
		Teleport: &TeleportResult{
			DMQueueRouted: true,
		},
	}
	assert.True(t, result.Teleport.DMQueueRouted)
	assert.False(t, result.Teleport.CasterMoved)
}

func TestCastResult_TeleportWithCompanion(t *testing.T) {
	result := CastResult{
		CasterName: "Gandalf",
		SpellName:  "Dimension Door",
		SpellLevel: 4,
		Teleport: &TeleportResult{
			CasterMoved:      true,
			CasterDestCol:    "H",
			CasterDestRow:    10,
			CompanionMoved:   true,
			CompanionName:    "Frodo",
			CompanionDestCol: "H",
			CompanionDestRow: 11,
		},
	}
	assert.True(t, result.Teleport.CompanionMoved)
	assert.Equal(t, "Frodo", result.Teleport.CompanionName)
}

// TDD Cycle 8: FormatCastLog includes teleportation for self teleport
func TestFormatCastLog_TeleportSelf(t *testing.T) {
	result := CastResult{
		CasterName:     "Gandalf",
		SpellName:      "Misty Step",
		SpellLevel:     2,
		SlotUsed:       2,
		SlotsRemaining: 2,
		IsBonusAction:  true,
		Teleport: &TeleportResult{
			CasterMoved:   true,
			CasterDestCol: "D",
			CasterDestRow: 5,
		},
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "Gandalf teleports to D5")
}

func TestFormatCastLog_TeleportWithCompanion(t *testing.T) {
	result := CastResult{
		CasterName:     "Gandalf",
		SpellName:      "Dimension Door",
		SpellLevel:     4,
		SlotUsed:       4,
		SlotsRemaining: 0,
		Teleport: &TeleportResult{
			CasterMoved:      true,
			CasterDestCol:    "H",
			CasterDestRow:    10,
			CompanionMoved:   true,
			CompanionName:    "Frodo",
			CompanionDestCol: "H",
			CompanionDestRow: 11,
		},
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "Gandalf teleports to H10")
	assert.Contains(t, log, "Frodo teleports to H11")
}

func TestFormatCastLog_TeleportDMQueue(t *testing.T) {
	result := CastResult{
		CasterName:     "Gandalf",
		SpellName:      "Teleport",
		SpellLevel:     7,
		SlotUsed:       7,
		SlotsRemaining: 0,
		ResolutionMode: "dm_required",
		Teleport: &TeleportResult{
			DMQueueRouted: true,
		},
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "Routed to DM")
}

func TestFormatCastLog_TeleportAdditionalEffects(t *testing.T) {
	result := CastResult{
		CasterName:     "Gandalf",
		SpellName:      "Thunder Step",
		SpellLevel:     3,
		SlotUsed:       3,
		SlotsRemaining: 1,
		Teleport: &TeleportResult{
			CasterMoved:       true,
			CasterDestCol:     "D",
			CasterDestRow:     5,
			AdditionalEffects: "3d10 thunder to creatures within 10ft of departure",
		},
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "3d10 thunder")
}

func makeMistyStepWithTeleport() refdata.Spell {
	return refdata.Spell{
		ID:             "misty-step",
		Name:           "Misty Step",
		Level:          2,
		CastingTime:    "1 bonus action",
		RangeType:      "self",
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Teleport: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"target":"self","range_ft":30,"requires_sight":true}`),
			Valid:      true,
		},
	}
}

func makeDimensionDoorWithTeleport() refdata.Spell {
	return refdata.Spell{
		ID:             "dimension-door",
		Name:           "Dimension Door",
		Level:          4,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 500, Valid: true},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Teleport: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"target":"self+creature","range_ft":500,"requires_sight":false,"companion_range_ft":5}`),
			Valid:      true,
		},
	}
}

func makeTeleportSpell() refdata.Spell {
	return refdata.Spell{
		ID:             "teleport",
		Name:           "Teleport",
		Level:          7,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Teleport: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"target":"party","requires_sight":false}`),
			Valid:      true,
		},
	}
}

func makeThunderStepWithTeleport() refdata.Spell {
	return refdata.Spell{
		ID:             "thunder-step",
		Name:           "Thunder Step",
		Level:          3,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 90, Valid: true},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Teleport: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"target":"self+creature","range_ft":90,"requires_sight":true,"companion_range_ft":5,"additional_effects":"3d10 thunder to creatures within 10ft of departure"}`),
			Valid:      true,
		},
	}
}

func makeWizardCharacterWithHighSlots(id uuid.UUID) refdata.Character {
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
		"4": {Current: 1, Max: 1},
		"7": {Current: 1, Max: 1},
	})
	scoresJSON, _ := json.Marshal(AbilityScores{
		Str: 8, Dex: 14, Con: 12, Int: 18, Wis: 10, Cha: 10,
	})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "wizard", Level: 13}})
	return refdata.Character{
		ID:               id,
		Name:             "Gandalf",
		ProficiencyBonus: 5,
		Classes:          classesJSON,
		AbilityScores:    scoresJSON,
		SpellSlots:       pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		Level:            13,
	}
}

// TDD Cycle 9: Cast integration — self teleport (Misty Step)
func TestCast_TeleportSelf(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	turnID := uuid.New()

	var posUpdated bool
	var updatedCol string
	var updatedRow int32
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeMistyStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true, BonusActionSpellCast: true}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		posUpdated = true
		updatedCol = arg.PositionCol
		updatedRow = arg.PositionRow
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	// Explicit sight context: caster at E5 (col=4, row=4), dest F6 (col=5, row=5).
	fow := &renderer.FogOfWar{Width: 10, Height: 10, States: make([]renderer.VisibilityState, 100)}
	fow.States[5*10+5] = renderer.Visible // F6 visible
	cmd := CastCommand{
		SpellID:         "misty-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: turnID, CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "F",
		TeleportDestRow: 6,
		Walls:           []renderer.WallSegment{{X1: 9, Y1: 0, X2: 9, Y2: 9}},
		FogOfWar:        fow,
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Misty Step", result.SpellName)
	require.NotNil(t, result.Teleport)
	assert.True(t, result.Teleport.CasterMoved)
	assert.Equal(t, "F", result.Teleport.CasterDestCol)
	assert.Equal(t, int32(6), result.Teleport.CasterDestRow)
	assert.True(t, posUpdated)
	assert.Equal(t, "F", updatedCol)
	assert.Equal(t, int32(6), updatedRow)
}

// TDD Cycle 10: Cast integration — DM queue teleport (Teleport spell)
func TestCast_TeleportDMQueue(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacterWithHighSlots(charID)
	caster := makeSpellCaster(charID)
	turnID := uuid.New()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeTeleportSpell(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: true, ActionSpellCast: true}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:     "teleport",
		CasterID:    caster.ID,
		Turn:        refdata.Turn{ID: turnID, CombatantID: caster.ID},
		EncounterID: caster.EncounterID,
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.NotNil(t, result.Teleport)
	assert.True(t, result.Teleport.DMQueueRouted)
	assert.False(t, result.Teleport.CasterMoved)
	assert.Equal(t, "dm_required", result.ResolutionMode)
}

// TDD Cycle 11: Cast integration — self+creature teleport (Dimension Door)
func TestCast_TeleportSelfPlusCreature(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacterWithHighSlots(charID)
	caster := makeSpellCaster(charID)
	companion := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Frodo",
		PositionCol: "E",
		PositionRow: 6, // adjacent to caster at E5
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
	turnID := uuid.New()

	var casterPosUpdated, companionPosUpdated bool
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeDimensionDoorWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		if id == companion.ID {
			return companion, nil
		}
		return refdata.Combatant{}, fmt.Errorf("unknown combatant %s", id)
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, companion}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: true, ActionSpellCast: true}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		if arg.ID == caster.ID {
			casterPosUpdated = true
		}
		if arg.ID == companion.ID {
			companionPosUpdated = true
		}
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, DisplayName: "updated", Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:          "dimension-door",
		CasterID:         caster.ID,
		Turn:             refdata.Turn{ID: turnID, CombatantID: caster.ID},
		EncounterID:      caster.EncounterID,
		TeleportDestCol:  "M",
		TeleportDestRow:  15,
		CompanionID:      companion.ID,
		CompanionDestCol: "M",
		CompanionDestRow: 16,
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.NotNil(t, result.Teleport)
	assert.True(t, result.Teleport.CasterMoved)
	assert.Equal(t, "M", result.Teleport.CasterDestCol)
	assert.Equal(t, int32(15), result.Teleport.CasterDestRow)
	assert.True(t, result.Teleport.CompanionMoved)
	assert.Equal(t, "Frodo", result.Teleport.CompanionName)
	assert.Equal(t, "M", result.Teleport.CompanionDestCol)
	assert.Equal(t, int32(16), result.Teleport.CompanionDestRow)
	assert.True(t, casterPosUpdated)
	assert.True(t, companionPosUpdated)
}

// TDD Cycle 12: Cast integration — teleport rejects occupied destination
func TestCast_TeleportOccupiedDestination(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	blocker := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Goblin",
		PositionCol: "F",
		PositionRow: 6,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, blocker}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "misty-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "F",
		TeleportDestRow: 6, // occupied by blocker
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "occupied")
}

// TDD Cycle 13: Cast integration — teleport out of range
func TestCast_TeleportOutOfRange(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "misty-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "A",
		TeleportDestRow: 50, // very far
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "teleport range")
}

func TestCast_TeleportRequiresSightRejectsDestinationBehindWall(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "A"
	caster.PositionRow = 3

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "misty-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "D",
		TeleportDestRow: 3,
		Walls:           []renderer.WallSegment{{X1: 2, Y1: 0, X2: 2, Y2: 5}},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.EqualError(t, err, "target has full cover — no line of sight")
}

func TestCast_TeleportRequiresSightRejectsDestinationNotVisibleInFoW(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "A"
	caster.PositionRow = 3

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}

	// FoW where destination D3 (col=3, row=2) is Unexplored (not visible).
	fow := &renderer.FogOfWar{Width: 10, Height: 10, States: make([]renderer.VisibilityState, 100)}
	// All tiles default to Unexplored (0); destination is NOT visible.

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "misty-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "D",
		TeleportDestRow: 3,
		FogOfWar:        fow,
		Walls:           []renderer.WallSegment{}, // empty but non-nil to pass fail-closed gate
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.EqualError(t, err, "target has full cover — no line of sight")
}

func TestCast_TeleportRequiresSightAllowsDestinationWithLineOfSight(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.PositionCol = "A"
	caster.PositionRow = 3

	var posUpdated bool
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true, BonusActionSpellCast: true}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		posUpdated = true
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}

	// Explicit FoW with destination visible and non-blocking walls (SR-044).
	// Caster at A3 (col=0, row=2), destination D3 (col=3, row=2).
	// Wall at col 5 does not block the path.
	fow := &renderer.FogOfWar{Width: 10, Height: 10, States: make([]renderer.VisibilityState, 100)}
	fow.States[2*10+3] = renderer.Visible // D3 (col=3, row=2) is visible
	walls := []renderer.WallSegment{{X1: 5, Y1: 0, X2: 5, Y2: 5}} // wall far to the right

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "misty-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "D",
		TeleportDestRow: 3,
		Walls:           walls,
		FogOfWar:        fow,
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.NotNil(t, result.Teleport)
	assert.True(t, result.Teleport.CasterMoved)
	assert.True(t, posUpdated)
}

// TDD Cycle 14: Cast integration — Thunder Step with additional effects
func TestCast_TeleportWithAdditionalEffects(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	turnID := uuid.New()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeThunderStepWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: true, ActionSpellCast: true}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	// Explicit sight context: caster at E5 (col=4, row=4), dest F6 (col=5, row=5).
	fow := &renderer.FogOfWar{Width: 10, Height: 10, States: make([]renderer.VisibilityState, 100)}
	fow.States[5*10+5] = renderer.Visible // F6 visible
	cmd := CastCommand{
		SpellID:         "thunder-step",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: turnID, CombatantID: caster.ID},
		EncounterID:     caster.EncounterID,
		TeleportDestCol: "F",
		TeleportDestRow: 6,
		Walls:           []renderer.WallSegment{{X1: 9, Y1: 0, X2: 9, Y2: 9}},
		FogOfWar:        fow,
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	require.NotNil(t, result.Teleport)
	assert.True(t, result.Teleport.CasterMoved)
	assert.Equal(t, "3d10 thunder to creatures within 10ft of departure", result.Teleport.AdditionalEffects)
}

// TDD Cycle 15: Cast integration — companion too far for Dimension Door
func TestCast_TeleportCompanionTooFar(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacterWithHighSlots(charID)
	caster := makeSpellCaster(charID)
	companion := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Frodo",
		PositionCol: "E",
		PositionRow: 20, // far from caster at E5
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeDimensionDoorWithTeleport(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return companion, nil
	}
	store.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{caster, companion}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:          "dimension-door",
		CasterID:         caster.ID,
		Turn:             refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		EncounterID:      caster.EncounterID,
		TeleportDestCol:  "M",
		TeleportDestRow:  15,
		CompanionID:      companion.ID,
		CompanionDestCol: "M",
		CompanionDestRow: 16,
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "companion")
}
