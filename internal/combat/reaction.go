package combat

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ErrReactionNotActive is returned when trying to resolve a non-active reaction declaration.
var ErrReactionNotActive = fmt.Errorf("reaction declaration is not active")

// ErrReactionAlreadyUsed is returned when a combatant tries to use a reaction but already used one this round.
var ErrReactionAlreadyUsed = fmt.Errorf("reaction already used this round")

// DeclareReaction creates a new reaction declaration for a combatant in an encounter.
func (s *Service) DeclareReaction(ctx context.Context, encounterID, combatantID uuid.UUID, description string) (refdata.ReactionDeclaration, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return refdata.ReactionDeclaration{}, fmt.Errorf("description must not be empty")
	}

	return s.store.CreateReactionDeclaration(ctx, refdata.CreateReactionDeclarationParams{
		EncounterID: encounterID,
		CombatantID: combatantID,
		Description: description,
	})
}

// CancelReaction cancels a specific reaction declaration by ID.
func (s *Service) CancelReaction(ctx context.Context, declarationID uuid.UUID) (refdata.ReactionDeclaration, error) {
	decl, err := s.store.CancelReactionDeclaration(ctx, declarationID)
	if err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("cancelling reaction: %w", err)
	}
	return decl, nil
}

// CancelReactionByDescription cancels the first active declaration whose description
// contains the given substring (case-insensitive).
func (s *Service) CancelReactionByDescription(ctx context.Context, combatantID, encounterID uuid.UUID, descSubstring string) (refdata.ReactionDeclaration, error) {
	active, err := s.store.ListActiveReactionDeclarationsByCombatant(ctx, refdata.ListActiveReactionDeclarationsByCombatantParams{
		CombatantID: combatantID,
		EncounterID: encounterID,
	})
	if err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("listing active reactions: %w", err)
	}

	lower := strings.ToLower(descSubstring)
	for _, d := range active {
		if strings.Contains(strings.ToLower(d.Description), lower) {
			return s.store.CancelReactionDeclaration(ctx, d.ID)
		}
	}

	return refdata.ReactionDeclaration{}, fmt.Errorf("no active reaction matching %q", descSubstring)
}

// CancelAllReactions cancels all active reaction declarations for a combatant in an encounter.
func (s *Service) CancelAllReactions(ctx context.Context, combatantID, encounterID uuid.UUID) error {
	return s.store.CancelAllReactionDeclarationsByCombatant(ctx, refdata.CancelAllReactionDeclarationsByCombatantParams{
		CombatantID: combatantID,
		EncounterID: encounterID,
	})
}

// ResolveReaction marks a reaction declaration as used (DM resolves it),
// and sets reaction_used=true on the combatant's current turn.
func (s *Service) ResolveReaction(ctx context.Context, declarationID uuid.UUID, roundNumber int32) (refdata.ReactionDeclaration, error) {
	decl, err := s.store.GetReactionDeclaration(ctx, declarationID)
	if err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("getting reaction declaration: %w", err)
	}
	if decl.Status != "active" {
		return refdata.ReactionDeclaration{}, fmt.Errorf("status=%q: %w", decl.Status, ErrReactionNotActive)
	}

	// Find the declaring combatant's turn for this round to mark reaction_used
	activeTurn, err := s.store.GetActiveTurnByEncounterID(ctx, decl.EncounterID)
	if err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("getting active turn: %w", err)
	}

	turns, err := s.store.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: decl.EncounterID,
		RoundNumber: activeTurn.RoundNumber,
	})
	if err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("listing turns for round: %w", err)
	}

	var declarantTurn *refdata.Turn
	for i := range turns {
		if turns[i].CombatantID == decl.CombatantID {
			declarantTurn = &turns[i]
			break
		}
	}
	if declarantTurn == nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("no turn found for declaring combatant in current round")
	}

	// Check if this combatant already used their reaction this round
	if declarantTurn.ReactionUsed {
		return refdata.ReactionDeclaration{}, fmt.Errorf("combatant already used reaction this round: %w", ErrReactionAlreadyUsed)
	}

	// Mark the declaring combatant's turn's reaction as used
	declarantTurn.ReactionUsed = true
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(*declarantTurn)); err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("marking reaction used on turn: %w", err)
	}

	// Mark the declaration as used
	resolved, err := s.store.UpdateReactionDeclarationStatusUsed(ctx, refdata.UpdateReactionDeclarationStatusUsedParams{
		ID:          declarationID,
		UsedOnRound: sql.NullInt32{Int32: roundNumber, Valid: true},
	})
	if err != nil {
		return refdata.ReactionDeclaration{}, fmt.Errorf("updating reaction status to used: %w", err)
	}

	return resolved, nil
}

// ListActiveReactions returns all active reaction declarations for an encounter.
func (s *Service) ListActiveReactions(ctx context.Context, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
	return s.store.ListActiveReactionDeclarationsByEncounter(ctx, encounterID)
}

// ListReactionsByCombatant returns all reaction declarations for a combatant in an encounter.
func (s *Service) ListReactionsByCombatant(ctx context.Context, combatantID, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error) {
	return s.store.ListReactionDeclarationsByCombatant(ctx, refdata.ListReactionDeclarationsByCombatantParams{
		CombatantID: combatantID,
		EncounterID: encounterID,
	})
}

// CleanupReactionsOnEncounterEnd deletes all reaction declarations for an encounter.
func (s *Service) CleanupReactionsOnEncounterEnd(ctx context.Context, encounterID uuid.UUID) error {
	return s.store.DeleteReactionDeclarationsByEncounter(ctx, encounterID)
}
