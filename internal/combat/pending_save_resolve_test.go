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

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ISSUE-043 TDD: resolveCombatantSaveBonus generalizes resolveTargetConSave to
// any ability keyed by the ability string.
func TestResolveCombatantSaveBonus(t *testing.T) {
	t.Run("creature with explicit saving_throws returns it", func(t *testing.T) {
		store := defaultMockStore()
		store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
			return refdata.Creature{
				AbilityScores: json.RawMessage(`{"dex":14}`),
				SavingThrows:  pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"dex":5}`), Valid: true},
			}, nil
		}
		svc := NewService(store)
		comb := refdata.Combatant{CreatureRefID: sql.NullString{String: "goblin", Valid: true}}
		bonus, err := svc.resolveCombatantSaveBonus(context.Background(), comb, "dex")
		require.NoError(t, err)
		assert.Equal(t, 5, bonus, "explicit saving_throws.dex wins over the ability mod")
	})

	t.Run("creature without explicit save falls back to ability mod", func(t *testing.T) {
		store := defaultMockStore()
		store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
			return refdata.Creature{AbilityScores: json.RawMessage(`{"con":16}`)}, nil
		}
		svc := NewService(store)
		comb := refdata.Combatant{CreatureRefID: sql.NullString{String: "goblin", Valid: true}}
		bonus, err := svc.resolveCombatantSaveBonus(context.Background(), comb, "con")
		require.NoError(t, err)
		assert.Equal(t, 3, bonus, "CON 16 → +3 modifier")
	})

	t.Run("PC uses character ability mod", func(t *testing.T) {
		charID := uuid.New()
		store := defaultMockStore()
		store.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
			return refdata.Character{AbilityScores: json.RawMessage(`{"wis":12}`)}, nil
		}
		svc := NewService(store)
		comb := refdata.Combatant{CharacterID: uuid.NullUUID{UUID: charID, Valid: true}}
		bonus, err := svc.resolveCombatantSaveBonus(context.Background(), comb, "wis")
		require.NoError(t, err)
		assert.Equal(t, 1, bonus, "WIS 12 → +1 modifier")
	})

	t.Run("NPC without creature ref defaults to +0", func(t *testing.T) {
		svc := NewService(defaultMockStore())
		bonus, err := svc.resolveCombatantSaveBonus(context.Background(), refdata.Combatant{}, "str")
		require.NoError(t, err)
		assert.Equal(t, 0, bonus)
	})
}

func newDamagingFireball() refdata.Spell {
	fb := makeFireball()
	fb.SaveEffect = sql.NullString{String: "half_damage", Valid: true}
	fb.Damage = pqtype.NullRawMessage{
		RawMessage: json.RawMessage(`{"dice":"8d6","damage_type":"fire"}`),
		Valid:      true,
	}
	return fb
}

// rollerFor returns a roller where d20 rolls d20Face and every other die rolls
// dmgFace, so a test can independently fix the save roll and the damage roll.
func rollerFor(d20Face, dmgFace int) *dice.Roller {
	return dice.NewRoller(func(max int) int {
		if max == 20 {
			return d20Face
		}
		return dmgFace
	})
}

func TestResolveMonsterPendingSave_FailAppliesFullDamage(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	comb := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Goblin",
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		IsNpc:         true, IsAlive: true, HpMax: 30, HpCurrent: 30,
		Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return comb, nil }
	store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"dex":10}`)}, nil // +0
	}
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}}, nil
	}
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", RollResult: sql.NullInt32{Int32: 4, Valid: true}, Success: sql.NullBool{Bool: false, Valid: true}}}, nil
	}
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return newDamagingFireball(), nil }
	// Mirror the DB RETURNING clause: the updated row carries its source.
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: arg.ID, Source: source, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}
	var hpUpdates []refdata.UpdateCombatantHPParams
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpUpdates = append(hpUpdates, arg)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)
	svc.SetRoller(rollerFor(4, 4)) // d20=4 → 4 < 15 fail; 8d6 of 4 = 32 fire

	res, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.NoError(t, err)
	assert.False(t, res.Success, "4 vs DC 15 must fail")
	assert.Equal(t, 4, res.NaturalRoll)
	assert.Equal(t, 0, res.SaveBonus)
	assert.Equal(t, 4, res.Total)
	assert.Equal(t, "Goblin", res.CombatantName)
	require.NotNil(t, res.Damage, "all rows resolved → damage applied")
	require.Len(t, res.Damage.Targets, 1)
	assert.Equal(t, 32, res.Damage.Targets[0].DamageDealt, "failed save takes full 8d6=32")
	require.Len(t, hpUpdates, 1, "HP write must occur")
}

func TestResolveMonsterPendingSave_SuccessAppliesHalfDamage(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	comb := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Goblin",
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		IsNpc:         true, IsAlive: true, HpMax: 30, HpCurrent: 30,
		Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return comb, nil }
	store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"dex":10}`)}, nil
	}
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}}, nil
	}
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", RollResult: sql.NullInt32{Int32: 20, Valid: true}, Success: sql.NullBool{Bool: true, Valid: true}}}, nil
	}
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return newDamagingFireball(), nil }
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: arg.ID, Source: source, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)
	svc.SetRoller(rollerFor(20, 4)) // d20=20 → success; 8d6 of 4 = 32, halved = 16

	res, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.NoError(t, err)
	assert.True(t, res.Success, "20 vs DC 15 succeeds")
	require.NotNil(t, res.Damage)
	require.Len(t, res.Damage.Targets, 1)
	assert.True(t, res.Damage.Targets[0].SaveSuccess)
	assert.Equal(t, 16, res.Damage.Targets[0].DamageDealt, "successful save halves 32 → 16")
}

func TestResolveMonsterPendingSave_PlayerCombatantRejected(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "pending"}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, DisplayName: "Aria", CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.ErrorIs(t, err, ErrPlayerSaveViaDiscord)
}

func TestResolveMonsterPendingSave_AlreadyResolvedRejected(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	// ISSUE-044: 'applied' is the terminal state → 409. A 'rolled' row is NOT
	// rejected — it is recovered (see TestResolveMonsterPendingSave_RecoversRolledRow).
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: uuid.New(), Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "applied"}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.ErrorIs(t, err, ErrSaveAlreadyResolved)
}

// TestResolveMonsterPendingSave_RecoversRolledRow (ISSUE-044 white-box): a row
// already 'rolled' (failed) re-drives the apply step WITHOUT re-rolling the d20,
// preserving the stored success, and applies full damage.
func TestResolveMonsterPendingSave_RecoversRolledRow(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	comb := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Goblin",
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		IsNpc:         true, IsAlive: true, HpMax: 30, HpCurrent: 30,
		Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", RollResult: sql.NullInt32{Int32: 4, Valid: true}, Success: sql.NullBool{Bool: false, Valid: true}}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return comb, nil }
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", RollResult: sql.NullInt32{Int32: 4, Valid: true}, Success: sql.NullBool{Bool: false, Valid: true}}}, nil
	}
	store.getSpellFn = func(_ context.Context, _ string) (refdata.Spell, error) { return newDamagingFireball(), nil }
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
	}

	svc := NewService(store)
	d20Rolled := false
	svc.SetRoller(dice.NewRoller(func(max int) int {
		if max == 20 {
			d20Rolled = true
			return 20 // would flip the save to success if (wrongly) re-rolled
		}
		return 4
	}))

	res, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.NoError(t, err)
	assert.False(t, d20Rolled, "recovery must not re-roll the d20")
	assert.False(t, res.Success, "stored failed save preserved")
	assert.Equal(t, 4, res.Total, "reports the stored roll total")
	require.NotNil(t, res.Damage)
	require.Len(t, res.Damage.Targets, 1)
	assert.Equal(t, 32, res.Damage.Targets[0].DamageDealt, "failed save → full 8d6=32")
}

func TestResolveMonsterPendingSave_WrongEncounterRejected(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: uuid.New(), CombatantID: uuid.New(), Ability: "dex", Dc: 15, Source: AoEPendingSaveSource("fireball"), Status: "pending"}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.ErrorIs(t, err, ErrSaveWrongEncounter)
}

func TestResolveMonsterPendingSave_MissingRowNotFound(t *testing.T) {
	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, _ uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{}, sql.ErrNoRows
	}
	svc := NewService(store)
	_, err := svc.ResolveMonsterPendingSave(context.Background(), uuid.New(), uuid.New())
	require.ErrorIs(t, err, ErrPendingSaveNotFound)
}

func TestResolveMonsterPendingSave_MultiTargetPendingDefersDamage(t *testing.T) {
	encounterID := uuid.New()
	saveID := uuid.New()
	combatantID := uuid.New()
	otherID := uuid.New()
	source := AoEPendingSaveSource("fireball")

	comb := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Goblin A",
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		IsNpc:         true, IsAlive: true, HpMax: 30, HpCurrent: 30,
		Conditions: json.RawMessage(`[]`),
	}

	store := defaultMockStore()
	store.getPendingSaveFn = func(_ context.Context, id uuid.UUID) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: id, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}, nil
	}
	store.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return comb, nil }
	store.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{AbilityScores: json.RawMessage(`{"dex":10}`)}, nil
	}
	store.listPendingSavesByCombatantFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		return []refdata.PendingSafe{{ID: saveID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "pending"}}, nil
	}
	store.listSavesByEncounterFn = func(_ context.Context, _ uuid.UUID) ([]refdata.PendingSafe, error) {
		// This combatant's row now rolled; another combatant's row still pending.
		return []refdata.PendingSafe{
			{ID: saveID, EncounterID: encounterID, CombatantID: combatantID, Ability: "dex", Dc: 15, Source: source, Status: "rolled", Success: sql.NullBool{Bool: true, Valid: true}},
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: otherID, Ability: "dex", Dc: 15, Source: source, Status: "pending"},
		}, nil
	}
	store.updatePendingSaveResultFn = func(_ context.Context, arg refdata.UpdatePendingSaveResultParams) (refdata.PendingSafe, error) {
		return refdata.PendingSafe{ID: arg.ID, Source: source, Status: "rolled", RollResult: arg.RollResult, Success: arg.Success}, nil
	}
	hpCalls := 0
	store.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpCalls++
		return refdata.Combatant{ID: arg.ID}, nil
	}

	svc := NewService(store)
	svc.SetRoller(rollerFor(20, 4))

	res, err := svc.ResolveMonsterPendingSave(context.Background(), encounterID, saveID)
	require.NoError(t, err)
	assert.True(t, res.Success, "the rolled row still reports its own outcome")
	assert.Nil(t, res.Damage, "damage waits until every target's save is resolved")
	assert.Equal(t, 0, hpCalls, "no HP applied while a save is still pending")
}
