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

// validTurnEncounter wires getEncounterFn to return an encounter whose
// current_turn_id is set, so notifyDroppedToZero can resolve the action_log
// parent turn.
func validTurnEncounter(ms *mockStore, turnID uuid.UUID) {
	ms.getEncounterFn = func(_ context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true}}, nil
	}
}

// downedRows filters captured action_log params down to the "downed" type.
func downedRows(rows []refdata.CreateActionLogParams) []refdata.CreateActionLogParams {
	out := make([]refdata.CreateActionLogParams, 0, len(rows))
	for _, r := range rows {
		if r.ActionType == actionTypeDowned {
			out = append(out, r)
		}
	}
	return out
}

func TestFormatDroppedToZeroLog(t *testing.T) {
	pcDying := formatDroppedToZeroLog("Forge", false, false)
	assert.Contains(t, pcDying, "Forge")
	assert.Contains(t, pcDying, "unconscious and dying")

	pcKilled := formatDroppedToZeroLog("Forge", false, true)
	assert.Contains(t, pcKilled, "Forge")
	assert.Contains(t, pcKilled, "killed outright")

	npc := formatDroppedToZeroLog("Ghoul", true, false)
	assert.Contains(t, npc, "Ghoul")
	assert.Contains(t, npc, "defeated")

	// NPCs never have death saves; the instant-death flag is irrelevant.
	npcInstant := formatDroppedToZeroLog("Ghoul", true, true)
	assert.Contains(t, npcInstant, "defeated")
	assert.NotContains(t, npcInstant, "killed outright")
}

func TestApplyDamage_PCDropToZero_LogsDownedAndPostsCombatLog(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Forge",
		HpMax: 20, HpCurrent: 5, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 12, DamageType: "fire",
	})
	require.NoError(t, err)

	rows := downedRows(*logged)
	require.Len(t, rows, 1, "expected one downed action_log row")
	got := rows[0]
	assert.Equal(t, turnID, got.TurnID)
	assert.Equal(t, encounterID, got.EncounterID)
	assert.Equal(t, combatantID, got.ActorID)
	assert.False(t, got.TargetID.Valid)
	assert.Contains(t, got.Description.String, "Forge")
	assert.Contains(t, got.Description.String, "unconscious and dying")

	posts := cl.all()
	require.Len(t, posts, 1, "expected one #combat-log post")
	assert.Equal(t, encounterID, posts[0].encounterID)
	assert.Contains(t, posts[0].content, "Forge")
	assert.Contains(t, posts[0].content, "dying")
}

func TestApplyDamage_NPCDropToZero_LogsDefeated(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Ghoul",
		HpMax: 22, HpCurrent: 6, IsAlive: true, IsNpc: true,
		Conditions: json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 30, DamageType: "slashing",
	})
	require.NoError(t, err)

	rows := downedRows(*logged)
	require.Len(t, rows, 1)
	assert.Equal(t, combatantID, rows[0].ActorID)
	assert.Contains(t, rows[0].Description.String, "Ghoul")
	assert.Contains(t, rows[0].Description.String, "defeated")

	posts := cl.all()
	require.Len(t, posts, 1)
	assert.Contains(t, posts[0].content, "defeated")
}

func TestApplyDamage_SurvivesAboveZero_NoDownedEvent(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Vale",
		HpMax: 24, HpCurrent: 24, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 7, DamageType: "fire",
	})
	require.NoError(t, err)

	assert.Empty(t, downedRows(*logged), "survivor must not log a downed event")
	assert.Empty(t, cl.all(), "survivor must not post to #combat-log")
}

func TestApplyDamage_DamageAtZero_NoDownedEvent(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	// Already at 0 HP (dying); another hit adds a death-save failure but is NOT
	// a fresh drop-to-0 transition.
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Forge",
		HpMax: 20, HpCurrent: 0, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
		DeathSaves: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"successes":0,"failures":1}`), Valid: true},
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 4, DamageType: "fire",
	})
	require.NoError(t, err)

	assert.Empty(t, downedRows(*logged), "damage-at-0 is not a drop-to-0 transition")
	assert.Empty(t, cl.all())
}

func TestApplyDamage_OverrideDropToZero_NoDownedEvent(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	// DM HP override / undo path: not a live combat event, so no notification.
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID, DisplayName: "Forge",
		HpMax: 20, HpCurrent: 20, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: target, RawDamage: 20, DamageType: "fire",
		Override: true,
	})
	require.NoError(t, err)

	assert.Empty(t, downedRows(*logged), "override drop must not log a downed event")
	assert.Empty(t, cl.all())
}

func TestApplyDamage_WildShapeDropToZero_NoDownedEvent(t *testing.T) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()

	snap := WildShapeSnapshot{HpMax: 28, HpCurrent: 25, Ac: 16, SpeedFt: 30}
	snapJSON, _ := json.Marshal(snap)
	wild := refdata.Combatant{
		ID: combatantID, EncounterID: encounterID, DisplayName: "Keyleth",
		IsWildShaped:      true,
		WildShapeOriginal: pqtype.NullRawMessage{RawMessage: snapJSON, Valid: true},
		HpMax:             11, HpCurrent: 11, Ac: 13, IsAlive: true, IsNpc: false,
		Conditions: json.RawMessage(`[]`),
	}

	ms, _ := applyDamageMockStore()
	validTurnEncounter(ms, turnID)
	ms.getCombatantFn = func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return wild, nil }
	ms.updateCombatantWildShapeFn = func(_ context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, HpMax: arg.HpMax, HpCurrent: arg.HpCurrent, Ac: arg.Ac, IsAlive: true}, nil
	}
	logged := captureActionLog(ms)
	svc := NewService(ms)
	cl := &fakeCombatLog{}
	svc.SetCombatLogNotifier(cl)

	// 16 damage on an 11-HP beast form auto-reverts the druid; it is NOT a
	// dying drop-to-0.
	_, err := svc.ApplyDamage(context.Background(), ApplyDamageInput{
		EncounterID: encounterID, Target: wild, RawDamage: 16, DamageType: "fire",
	})
	require.NoError(t, err)

	assert.Empty(t, downedRows(*logged), "wild-shape revert is not a dying drop-to-0")
	assert.Empty(t, cl.all())
}
