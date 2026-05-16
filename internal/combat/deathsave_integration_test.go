package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ab/dndnd/internal/refdata"
)

// nullRaw is a tiny helper for building pqtype.NullRawMessage values from a JSON string.
func nullRaw(s string) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{RawMessage: json.RawMessage(s), Valid: true}
}

// --- C-43 wiring: ApplyDamage routes through ProcessDropToZeroHP / ApplyDamageAtZeroHP ---

// deathSaveDamageStore wires a mockStore for the death-save damage integration
// tests. It captures HP, conditions, and death save updates so the assertion
// path can verify the full state machine without a real DB.
type deathSaveDamageStore struct {
	*mockStore
	combatant        *refdata.Combatant
	appliedConds     []CombatCondition
	deathSaveWrites  []DeathSaves
	pendingSaveCalls int
}

func newDeathSaveDamageStore(c *refdata.Combatant) *deathSaveDamageStore {
	ms := defaultMockStore()
	store := &deathSaveDamageStore{mockStore: ms, combatant: c}

	// HP update mirrors the in-memory combatant so subsequent reads see the
	// post-damage state (used by the unconscious / prone application step).
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		c.HpCurrent = arg.HpCurrent
		c.TempHp = arg.TempHp
		c.IsAlive = arg.IsAlive
		return *c, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return *c, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		store.appliedConds = conds
		c.Conditions = arg.Conditions
		c.ExhaustionLevel = arg.ExhaustionLevel
		return *c, nil
	}
	ms.updateCombatantDeathSavesFn = func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		ds, _ := ParseDeathSaves(arg.DeathSaves.RawMessage)
		store.deathSaveWrites = append(store.deathSaveWrites, ds)
		c.DeathSaves = arg.DeathSaves
		return *c, nil
	}
	ms.getCombatantConcentrationFn = func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
		return refdata.GetCombatantConcentrationRow{}, nil
	}
	ms.createPendingSaveFn = func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
		store.pendingSaveCalls++
		return refdata.PendingSafe{}, nil
	}
	return store
}

// Prone+unconscious applied at drop-to-0 (C-43-prone-on-drop).
func TestApplyDamage_DropToZero_AppliesProneAndUnconscious(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 5, IsAlive: true, IsNpc: false,
		DisplayName: "Aria", Conditions: json.RawMessage(`[]`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 12, DamageType: "slashing",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(0), res.NewHP)
	assert.True(t, res.IsAlive, "PC at 0 HP must remain alive (dying, not dead)")
	assert.False(t, res.Killed)

	names := map[string]bool{}
	for _, c := range store.appliedConds {
		names[c.Condition] = true
	}
	assert.True(t, names["unconscious"], "drop to 0 must apply unconscious; got %v", store.appliedConds)
	assert.True(t, names["prone"], "drop to 0 must apply prone; got %v", store.appliedConds)
}

// Instant death overflow rule (C-43-instant-death).
func TestApplyDamage_InstantDeath_OverflowExceedsMax(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 20, IsAlive: true, IsNpc: false,
		DisplayName: "Aria", Conditions: json.RawMessage(`[]`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	// 41 damage on full HP (20) = 21 overflow >= 20 maxHP -> instant death.
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 41, DamageType: "slashing",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(0), res.NewHP)
	assert.False(t, res.IsAlive, "overflow >= maxHP must be instant death")
	assert.True(t, res.Killed)
	assert.True(t, res.InstantDeath)
	// Dying conditions must NOT be applied on instant death.
	for _, c := range store.appliedConds {
		assert.NotEqual(t, "unconscious", c.Condition, "instant death must skip dying conditions")
	}
}

// Just under instant death threshold: overflow < maxHP -> dying flow.
func TestApplyDamage_OverflowJustBelowMax_DyingNotDead(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 20, IsAlive: true, IsNpc: false,
		DisplayName: "Aria", Conditions: json.RawMessage(`[]`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	// 39 damage on full 20 -> 19 overflow < 20 maxHP -> dying.
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 39, DamageType: "slashing",
	})
	require.NoError(t, err)
	assert.True(t, res.IsAlive)
	assert.False(t, res.Killed)
	assert.False(t, res.InstantDeath)
}

// Damage at 0 HP increments death save failures by 1 (C-43-damage-at-0hp).
func TestApplyDamage_AtZeroHP_NormalHitAddsOneFailure(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 0, IsAlive: true, IsNpc: false,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[{"condition":"unconscious"},{"condition":"prone"}]`),
		DeathSaves:  nullRaw(`{"successes":0,"failures":0}`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 5, DamageType: "slashing",
	})
	require.NoError(t, err)
	require.Len(t, store.deathSaveWrites, 1, "exactly one death save write expected")
	assert.Equal(t, 1, store.deathSaveWrites[0].Failures)
}

// Critical hit at 0 HP adds 2 failures (C-43-damage-at-0hp).
func TestApplyDamage_AtZeroHP_CriticalAddsTwoFailures(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 0, IsAlive: true, IsNpc: false,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[{"condition":"unconscious"},{"condition":"prone"}]`),
		DeathSaves:  nullRaw(`{"successes":0,"failures":0}`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 5, DamageType: "slashing",
		IsCritical: true,
	})
	require.NoError(t, err)
	require.Len(t, store.deathSaveWrites, 1)
	assert.Equal(t, 2, store.deathSaveWrites[0].Failures)
}

// Damage-at-0 instant-death overflow takes precedence over failure tally.
func TestApplyDamage_AtZeroHP_OverflowGreaterThanMaxInstantDeath(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 0, IsAlive: true, IsNpc: false,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[{"condition":"unconscious"},{"condition":"prone"}]`),
		DeathSaves:  nullRaw(`{"successes":0,"failures":0}`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	// 25 damage on 0/20 -> overflow = 25 >= 20 maxHP -> instant death.
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 25, DamageType: "slashing",
	})
	require.NoError(t, err)
	assert.False(t, res.IsAlive)
	assert.True(t, res.Killed)
	assert.True(t, res.InstantDeath)
	assert.Empty(t, store.deathSaveWrites, "instant death must skip death save tally")
}

// 3 failures = dead (existing rule, re-asserted at integration point).
func TestApplyDamage_AtZeroHP_ThirdFailureKills(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 0, IsAlive: true, IsNpc: false,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[{"condition":"unconscious"},{"condition":"prone"}]`),
		DeathSaves:  nullRaw(`{"successes":1,"failures":2}`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 3, DamageType: "slashing",
	})
	require.NoError(t, err)
	require.Len(t, store.deathSaveWrites, 1)
	assert.Equal(t, 3, store.deathSaveWrites[0].Failures)
	assert.False(t, res.IsAlive)
	assert.True(t, res.Killed)
}

// --- C-43-heal-reset: HealFromZeroHP wiring at heal call sites ---

// Lay on Hands healing a dying combatant must reset death save tallies.
func TestLayOnHands_HealsFromZero_ResetsDeathSaves(t *testing.T) {
	encID := uuid.New()
	combatantID := uuid.New()
	paladinID := uuid.New()
	charID := uuid.New()

	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 20, HpCurrent: 0, IsAlive: true, IsNpc: false,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[{"condition":"unconscious"},{"condition":"prone"}]`),
		DeathSaves:  nullRaw(`{"successes":0,"failures":2}`),
		PositionCol: "A", PositionRow: 1,
	}
	paladin := refdata.Combatant{
		ID: paladinID, EncounterID: encID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Galahad", HpCurrent: 30, HpMax: 30, IsAlive: true,
		PositionCol: "A", PositionRow: 1,
	}
	ms := defaultMockStore()
	store := &deathSaveDamageStore{mockStore: ms, combatant: target}
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		target.HpCurrent = arg.HpCurrent
		target.TempHp = arg.TempHp
		target.IsAlive = arg.IsAlive
		return *target, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return *target, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		store.appliedConds = conds
		target.Conditions = arg.Conditions
		return *target, nil
	}
	ms.updateCombatantDeathSavesFn = func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		ds, _ := ParseDeathSaves(arg.DeathSaves.RawMessage)
		store.deathSaveWrites = append(store.deathSaveWrites, ds)
		target.DeathSaves = arg.DeathSaves
		return *target, nil
	}
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:          charID,
			Classes:     json.RawMessage(`[{"class":"Paladin","level":3}]`),
			FeatureUses: nullRaw(`{"lay-on-hands":{"current":15,"max":15,"recharge":"long"}}`),
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, ActionUsed: arg.ActionUsed}, nil
	}

	svc := NewService(ms)
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encID, CombatantID: paladinID, ActionUsed: false}
	res, err := svc.LayOnHands(context.Background(), LayOnHandsCommand{
		Paladin: paladin, Target: *target,
		Turn: turn,
		HP:   5,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(5), res.HPRestored)

	// Death saves must have been reset on heal from 0 HP.
	require.NotEmpty(t, store.deathSaveWrites, "lay on hands healing from 0 HP must reset death saves")
	last := store.deathSaveWrites[len(store.deathSaveWrites)-1]
	assert.Equal(t, 0, last.Failures, "failures must reset on heal-from-0")
	assert.Equal(t, 0, last.Successes, "successes must reset on heal-from-0")
}

// --- SR-051: Heal-from-0 must leave prone condition intact (spec rule) ---

func TestResetDyingState_LeavesProne(t *testing.T) {
	combatantID := uuid.New()

	conds := json.RawMessage(`[{"condition":"unconscious","duration_rounds":0},{"condition":"prone","duration_rounds":0}]`)

	target := &refdata.Combatant{
		ID:         combatantID,
		HpCurrent:  0,
		HpMax:      20,
		IsAlive:    true,
		IsNpc:      false,
		Conditions: conds,
		DeathSaves: nullRaw(`{"successes":1,"failures":2}`),
	}

	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return *target, nil
	}
	ms.updateCombatantDeathSavesFn = func(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error) {
		target.DeathSaves = arg.DeathSaves
		return *target, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		target.Conditions = arg.Conditions
		return *target, nil
	}

	svc := NewService(ms)
	updated, err := svc.MaybeResetDeathSavesOnHeal(context.Background(), *target, 5)
	require.NoError(t, err)

	// Unconscious must be removed.
	assert.False(t, HasCondition(updated.Conditions, "unconscious"), "unconscious must be removed on heal-from-0")
	// Prone must remain (spec: "Status → conscious, still prone").
	assert.True(t, HasCondition(updated.Conditions, "prone"), "prone must remain after heal-from-0 (spec rule)")
}

// --- C-H07: Damage-at-0 with temp HP still triggers instant death when
// remaining damage (after temp HP absorption) >= maxHP.
func TestApplyDamage_AtZeroHP_TempHPAbsorbedStillInstantDeath(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 18, HpCurrent: 0, TempHp: 5, IsAlive: true, IsNpc: false,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[{"condition":"unconscious"},{"condition":"prone"}]`),
		DeathSaves:  nullRaw(`{"successes":0,"failures":0}`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	// 25 damage, 5 absorbed by temp HP -> 20 adjusted >= 18 maxHP -> instant death.
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 25, DamageType: "slashing",
	})
	require.NoError(t, err)
	assert.False(t, res.IsAlive, "remaining damage after temp HP (20) >= maxHP (18) must be instant death")
	assert.True(t, res.Killed)
	assert.True(t, res.InstantDeath)
	assert.Empty(t, store.deathSaveWrites, "instant death must skip death save tally")
}


// C-H01: Instant death must trigger when massive damage drops PC from low HP.
// PC at 1 HP, maxHP 10, takes 15 damage → overflow = 14 >= 10 → instant death.
func TestApplyDamage_InstantDeath_LowHPMassiveDamage(t *testing.T) {
	combatantID := uuid.New()
	encID := uuid.New()
	target := &refdata.Combatant{
		ID: combatantID, EncounterID: encID,
		HpMax: 10, HpCurrent: 1, IsAlive: true, IsNpc: false,
		DisplayName: "Aria", Conditions: json.RawMessage(`[]`),
	}
	store := newDeathSaveDamageStore(target)
	svc := NewService(store.mockStore)

	// 15 damage on 1 HP: overflow = 15 - 1 = 14 >= 10 maxHP → instant death.
	res, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encID, Target: *target, RawDamage: 15, DamageType: "slashing",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(0), res.NewHP)
	assert.False(t, res.IsAlive, "overflow (14) >= maxHP (10) must be instant death")
	assert.True(t, res.Killed)
	assert.True(t, res.InstantDeath)
}
