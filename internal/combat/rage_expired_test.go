package combat

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// rageExpiredRows filters captured action_log params down to the
// "rage_expired" type (mirrors downedRows).
func rageExpiredRows(rows []refdata.CreateActionLogParams) []refdata.CreateActionLogParams {
	out := make([]refdata.CreateActionLogParams, 0, len(rows))
	for _, r := range rows {
		if r.ActionType == actionTypeRageExpired {
			out = append(out, r)
		}
	}
	return out
}

// A raging barbarian who neither attacked nor took damage this round has their
// rage lapse at end of turn — that drop must surface on #combat-log AND the DM
// Console timeline (action_log), not just silently in the DB. (ISSUE-041)
func TestMaybeEndRageOnTurnEnd_Lapsed_LogsRageExpired(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()

	ms := &mockStore{}
	ms.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: id, EncounterID: encounterID, DisplayName: "Forge",
			IsRaging: true, RageAttackedThisRound: false, RageTookDamageThisRound: false,
		}, nil
	}
	var ragePersisted *refdata.UpdateCombatantRageParams
	ms.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		ragePersisted = &arg
		return refdata.Combatant{ID: arg.ID}, nil
	}
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)

	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	svc.maybeEndRageOnTurnEnd(context.Background(), combatantID)

	// Rage was cleared + persisted.
	require.NotNil(t, ragePersisted, "expected rage state to be persisted")
	assert.False(t, ragePersisted.IsRaging, "rage must be cleared")

	// #combat-log post.
	posts := cl.all()
	require.Len(t, posts, 1, "expected one #combat-log post")
	assert.Equal(t, encounterID, posts[0].encounterID)
	assert.Contains(t, posts[0].content, "Forge")
	assert.Contains(t, posts[0].content, "Rage ends")

	// action_log row (DM Console timeline), parented to the rager's current turn.
	rows := rageExpiredRows(*logged)
	require.Len(t, rows, 1, "expected one rage_expired action_log row")
	got := rows[0]
	assert.Equal(t, turnID, got.TurnID)
	assert.Equal(t, encounterID, got.EncounterID)
	assert.Equal(t, combatantID, got.ActorID)
	assert.False(t, got.TargetID.Valid)
	assert.Contains(t, got.Description.String, "Forge")
}

// When the barbarian attacked this round, rage holds — no clear, no logging.
func TestMaybeEndRageOnTurnEnd_AttackedThisRound_NoLog(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	ms := &mockStore{}
	ms.getCombatantFn = func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID: id, EncounterID: encounterID, DisplayName: "Forge",
			IsRaging: true, RageAttackedThisRound: true,
		}, nil
	}
	rageUpdated := false
	ms.updateCombatantRageFn = func(_ context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
		rageUpdated = true
		return refdata.Combatant{ID: arg.ID}, nil
	}
	logged := captureActionLog(ms)

	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	svc.maybeEndRageOnTurnEnd(context.Background(), combatantID)

	assert.False(t, rageUpdated, "rage must not be cleared when it holds")
	assert.Empty(t, cl.all(), "no combat-log post when rage holds")
	assert.Empty(t, rageExpiredRows(*logged), "no action_log row when rage holds")
}
