package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: IsBonusActionSpell detects bonus action casting time
func TestIsBonusActionSpell(t *testing.T) {
	tests := []struct {
		name        string
		castingTime string
		want        bool
	}{
		{"bonus action", "1 bonus action", true},
		{"action", "1 action", false},
		{"reaction", "1 reaction", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spell := refdata.Spell{CastingTime: tc.castingTime}
			assert.Equal(t, tc.want, IsBonusActionSpell(spell))
		})
	}
}

// TDD Cycle 2: ValidateBonusActionSpellRestriction enforces both directions
func TestValidateBonusActionSpellRestriction(t *testing.T) {
	tests := []struct {
		name    string
		turn    refdata.Turn
		spell   refdata.Spell
		wantErr string
	}{
		{
			name:    "no restriction - action spell, no prior BA spell",
			turn:    refdata.Turn{},
			spell:   refdata.Spell{CastingTime: "1 action", Level: 1},
			wantErr: "",
		},
		{
			name:    "no restriction - bonus action spell, no prior action spell",
			turn:    refdata.Turn{},
			spell:   refdata.Spell{CastingTime: "1 bonus action", Level: 1},
			wantErr: "",
		},
		{
			name:    "forward: BA spell cast, action cantrip OK",
			turn:    refdata.Turn{BonusActionSpellCast: true},
			spell:   refdata.Spell{CastingTime: "1 action", Level: 0},
			wantErr: "",
		},
		{
			name:    "forward: BA spell cast, action leveled spell rejected",
			turn:    refdata.Turn{BonusActionSpellCast: true},
			spell:   refdata.Spell{CastingTime: "1 action", Level: 1},
			wantErr: "bonus action spell",
		},
		{
			name:    "reverse: action leveled spell cast, BA spell rejected",
			turn:    refdata.Turn{ActionSpellCast: true},
			spell:   refdata.Spell{CastingTime: "1 bonus action", Level: 1},
			wantErr: "leveled spell with your action",
		},
		{
			name:    "no restriction - cantrip action after nothing",
			turn:    refdata.Turn{},
			spell:   refdata.Spell{CastingTime: "1 action", Level: 0},
			wantErr: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBonusActionSpellRestriction(tc.turn, tc.spell)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TDD Cycle 3: ValidateSpellSlot checks slot availability
func TestValidateSpellSlot(t *testing.T) {
	tests := []struct {
		name       string
		slots      map[int]SlotInfo
		spellLevel int
		wantErr    string
	}{
		{
			name:       "cantrip needs no slot",
			slots:      nil,
			spellLevel: 0,
			wantErr:    "",
		},
		{
			name:       "has slot available",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}},
			spellLevel: 1,
			wantErr:    "",
		},
		{
			name:       "no slots remaining",
			slots:      map[int]SlotInfo{1: {Current: 0, Max: 4}},
			spellLevel: 1,
			wantErr:    "no 1st-level spell slots remaining",
		},
		{
			name:       "slot level not found",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}},
			spellLevel: 3,
			wantErr:    "no 3rd-level spell slots remaining",
		},
		{
			name:       "nil slots for leveled spell",
			slots:      nil,
			spellLevel: 1,
			wantErr:    "no 1st-level spell slots remaining",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpellSlot(tc.slots, tc.spellLevel)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TDD Cycle 4: DeductSpellSlot reduces current slot count
func TestDeductSpellSlot(t *testing.T) {
	t.Run("deducts from available slot", func(t *testing.T) {
		slots := map[int]SlotInfo{1: {Current: 3, Max: 4}, 2: {Current: 2, Max: 3}}
		result := DeductSpellSlot(slots, 1)
		assert.Equal(t, 2, result[1].Current)
		assert.Equal(t, 4, result[1].Max)
		// other slots unchanged
		assert.Equal(t, 2, result[2].Current)
	})

	t.Run("cantrip returns slots unchanged", func(t *testing.T) {
		slots := map[int]SlotInfo{1: {Current: 3, Max: 4}}
		result := DeductSpellSlot(slots, 0)
		assert.Equal(t, 3, result[1].Current)
	})
}

// TDD Cycle 5: ValidateSpellRange checks distance constraints
func TestValidateSpellRange(t *testing.T) {
	tests := []struct {
		name    string
		spell   refdata.Spell
		dist    int
		wantErr string
	}{
		{
			name:    "self spell always valid",
			spell:   refdata.Spell{RangeType: "self"},
			dist:    0,
			wantErr: "",
		},
		{
			name:    "touch within 5ft",
			spell:   refdata.Spell{RangeType: "touch"},
			dist:    5,
			wantErr: "",
		},
		{
			name:    "touch out of range",
			spell:   refdata.Spell{RangeType: "touch"},
			dist:    10,
			wantErr: "out of range",
		},
		{
			name:    "ranged within range",
			spell:   refdata.Spell{RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 120, Valid: true}},
			dist:    100,
			wantErr: "",
		},
		{
			name:    "ranged out of range",
			spell:   refdata.Spell{RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 30, Valid: true}},
			dist:    35,
			wantErr: "out of range",
		},
		{
			name:    "sight always in range",
			spell:   refdata.Spell{RangeType: "sight"},
			dist:    999,
			wantErr: "",
		},
		{
			name:    "unlimited always in range",
			spell:   refdata.Spell{RangeType: "unlimited"},
			dist:    999,
			wantErr: "",
		},
		{
			name:    "self radius always valid",
			spell:   refdata.Spell{RangeType: "self (radius)"},
			dist:    0,
			wantErr: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpellRange(tc.spell, tc.dist)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

// TDD Cycle 6: SpellAttackModifier calculates prof + ability mod
func TestSpellAttackModifier(t *testing.T) {
	// proficiency bonus 3, ability score 16 (mod +3) = 6
	assert.Equal(t, 6, SpellAttackModifier(3, 16))
	// proficiency bonus 2, ability score 10 (mod +0) = 2
	assert.Equal(t, 2, SpellAttackModifier(2, 10))
	// proficiency bonus 4, ability score 20 (mod +5) = 9
	assert.Equal(t, 9, SpellAttackModifier(4, 20))
}

// TDD Cycle 7: ResolveConcentration determines what happens to concentration
func TestResolveConcentration(t *testing.T) {
	t.Run("non-concentration spell, no current", func(t *testing.T) {
		spell := refdata.Spell{Concentration: sql.NullBool{Bool: false, Valid: true}}
		result := ResolveConcentration("", spell)
		assert.False(t, result.DroppedPrevious)
		assert.Equal(t, "", result.PreviousSpell)
		assert.Equal(t, "", result.NewConcentration)
	})

	t.Run("concentration spell, no current", func(t *testing.T) {
		spell := refdata.Spell{
			Name:          "Bless",
			Concentration: sql.NullBool{Bool: true, Valid: true},
		}
		result := ResolveConcentration("", spell)
		assert.False(t, result.DroppedPrevious)
		assert.Equal(t, "Bless", result.NewConcentration)
	})

	t.Run("concentration spell, replaces current", func(t *testing.T) {
		spell := refdata.Spell{
			Name:          "Hold Person",
			Concentration: sql.NullBool{Bool: true, Valid: true},
		}
		result := ResolveConcentration("Bless", spell)
		assert.True(t, result.DroppedPrevious)
		assert.Equal(t, "Bless", result.PreviousSpell)
		assert.Equal(t, "Hold Person", result.NewConcentration)
	})

	t.Run("non-concentration spell, keeps current", func(t *testing.T) {
		spell := refdata.Spell{Concentration: sql.NullBool{Bool: false, Valid: true}}
		result := ResolveConcentration("Bless", spell)
		assert.False(t, result.DroppedPrevious)
		assert.Equal(t, "Bless", result.NewConcentration)
	})
}

// TDD Cycle 8: SpellcastingAbility returns correct ability for class
func TestSpellcastingAbility(t *testing.T) {
	tests := []struct {
		name  string
		class string
		want  string
	}{
		{"wizard", "wizard", "int"},
		{"cleric", "cleric", "wis"},
		{"druid", "druid", "wis"},
		{"ranger", "ranger", "wis"},
		{"bard", "bard", "cha"},
		{"paladin", "paladin", "cha"},
		{"sorcerer", "sorcerer", "cha"},
		{"warlock", "warlock", "cha"},
		{"unknown", "barbarian", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SpellcastingAbilityForClass(tc.class))
		})
	}
}

// TDD Cycle 9: FormatCastLog produces correct combat log text
func TestFormatCastLog(t *testing.T) {
	t.Run("basic cast", func(t *testing.T) {
		result := CastResult{
			CasterName:    "Gandalf",
			SpellName:     "Fireball",
			SpellLevel:    3,
			IsBonusAction: false,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Gandalf")
		assert.Contains(t, log, "Fireball")
	})

	t.Run("bonus action cast", func(t *testing.T) {
		result := CastResult{
			CasterName:    "Gandalf",
			SpellName:     "Misty Step",
			SpellLevel:    2,
			IsBonusAction: true,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "bonus action")
	})

	t.Run("concentration dropped", func(t *testing.T) {
		result := CastResult{
			CasterName: "Gandalf",
			SpellName:  "Hold Person",
			SpellLevel: 2,
			Concentration: ConcentrationResult{
				DroppedPrevious:  true,
				PreviousSpell:    "Bless",
				NewConcentration: "Hold Person",
			},
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Bless")
		assert.Contains(t, log, "concentration")
	})

	t.Run("spell attack hit", func(t *testing.T) {
		result := CastResult{
			CasterName:  "Gandalf",
			SpellName:   "Fire Bolt",
			SpellLevel:  0,
			IsAttack:    true,
			AttackRoll:  18,
			AttackTotal: 24,
			TargetAC:    15,
			Hit:         true,
			TargetName:  "Goblin",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Fire Bolt")
		assert.Contains(t, log, "Hit")
	})

	t.Run("spell attack miss", func(t *testing.T) {
		result := CastResult{
			CasterName:  "Gandalf",
			SpellName:   "Fire Bolt",
			SpellLevel:  0,
			IsAttack:    true,
			AttackRoll:  5,
			AttackTotal: 11,
			TargetAC:    15,
			Hit:         false,
			TargetName:  "Goblin",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Miss")
	})

	t.Run("save-based spell", func(t *testing.T) {
		result := CastResult{
			CasterName:  "Gandalf",
			SpellName:   "Fireball",
			SpellLevel:  3,
			SaveDC:      15,
			SaveAbility: "dex",
			TargetName:  "Goblin",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "DC 15")
		assert.Contains(t, log, "DEX")
	})

	t.Run("dm_required resolution", func(t *testing.T) {
		result := CastResult{
			CasterName:     "Gandalf",
			SpellName:      "Polymorph",
			SpellLevel:     4,
			ResolutionMode: "dm_required",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "DM")
	})

	t.Run("cantrip shows no slot usage", func(t *testing.T) {
		result := CastResult{
			CasterName: "Gandalf",
			SpellName:  "Fire Bolt",
			SpellLevel: 0,
		}
		log := FormatCastLog(result)
		assert.NotContains(t, log, "slot")
	})

	t.Run("leveled spell shows slot usage", func(t *testing.T) {
		result := CastResult{
			CasterName:     "Gandalf",
			SpellName:      "Shield",
			SpellLevel:     1,
			SlotUsed:       1,
			SlotsRemaining: 3,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "slot")
	})
}

// --- Integration tests for Cast service method ---

// testRoller creates a deterministic roller for tests (always rolls 10 on a d20).
func testRoller() *dice.Roller {
	return dice.NewRoller(func(n int) int { return 10 })
}

// helper to make a wizard character with spell slots
func makeWizardCharacter(id uuid.UUID) refdata.Character {
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
	})
	scoresJSON, _ := json.Marshal(AbilityScores{
		Str: 8, Dex: 14, Con: 12, Int: 18, Wis: 10, Cha: 10,
	})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "wizard", Level: 8}})
	return refdata.Character{
		ID:               id,
		Name:             "Gandalf",
		ProficiencyBonus: 3,
		Classes:          classesJSON,
		AbilityScores:    scoresJSON,
		SpellSlots:       pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		Level:            8,
	}
}

func makeFireball() refdata.Spell {
	return refdata.Spell{
		ID:             "fireball",
		Name:           "Fireball",
		Level:          3,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 150, Valid: true},
		SaveAbility:    sql.NullString{String: "dex", Valid: true},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}
}

func makeFireBolt() refdata.Spell {
	return refdata.Spell{
		ID:             "fire-bolt",
		Name:           "Fire Bolt",
		Level:          0,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 120, Valid: true},
		AttackType:     sql.NullString{String: "ranged", Valid: true},
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}
}

func makeMistyStep() refdata.Spell {
	return refdata.Spell{
		ID:             "misty-step",
		Name:           "Misty Step",
		Level:          2,
		CastingTime:    "1 bonus action",
		RangeType:      "self",
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}
}

func makeHoldPerson() refdata.Spell {
	return refdata.Spell{
		ID:             "hold-person",
		Name:           "Hold Person",
		Level:          2,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 60, Valid: true},
		SaveAbility:    sql.NullString{String: "wis", Valid: true},
		Concentration:  sql.NullBool{Bool: true, Valid: true},
		ResolutionMode: "auto",
	}
}

func makeSpellCaster(charID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Gandalf",
		PositionCol: "E",
		PositionRow: 5,
		IsAlive:     true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func makeSpellTarget() refdata.Combatant {
	return refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Goblin",
		PositionCol: "E",
		PositionRow: 8,
		Ac:          13,
		IsAlive:     true,
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// TDD Cycle 10: Cast service method - slot deduction for leveled spell
func TestCast_SlotDeduction(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	turnID := uuid.New()

	var savedSlots pqtype.NullRawMessage
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
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		savedSlots = arg.SpellSlots
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: turnID, CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Fireball", result.SpellName)
	assert.Equal(t, 3, result.SpellLevel)
	assert.Equal(t, 3, result.SlotUsed)

	// Verify slot was deducted
	require.True(t, savedSlots.Valid)
	var slots map[string]SlotInfo
	require.NoError(t, json.Unmarshal(savedSlots.RawMessage, &slots))
	assert.Equal(t, 1, slots["3"].Current) // was 2, now 1
}

// TDD Cycle 11: Cast rejects when no spell slots
func TestCast_NoSlotsRemaining(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	// deplete 3rd-level slots
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 0, Max: 2},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
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
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no 3rd-level spell slots remaining")
}

// TDD Cycle 12: Cast enforces range
func TestCast_RangeEnforcement(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	// Put target far away (row 40 = ~175ft from row 5)
	target := makeSpellTarget()
	target.PositionRow = 40

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeFireball(), nil // range 150ft
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
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// TDD Cycle 13: Cast with spell attack roll
func TestCast_SpellAttackRoll(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6 // close

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeFireBolt(), nil
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
		SpellID:  "fire-bolt",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.IsAttack)
	assert.True(t, result.AttackRoll > 0)
	// Spell attack mod should be prof(3) + INT mod(+4) = 7
	assert.Equal(t, result.AttackRoll+7, result.AttackTotal)
}

// TDD Cycle 14: Cast bonus action spell uses bonus action resource
func TestCast_BonusActionSpell(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	var savedTurn refdata.UpdateTurnActionsParams
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStep(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		savedTurn = arg
		return refdata.Turn{
			ID: arg.ID, BonusActionUsed: arg.BonusActionUsed,
			BonusActionSpellCast: arg.BonusActionSpellCast,
		}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "misty-step",
		CasterID: caster.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.IsBonusAction)
	assert.True(t, savedTurn.BonusActionUsed)
	assert.True(t, savedTurn.BonusActionSpellCast)
}

// TDD Cycle 15: Cast bonus action restriction - forward direction
func TestCast_BonusActionRestrictionForward(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
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
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, BonusActionSpellCast: true},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action spell")
}

// TDD Cycle 16: Cast bonus action restriction - reverse direction
func TestCast_BonusActionRestrictionReverse(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStep(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "misty-step",
		CasterID: caster.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, ActionSpellCast: true},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leveled spell with your action")
}

// TDD Cycle 17: Cast concentration tracking - new drops old
func TestCast_ConcentrationDropsOld(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeHoldPerson(), nil
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
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:              "hold-person",
		CasterID:             caster.ID,
		TargetID:             target.ID,
		Turn:                 refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		CurrentConcentration: "Bless",
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, result.Concentration.DroppedPrevious)
	assert.Equal(t, "Bless", result.Concentration.PreviousSpell)
	assert.Equal(t, "Hold Person", result.Concentration.NewConcentration)
}

// TDD Cycle 18: Cast with save-based spell populates save DC
func TestCast_SaveDC(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
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
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	// DC = 8 + prof(3) + INT mod(+4) = 15
	assert.Equal(t, 15, result.SaveDC)
	assert.Equal(t, "dex", result.SaveAbility)
}

// TDD Cycle 19: Cast cantrip does not consume slot
func TestCast_CantripNoSlot(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6

	slotUpdateCalled := false
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeFireBolt(), nil
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
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		slotUpdateCalled = true
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "fire-bolt",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, 0, result.SpellLevel)
	assert.False(t, slotUpdateCalled)
}

// TDD Cycle 20: Cast self spell needs no target
func TestCast_SelfSpellNoTarget(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStep(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}
	store.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "misty-step",
		CasterID: caster.ID,
		// No TargetID
		Turn: refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Misty Step", result.SpellName)
}

// TDD Cycle 21: Cast dm_required spell marks resolution mode
func TestCast_DMRequired(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	polymorph := refdata.Spell{
		ID:             "polymorph",
		Name:           "Polymorph",
		Level:          4,
		CastingTime:    "1 action",
		RangeType:      "ranged",
		RangeFt:        sql.NullInt32{Int32: 60, Valid: true},
		ResolutionMode: "dm_required",
		Concentration:  sql.NullBool{Bool: true, Valid: true},
	}

	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
		"4": {Current: 1, Max: 1},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return polymorph, nil
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
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "polymorph",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "dm_required", result.ResolutionMode)
}

// TDD Cycle 22: Cast action spell sets ActionSpellCast flag
func TestCast_ActionSpellCastFlag(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	var savedTurn refdata.UpdateTurnActionsParams
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
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
		savedTurn = arg
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed, ActionSpellCast: arg.ActionSpellCast}, nil
	}
	store.updateCharacterSpellSlotsFn = func(_ context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.True(t, savedTurn.ActionUsed)
	assert.True(t, savedTurn.ActionSpellCast) // leveled action spell
}

// TDD Cycle 23: Cast action resource validation - action already used
func TestCast_ActionAlreadyUsed(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
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
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, ActionUsed: true},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

// Edge case: abilityScoreByName covers all branches
func TestAbilityScoreByName(t *testing.T) {
	scores := AbilityScores{Str: 10, Dex: 12, Con: 14, Int: 16, Wis: 18, Cha: 20}
	assert.Equal(t, 10, abilityScoreByName(scores, "str"))
	assert.Equal(t, 12, abilityScoreByName(scores, "dex"))
	assert.Equal(t, 14, abilityScoreByName(scores, "con"))
	assert.Equal(t, 16, abilityScoreByName(scores, "int"))
	assert.Equal(t, 18, abilityScoreByName(scores, "wis"))
	assert.Equal(t, 20, abilityScoreByName(scores, "cha"))
	assert.Equal(t, 0, abilityScoreByName(scores, "unknown"))
}

// Edge case: resolveSpellcastingAbilityScore with no spellcasting class
func TestResolveSpellcastingAbilityScore_NoSpellcaster(t *testing.T) {
	classes := []CharacterClass{{Class: "barbarian", Level: 5}}
	scores := AbilityScores{Str: 18, Int: 8}
	assert.Equal(t, 0, resolveSpellcastingAbilityScore(classes, scores))
}

// Edge case: parseIntKeyedSlots with non-numeric keys
func TestParseIntKeyedSlots_NonNumericKeys(t *testing.T) {
	raw := []byte(`{"abc": {"current": 2, "max": 3}, "1": {"current": 1, "max": 2}}`)
	slots, err := parseIntKeyedSlots(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, len(slots))
	assert.Equal(t, SlotInfo{Current: 1, Max: 2}, slots[1])
}

// Edge case: ValidateSpellRange with ranged but no RangeFt set
func TestValidateSpellRange_RangedNoRangeFt(t *testing.T) {
	spell := refdata.Spell{RangeType: "ranged"}
	err := ValidateSpellRange(spell, 999)
	assert.NoError(t, err) // no range specified, so no enforcement
}

// Edge case: Cast bonus action already used
func TestCast_BonusActionAlreadyUsed(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStep(), nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "misty-step",
		CasterID: caster.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID, BonusActionUsed: true},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource already spent")
}

// Edge case: Cast spell not found
func TestCast_SpellNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return refdata.Spell{}, fmt.Errorf("spell %q not found", id)
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  "nonexistent",
		CasterID: uuid.New(),
		Turn:     refdata.Turn{ID: uuid.New()},
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "looking up spell")
}

// Edge case: intToStringKeyedSlots round-trip
func TestIntToStringKeyedSlots(t *testing.T) {
	slots := map[int]SlotInfo{1: {Current: 3, Max: 4}, 2: {Current: 1, Max: 3}}
	strSlots := intToStringKeyedSlots(slots)
	assert.Equal(t, SlotInfo{Current: 3, Max: 4}, strSlots["1"])
	assert.Equal(t, SlotInfo{Current: 1, Max: 3}, strSlots["2"])
}
