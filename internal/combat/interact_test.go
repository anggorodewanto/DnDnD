package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestInteract_FirstFreeAutoResolvable(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	svc := NewService(ms)
	result, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "draw longsword",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.FreeInteractUsed)
	assert.False(t, result.Turn.ActionUsed, "first interact should not cost action")
	assert.True(t, result.AutoResolved, "draw weapon should auto-resolve")
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "draw longsword")
	assert.Empty(t, result.DMQueueMessage, "auto-resolved should not go to DM queue")
}

func TestInteract_FirstFreeDMQueue(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.createPendingActionFn = func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			ActionText:  arg.ActionText,
			Status:      "pending",
		}, nil
	}

	svc := NewService(ms)
	result, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "search the chest for traps",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.FreeInteractUsed)
	assert.False(t, result.Turn.ActionUsed, "first interact should not cost action")
	assert.False(t, result.AutoResolved, "non-standard interaction should go to DM")
	assert.Contains(t, result.DMQueueMessage, "Thorn")
	assert.Contains(t, result.DMQueueMessage, "search the chest for traps")
	assert.Equal(t, "pending", result.PendingAction.Status)
}

func TestInteract_SecondCostsAction(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID
	turn.FreeInteractUsed = true // first interact already used

	svc := NewService(ms)
	result, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "draw dagger",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed, "second interact should cost the action")
	assert.True(t, result.Turn.FreeInteractUsed)
	assert.True(t, result.AutoResolved)
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "draw dagger")
}

func TestInteract_SecondRejectedWhenActionSpent(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID
	turn.FreeInteractUsed = true
	turn.ActionUsed = true

	svc := NewService(ms)
	_, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "draw dagger",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Free interaction already used and action is spent")
}

func TestInteract_CannotActWhileIncapacitated(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	combatant.Conditions = json.RawMessage(`[{"condition":"incapacitated"}]`)
	turn := makeBasicTurn()

	svc := NewService(ms)
	_, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "draw longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "incapacitated")
}

func TestInteract_UpdateTurnActionsFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "draw longsword",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating turn actions")
}

func TestInteract_CreatePendingActionFails(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.createPendingActionFn = func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "search the chest for traps",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating pending action")
}

func TestInteract_AutoResolvablePatterns(t *testing.T) {
	tests := []struct {
		desc     string
		expected bool
	}{
		{"draw longsword", true},
		{"Draw Shortsword", true},
		{"sheathe rapier", true},
		{"sheath my sword", true},
		{"stow shield", true},
		{"open the door", true},
		{"close the chest", true},
		{"pick up the key", true},
		{"grab the rope", true},
		{"pull out a potion", true},
		{"put away my wand", true},
		{"search the chest for traps", false},
		{"disarm the trap", false},
		{"light a torch", false},
		{"read the inscription", false},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expected, isAutoResolvable(tc.desc))
		})
	}
}

func TestInteract_DMQueueMessageFormat(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID

	ms.createPendingActionFn = func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			ActionText:  arg.ActionText,
			Status:      "pending",
		}, nil
	}

	svc := NewService(ms)
	result, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "search the chest",
	})

	require.NoError(t, err)
	assert.Equal(t, `🤚 **Interact** — Thorn: "search the chest"`, result.DMQueueMessage)
	assert.Equal(t, `🤚 Thorn: "search the chest" — sent to DM queue`, result.CombatLog)
}

func TestInteract_SecondInteractDMQueue(t *testing.T) {
	_, combatantID, _, ms := makeStdTestSetup()
	encounterID := uuid.New()

	combatant := makeNPCCombatant(combatantID, encounterID, "Thorn")
	turn := makeBasicTurn()
	turn.EncounterID = encounterID
	turn.CombatantID = combatantID
	turn.FreeInteractUsed = true // first interact already used

	ms.createPendingActionFn = func(ctx context.Context, arg refdata.CreatePendingActionParams) (refdata.PendingAction, error) {
		return refdata.PendingAction{
			ID:          uuid.New(),
			EncounterID: arg.EncounterID,
			CombatantID: arg.CombatantID,
			ActionText:  arg.ActionText,
			Status:      "pending",
		}, nil
	}

	svc := NewService(ms)
	result, err := svc.Interact(context.Background(), InteractCommand{
		Combatant:   combatant,
		Turn:        turn,
		Description: "search the chest",
	})

	require.NoError(t, err)
	assert.True(t, result.Turn.ActionUsed, "second interact costs action")
	assert.False(t, result.AutoResolved)
	assert.NotEmpty(t, result.DMQueueMessage)
}
