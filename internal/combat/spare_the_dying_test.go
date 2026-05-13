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

// makeClericCharacter mirrors makeWizardCharacter but with a cleric class so
// the spellcasting-ability resolution picks WIS (the casting stat for
// spare-the-dying). Spare the Dying is a cantrip, so no leveled slot is
// consumed; we still seed a slot pool to mirror the standard fixtures.
func makeClericCharacter(id uuid.UUID) refdata.Character {
	slotsJSON, _ := json.Marshal(map[string]SlotInfo{
		"1": {Current: 4, Max: 4},
		"2": {Current: 3, Max: 3},
	})
	scoresJSON, _ := json.Marshal(AbilityScores{
		Str: 10, Dex: 12, Con: 14, Int: 10, Wis: 16, Cha: 12,
	})
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "cleric", Level: 5}})
	return refdata.Character{
		ID:               id,
		Name:             "Pelor",
		ProficiencyBonus: 3,
		Classes:          classesJSON,
		AbilityScores:    scoresJSON,
		SpellSlots:       pqtype.NullRawMessage{RawMessage: slotsJSON, Valid: true},
		Level:            5,
	}
}

// makeSpareTheDying returns the spare-the-dying cantrip refdata fixture used
// by the SR-017 tests. Mirrors the seeded shape (touch, cantrip, auto).
func makeSpareTheDying() refdata.Spell {
	return refdata.Spell{
		ID:             SpareTheDyingSpellID,
		Name:           "Spare the Dying",
		Level:          0,
		CastingTime:    "1 action",
		RangeType:      "touch",
		Components:     []string{"V", "S"},
		Duration:       "Instantaneous",
		ResolutionMode: "auto",
		Concentration:  sql.NullBool{Bool: false, Valid: true},
	}
}

// makeDyingTarget builds a PC at 0 HP with the given death-save tallies. The
// target sits adjacent to the standard makeSpellCaster (E5 -> E6 = 5ft) so
// touch-range validation passes.
func makeDyingTarget(t *testing.T, successes, failures int) refdata.Combatant {
	t.Helper()
	dsJSON, err := json.Marshal(DeathSaves{Successes: successes, Failures: failures})
	require.NoError(t, err)
	return refdata.Combatant{
		ID:          uuid.New(),
		DisplayName: "Aria",
		PositionCol: "E",
		PositionRow: 6,
		HpMax:       12,
		HpCurrent:   0,
		IsAlive:     true,
		IsNpc:       false,
		Conditions: mustMarshalConds(t, []CombatCondition{
			{Condition: "unconscious"},
			{Condition: "prone"},
		}),
		DeathSaves: pqtype.NullRawMessage{RawMessage: dsJSON, Valid: true},
	}
}

// SR-017 cycle 1 (red): /cast spare-the-dying on a downed PC mid-death-saves
// must stabilize them (Successes=3, IsStable) and persist via
// UpdateCombatantDeathSaves. The CastResult exposes the stabilize message so
// FormatCastLog can mirror the 🩹 line.
func TestServiceCast_SpareTheDying_StabilizesDyingTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeClericCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeDyingTarget(t, 1, 2)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSpareTheDying(), nil }
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

	var savedDS pqtype.NullRawMessage
	var savedID uuid.UUID
	store.updateCombatantDeathSavesFn = func(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		savedID = arg.ID
		savedDS = arg.DeathSaves
		return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  SpareTheDyingSpellID,
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)

	assert.Equal(t, target.ID, savedID, "stabilize must write to the dying target's combatant row")
	require.True(t, savedDS.Valid, "stabilize must persist a non-null death-save JSON")
	ds, dsErr := ParseDeathSaves(savedDS.RawMessage)
	require.NoError(t, dsErr)
	assert.Equal(t, 3, ds.Successes, "stabilize sets successes to 3")
	assert.Equal(t, 2, ds.Failures, "stabilize preserves prior failures (no death save was rolled)")

	assert.NotEmpty(t, result.StabilizeMessage, "CastResult must surface the stabilize log line")
	assert.Contains(t, result.StabilizeMessage, "Aria")
	assert.Contains(t, result.StabilizeMessage, "Spare the Dying")
	assert.Equal(t, 0, result.SlotUsed, "spare-the-dying is a cantrip — no slot consumed")

	log := FormatCastLog(result)
	assert.Contains(t, log, "Spare the Dying")
	assert.Contains(t, log, "stabilized")
}

// SR-017 cycle 2 (red): casting on a PC at 0 HP with no death-save tallies
// yet should still stabilize per spec — Successes=3, no rolls needed.
func TestServiceCast_SpareTheDying_StabilizesWithNoPriorSaves(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeClericCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeDyingTarget(t, 0, 0)

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSpareTheDying(), nil }
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

	var savedDS pqtype.NullRawMessage
	stabilizeCalls := 0
	store.updateCombatantDeathSavesFn = func(_ context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		stabilizeCalls++
		savedDS = arg.DeathSaves
		return refdata.Combatant{ID: arg.ID, DeathSaves: arg.DeathSaves}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  SpareTheDyingSpellID,
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)

	assert.Equal(t, 1, stabilizeCalls, "stabilize must persist exactly once even when no prior death saves existed")
	require.True(t, savedDS.Valid)
	ds, dsErr := ParseDeathSaves(savedDS.RawMessage)
	require.NoError(t, dsErr)
	assert.Equal(t, 3, ds.Successes)
	assert.Equal(t, 0, ds.Failures)
	assert.NotEmpty(t, result.StabilizeMessage)
}

// SR-017 cycle 3 (red): casting spare-the-dying on a non-dying target (alive
// at >0 HP) is a no-op for the stabilize machinery — the spell description
// says "a living creature that has 0 hit points". The cast still succeeds
// (the action is consumed) but UpdateCombatantDeathSaves is NEVER called and
// CastResult.StabilizeMessage stays empty.
func TestServiceCast_SpareTheDying_NoopWhenTargetNotDying(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	char := makeClericCharacter(charID)
	caster := makeSpellCaster(charID)
	target := makeSpellTarget() // alive, HpCurrent default 0 but IsAlive true and not at 0 HP semantics
	target.HpCurrent = 8
	target.HpMax = 8
	target.PositionRow = 6 // ensure adjacency for touch range
	target.IsNpc = false

	store := defaultMockStore()
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return makeSpareTheDying(), nil }
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

	stabilizeCalls := 0
	store.updateCombatantDeathSavesFn = func(_ context.Context, _ refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		stabilizeCalls++
		return refdata.Combatant{}, nil
	}

	svc := NewService(store)
	cmd := CastCommand{
		SpellID:  SpareTheDyingSpellID,
		CasterID: caster.ID,
		TargetID: target.ID,
		Turn:     refdata.Turn{ID: uuid.New(), CombatantID: caster.ID},
	}
	result, err := svc.Cast(ctx, cmd, testRoller())
	require.NoError(t, err)

	assert.Equal(t, 0, stabilizeCalls, "non-dying target must not trigger UpdateCombatantDeathSaves")
	assert.Empty(t, result.StabilizeMessage, "non-dying target produces no stabilize log line")
}
