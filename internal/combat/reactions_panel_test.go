package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: ListReactionsForPanel returns enriched reaction data ---

func TestListReactionsForPanel_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	reactionID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		assert.Equal(t, encounterID, eid)
		return []refdata.ReactionDeclaration{
			{
				ID:          reactionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Description: "Shield if I get hit",
				Status:      "active",
			},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{EncounterID: encounterID, RoundNumber: 1, CombatantID: uuid.New()}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{CombatantID: combatantID, ReactionUsed: false},
		}, nil
	}

	svc := NewService(store)
	result, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, reactionID, result[0].ID)
	assert.Equal(t, "Shield if I get hit", result[0].Description)
	assert.Equal(t, "active", result[0].Status)
	assert.Equal(t, "Aragorn", result[0].CombatantDisplayName)
	assert.Equal(t, "AR", result[0].CombatantShortID)
	assert.False(t, result[0].ReactionUsedThisRound)
	assert.False(t, result[0].IsReadiedAction)
}

// --- TDD Cycle 2: Dormant status when combatant already used reaction this round ---

func TestListReactionsForPanel_DormantWhenReactionUsed(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	reactionID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{
				ID:          reactionID,
				EncounterID: encounterID,
				CombatantID: combatantID,
				Description: "Shield if I get hit",
				Status:      "active",
			},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{EncounterID: encounterID, RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{CombatantID: combatantID, ReactionUsed: true},
		}, nil
	}

	svc := NewService(store)
	result, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].ReactionUsedThisRound)
	assert.Equal(t, "active", result[0].Status)
}

// --- TDD Cycle 3: Empty reactions returns empty list ---

func TestListReactionsForPanel_Empty(t *testing.T) {
	encounterID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{}, nil
	}

	svc := NewService(store)
	result, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

// --- TDD Cycle 4: Used reactions are included ---

func TestListReactionsForPanel_IncludesUsedReactions(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	activeID := uuid.New()
	usedID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: activeID, EncounterID: encounterID, CombatantID: combatantID, Description: "Shield", Status: "active"},
			{ID: usedID, EncounterID: encounterID, CombatantID: combatantID, Description: "Counterspell", Status: "used"},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{EncounterID: encounterID, RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{CombatantID: combatantID, ReactionUsed: true}}, nil
	}

	svc := NewService(store)
	result, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "active", result[0].Status)
	assert.Equal(t, "used", result[1].Status)
}

// --- TDD Cycle 5: No active turn (encounter just started) still works ---

func TestListReactionsForPanel_NoActiveTurn(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	reactionID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: reactionID, EncounterID: encounterID, CombatantID: combatantID, Description: "Shield", Status: "active"},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, sql.ErrNoRows
	}

	svc := NewService(store)
	result, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.False(t, result[0].ReactionUsedThisRound)
}

// --- TDD Cycle 6: Readied action flag ---

func TestListReactionsForPanel_ReadiedAction(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	reactionID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: reactionID, EncounterID: encounterID, CombatantID: combatantID, Description: "Ready: attack when enemy approaches", Status: "active", IsReadiedAction: true},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{EncounterID: encounterID, RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{{CombatantID: combatantID, ReactionUsed: false}}, nil
	}

	svc := NewService(store)
	result, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.True(t, result[0].IsReadiedAction)
}

// --- TDD Cycle 7: Store error propagation ---

func TestListReactionsForPanel_StoreError(t *testing.T) {
	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return nil, assert.AnError
	}

	svc := NewService(store)
	_, err := svc.ListReactionsForPanel(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestListReactionsForPanel_CombatantStoreError(t *testing.T) {
	encounterID := uuid.New()
	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), Description: "Shield", Status: "active"},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return nil, assert.AnError
	}

	svc := NewService(store)
	_, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.Error(t, err)
}

func TestListReactionsForPanel_ActiveTurnErrorNonErrNoRows(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	store := defaultMockStore()
	store.listReactionDeclarationsByEncounterFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: combatantID, Description: "Shield", Status: "active"},
		}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{ID: combatantID, ShortID: "AR", DisplayName: "Aragorn", Conditions: json.RawMessage(`[]`)},
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, assert.AnError
	}

	svc := NewService(store)
	_, err := svc.ListReactionsForPanel(context.Background(), encounterID)
	require.Error(t, err)
}
