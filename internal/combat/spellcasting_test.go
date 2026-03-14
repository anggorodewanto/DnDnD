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
	assert.Contains(t, err.Error(), "no spell slots remaining")
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

// Edge case: AbilityScores.ScoreByName covers all branches
func TestAbilityScores_ScoreByName(t *testing.T) {
	scores := AbilityScores{Str: 10, Dex: 12, Con: 14, Int: 16, Wis: 18, Cha: 20}
	assert.Equal(t, 10, scores.ScoreByName("str"))
	assert.Equal(t, 12, scores.ScoreByName("dex"))
	assert.Equal(t, 14, scores.ScoreByName("con"))
	assert.Equal(t, 16, scores.ScoreByName("int"))
	assert.Equal(t, 18, scores.ScoreByName("wis"))
	assert.Equal(t, 20, scores.ScoreByName("cha"))
	assert.Equal(t, 0, scores.ScoreByName("unknown"))
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

// === Phase 60: Upcasting, Ritual, Cantrip Scaling ===

// Edge case: parseDiceExpr with no 'd' prefix
func TestParseDiceExpr(t *testing.T) {
	// normal case
	count, die := parseDiceExpr("8d6")
	assert.Equal(t, 8, count)
	assert.Equal(t, "d6", die)

	// with modifier suffix
	count, die = parseDiceExpr("1d8+mod")
	assert.Equal(t, 1, count)
	assert.Equal(t, "d8+mod", die)

	// no 'd' at all
	count, die = parseDiceExpr("hello")
	assert.Equal(t, 0, count)
	assert.Equal(t, "hello", die)

	// invalid count before d
	count, die = parseDiceExpr("xd6")
	assert.Equal(t, 1, count)
	assert.Equal(t, "d6", die)
}

// Edge case: Cast with upcast damage scaling (fireball with damage JSON)
func TestCast_UpcastDamageScaling(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	fireball := makeFireball()
	fireball.Damage = pqtype.NullRawMessage{
		RawMessage: []byte(`{"dice": "8d6", "type": "fire", "higher_level_dice": "1d6"}`),
		Valid:      true,
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return fireball, nil
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
		return refdata.Character{ID: arg.ID, SpellSlots: arg.SpellSlots}, nil
	}

	svc := NewService(store)

	// Upcast fireball to 5th level
	cmd := CastCommand{
		SpellID:  "fireball",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	// Need 5th level slots
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
		"4": {Current: 1, Max: 1},
		"5": {Current: 1, Max: 1},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}

	cmd.SlotLevel = 5
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "10d6", result.ScaledDamageDice)
	assert.Equal(t, "fire", result.DamageType)
	assert.Equal(t, 5, result.SlotUsed)
}

// Edge case: Cast with healing upcast
func TestCast_UpcastHealingScaling(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	// Make this a cleric for healing spells
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 8}})
	char.Classes = classesJSON
	scoresJSON, _ := json.Marshal(AbilityScores{
		Str: 10, Dex: 10, Con: 14, Int: 10, Wis: 18, Cha: 10,
	})
	char.AbilityScores = scoresJSON

	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6

	cureWounds := refdata.Spell{
		ID:             "cure-wounds",
		Name:           "Cure Wounds",
		Level:          1,
		CastingTime:    "1 action",
		RangeType:      "touch",
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
		Healing: pqtype.NullRawMessage{
			RawMessage: []byte(`{"dice": "1d8+mod", "higher_level_dice": "1d8"}`),
			Valid:      true,
		},
	}

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return cureWounds, nil
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

	// Upcast cure wounds to 3rd level
	cmd := CastCommand{
		SpellID:  "cure-wounds",
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		SlotLevel: 3,
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "3d8+mod", result.ScaledHealingDice)
	assert.Equal(t, 3, result.SlotUsed)
}

// Edge case: ritual casting with non-ritual class
func TestCast_RitualNonRitualClass(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	// Make this a sorcerer (no ritual casting)
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "sorcerer", Level: 8}})
	char.Classes = classesJSON
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return refdata.Spell{
			ID:             "detect-magic",
			Name:           "Detect Magic",
			Level:          1,
			CastingTime:    "1 action",
			RangeType:      "self",
			Ritual:         sql.NullBool{Bool: true, Valid: true},
			Concentration:  sql.NullBool{Bool: true, Valid: true},
			ResolutionMode: "auto",
		}, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "detect-magic",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		IsRitual:        true,
		EncounterStatus: "preparing",
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have the Ritual Casting feature")
}

// Edge case: FormatCastLog with ritual flag
func TestFormatCastLog_Ritual(t *testing.T) {
	result := CastResult{
		CasterName: "Gandalf",
		SpellName:  "Detect Magic",
		SpellLevel: 1,
		IsRitual:   true,
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "ritual")
	assert.NotContains(t, log, "slot")
}

// Edge case: FormatCastLog with upcast
func TestFormatCastLog_Upcast(t *testing.T) {
	result := CastResult{
		CasterName:      "Gandalf",
		SpellName:       "Fireball",
		SpellLevel:      3,
		SlotUsed:        5,
		SlotsRemaining:  0,
		ScaledDamageDice: "10d6",
		DamageType:      "fire",
	}
	log := FormatCastLog(result)
	assert.Contains(t, log, "5th-level slot")
	assert.Contains(t, log, "10d6 fire")
}

// TDD Cycle P60-2: CantripDiceMultiplier scales by character level
func TestCantripDiceMultiplier(t *testing.T) {
	tests := []struct {
		charLevel int
		want      int
	}{
		{1, 1}, {4, 1},
		{5, 2}, {10, 2},
		{11, 3}, {16, 3},
		{17, 4}, {20, 4},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("level_%d", tc.charLevel), func(t *testing.T) {
			assert.Equal(t, tc.want, CantripDiceMultiplier(tc.charLevel))
		})
	}
}

// TDD Cycle P60-3: ParseSpellDamage parses damage JSON
func TestParseSpellDamage(t *testing.T) {
	t.Run("simple damage", func(t *testing.T) {
		raw := []byte(`{"dice": "8d6", "type": "fire", "higher_level_dice": "1d6"}`)
		d, err := ParseSpellDamage(raw)
		require.NoError(t, err)
		assert.Equal(t, "8d6", d.Dice)
		assert.Equal(t, "fire", d.DamageType)
		assert.Equal(t, "1d6", d.HigherLevelDice)
		assert.False(t, d.CantripScaling)
	})

	t.Run("cantrip with scaling", func(t *testing.T) {
		raw := []byte(`{"dice": "1d8", "type": "fire", "cantrip_scaling": true}`)
		d, err := ParseSpellDamage(raw)
		require.NoError(t, err)
		assert.Equal(t, "1d8", d.Dice)
		assert.True(t, d.CantripScaling)
	})

	t.Run("multi-ray format", func(t *testing.T) {
		raw := []byte(`{"dice": "3x2d6", "type": "fire", "higher_level_dice": "1x2d6"}`)
		d, err := ParseSpellDamage(raw)
		require.NoError(t, err)
		assert.Equal(t, "3x2d6", d.Dice)
		assert.Equal(t, "1x2d6", d.HigherLevelDice)
	})

	t.Run("empty returns error", func(t *testing.T) {
		_, err := ParseSpellDamage(nil)
		assert.Error(t, err)
	})
}

// TDD Cycle P60-4: ScaleSpellDice computes the dice string for upcast/cantrip
func TestScaleSpellDice(t *testing.T) {
	tests := []struct {
		name       string
		baseDice   string
		higherDice string
		cantrip    bool
		spellLevel int
		slotLevel  int
		charLevel  int
		want       string
	}{
		{
			name:       "fireball at base level",
			baseDice:   "8d6",
			higherDice: "1d6",
			spellLevel: 3,
			slotLevel:  3,
			want:       "8d6",
		},
		{
			name:       "fireball upcast to 5th",
			baseDice:   "8d6",
			higherDice: "1d6",
			spellLevel: 3,
			slotLevel:  5,
			want:       "10d6",
		},
		{
			name:       "fireball upcast to 4th",
			baseDice:   "8d6",
			higherDice: "1d6",
			spellLevel: 3,
			slotLevel:  4,
			want:       "9d6",
		},
		{
			name:       "scorching ray upcast to 3rd (ray format)",
			baseDice:   "3x2d6",
			higherDice: "1x2d6",
			spellLevel: 2,
			slotLevel:  3,
			want:       "4x2d6",
		},
		{
			name:       "scorching ray upcast to 5th",
			baseDice:   "3x2d6",
			higherDice: "1x2d6",
			spellLevel: 2,
			slotLevel:  5,
			want:       "6x2d6",
		},
		{
			name:      "cantrip at level 1",
			baseDice:  "1d8",
			cantrip:   true,
			charLevel: 1,
			want:      "1d8",
		},
		{
			name:      "cantrip at level 5",
			baseDice:  "1d8",
			cantrip:   true,
			charLevel: 5,
			want:      "2d8",
		},
		{
			name:      "cantrip at level 11",
			baseDice:  "1d10",
			cantrip:   true,
			charLevel: 11,
			want:      "3d10",
		},
		{
			name:      "cantrip at level 17",
			baseDice:  "1d10",
			cantrip:   true,
			charLevel: 17,
			want:      "4d10",
		},
		{
			name:       "no higher dice, no upcast effect",
			baseDice:   "2d10",
			higherDice: "",
			spellLevel: 2,
			slotLevel:  4,
			want:       "2d10",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := SpellDamageInfo{
				Dice:           tc.baseDice,
				HigherLevelDice: tc.higherDice,
				CantripScaling: tc.cantrip,
			}
			result := ScaleSpellDice(d, tc.spellLevel, tc.slotLevel, tc.charLevel)
			assert.Equal(t, tc.want, result)
		})
	}
}

// TDD Cycle P60-5: ParseSpellHealing and ScaleHealingDice
func TestScaleHealingDice(t *testing.T) {
	t.Run("cure wounds at base level", func(t *testing.T) {
		raw := []byte(`{"dice": "1d8+mod", "higher_level_dice": "1d8"}`)
		h, err := ParseSpellHealing(raw)
		require.NoError(t, err)
		result := ScaleHealingDice(h, 1, 1)
		assert.Equal(t, "1d8+mod", result)
	})

	t.Run("cure wounds upcast to 3rd", func(t *testing.T) {
		raw := []byte(`{"dice": "1d8+mod", "higher_level_dice": "1d8"}`)
		h, err := ParseSpellHealing(raw)
		require.NoError(t, err)
		result := ScaleHealingDice(h, 1, 3)
		assert.Equal(t, "3d8+mod", result)
	})

	t.Run("cure wounds upcast to 2nd", func(t *testing.T) {
		raw := []byte(`{"dice": "1d8+mod", "higher_level_dice": "1d8"}`)
		h, err := ParseSpellHealing(raw)
		require.NoError(t, err)
		result := ScaleHealingDice(h, 1, 2)
		assert.Equal(t, "2d8+mod", result)
	})

	t.Run("no higher_level_dice", func(t *testing.T) {
		raw := []byte(`{"dice": "2d8+mod"}`)
		h, err := ParseSpellHealing(raw)
		require.NoError(t, err)
		result := ScaleHealingDice(h, 2, 5)
		assert.Equal(t, "2d8+mod", result)
	})

	t.Run("empty returns error", func(t *testing.T) {
		_, err := ParseSpellHealing(nil)
		assert.Error(t, err)
	})
}

// TDD Cycle P60-6: ValidateRitual checks ritual casting rules
func TestValidateRitual(t *testing.T) {
	tests := []struct {
		name            string
		spellRitual     bool
		encounterStatus string
		className       string
		wantErr         string
	}{
		{
			name:            "valid ritual: wizard out of combat",
			spellRitual:     true,
			encounterStatus: "preparing",
			className:       "wizard",
		},
		{
			name:            "valid ritual: cleric out of combat",
			spellRitual:     true,
			encounterStatus: "completed",
			className:       "cleric",
		},
		{
			name:            "spell is not ritual",
			spellRitual:     false,
			encounterStatus: "preparing",
			className:       "wizard",
			wantErr:         "cannot be cast as a ritual",
		},
		{
			name:            "in active combat",
			spellRitual:     true,
			encounterStatus: "active",
			className:       "wizard",
			wantErr:         "cannot cast rituals during active combat",
		},
		{
			name:            "class cannot ritual cast",
			spellRitual:     true,
			encounterStatus: "preparing",
			className:       "sorcerer",
			wantErr:         "does not have the Ritual Casting feature",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRitual(tc.spellRitual, tc.encounterStatus, tc.className)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TDD Cycle P60-7: HasRitualCasting checks class feature
func TestHasRitualCasting(t *testing.T) {
	assert.True(t, HasRitualCasting("wizard"))
	assert.True(t, HasRitualCasting("cleric"))
	assert.True(t, HasRitualCasting("druid"))
	assert.True(t, HasRitualCasting("bard"))
	assert.False(t, HasRitualCasting("sorcerer"))
	assert.False(t, HasRitualCasting("warlock"))
	assert.False(t, HasRitualCasting("paladin"))
	assert.False(t, HasRitualCasting("ranger"))
	assert.False(t, HasRitualCasting("barbarian"))
}

// TDD Cycle P60-8: Cast with explicit upcast slot
func TestCast_UpcastExplicitSlot(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()

	var savedSlots pqtype.NullRawMessage
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
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}

	// Upcast fireball (3rd level) using a 2nd level slot -> should fail
	cmd.SlotLevel = 2
	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slot level 2 is below spell level 3")

	// Upcast fireball to 2nd level slot which doesn't exist at level 2 for this spell
	// Actually, let's test valid upcast: 3rd level spell into 3rd level slot (not upcast)
	cmd.SlotLevel = 3
	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, 3, result.SlotUsed)

	// Verify slot 3 was deducted
	require.True(t, savedSlots.Valid)
	var slots map[string]SlotInfo
	require.NoError(t, json.Unmarshal(savedSlots.RawMessage, &slots))
	assert.Equal(t, 1, slots["3"].Current) // was 2, now 1
}

// TDD Cycle P60-9: Cast with auto-select slot skips depleted levels
func TestCast_UpcastAutoSelect(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	// Deplete 1st level slots, spell is level 1
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 0, Max: 4},
		"2": {Current: 3, Max: 3},
		"3": {Current: 2, Max: 2},
	})
	char.SpellSlots = pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true}

	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return makeMistyStep(), nil // level 2 bonus action spell
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
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		// SlotLevel 0 = auto
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, 2, result.SlotUsed) // Auto-selected 2nd level
}

// TDD Cycle P60-10: Cast ritual spell uses no slot
func TestCast_RitualNoSlot(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	slotUpdateCalled := false
	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return refdata.Spell{
			ID:             "detect-magic",
			Name:           "Detect Magic",
			Level:          1,
			CastingTime:    "1 action",
			RangeType:      "self",
			Ritual:         sql.NullBool{Bool: true, Valid: true},
			Concentration:  sql.NullBool{Bool: true, Valid: true},
			ResolutionMode: "auto",
		}, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
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
		SpellID:         "detect-magic",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		IsRitual:        true,
		EncounterStatus: "preparing",
	}

	result, err := svc.Cast(context.Background(), cmd, testRoller())
	require.NoError(t, err)
	assert.Equal(t, "Detect Magic", result.SpellName)
	assert.True(t, result.IsRitual)
	assert.False(t, slotUpdateCalled, "ritual should not consume slot")
	assert.Equal(t, 0, result.SlotUsed)
}

// TDD Cycle P60-11: Cast ritual in active combat fails
func TestCast_RitualInCombatFails(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	caster := makeSpellCaster(charID)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return refdata.Spell{
			ID:             "detect-magic",
			Name:           "Detect Magic",
			Level:          1,
			CastingTime:    "1 action",
			RangeType:      "self",
			Ritual:         sql.NullBool{Bool: true, Valid: true},
			Concentration:  sql.NullBool{Bool: true, Valid: true},
			ResolutionMode: "auto",
		}, nil
	}
	store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return caster, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:         "detect-magic",
		CasterID:        caster.ID,
		Turn:            refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
		IsRitual:        true,
		EncounterStatus: "active",
	}

	_, err := svc.Cast(context.Background(), cmd, testRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cast rituals during active combat")
}

// TDD Cycle P60-12: Cast cantrip with scaling populates CharacterLevel
func TestCast_CantripScaling(t *testing.T) {
	charID := uuid.New()
	char := makeWizardCharacter(charID)
	char.Level = 11 // Level 11 = 3 dice for cantrips
	caster := makeSpellCaster(charID)
	target := makeSpellTarget()
	target.PositionRow = 6

	store := defaultMockStore()
	firebolt := makeFireBolt()
	firebolt.Damage = pqtype.NullRawMessage{
		RawMessage: []byte(`{"dice": "1d10", "type": "fire", "cantrip_scaling": true}`),
		Valid:      true,
	}
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) {
		return firebolt, nil
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
	assert.Equal(t, "3d10", result.ScaledDamageDice)
	assert.Equal(t, "fire", result.DamageType)
}

// TDD Cycle P60-1: SelectSpellSlot picks lowest available slot >= spell level
func TestSelectSpellSlot(t *testing.T) {
	tests := []struct {
		name       string
		slots      map[int]SlotInfo
		spellLevel int
		slotLevel  int // 0 means "auto-select"
		wantSlot   int
		wantErr    string
	}{
		{
			name:       "auto-select lowest available",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}, 2: {Current: 3, Max: 3}, 3: {Current: 2, Max: 2}},
			spellLevel: 1,
			slotLevel:  0,
			wantSlot:   1,
		},
		{
			name:       "auto-select skips depleted",
			slots:      map[int]SlotInfo{1: {Current: 0, Max: 4}, 2: {Current: 3, Max: 3}, 3: {Current: 2, Max: 2}},
			spellLevel: 1,
			slotLevel:  0,
			wantSlot:   2,
		},
		{
			name:       "auto-select for higher level spell",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}, 2: {Current: 3, Max: 3}, 3: {Current: 2, Max: 2}},
			spellLevel: 3,
			slotLevel:  0,
			wantSlot:   3,
		},
		{
			name:       "explicit slot level",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}, 2: {Current: 3, Max: 3}, 3: {Current: 2, Max: 2}},
			spellLevel: 1,
			slotLevel:  3,
			wantSlot:   3,
		},
		{
			name:       "explicit slot level too low",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}, 2: {Current: 3, Max: 3}, 3: {Current: 2, Max: 2}},
			spellLevel: 3,
			slotLevel:  2,
			wantErr:    "slot level 2 is below spell level 3",
		},
		{
			name:       "explicit slot no remaining",
			slots:      map[int]SlotInfo{1: {Current: 2, Max: 4}, 2: {Current: 0, Max: 3}},
			spellLevel: 1,
			slotLevel:  2,
			wantErr:    "no 2nd-level spell slots remaining",
		},
		{
			name:       "auto-select no slots available at any level",
			slots:      map[int]SlotInfo{1: {Current: 0, Max: 4}, 2: {Current: 0, Max: 3}},
			spellLevel: 1,
			slotLevel:  0,
			wantErr:    "no spell slots remaining",
		},
		{
			name:       "cantrip returns 0",
			slots:      nil,
			spellLevel: 0,
			slotLevel:  0,
			wantSlot:   0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slot, err := SelectSpellSlot(tc.slots, tc.spellLevel, tc.slotLevel)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSlot, slot)
		})
	}
}
