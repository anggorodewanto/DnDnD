package combat

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: DeclareReaction creates a reaction declaration ---

func TestDeclareReaction_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.createReactionDeclarationFn = func(ctx context.Context, arg refdata.CreateReactionDeclarationParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, encounterID, arg.EncounterID)
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, "Shield if I get hit", arg.Description)
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Description: "Shield if I get hit",
			Status:      "active",
		}, nil
	}

	svc := NewService(store)
	decl, err := svc.DeclareReaction(context.Background(), encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)
	assert.Equal(t, declID, decl.ID)
	assert.Equal(t, "active", decl.Status)
	assert.Equal(t, "Shield if I get hit", decl.Description)
}

func TestDeclareReaction_EmptyDescription(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.DeclareReaction(context.Background(), uuid.New(), uuid.New(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description must not be empty")
}

func TestDeclareReaction_WhitespaceOnlyDescription(t *testing.T) {
	store := defaultMockStore()
	svc := NewService(store)

	_, err := svc.DeclareReaction(context.Background(), uuid.New(), uuid.New(), "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description must not be empty")
}

// --- TDD Cycle 2: CancelReaction cancels a declaration ---

func TestCancelReaction_Success(t *testing.T) {
	declID := uuid.New()

	store := defaultMockStore()
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, declID, id)
		return refdata.ReactionDeclaration{ID: declID, Status: "cancelled"}, nil
	}

	svc := NewService(store)
	decl, err := svc.CancelReaction(context.Background(), declID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", decl.Status)
}

func TestCancelReaction_NotFound(t *testing.T) {
	store := defaultMockStore()
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, sql.ErrNoRows
	}

	svc := NewService(store)
	_, err := svc.CancelReaction(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- TDD Cycle 3: CancelReactionByDescription cancels matching declaration ---

func TestCancelReactionByDescription_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: declID, Description: "Shield if I get hit", Status: "active"},
			{ID: uuid.New(), Description: "Counterspell if enemy casts", Status: "active"},
		}, nil
	}
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, declID, id)
		return refdata.ReactionDeclaration{ID: declID, Status: "cancelled"}, nil
	}

	svc := NewService(store)
	decl, err := svc.CancelReactionByDescription(context.Background(), combatantID, encounterID, "shield")
	require.NoError(t, err)
	assert.Equal(t, declID, decl.ID)
}

func TestCancelReactionByDescription_NoMatch(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), Description: "Shield if I get hit", Status: "active"},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.CancelReactionByDescription(context.Background(), combatantID, encounterID, "counterspell")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active reaction matching")
}

func TestCancelReactionByDescription_ListError(t *testing.T) {
	store := defaultMockStore()
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.CancelReactionByDescription(context.Background(), uuid.New(), uuid.New(), "shield")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing active reactions")
}

func TestResolveReaction_GetActiveTurnError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, EncounterID: encounterID, Status: "active"}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("no active turn")
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting active turn")
}

func TestResolveReaction_UpdateTurnError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, EncounterID: encounterID, CombatantID: combatantID, Status: "active"}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, errors.New("update failed")
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marking reaction used on turn")
}

func TestResolveReaction_UpdateDeclarationError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, EncounterID: encounterID, CombatantID: combatantID, Status: "active"}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 1, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("update failed")
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating reaction status to used")
}

func TestResolveReaction_GetDeclarationError(t *testing.T) {
	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{}, errors.New("not found")
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting reaction declaration")
}

// --- TDD Cycle 4: CancelAllReactions ---

func TestCancelAllReactions_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var called bool
	store := defaultMockStore()
	store.cancelAllReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.CancelAllReactionDeclarationsByCombatantParams) error {
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, encounterID, arg.EncounterID)
		called = true
		return nil
	}

	svc := NewService(store)
	err := svc.CancelAllReactions(context.Background(), combatantID, encounterID)
	require.NoError(t, err)
	assert.True(t, called)
}

// --- TDD Cycle 5: ResolveReaction marks declaration used and sets turn reaction_used ---

func TestResolveReaction_Success(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Status:      "active",
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:          turnID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			RoundNumber: 2,
			Status:      "active",
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 2, Status: "active"},
		}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		assert.Equal(t, declID, arg.ID)
		assert.Equal(t, int32(2), arg.UsedOnRound.Int32)
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		assert.Equal(t, turnID, arg.ID)
		assert.True(t, arg.ReactionUsed)
		return refdata.Turn{ID: turnID, ReactionUsed: true}, nil
	}

	svc := NewService(store)
	decl, err := svc.ResolveReaction(context.Background(), declID)
	require.NoError(t, err)
	assert.Equal(t, "used", decl.Status)
}

func TestResolveReaction_AlreadyUsed(t *testing.T) {
	declID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrReactionNotActive)
}

func TestResolveReaction_Cancelled(t *testing.T) {
	declID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "cancelled"}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrReactionNotActive)
}

// --- TDD Cycle: ResolveReaction rejects if combatant already used reaction this round ---

func TestResolveReaction_ReactionAlreadyUsedThisRound(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Status:      "active",
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 2, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 2, Status: "completed", ReactionUsed: true},
		}, nil
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrReactionAlreadyUsed)
}

func TestResolveReaction_NoTurnForDeclarant(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: combatantID,
			Status:      "active",
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil // no turn for this combatant
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no turn found for declaring combatant")
}

func TestResolveReaction_ListTurnsError(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, EncounterID: encounterID, CombatantID: uuid.New(), Status: "active"}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, RoundNumber: 1, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, errors.New("db error")
	}

	svc := NewService(store)
	_, err := svc.ResolveReaction(context.Background(), declID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing turns for round")
}

// --- TDD Cycle: ResolveReaction marks the declaring combatant's turn, not the active turn ---

func TestResolveReaction_MarksDeclaringCombatantTurn(t *testing.T) {
	declID := uuid.New()
	encounterID := uuid.New()
	aragornID := uuid.New()  // declaring combatant
	goblinID := uuid.New()   // active turn combatant (different!)
	aragornTurnID := uuid.New()
	goblinTurnID := uuid.New()

	store := defaultMockStore()
	store.getReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{
			ID:          declID,
			EncounterID: encounterID,
			CombatantID: aragornID,
			Status:      "active",
		}, nil
	}
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{
			ID:          goblinTurnID,
			EncounterID: encounterID,
			CombatantID: goblinID,
			RoundNumber: 2,
			Status:      "active",
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		assert.Equal(t, encounterID, arg.EncounterID)
		assert.Equal(t, int32(2), arg.RoundNumber)
		return []refdata.Turn{
			{ID: aragornTurnID, EncounterID: encounterID, CombatantID: aragornID, RoundNumber: 2, Status: "completed"},
			{ID: goblinTurnID, EncounterID: encounterID, CombatantID: goblinID, RoundNumber: 2, Status: "active"},
		}, nil
	}
	store.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		// Must update Aragorn's turn, NOT goblin's
		assert.Equal(t, aragornTurnID, arg.ID, "should mark reaction_used on declaring combatant's turn")
		assert.True(t, arg.ReactionUsed)
		return refdata.Turn{ID: aragornTurnID, ReactionUsed: true}, nil
	}
	store.updateReactionDeclarationStatusUsedFn = func(ctx context.Context, arg refdata.UpdateReactionDeclarationStatusUsedParams) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: declID, Status: "used"}, nil
	}

	svc := NewService(store)
	decl, err := svc.ResolveReaction(context.Background(), declID)
	require.NoError(t, err)
	assert.Equal(t, "used", decl.Status)
}

// --- TDD Cycle 6: ListActiveReactions ---

func TestListActiveReactions_Success(t *testing.T) {
	encounterID := uuid.New()

	store := defaultMockStore()
	store.listActiveReactionDeclarationsByEncounterFn = func(ctx context.Context, encID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
		assert.Equal(t, encounterID, encID)
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), Description: "Shield if I get hit", Status: "active"},
			{ID: uuid.New(), Description: "Counterspell if enemy casts", Status: "active"},
		}, nil
	}

	svc := NewService(store)
	decls, err := svc.ListActiveReactions(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Len(t, decls, 2)
}

// --- TDD Cycle 7: ListReactionsByCombatant ---

func TestListReactionsByCombatant_Success(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.listReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		assert.Equal(t, combatantID, arg.CombatantID)
		assert.Equal(t, encounterID, arg.EncounterID)
		return []refdata.ReactionDeclaration{
			{ID: uuid.New(), Description: "Shield if I get hit"},
		}, nil
	}

	svc := NewService(store)
	decls, err := svc.ListReactionsByCombatant(context.Background(), combatantID, encounterID)
	require.NoError(t, err)
	assert.Len(t, decls, 1)
}

// --- TDD Cycle 8: CleanupReactionsOnEncounterEnd ---

// --- TDD Cycle: CanDeclareReaction pre-declare validation ---

func TestCanDeclareReaction_Available(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 3, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 3, ReactionUsed: false},
		}, nil
	}

	svc := NewService(store)
	ok, err := svc.CanDeclareReaction(context.Background(), encounterID, combatantID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanDeclareReaction_AlreadyUsedThisRound(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	turnID := uuid.New()

	store := defaultMockStore()
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 3, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: turnID, EncounterID: encounterID, CombatantID: combatantID, RoundNumber: 3, ReactionUsed: true},
		}, nil
	}

	svc := NewService(store)
	ok, err := svc.CanDeclareReaction(context.Background(), encounterID, combatantID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCanDeclareReaction_NoTurnForCombatantAllowsDeclare(t *testing.T) {
	// Combatant may not have a turn row yet (e.g. declaring from outside of
	// their own turn). Without evidence of reaction_used=true, allow declare.
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 3, Status: "active"}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: uuid.New(), EncounterID: encounterID, CombatantID: uuid.New(), RoundNumber: 3, ReactionUsed: false},
		}, nil
	}

	svc := NewService(store)
	ok, err := svc.CanDeclareReaction(context.Background(), encounterID, combatantID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanDeclareReaction_NoActiveTurnAllowsDeclare(t *testing.T) {
	// When no active turn exists (e.g. encounter between rounds), default to
	// permitting the declaration — the resolve path is authoritative.
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{}, sql.ErrNoRows
	}

	svc := NewService(store)
	ok, err := svc.CanDeclareReaction(context.Background(), encounterID, combatantID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanDeclareReaction_ListTurnsError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getActiveTurnByEncounterIDFn = func(ctx context.Context, encID uuid.UUID) (refdata.Turn, error) {
		return refdata.Turn{RoundNumber: 1}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return nil, errors.New("boom")
	}

	svc := NewService(store)
	_, err := svc.CanDeclareReaction(context.Background(), encounterID, combatantID)
	require.Error(t, err)
}

func TestCleanupReactionsOnEncounterEnd_Success(t *testing.T) {
	encounterID := uuid.New()

	var called bool
	store := defaultMockStore()
	store.deleteReactionDeclarationsByEncounterFn = func(ctx context.Context, encID uuid.UUID) error {
		assert.Equal(t, encounterID, encID)
		called = true
		return nil
	}

	svc := NewService(store)
	err := svc.CleanupReactionsOnEncounterEnd(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, called)
}
