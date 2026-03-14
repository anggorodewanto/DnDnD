package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: FeatureKeySorceryPoints constant exists
func TestFeatureKeySorceryPoints(t *testing.T) {
	assert.Equal(t, "sorcery-points", FeatureKeySorceryPoints)
}

// helper to make a sorcerer character
func makeSorcererCharacter(id uuid.UUID, sorcLevel int, sorceryPoints int) refdata.Character {
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
	})
	scoresJSON, _ := json.Marshal(AbilityScores{
		Str: 8, Dex: 14, Con: 12, Int: 10, Wis: 10, Cha: 18,
	})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Sorcerer", Level: sorcLevel}})
	featureUsesJSON, _ := json.Marshal(map[string]int{
		FeatureKeySorceryPoints: sorceryPoints,
	})
	return refdata.Character{
		ID:               id,
		Name:             "Elara",
		ProficiencyBonus: 3,
		Classes:          classesJSON,
		AbilityScores:    scoresJSON,
		SpellSlots:       pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		FeatureUses:      pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		Level:            int32(sorcLevel),
	}
}

func makeSorcererCombatant(charID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Elara",
		PositionCol: "E",
		PositionRow: 5,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// TDD Cycle 2: SorceryPointCost returns correct costs for metamagic options
func TestSorceryPointCost(t *testing.T) {
	tests := []struct {
		metamagic string
		spellLvl  int
		want      int
	}{
		{"careful", 3, 1},
		{"distant", 3, 1},
		{"empowered", 3, 1},
		{"extended", 3, 1},
		{"heightened", 3, 3},
		{"quickened", 3, 2},
		{"subtle", 3, 1},
		{"twinned", 3, 3},   // spell level
		{"twinned", 0, 1},   // cantrip costs 1
		{"twinned", 1, 1},
		{"twinned", 5, 5},
	}
	for _, tc := range tests {
		t.Run(tc.metamagic, func(t *testing.T) {
			cost, err := SorceryPointCost(tc.metamagic, tc.spellLvl)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cost)
		})
	}
}

func TestSorceryPointCost_InvalidMetamagic(t *testing.T) {
	_, err := SorceryPointCost("invalid", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metamagic")
}

// TDD Cycle 3: ValidateMetamagic checks one-per-spell rule
func TestValidateMetamagic_OnePerSpell(t *testing.T) {
	// Two non-empowered metamagic options should fail
	err := ValidateMetamagic([]string{"careful", "quickened"}, 3, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one Metamagic option")
}

func TestValidateMetamagic_EmpoweredCanCombine(t *testing.T) {
	// Empowered + one other is OK
	err := ValidateMetamagic([]string{"empowered", "quickened"}, 3, 5)
	assert.NoError(t, err)
}

func TestValidateMetamagic_EmpoweredPlusTwoOthers(t *testing.T) {
	// Empowered + two others is NOT OK (more than 2 total, or more than 1 non-empowered)
	err := ValidateMetamagic([]string{"empowered", "quickened", "careful"}, 3, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one Metamagic option")
}

func TestValidateMetamagic_InsufficientPoints(t *testing.T) {
	err := ValidateMetamagic([]string{"heightened"}, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient sorcery points")
	assert.Contains(t, err.Error(), "3")
	assert.Contains(t, err.Error(), "2")
}

func TestValidateMetamagic_SufficientPoints(t *testing.T) {
	err := ValidateMetamagic([]string{"quickened"}, 3, 5)
	assert.NoError(t, err)
}

func TestValidateMetamagic_Empty(t *testing.T) {
	err := ValidateMetamagic(nil, 3, 5)
	assert.NoError(t, err)
}

func TestValidateMetamagic_EmpoweredComboCost(t *testing.T) {
	// empowered(1) + quickened(2) = 3 total, with 3 SP available
	err := ValidateMetamagic([]string{"empowered", "quickened"}, 3, 3)
	assert.NoError(t, err)

	// empowered(1) + quickened(2) = 3 total, with 2 SP available
	err = ValidateMetamagic([]string{"empowered", "quickened"}, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient sorcery points")
}

// TDD Cycle 4: MetamagicTotalCost sums up costs
func TestMetamagicTotalCost(t *testing.T) {
	cost, err := MetamagicTotalCost([]string{"empowered", "quickened"}, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, cost) // 1 + 2

	cost, err = MetamagicTotalCost([]string{"twinned"}, 5)
	require.NoError(t, err)
	assert.Equal(t, 5, cost)

	cost, err = MetamagicTotalCost(nil, 3)
	require.NoError(t, err)
	assert.Equal(t, 0, cost)
}

// TDD Cycle 5: Font of Magic — slot to points conversion
func TestFontOfMagic_SlotToPoints(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 3) // 3/5 sorcery points
	caster := makeSorcererCombatant(charID)

	var savedFeatureUses pqtype.NullRawMessage
	var savedSlots pqtype.NullRawMessage
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		savedFeatureUses = arg.FeatureUses
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		savedSlots = arg.SpellSlots
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(store)
	result, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  caster.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		SlotLevel: 2,
	})

	require.NoError(t, err)
	assert.Equal(t, 5, result.PointsRemaining) // 3 + 2 = 5
	assert.Contains(t, result.CombatLog, "2nd-level spell slot")
	assert.Contains(t, result.CombatLog, "2 sorcery points")

	// Verify feature uses were updated
	require.True(t, savedFeatureUses.Valid)
	var uses map[string]int
	require.NoError(t, json.Unmarshal(savedFeatureUses.RawMessage, &uses))
	assert.Equal(t, 5, uses[FeatureKeySorceryPoints])

	// Verify slot was deducted
	require.True(t, savedSlots.Valid)
}

func TestFontOfMagic_SlotToPoints_ExceedsMax(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 4) // 4/5 sorcery points
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  caster.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		SlotLevel: 2, // would give 4+2=6 > max 5
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceed")
}

func TestFontOfMagic_SlotToPoints_NotSorcerer(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID) // wizard, not sorcerer
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  caster.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		SlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Sorcerer")
}

func TestFontOfMagic_SlotToPoints_BonusActionRequired(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 3)
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  caster.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, BonusActionUsed: true},
		SlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

// TDD Cycle 6: Font of Magic — points to slot conversion
func TestFontOfMagic_PointsToSlot(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5) // 5/5 sorcery points
	caster := makeSorcererCombatant(charID)

	var savedFeatureUses pqtype.NullRawMessage
	var savedSlots pqtype.NullRawMessage
	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		savedFeatureUses = arg.FeatureUses
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		savedSlots = arg.SpellSlots
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}

	svc := NewService(store)
	result, err := svc.FontOfMagicCreateSlot(context.Background(), FontOfMagicCommand{
		CasterID:       caster.ID,
		Turn:           refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		CreateSlotLevel: 3, // costs 5 SP
	})

	require.NoError(t, err)
	assert.Equal(t, 0, result.PointsRemaining) // 5 - 5 = 0
	assert.Contains(t, result.CombatLog, "3rd-level spell slot")

	// Verify feature uses were updated
	require.True(t, savedFeatureUses.Valid)
	var uses map[string]int
	require.NoError(t, json.Unmarshal(savedFeatureUses.RawMessage, &uses))
	assert.Equal(t, 0, uses[FeatureKeySorceryPoints])

	// Verify slot was added
	require.True(t, savedSlots.Valid)
	var slots map[string]SlotInfo
	require.NoError(t, json.Unmarshal(savedSlots.RawMessage, &slots))
	assert.Equal(t, 3, slots["3"].Current) // was 2, now 3
}

func TestFontOfMagic_PointsToSlot_InsufficientPoints(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 2) // 2/5 sorcery points
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicCreateSlot(context.Background(), FontOfMagicCommand{
		CasterID:       caster.ID,
		Turn:           refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		CreateSlotLevel: 2, // costs 3 SP, but only 2 available
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestFontOfMagic_PointsToSlot_Above5thLevel(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 10, 10)
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicCreateSlot(context.Background(), FontOfMagicCommand{
		CasterID:       caster.ID,
		Turn:           refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		CreateSlotLevel: 6,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "5th")
}

// TDD Cycle 7: SlotToPointsCost returns the cost table for creating slots
func TestSlotCreationCost(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 2},
		{2, 3},
		{3, 5},
		{4, 6},
		{5, 7},
	}
	for _, tc := range tests {
		cost, err := SlotCreationCost(tc.level)
		require.NoError(t, err)
		assert.Equal(t, tc.want, cost, "level %d", tc.level)
	}
}

func TestSlotCreationCost_InvalidLevel(t *testing.T) {
	_, err := SlotCreationCost(0)
	assert.Error(t, err)

	_, err = SlotCreationCost(6)
	assert.Error(t, err)
}

// TDD Cycle 8: Format functions for Font of Magic
func TestFormatFontOfMagicConvert(t *testing.T) {
	log := FormatFontOfMagicConvert("Elara", 2, 2, 5)
	assert.Contains(t, log, "Elara")
	assert.Contains(t, log, "2nd-level spell slot")
	assert.Contains(t, log, "2 sorcery points")
	assert.Contains(t, log, "5 SP remaining")
}

func TestFormatFontOfMagicCreate(t *testing.T) {
	log := FormatFontOfMagicCreate("Elara", 3, 5, 0)
	assert.Contains(t, log, "Elara")
	assert.Contains(t, log, "3rd-level spell slot")
	assert.Contains(t, log, "5 SP")
	assert.Contains(t, log, "0 SP")
}

// TDD Cycle 9: Metamagic on CastCommand — integration with cast
func TestCastCommand_MetamagicFields(t *testing.T) {
	cmd := CastCommand{
		Metamagic: []string{"quickened"},
	}
	assert.Equal(t, []string{"quickened"}, cmd.Metamagic)
}

// TDD Cycle 10: Cast with metamagic deducts sorcery points
func TestCast_WithMetamagic_DeductsSorceryPoints(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5)
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()
	turnID := uuid.New()

	var savedFeatureUses pqtype.NullRawMessage
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}
	store.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		savedFeatureUses = arg.FeatureUses
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:   "fireball",
		CasterID:  caster.ID,
		TargetID:  target.ID,
		Turn:      refdata.Turn{ID: turnID, CombatantID: caster.ID},
		Metamagic: []string{"careful"},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Fireball", result.SpellName)
	assert.Equal(t, 1, result.MetamagicCost)
	assert.Equal(t, 4, result.SorceryPointsRemaining)

	// Verify sorcery points were deducted
	require.True(t, savedFeatureUses.Valid)
	var uses map[string]int
	require.NoError(t, json.Unmarshal(savedFeatureUses.RawMessage, &uses))
	assert.Equal(t, 4, uses[FeatureKeySorceryPoints])
}

// TDD Cycle 11: Cast with metamagic rejects insufficient points
func TestCast_WithMetamagic_InsufficientPoints(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 1) // only 1 SP
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:   "fireball",
		CasterID:  caster.ID,
		TargetID:  target.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		Metamagic: []string{"quickened"}, // costs 2 SP but only 1 available
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient sorcery points")
}

// TDD Cycle 12: Cast with two non-empowered metamagic rejects
func TestCast_WithMetamagic_TwoNonEmpoweredRejects(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 10)
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:   "fireball",
		CasterID:  caster.ID,
		TargetID:  target.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		Metamagic: []string{"careful", "quickened"},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one Metamagic option")
}

// TDD Cycle 13: Quickened spell uses bonus action
func TestCast_QuickenedSpell_UsesBonusAction(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5)
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()

	var savedTurnParams refdata.UpdateTurnActionsParams
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil // normally 1 action, but quickened makes it bonus action
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		savedTurnParams = arg
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed, BonusActionSpellCast: arg.BonusActionSpellCast}, nil
	}
	store.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:   "fireball",
		CasterID:  caster.ID,
		TargetID:  target.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		Metamagic: []string{"quickened"},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.IsBonusAction) // quickened makes it a bonus action
	assert.True(t, savedTurnParams.BonusActionUsed)
	assert.True(t, savedTurnParams.BonusActionSpellCast)
}

// TDD Cycle 14: Quickened spell rejects when bonus action already used
func TestCast_QuickenedSpell_BonusActionAlreadyUsed(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5)
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:   "fireball",
		CasterID:  caster.ID,
		TargetID:  target.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, BonusActionUsed: true},
		Metamagic: []string{"quickened"},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

// TDD Cycle 15: Cast without metamagic on non-sorcerer should still work
func TestCast_NoMetamagic_NonSorcerer(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		// No metamagic
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Fireball", result.SpellName)
	assert.Equal(t, 0, result.MetamagicCost)
}

// TDD Cycle 16: Font of Magic requires level 2+ Sorcerer
func TestFontOfMagic_RequiresLevel2(t *testing.T) {
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Sorcerer", Level: 1}})
	featureUsesJSON, _ := json.Marshal(map[string]int{FeatureKeySorceryPoints: 1})
	char := refdata.Character{
		ID:          charID,
		Name:        "NewSorc",
		Classes:     classesJSON,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		SpellSlots:  pqtype.NullRawMessage{RawMessage: []byte(`{"1":{"current":2,"max":2}}`), Valid: true},
		Level:       1,
	}
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  caster.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		SlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "level 2")
}

// TDD Cycle 17: MetamagicTotalCost error propagation
func TestMetamagicTotalCost_InvalidOption(t *testing.T) {
	_, err := MetamagicTotalCost([]string{"invalid"}, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metamagic")
}

// TDD Cycle 18: ValidateMetamagic with unknown option
func TestValidateMetamagic_UnknownOption(t *testing.T) {
	err := ValidateMetamagic([]string{"bogus"}, 3, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown metamagic")
}

// TDD Cycle 19: Font of Magic — NPC cannot use
func TestFontOfMagic_SlotToPoints_NPC(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Goblin",
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
		// No CharacterID
	}

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return npc, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  npc.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: npc.ID},
		SlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "player character")
}

func TestFontOfMagic_CreateSlot_NPC(t *testing.T) {
	npc := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Goblin",
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return npc, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicCreateSlot(context.Background(), FontOfMagicCommand{
		CasterID:       npc.ID,
		Turn:           refdata.Turn{ID: uuid.New(), CombatantID: npc.ID},
		CreateSlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "player character")
}

// TDD Cycle 20: Font of Magic CreateSlot bonus action required
func TestFontOfMagic_CreateSlot_BonusActionRequired(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5)
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicCreateSlot(context.Background(), FontOfMagicCommand{
		CasterID:       caster.ID,
		Turn:           refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, BonusActionUsed: true},
		CreateSlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}

// TDD Cycle 21: Font of Magic CreateSlot requires level 2+
func TestFontOfMagic_CreateSlot_RequiresLevel2(t *testing.T) {
	charID := uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Sorcerer", Level: 1}})
	featureUsesJSON, _ := json.Marshal(map[string]int{FeatureKeySorceryPoints: 2})
	char := refdata.Character{
		ID:          charID,
		Name:        "NewSorc",
		Classes:     classesJSON,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		SpellSlots:  pqtype.NullRawMessage{RawMessage: []byte(`{"1":{"current":2,"max":2}}`), Valid: true},
		Level:       1,
	}
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicCreateSlot(context.Background(), FontOfMagicCommand{
		CasterID:       caster.ID,
		Turn:           refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		CreateSlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "level 2")
}

// TDD Cycle 22: Font of Magic — no slot available at requested level
func TestFontOfMagic_SlotToPoints_NoSlotAvailable(t *testing.T) {
	charID := uuid.New()
	// Sorcerer with depleted 1st-level slots
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 0, Max: 4},
		"2": {Current: 3, Max: 3},
	})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Sorcerer", Level: 5}})
	featureUsesJSON, _ := json.Marshal(map[string]int{FeatureKeySorceryPoints: 2})
	char := refdata.Character{
		ID:          charID,
		Name:        "Elara",
		Classes:     classesJSON,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
		SpellSlots:  pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		Level:       5,
	}
	caster := makeSorcererCombatant(charID)

	store := defaultMockStore()
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	svc := NewService(store)
	_, err := svc.FontOfMagicConvertSlot(context.Background(), FontOfMagicCommand{
		CasterID:  caster.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		SlotLevel: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 1st-level spell slots remaining")
}

// TDD Cycle 23: Quickened spell must obey bonus-action-spell restriction (reverse direction)
// If a leveled action spell was already cast this turn, a quickened leveled spell should be rejected
// because quickened changes it to a bonus action spell.
func TestCast_QuickenedSpell_RejectedAfterActionSpellCast(t *testing.T) {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5)
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return makeFireball(), nil // "1 action" casting time, level 3
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:   "fireball",
		CasterID:  caster.ID,
		TargetID:  target.ID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, ActionSpellCast: true},
		Metamagic: []string{"quickened"},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leveled spell with your action")
}

// TDD Cycle 24: ValidateMetamagic normalizes inputs to lowercase
func TestValidateMetamagic_CaseInsensitive(t *testing.T) {
	// "Quickened" (uppercase) should be recognized and not rejected as unknown
	err := ValidateMetamagic([]string{"Quickened"}, 3, 5)
	assert.NoError(t, err)

	// "EMPOWERED" + "Quickened" should be valid combo (empowered can combine)
	err = ValidateMetamagic([]string{"EMPOWERED", "Quickened"}, 3, 5)
	assert.NoError(t, err)

	// Two non-empowered with mixed case should still fail
	err = ValidateMetamagic([]string{"Careful", "Quickened"}, 3, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one Metamagic option")
}
