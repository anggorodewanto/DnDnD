package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// helper: make a fireball with AoE for metamagic tests
func makeFireballWithAoE() refdata.Spell {
	s := makeFireball()
	s.AreaOfEffect = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
		Valid:      true,
	}
	s.Damage = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"dice":"8d6","type":"fire"}`),
		Valid:      true,
	}
	s.Duration = "Instantaneous"
	s.Components = []string{"V", "S", "M"}
	return s
}

// helper: a single-target ranged spell with a save
func makeSingleTargetSaveSpell() refdata.Spell {
	return refdata.Spell{
		ID:          "hold-person",
		Name:        "Hold Person",
		Level:       2,
		CastingTime: "1 action",
		RangeType:   "ranged",
		RangeFt:     sql.NullInt32{Int32: 60, Valid: true},
		SaveAbility: sql.NullString{String: "wis", Valid: true},
		Duration:    "Up to 1 minute",
		Components:  []string{"V", "S", "M"},
		Concentration: sql.NullBool{Bool: true, Valid: true},
		ResolutionMode: "auto",
	}
}

// helper: a touch spell
func makeCureWounds() refdata.Spell {
	return refdata.Spell{
		ID:          "cure-wounds",
		Name:        "Cure Wounds",
		Level:       1,
		CastingTime: "1 action",
		RangeType:   "touch",
		Duration:    "Instantaneous",
		Components:  []string{"V", "S"},
		Healing: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"dice":"1d8+mod","higher_level_dice":"1d8"}`),
			Valid:      true,
		},
		ResolutionMode: "auto",
	}
}

// helper: a self-range spell
func makeShield() refdata.Spell {
	return refdata.Spell{
		ID:          "shield",
		Name:        "Shield",
		Level:       1,
		CastingTime: "1 reaction",
		RangeType:   "self",
		Duration:    "1 round",
		Components:  []string{"V", "S"},
		ResolutionMode: "auto",
	}
}

// TDD Cycle 1: Careful Spell requires AoE and a save
func TestValidateMetamagicOptions_CarefulSpell(t *testing.T) {
	t.Run("valid: spell has AoE and save", func(t *testing.T) {
		spell := makeFireballWithAoE()
		err := ValidateMetamagicOptions([]string{"careful"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: spell has no AoE", func(t *testing.T) {
		spell := makeFireball() // no AoE
		err := ValidateMetamagicOptions([]string{"careful"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "area of effect")
	})

	t.Run("rejected: spell has no save", func(t *testing.T) {
		spell := makeFireBolt() // no save
		spell.AreaOfEffect = pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"shape":"sphere","radius_ft":20}`),
			Valid:      true,
		}
		err := ValidateMetamagicOptions([]string{"careful"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "saving throw")
	})
}

// TDD Cycle 2: Distant Spell requires range > 0 or touch
func TestValidateMetamagicOptions_DistantSpell(t *testing.T) {
	t.Run("valid: ranged spell", func(t *testing.T) {
		spell := makeFireball()
		err := ValidateMetamagicOptions([]string{"distant"}, spell)
		assert.NoError(t, err)
	})

	t.Run("valid: touch spell", func(t *testing.T) {
		spell := makeCureWounds()
		err := ValidateMetamagicOptions([]string{"distant"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: self-range spell", func(t *testing.T) {
		spell := makeShield()
		err := ValidateMetamagicOptions([]string{"distant"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "self")
	})
}

// TDD Cycle 3: Empowered Spell requires spell to deal damage
func TestValidateMetamagicOptions_EmpoweredSpell(t *testing.T) {
	t.Run("valid: spell deals damage", func(t *testing.T) {
		spell := makeFireballWithAoE()
		err := ValidateMetamagicOptions([]string{"empowered"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: spell deals no damage", func(t *testing.T) {
		spell := makeCureWounds() // healing, no damage
		err := ValidateMetamagicOptions([]string{"empowered"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "damage")
	})
}

// TDD Cycle 4: Extended Spell requires duration >= 1 minute
func TestValidateMetamagicOptions_ExtendedSpell(t *testing.T) {
	t.Run("valid: 1 minute duration", func(t *testing.T) {
		spell := makeSingleTargetSaveSpell() // "Up to 1 minute"
		err := ValidateMetamagicOptions([]string{"extended"}, spell)
		assert.NoError(t, err)
	})

	t.Run("valid: 1 hour duration", func(t *testing.T) {
		spell := makeSingleTargetSaveSpell()
		spell.Duration = "1 hour"
		err := ValidateMetamagicOptions([]string{"extended"}, spell)
		assert.NoError(t, err)
	})

	t.Run("valid: 8 hours duration", func(t *testing.T) {
		spell := makeSingleTargetSaveSpell()
		spell.Duration = "8 hours"
		err := ValidateMetamagicOptions([]string{"extended"}, spell)
		assert.NoError(t, err)
	})

	t.Run("valid: 10 minutes", func(t *testing.T) {
		spell := makeSingleTargetSaveSpell()
		spell.Duration = "10 minutes"
		err := ValidateMetamagicOptions([]string{"extended"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: instantaneous", func(t *testing.T) {
		spell := makeFireball()
		spell.Duration = "Instantaneous"
		err := ValidateMetamagicOptions([]string{"extended"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duration")
	})

	t.Run("rejected: 1 round", func(t *testing.T) {
		spell := makeShield()
		err := ValidateMetamagicOptions([]string{"extended"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duration")
	})
}

// TDD Cycle 5: Heightened Spell requires a save
func TestValidateMetamagicOptions_HeightenedSpell(t *testing.T) {
	t.Run("valid: spell requires a save", func(t *testing.T) {
		spell := makeFireball()
		err := ValidateMetamagicOptions([]string{"heightened"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: spell has no save", func(t *testing.T) {
		spell := makeFireBolt()
		err := ValidateMetamagicOptions([]string{"heightened"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "saving throw")
	})
}

// TDD Cycle 6: Quickened Spell requires casting time of "1 action"
func TestValidateMetamagicOptions_QuickenedSpell(t *testing.T) {
	t.Run("valid: 1 action spell", func(t *testing.T) {
		spell := makeFireball()
		err := ValidateMetamagicOptions([]string{"quickened"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: bonus action spell", func(t *testing.T) {
		spell := makeMistyStep()
		err := ValidateMetamagicOptions([]string{"quickened"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 action")
	})

	t.Run("rejected: reaction spell", func(t *testing.T) {
		spell := makeShield()
		err := ValidateMetamagicOptions([]string{"quickened"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 action")
	})
}

// TDD Cycle 7: Subtle Spell has no restrictions
func TestValidateMetamagicOptions_SubtleSpell(t *testing.T) {
	t.Run("valid: any spell", func(t *testing.T) {
		spell := makeFireball()
		err := ValidateMetamagicOptions([]string{"subtle"}, spell)
		assert.NoError(t, err)
	})

	t.Run("valid: spell with no components", func(t *testing.T) {
		spell := makeShield()
		spell.Components = nil
		err := ValidateMetamagicOptions([]string{"subtle"}, spell)
		assert.NoError(t, err)
	})
}

// TDD Cycle 8: Twinned Spell requires single-target, not self, not AoE
func TestValidateMetamagicOptions_TwinnedSpell(t *testing.T) {
	t.Run("valid: single-target ranged spell", func(t *testing.T) {
		spell := makeSingleTargetSaveSpell()
		err := ValidateMetamagicOptions([]string{"twinned"}, spell)
		assert.NoError(t, err)
	})

	t.Run("rejected: self-range spell", func(t *testing.T) {
		spell := makeShield()
		err := ValidateMetamagicOptions([]string{"twinned"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "self")
	})

	t.Run("rejected: AoE spell", func(t *testing.T) {
		spell := makeFireballWithAoE()
		err := ValidateMetamagicOptions([]string{"twinned"}, spell)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "area of effect")
	})
}

// TDD Cycle 9: CastResult has new metamagic fields
func TestCastResult_MetamagicFields(t *testing.T) {
	r := CastResult{
		CarefulSpellCreatures: 3,
		DistantRange:          "300 ft.",
		IsEmpowered:          true,
		EmpoweredRerolls:     4,
		ExtendedDuration:     "2 hours",
		IsHeightened:         true,
		IsSubtle:             true,
		TwinTargetName:       "Orc",
	}
	assert.Equal(t, 3, r.CarefulSpellCreatures)
	assert.Equal(t, "300 ft.", r.DistantRange)
	assert.True(t, r.IsEmpowered)
	assert.Equal(t, 4, r.EmpoweredRerolls)
	assert.Equal(t, "2 hours", r.ExtendedDuration)
	assert.True(t, r.IsHeightened)
	assert.True(t, r.IsSubtle)
	assert.Equal(t, "Orc", r.TwinTargetName)
}

// TDD Cycle 10: ApplyDistantSpell doubles range or converts touch to 30ft
func TestApplyDistantSpell(t *testing.T) {
	t.Run("doubles ranged spell range", func(t *testing.T) {
		spell := makeFireball() // 150ft ranged
		desc := ApplyDistantSpell(spell)
		assert.Equal(t, "300 ft.", desc)
	})

	t.Run("converts touch to 30ft", func(t *testing.T) {
		spell := makeCureWounds()
		desc := ApplyDistantSpell(spell)
		assert.Equal(t, "30 ft.", desc)
	})

	t.Run("no range data returns empty", func(t *testing.T) {
		spell := makeShield() // self
		desc := ApplyDistantSpell(spell)
		assert.Equal(t, "", desc)
	})
}

// TDD Cycle 11: ApplyExtendedSpell doubles duration (max 24 hours)
func TestApplyExtendedSpell(t *testing.T) {
	t.Run("1 minute becomes 2 minutes", func(t *testing.T) {
		assert.Equal(t, "2 minutes", ApplyExtendedSpell("1 minute"))
	})

	t.Run("up to 1 minute becomes up to 2 minutes", func(t *testing.T) {
		assert.Equal(t, "Up to 2 minutes", ApplyExtendedSpell("Up to 1 minute"))
	})

	t.Run("10 minutes becomes 20 minutes", func(t *testing.T) {
		assert.Equal(t, "20 minutes", ApplyExtendedSpell("10 minutes"))
	})

	t.Run("1 hour becomes 2 hours", func(t *testing.T) {
		assert.Equal(t, "2 hours", ApplyExtendedSpell("1 hour"))
	})

	t.Run("8 hours becomes 16 hours", func(t *testing.T) {
		assert.Equal(t, "16 hours", ApplyExtendedSpell("8 hours"))
	})

	t.Run("24 hours stays 24 hours (max)", func(t *testing.T) {
		assert.Equal(t, "24 hours", ApplyExtendedSpell("24 hours"))
	})

	t.Run("instantaneous returns empty", func(t *testing.T) {
		assert.Equal(t, "", ApplyExtendedSpell("Instantaneous"))
	})
}

// TDD Cycle 12: CarefulSpellCreatureCount returns CHA mod (min 1)
func TestCarefulSpellCreatureCount(t *testing.T) {
	assert.Equal(t, 4, CarefulSpellCreatureCount(18)) // mod = 4
	assert.Equal(t, 1, CarefulSpellCreatureCount(10)) // mod = 0, min 1
	assert.Equal(t, 1, CarefulSpellCreatureCount(8))  // mod = -1, min 1
	assert.Equal(t, 5, CarefulSpellCreatureCount(20)) // mod = 5
}

// TDD Cycle 13: EmpoweredRerollCount returns CHA mod (min 1)
func TestEmpoweredRerollCount(t *testing.T) {
	assert.Equal(t, 4, EmpoweredRerollCount(18)) // mod = 4
	assert.Equal(t, 1, EmpoweredRerollCount(10)) // mod = 0, min 1
	assert.Equal(t, 1, EmpoweredRerollCount(8))  // mod = -1, min 1
}

// TDD Cycle 14: Empty metamagic list passes validation
func TestValidateMetamagicOptions_Empty(t *testing.T) {
	err := ValidateMetamagicOptions(nil, makeFireball())
	assert.NoError(t, err)

	err = ValidateMetamagicOptions([]string{}, makeFireball())
	assert.NoError(t, err)
}

// TDD Cycle 15: Case-insensitive validation
func TestValidateMetamagicOptions_CaseInsensitive(t *testing.T) {
	spell := makeFireball()
	err := ValidateMetamagicOptions([]string{"Heightened"}, spell)
	assert.NoError(t, err)

	err = ValidateMetamagicOptions([]string{"QUICKENED"}, spell)
	assert.NoError(t, err)
}

// TDD Cycle 16: HasDurationAtLeastOneMinute helper
func TestHasDurationAtLeastOneMinute(t *testing.T) {
	assert.True(t, hasDurationAtLeastOneMinute("1 minute"))
	assert.True(t, hasDurationAtLeastOneMinute("Up to 1 minute"))
	assert.True(t, hasDurationAtLeastOneMinute("10 minutes"))
	assert.True(t, hasDurationAtLeastOneMinute("1 hour"))
	assert.True(t, hasDurationAtLeastOneMinute("8 hours"))
	assert.True(t, hasDurationAtLeastOneMinute("24 hours"))
	assert.True(t, hasDurationAtLeastOneMinute("7 days"))
	assert.False(t, hasDurationAtLeastOneMinute("Instantaneous"))
	assert.False(t, hasDurationAtLeastOneMinute("1 round"))
	assert.False(t, hasDurationAtLeastOneMinute(""))
}

// TDD Cycle 17: Empowered can combine with Careful in validation
func TestValidateMetamagicOptions_EmpoweredCombo(t *testing.T) {
	spell := makeFireballWithAoE()
	err := ValidateMetamagicOptions([]string{"empowered", "careful"}, spell)
	assert.NoError(t, err)
}

// Edge case: unknown option passes through (caught by ValidateMetamagic)
func TestValidateMetamagicOptions_UnknownPassesThrough(t *testing.T) {
	err := ValidateMetamagicOptions([]string{"unknown"}, makeFireball())
	assert.NoError(t, err) // ValidateMetamagicOptions doesn't reject unknown options — ValidateMetamagic does
}

// Edge case: hasDurationAtLeastOneMinute with zero amount
func TestHasDurationAtLeastOneMinute_ZeroAmount(t *testing.T) {
	assert.False(t, hasDurationAtLeastOneMinute("0 minutes"))
}

// Edge case: ApplyExtendedSpell with days
func TestApplyExtendedSpell_Days(t *testing.T) {
	// 1 day = 24 hours, doubled = 48 hours > 24 hours, capped at 24 hours
	assert.Equal(t, "24 hours", ApplyExtendedSpell("1 day"))
}

// Edge case: distant spell on ranged spell with no range value
func TestValidateMetamagicOptions_DistantNoRange(t *testing.T) {
	spell := refdata.Spell{RangeType: "ranged"} // no RangeFt
	err := ValidateMetamagicOptions([]string{"distant"}, spell)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self")
}

// TDD Cycle 18: Twinned rejects touch spells that target only self
// (touch spells are fine for twinning since they target another creature)
func TestValidateMetamagicOptions_TwinnedTouchSpell(t *testing.T) {
	spell := makeCureWounds() // touch, single target
	err := ValidateMetamagicOptions([]string{"twinned"}, spell)
	assert.NoError(t, err)
}

// metamagicTestFixture sets up the common sorcerer cast test infrastructure.
// Returns a service, caster ID, target ID, and a function to customize the store's getSpellFn.
type metamagicTestFixture struct {
	svc      *Service
	store    *mockStore
	casterID uuid.UUID
	targetID uuid.UUID
}

// newMetamagicTestFixture creates a standard sorcerer casting test setup with
// the given spell provider. Includes all store mocks needed for a successful cast.
func newMetamagicTestFixture(spell refdata.Spell) metamagicTestFixture {
	charID := uuid.New()
	char := makeSorcererCharacter(charID, 5, 5) // CHA 18 => mod 4
	caster := makeSorcererCombatant(charID)
	target := makeSpellTarget()

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, id string) (refdata.Spell, error) {
		return spell, nil
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
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}

	return metamagicTestFixture{
		svc:      NewService(store),
		store:    store,
		casterID: caster.ID,
		targetID: target.ID,
	}
}

// castWithMetamagic is a convenience method that casts with the given metamagic options.
func (f metamagicTestFixture) castWithMetamagic(metamagic []string) (CastResult, error) {
	return f.svc.Cast(context.Background(), CastCommand{
		SpellID:   "test-spell",
		CasterID:  f.casterID,
		TargetID:  f.targetID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: f.casterID},
		Metamagic: metamagic,
	}, testRoller())
}

// TDD Cycle 19: Cast with --careful on AoE spell populates CarefulSpellCreatures
func TestCast_CarefulSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeFireballWithAoE())
	result, err := f.castWithMetamagic([]string{"careful"})

	require.NoError(t, err)
	assert.Equal(t, 4, result.CarefulSpellCreatures) // CHA mod = 4
}

// TDD Cycle 20: Cast with --careful on non-AoE spell is rejected
func TestCast_CarefulSpell_RejectsNonAoE(t *testing.T) {
	f := newMetamagicTestFixture(makeFireball()) // no AoE
	_, err := f.castWithMetamagic([]string{"careful"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "area of effect")
}

// TDD Cycle 21: Cast with --distant doubles range
func TestCast_DistantSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeFireball()) // 150ft ranged
	result, err := f.castWithMetamagic([]string{"distant"})

	require.NoError(t, err)
	assert.Equal(t, "300 ft.", result.DistantRange)
}

// TDD Cycle 22: Cast with --empowered sets flags
func TestCast_EmpoweredSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeFireballWithAoE()) // has damage
	result, err := f.castWithMetamagic([]string{"empowered"})

	require.NoError(t, err)
	assert.True(t, result.IsEmpowered)
	assert.Equal(t, 4, result.EmpoweredRerolls) // CHA 18, mod 4
}

// TDD Cycle 23: Cast with --extended doubles duration
func TestCast_ExtendedSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeSingleTargetSaveSpell()) // "Up to 1 minute"
	result, err := f.castWithMetamagic([]string{"extended"})

	require.NoError(t, err)
	assert.Equal(t, "Up to 2 minutes", result.ExtendedDuration)
}

// TDD Cycle 24: Cast with --heightened sets flag
func TestCast_HeightenedSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeFireball()) // has save
	result, err := f.castWithMetamagic([]string{"heightened"})

	require.NoError(t, err)
	assert.True(t, result.IsHeightened)
}

// TDD Cycle 25: Cast with --subtle sets flag
func TestCast_SubtleSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeFireball())
	result, err := f.castWithMetamagic([]string{"subtle"})

	require.NoError(t, err)
	assert.True(t, result.IsSubtle)
}

// TDD Cycle 26: Empowered + Careful combo works
func TestCast_EmpoweredPlusCareful_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeFireballWithAoE())
	result, err := f.castWithMetamagic([]string{"empowered", "careful"})

	require.NoError(t, err)
	assert.True(t, result.IsEmpowered)
	assert.Equal(t, 4, result.EmpoweredRerolls)
	assert.Equal(t, 4, result.CarefulSpellCreatures)
	assert.Equal(t, 2, result.MetamagicCost) // 1 + 1
}

// TDD Cycle 27: Twinned Spell integration - resolves second target
func TestCast_TwinnedSpell_Integration(t *testing.T) {
	f := newMetamagicTestFixture(makeSingleTargetSaveSpell())
	twinTarget := refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Orc",
		PositionCol: "E",
		PositionRow: 6,
		Ac:          15,
		IsAlive:     true,
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}

	// Add twin target to combatant lookup
	originalGetCombatantFn := f.store.getCombatantFn
	f.store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		if id == twinTarget.ID {
			return twinTarget, nil
		}
		return originalGetCombatantFn(ctx, id)
	}

	result, err := f.svc.Cast(context.Background(), CastCommand{
		SpellID:      "hold-person",
		CasterID:     f.casterID,
		TargetID:     f.targetID,
		TwinTargetID: twinTarget.ID,
		Turn:         refdata.Turn{ID: uuid.New(), CombatantID: f.casterID},
		Metamagic:    []string{"twinned"},
	}, testRoller())

	require.NoError(t, err)
	assert.Equal(t, "Orc", result.TwinTargetName)
	assert.Equal(t, twinTarget.ID.String(), result.TwinTargetID)
	assert.Equal(t, 2, result.MetamagicCost) // spell level 2 = 2 SP
}

// TDD Cycle 28: Twinned Spell rejects self-range spell
func TestCast_TwinnedSpell_RejectsSelfRange(t *testing.T) {
	f := newMetamagicTestFixture(makeShield()) // self-range
	// Override combatant lookup to only return caster (no target needed)
	f.store.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          f.casterID,
			DisplayName: "Sorcerer",
			CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			PositionCol: "C",
			PositionRow: 3,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}

	_, err := f.svc.Cast(context.Background(), CastCommand{
		SpellID:   "shield",
		CasterID:  f.casterID,
		Turn:      refdata.Turn{ID: uuid.New(), CombatantID: f.casterID},
		Metamagic: []string{"twinned"},
	}, testRoller())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "self")
}

// TDD Cycle 30: FormatCastLog includes metamagic info
func TestFormatCastLog_MetamagicEffects(t *testing.T) {
	t.Run("careful spell info", func(t *testing.T) {
		result := CastResult{
			CasterName:            "Elara",
			SpellName:             "Fireball",
			SpellLevel:            3,
			SaveDC:                15,
			SaveAbility:           "dex",
			CarefulSpellCreatures: 4,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Careful")
		assert.Contains(t, log, "4")
	})

	t.Run("distant spell info", func(t *testing.T) {
		result := CastResult{
			CasterName:   "Elara",
			SpellName:    "Fireball",
			DistantRange: "300 ft.",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Distant")
		assert.Contains(t, log, "300 ft.")
	})

	t.Run("empowered spell info", func(t *testing.T) {
		result := CastResult{
			CasterName:       "Elara",
			SpellName:        "Fireball",
			IsEmpowered:      true,
			EmpoweredRerolls: 4,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Empowered")
		assert.Contains(t, log, "4")
	})

	t.Run("extended spell info", func(t *testing.T) {
		result := CastResult{
			CasterName:       "Elara",
			SpellName:        "Hold Person",
			ExtendedDuration: "Up to 2 minutes",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Extended")
		assert.Contains(t, log, "Up to 2 minutes")
	})

	t.Run("heightened spell info", func(t *testing.T) {
		result := CastResult{
			CasterName:   "Elara",
			SpellName:    "Fireball",
			IsHeightened: true,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Heightened")
		assert.Contains(t, log, "disadvantage")
	})

	t.Run("subtle spell info", func(t *testing.T) {
		result := CastResult{
			CasterName: "Elara",
			SpellName:  "Fireball",
			IsSubtle:   true,
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Subtle")
	})

	t.Run("twinned spell info", func(t *testing.T) {
		result := CastResult{
			CasterName:     "Elara",
			SpellName:      "Hold Person",
			TwinTargetName: "Orc",
		}
		log := FormatCastLog(result)
		assert.Contains(t, log, "Twinned")
		assert.Contains(t, log, "Orc")
	})
}

// TDD Cycle 29: Twinned Spell rejects AoE spell
func TestCast_TwinnedSpell_RejectsAoE(t *testing.T) {
	f := newMetamagicTestFixture(makeFireballWithAoE())
	_, err := f.castWithMetamagic([]string{"twinned"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "area of effect")
}
