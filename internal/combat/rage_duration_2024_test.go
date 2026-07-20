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

// rageWorld is a write-through in-memory combatant for the 2024 Rage duration
// tests. GetCombatant hands back whatever UpdateCombatantRage last persisted,
// so a multi-turn sequence (activate → attack → end turn → end turn) can be
// driven through the real service hooks instead of asserting on pure helpers.
type rageWorld struct {
	c refdata.Combatant
}

func newRageWorld(t *testing.T) (*rageWorld, *mockStore, *Service) {
	t.Helper()

	w := &rageWorld{
		c: refdata.Combatant{
			ID:          uuid.New(),
			EncounterID: uuid.New(),
			DisplayName: "Forge Anvilbearer",
			IsAlive:     true,
			HpCurrent:   30,
			Conditions:  json.RawMessage(`[]`),
		},
	}

	ms := defaultMockStore()
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
		return w.c, nil
	}
	ms.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		w.c.IsRaging = arg.IsRaging
		w.c.RageRoundsRemaining = arg.RageRoundsRemaining
		w.c.RageAttackedThisRound = arg.RageAttackedThisRound
		w.c.RageTookDamageThisRound = arg.RageTookDamageThisRound
		w.c.RageStartedRound = arg.RageStartedRound
		return w.c, nil
	}
	validTurnEncounter(ms, uuid.New())

	return w, ms, NewService(ms)
}

// barbarianCharStore points the mock store's character/turn lookups at a
// level-5 Barbarian with rage uses left, so ActivateRage runs end to end.
func barbarianCharStore(ms *mockStore, charID uuid.UUID) {
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return refdata.Character{
			ID:      charID,
			Classes: json.RawMessage(`[{"class":"Barbarian","level":5}]`),
			FeatureUses: pqtype.NullRawMessage{
				RawMessage: json.RawMessage(`{"rage":{"current":3,"max":3,"recharge":"long"}}`),
				Valid:      true,
			},
		}, nil
	}
	ms.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID}, nil
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}
}

// activateRageOnRound runs the real /bonus rage path for the world's combatant
// in the given round.
func (w *rageWorld) activate(t *testing.T, svc *Service, ms *mockStore, round int32) {
	t.Helper()
	charID := uuid.New()
	w.c.CharacterID = uuid.NullUUID{UUID: charID, Valid: true}
	barbarianCharStore(ms, charID)

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: w.c,
		Turn:      refdata.Turn{ID: uuid.New(), RoundNumber: round},
	})
	require.NoError(t, err)
	require.True(t, w.c.IsRaging, "expected rage to be active after /bonus rage")
}

// THE LIVE REGRESSION (action log 03:06 attack / 03:17 rage_expired): Forge
// raged, swung a handaxe and MISSED, then ended his turn — and the rage
// evaporated. Under 2024 rules a rage NEVER ends on the turn it started, and an
// attack roll sustains it whether it hits or misses.
func TestRage2024_RagedThenMissedOneAttack_SurvivesTurnEnd(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 3)

	// The handaxe swing — a MISS still counts as "you made an attack roll".
	svc.markRageAttacked(context.Background(), w.c)

	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 3)

	assert.True(t, w.c.IsRaging, "rage must survive the turn it was activated on (missed attack)")
}

// 2024 grace window: rage lasts until the end of your NEXT turn, so doing
// literally nothing on the activation turn must not end it.
func TestRage2024_GraceWindow_IdleActivationTurnSurvives(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 3)

	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 3)

	assert.True(t, w.c.IsRaging, "rage must not end at the end of its own activation turn")
}

// ...but the turn AFTER the activation turn, with no qualifying activity, is
// the "end of your next turn" — rage lapses there.
func TestRage2024_SecondIdleTurn_EndsRage(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 3)

	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 3)
	require.True(t, w.c.IsRaging, "grace window should carry rage past round 3")

	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 4)

	assert.False(t, w.c.IsRaging, "rage must lapse at the end of the next idle turn")
}

// Ordering fix: a barbarian who ATTACKS first and rages afterwards on the same
// turn must still be credited. markRageAttacked used to early-return on the
// stale cmd.Attacker snapshot (IsRaging=false at attack time).
func TestRage2024_AttackedBeforeRagingSameTurn_StillCredited(t *testing.T) {
	w, ms, svc := newRageWorld(t)

	// Snapshot taken by the attack pipeline BEFORE rage was activated.
	staleAttacker := w.c

	w.activate(t, svc, ms, 5)
	svc.markRageAttacked(context.Background(), staleAttacker)

	assert.True(t, w.c.RageAttackedThisRound,
		"markRageAttacked must re-read live rage state, not trust the stale attacker snapshot")
}

// 2024 sustain condition (b): forcing an enemy to make a saving throw keeps the
// rage alive even with no attack roll and no damage taken.
func TestRage2024_ForcedSaveOnly_SurvivesTurnEnd(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 3)

	// Burn the grace window so only the forced save can sustain the rage.
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 3)
	require.True(t, w.c.IsRaging)

	svc.markRageForcedSave(context.Background(), w.c)
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 4)

	assert.True(t, w.c.IsRaging, "forcing a saving throw must sustain the rage")
}

// 2024 sustain condition (c): spend a Bonus Action to extend. /bonus rage while
// already raging extends instead of erroring.
func TestRage2024_BonusActionExtend_SurvivesTurnEnd(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 3)

	// Burn the activation grace window.
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 3)
	require.True(t, w.c.IsRaging)

	res, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: w.c,
		Turn:      refdata.Turn{ID: uuid.New(), RoundNumber: 4},
	})
	require.NoError(t, err, "/bonus rage while raging must extend, not error")
	assert.Contains(t, res.CombatLog, "extend")

	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 4)
	assert.True(t, w.c.IsRaging, "a bonus-action extension must sustain the rage")

	// ...and it is a real extension: the rage now survives to the end of the
	// NEXT turn too, then lapses on the following idle turn.
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 5)
	assert.False(t, w.c.IsRaging, "extension buys exactly one more turn")
}

// /bonus rage while raging must consume the bonus action.
func TestRage2024_BonusActionExtend_SpendsBonusAction(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 3)

	var persistedTurn *refdata.UpdateTurnActionsParams
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		persistedTurn = &arg
		return refdata.Turn{ID: arg.ID, BonusActionUsed: true}, nil
	}

	_, err := svc.ActivateRage(context.Background(), RageCommand{
		Combatant: w.c,
		Turn:      refdata.Turn{ID: uuid.New(), RoundNumber: 4},
	})
	require.NoError(t, err)
	require.NotNil(t, persistedTurn, "extend must persist the turn")
	assert.True(t, persistedTurn.BonusActionUsed, "extend must consume the bonus action")

	// No bonus action left → extending again fails.
	_, err = svc.ActivateRage(context.Background(), RageCommand{
		Combatant: w.c,
		Turn:      refdata.Turn{ID: uuid.New(), RoundNumber: 4, BonusActionUsed: true},
	})
	assert.Error(t, err, "extend must require an available bonus action")
}

// Guards the "true forever" bug: RageAttackedThisRound was never reset because
// DecrementRageRound was dead code, so once a barbarian attacked their rage
// could never expire again.
func TestRage2024_PerRoundFlagsResetBetweenRounds(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 1)

	// Round 1: attacked. Turn end sustains AND clears the per-round flags.
	svc.markRageAttacked(context.Background(), w.c)
	require.True(t, w.c.RageAttackedThisRound)
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 1)
	require.True(t, w.c.IsRaging)
	assert.False(t, w.c.RageAttackedThisRound, "attacked flag must reset at turn end")
	assert.False(t, w.c.RageTookDamageThisRound, "took-damage flag must reset at turn end")

	// Round 2: attacked again → sustained again.
	svc.markRageAttacked(context.Background(), w.c)
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 2)
	require.True(t, w.c.IsRaging)

	// Round 3: idle. Because the flags actually reset, the rage now lapses.
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 3)
	assert.False(t, w.c.IsRaging, "a stale attacked flag must not keep rage alive forever")
}

// Damage taken between the barbarian's turns must still sustain the rage: the
// per-round flags reset at TURN END (the start of the "since your last turn"
// window), never at turn start, or the enemy's hit would be forgotten.
func TestRage2024_DamageTakenBetweenTurns_SustainsRage(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 1)
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 1)
	require.True(t, w.c.IsRaging)

	// An enemy hits the barbarian during the enemy's turn (round 2).
	svc.markRageTookDamage(context.Background(), w.c)

	// Barbarian's own round-2 turn: no attack, but they were hit.
	svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, 2)
	assert.True(t, w.c.IsRaging, "damage taken since the last turn must sustain the rage")
}

// Hard cap: 10 minutes = 100 rounds.
func TestRage2024_RoundCapIs100Rounds(t *testing.T) {
	assert.Equal(t, 100, RageRounds, "2024 Rage lasts 10 minutes = 100 rounds")

	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 1)
	require.EqualValues(t, RageRounds, w.c.RageRoundsRemaining.Int32)

	// Sustain the rage every single turn; only the round cap can stop it.
	round := int32(1)
	for i := 0; i < RageRounds; i++ {
		require.True(t, w.c.IsRaging, "rage ended early at round %d", round)
		svc.markRageAttacked(context.Background(), w.c)
		svc.maybeEndRageOnTurnEnd(context.Background(), w.c.ID, round)
		round++
		// Turn start: the cap check.
		svc.maybeEndRageOnRoundCap(context.Background(), w.c)
	}

	assert.False(t, w.c.IsRaging, "rage must end once the 100-round cap is exhausted")
	assert.EqualValues(t, RageRounds+1, round, "sanity: loop ran the full cap")
}

// The round-cap sweep is wired into turn start (ShouldRageEndOnTurnStart was
// dead code before).
func TestRage2024_RoundCapSweep_EndsExhaustedRage(t *testing.T) {
	w, _, svc := newRageWorld(t)
	w.c.IsRaging = true
	w.c.RageRoundsRemaining = sql.NullInt32{Int32: 0, Valid: true}

	svc.maybeEndRageOnRoundCap(context.Background(), w.c)

	assert.False(t, w.c.IsRaging, "an exhausted rage must be cleared at turn start")
}

// A rage with rounds left is untouched by the turn-start sweep.
func TestRage2024_RoundCapSweep_LeavesLiveRageAlone(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 1)

	svc.maybeEndRageOnRoundCap(context.Background(), w.c)

	assert.True(t, w.c.IsRaging, "a rage with rounds remaining must survive turn start")
}

// 2024: rage ends early if you are Incapacitated.
func TestRage2024_Incapacitated_EndsRage(t *testing.T) {
	w, ms, svc := newRageWorld(t)
	w.activate(t, svc, ms, 1)

	svc.maybeEndRageOnIncapacitated(context.Background(), w.c)

	assert.False(t, w.c.IsRaging, "an incapacitated barbarian drops out of rage")
}
