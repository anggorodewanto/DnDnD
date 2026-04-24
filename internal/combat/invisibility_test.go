package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Cycle 1: CombatCondition.SourceSpell round-trips through JSON ---

func TestCombatCondition_SourceSpell_RoundTrip(t *testing.T) {
	cond := CombatCondition{
		Condition:         "invisible",
		DurationRounds:    0,
		SourceCombatantID: "caster-id-123",
		SourceSpell:       "invisibility",
	}
	data, err := json.Marshal(cond)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"source_spell":"invisibility"`)

	var parsed CombatCondition
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "invisibility", parsed.SourceSpell)
	assert.Equal(t, "caster-id-123", parsed.SourceCombatantID)
}

// --- Cycle 2: BreakInvisibilityOnAction helper ---

func TestBreakInvisibilityOnAction_RemovesStandardInvisibility(t *testing.T) {
	raw, err := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: "c1", SourceSpell: "invisibility"},
	})
	require.NoError(t, err)

	updated, removed, err := BreakInvisibilityOnAction(raw)
	require.NoError(t, err)
	assert.True(t, removed, "standard Invisibility should break on action")

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &conds))
	assert.Len(t, conds, 0)
}

func TestBreakInvisibilityOnAction_PreservesGreaterInvisibility(t *testing.T) {
	raw, err := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: "c1", SourceSpell: "greater-invisibility"},
	})
	require.NoError(t, err)

	updated, removed, err := BreakInvisibilityOnAction(raw)
	require.NoError(t, err)
	assert.False(t, removed, "Greater Invisibility must persist through attacks/casts")

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &conds))
	assert.Len(t, conds, 1)
	assert.Equal(t, "greater-invisibility", conds[0].SourceSpell)
}

func TestBreakInvisibilityOnAction_NoInvisibleCondition(t *testing.T) {
	raw, err := json.Marshal([]CombatCondition{
		{Condition: "blessed", SourceCombatantID: "c1"},
	})
	require.NoError(t, err)

	updated, removed, err := BreakInvisibilityOnAction(raw)
	require.NoError(t, err)
	assert.False(t, removed, "no-op when no invisible condition")

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &conds))
	assert.Len(t, conds, 1)
	assert.Equal(t, "blessed", conds[0].Condition)
}

func TestBreakInvisibilityOnAction_NonSpellInvisible_Preserved(t *testing.T) {
	// invisible condition without a source spell (e.g., racial trait) should not be broken
	raw, err := json.Marshal([]CombatCondition{
		{Condition: "invisible"},
	})
	require.NoError(t, err)

	updated, removed, err := BreakInvisibilityOnAction(raw)
	require.NoError(t, err)
	assert.False(t, removed, "non-spell invisibility (no SourceSpell) should not be auto-broken")

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &conds))
	assert.Len(t, conds, 1)
}

func TestBreakInvisibilityOnAction_EmptyConditions(t *testing.T) {
	updated, removed, err := BreakInvisibilityOnAction(nil)
	require.NoError(t, err)
	assert.False(t, removed)

	var conds []CombatCondition
	// empty/nil raw returns an empty array
	if len(updated) > 0 {
		require.NoError(t, json.Unmarshal(updated, &conds))
	}
	assert.Len(t, conds, 0)
}

func TestBreakInvisibilityOnAction_InvalidJSON(t *testing.T) {
	_, _, err := BreakInvisibilityOnAction(json.RawMessage(`invalid`))
	require.Error(t, err)
}

func TestBreakInvisibilityOnAction_PreservesOtherConditions(t *testing.T) {
	raw, err := json.Marshal([]CombatCondition{
		{Condition: "blessed", SourceCombatantID: "c2"},
		{Condition: "invisible", SourceCombatantID: "c1", SourceSpell: "invisibility"},
	})
	require.NoError(t, err)

	updated, removed, err := BreakInvisibilityOnAction(raw)
	require.NoError(t, err)
	assert.True(t, removed)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(updated, &conds))
	require.Len(t, conds, 1)
	assert.Equal(t, "blessed", conds[0].Condition)
}

// --- Cycle 3: Advantage/Hidden interaction ---

func TestDetectAdvantage_InvisibleAttackerAndHidden(t *testing.T) {
	// Attacker is both invisible (condition) AND hidden (is_visible=false via AttackerHidden flag).
	// Both advantage sources should be reported simultaneously.
	input := AdvantageInput{
		AttackerConditions: []CombatCondition{{Condition: "invisible"}},
		AttackerHidden:     true,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "attacker invisible")
	assert.Contains(t, advReasons, "attacker hidden")
	assert.Empty(t, disadvReasons)
}

func TestDetectAdvantage_InvisibleTargetAndHidden(t *testing.T) {
	// Target is both invisible AND hidden — both disadvantage sources reported.
	input := AdvantageInput{
		TargetConditions: []CombatCondition{{Condition: "invisible"}},
		TargetHidden:     true,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "target invisible")
	assert.Contains(t, disadvReasons, "target hidden")
	assert.Empty(t, advReasons)
}

// --- Helpers for integration tests ---

func makeInvisibilitySpell() refdata.Spell {
	return invisibilitySpellFixture("invisibility", "Invisibility", 2, true)
}

func makeGreaterInvisibilitySpell() refdata.Spell {
	return invisibilitySpellFixture("greater-invisibility", "Greater Invisibility", 4, true)
}

func makeSpellAreaOfEffect() refdata.Spell {
	// Small AoE spell (Fireball-ish): save-based, has area_of_effect, ranged.
	s := makeFireball()
	s.AreaOfEffect = pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"type":"sphere","size_ft":20}`), Valid: true}
	return s
}

// --- Cycle 4: Attack breaks standard Invisibility / preserves Greater ---

func TestServiceAttack_BreaksStandardInvisibility(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	invisibleConditions, err := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: attackerID.String(), SourceSpell: "invisibility"},
	})
	require.NoError(t, err)

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	var savedConds json.RawMessage
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		savedConds = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A", PositionRow: 1,
		IsAlive:    true,
		IsVisible:  true,
		Conditions: invisibleConditions,
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B", PositionRow: 1,
		Ac:         13,
		IsAlive:    true,
		IsNpc:      true,
		Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.InvisibilityBroken, "standard Invisibility should break on attack")

	// Verify the updated conditions no longer include invisible
	require.NotNil(t, savedConds)
	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(savedConds, &conds))
	for _, c := range conds {
		assert.NotEqual(t, "invisible", c.Condition, "invisible should be removed after attack")
	}
}

// --- Cycle 5: Cast breaks standard Invisibility / preserves Greater ---

func TestServiceCast_BreaksStandardInvisibility(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.Conditions = mustMarshalConds(t, []CombatCondition{
		{Condition: "invisible", SourceCombatantID: caster.ID.String(), SourceSpell: "invisibility"},
	})
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeFireball(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	var savedConds json.RawMessage
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		savedConds = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "fireball", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.InvisibilityBroken, "casting should break standard Invisibility")

	require.NotNil(t, savedConds)
	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(savedConds, &conds))
	for _, c := range conds {
		assert.NotEqual(t, "invisible", c.Condition)
	}
}

func TestServiceCast_PreservesGreaterInvisibility(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.Conditions = mustMarshalConds(t, []CombatCondition{
		{Condition: "invisible", SourceCombatantID: caster.ID.String(), SourceSpell: "greater-invisibility"},
	})
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeFireball(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}
	condUpdateCount := 0
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		condUpdateCount++
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "fireball", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)
	assert.False(t, result.InvisibilityBroken, "Greater Invisibility must persist through casting")
	assert.Equal(t, 0, condUpdateCount, "no condition update should happen when Greater Invisibility is present")
}

// --- Cycle 6: Casting Invisibility applies invisible condition ---

func TestServiceCast_Invisibility_AppliesConditionToTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	// target is adjacent (touch range): caster at E5, target at E6
	target.PositionRow = 6

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeInvisibilitySpell(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	var appliedID uuid.UUID
	var appliedConds json.RawMessage
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		appliedID = arg.ID
		appliedConds = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "invisibility", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.InvisibilityApplied, "invisibility should apply invisible condition")
	assert.Equal(t, target.ID.String(), result.InvisibilityTargetID)

	assert.Equal(t, target.ID, appliedID)
	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(appliedConds, &conds))
	found := false
	for _, c := range conds {
		if c.Condition != "invisible" {
			continue
		}
		found = true
		assert.Equal(t, "invisibility", c.SourceSpell)
		assert.Equal(t, caster.ID.String(), c.SourceCombatantID)
	}
	assert.True(t, found, "should have invisible condition")
}

func TestServiceCast_GreaterInvisibility_AppliesConditionToTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	// Greater Invisibility is 4th level — give wizard a 4th level slot
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
		"4": {Current: 1, Max: 1},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeGreaterInvisibilitySpell(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	var appliedConds json.RawMessage
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		appliedConds = arg.Conditions
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "greater-invisibility", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.InvisibilityApplied)
	assert.Equal(t, target.ID.String(), result.InvisibilityTargetID)

	var conds []CombatCondition
	require.NoError(t, json.Unmarshal(appliedConds, &conds))
	found := false
	for _, c := range conds {
		if c.Condition != "invisible" {
			continue
		}
		found = true
		assert.Equal(t, "greater-invisibility", c.SourceSpell)
	}
	assert.True(t, found)
}

func TestServiceCast_Invisibility_SelfTargetWhenNoTarget(t *testing.T) {
	// When the caster targets themselves (no explicit TargetID), the caster
	// receives the invisible condition.
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeInvisibilitySpell(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) { return caster, nil }
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	var appliedID uuid.UUID
	store.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		appliedID = arg.ID
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "invisibility", CasterID: caster.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.InvisibilityApplied)
	assert.Equal(t, caster.ID.String(), result.InvisibilityTargetID)
	assert.Equal(t, caster.ID, appliedID, "self-target: caster receives the condition")
}

// --- Cycle 8: FormatAttackLog surfaces InvisibilityBroken ---

func TestFormatAttackLog_InvisibilityBroken(t *testing.T) {
	result := AttackResult{
		AttackerName:       "Aria",
		TargetName:         "Goblin #1",
		WeaponName:         "Longsword",
		DistanceFt:         5,
		IsMelee:            true,
		Hit:                true,
		D20Roll:            dice.D20Result{Chosen: 15, Total: 20, Modifier: 5},
		DamageTotal:        10,
		DamageType:         "slashing",
		DamageDice:         "1d8+5",
		InvisibilityBroken: true,
	}
	log := FormatAttackLog(result)
	assert.Contains(t, log, "Invisibility ends")
}

func TestFormatAttackLog_InvisibilityNotBroken(t *testing.T) {
	result := AttackResult{
		AttackerName: "Aria",
		TargetName:   "Goblin #1",
		WeaponName:   "Longsword",
		DistanceFt:   5,
		IsMelee:      true,
		Hit:          true,
		D20Roll:      dice.D20Result{Chosen: 15, Total: 20, Modifier: 5},
		DamageTotal:  10,
		DamageType:   "slashing",
		DamageDice:   "1d8+5",
	}
	log := FormatAttackLog(result)
	assert.NotContains(t, log, "Invisibility ends")
}

// --- Cycle 10: pure ValidateSeeTarget unit tests ---

func TestValidateSeeTarget_AllowsVisibleTarget(t *testing.T) {
	target := refdata.Combatant{DisplayName: "Goblin", Conditions: json.RawMessage(`[]`)}
	err := ValidateSeeTarget(makeFireBolt(), target)
	assert.NoError(t, err)
}

func TestValidateSeeTarget_BlocksInvisibleTarget_SingleTargetSpell(t *testing.T) {
	target := refdata.Combatant{DisplayName: "Goblin", Conditions: mustMarshalConds(t, []CombatCondition{{Condition: "invisible"}})}
	err := ValidateSeeTarget(makeFireBolt(), target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Goblin")
	assert.Contains(t, err.Error(), "invisible")
}

func TestValidateSeeTarget_AllowsAoEAgainstInvisible(t *testing.T) {
	target := refdata.Combatant{DisplayName: "Goblin", Conditions: mustMarshalConds(t, []CombatCondition{{Condition: "invisible"}})}
	err := ValidateSeeTarget(makeSpellAreaOfEffect(), target)
	assert.NoError(t, err)
}

func TestValidateSeeTarget_AllowsSelfRadius(t *testing.T) {
	spell := makeFireBolt()
	spell.RangeType = "self (radius)"
	target := refdata.Combatant{DisplayName: "Self", Conditions: mustMarshalConds(t, []CombatCondition{{Condition: "invisible"}})}
	err := ValidateSeeTarget(spell, target)
	assert.NoError(t, err)
}

// --- Cycle 11: applyInvisibilityBreakOnCast error path ---

func TestApplyInvisibilityBreakOnCast_DBError(t *testing.T) {
	ctx := context.Background()
	store := defaultMockStore()
	store.updateCombatantConditionsFn = func(_ context.Context, _ refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{}, assertAnError
	}
	svc := NewService(store)

	caster := refdata.Combatant{
		ID: uuid.New(),
		Conditions: mustMarshalConds(t, []CombatCondition{
			{Condition: "invisible", SourceSpell: "invisibility"},
		}),
	}
	_, err := svc.applyInvisibilityBreakOnCast(ctx, caster)
	require.Error(t, err)
}

var assertAnError = errorString("boom")

type errorString string

func (e errorString) Error() string { return string(e) }

// --- Cycle 9: FormatCastLog surfaces InvisibilityBroken / Applied ---

func TestFormatCastLog_InvisibilityBroken(t *testing.T) {
	result := CastResult{
		CasterName:         "Gandalf",
		SpellName:          "Fire Bolt",
		InvisibilityBroken: true,
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "Invisibility ends")
}

func TestFormatCastLog_InvisibilityApplied(t *testing.T) {
	result := CastResult{
		CasterName:          "Gandalf",
		SpellName:           "Invisibility",
		TargetName:          "Frodo",
		InvisibilityApplied: true,
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "invisible")
}

// --- Cycle 7: See-the-target validation ---

func TestServiceCast_SingleTargetSpell_RejectedWhenTargetInvisible(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6
	target.Conditions = mustMarshalConds(t, []CombatCondition{
		{Condition: "invisible", SourceCombatantID: "other", SourceSpell: "invisibility"},
	})

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeFireBolt(), nil } // single-target, no AoE
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "fire-bolt", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	_, err := svc.Cast(ctx, cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invisible")
}

func TestServiceCast_AoESpell_AllowedAgainstInvisibleTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6
	target.Conditions = mustMarshalConds(t, []CombatCondition{
		{Condition: "invisible", SourceCombatantID: "other", SourceSpell: "invisibility"},
	})

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSpellAreaOfEffect(), nil }
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == caster.ID {
			return caster, nil
		}
		return target, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "fireball", CasterID: caster.ID, TargetID: target.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	_, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err, "AoE spells should bypass see-target check")
}

func TestServiceCast_SelfTargetSpell_AllowedWhenSelfInvisible(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	caster.Conditions = mustMarshalConds(t, []CombatCondition{
		{Condition: "invisible", SourceCombatantID: caster.ID.String(), SourceSpell: "greater-invisibility"},
	})

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeMistyStep(), nil } // self range
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) { return caster, nil }
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{SpellID: "misty-step", CasterID: caster.ID, Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID}}
	_, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err, "self-target spells should not be blocked by self-invisibility")
}

func mustMarshalConds(t *testing.T, conds []CombatCondition) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(conds)
	require.NoError(t, err)
	return raw
}

func TestServiceAttack_PreservesGreaterInvisibility(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	invisibleConditions, err := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: attackerID.String(), SourceSpell: "greater-invisibility"},
	})
	require.NoError(t, err)

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	// Fail the test if conditions are updated — Greater Invisibility must not be touched
	condUpdateCount := 0
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		condUpdateCount++
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 15
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A", PositionRow: 1,
		IsAlive:    true,
		IsVisible:  true,
		Conditions: invisibleConditions,
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B", PositionRow: 1,
		Ac:         13,
		IsAlive:    true,
		IsNpc:      true,
		Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.False(t, result.InvisibilityBroken, "Greater Invisibility must not break on attack")
	assert.Equal(t, 0, condUpdateCount, "conditions should not be updated for Greater Invisibility")
}
