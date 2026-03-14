package combat

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// SkipTurnNow completes the turn immediately (DM override).
func (s *Service) SkipTurnNow(ctx context.Context, turnID uuid.UUID) error {
	_, err := s.store.CompleteTurn(ctx, turnID)
	if err != nil {
		return fmt.Errorf("completing turn for skip: %w", err)
	}
	return nil
}

// ExtendTimer adds additional hours to the turn's timeout_at (DM override).
func (s *Service) ExtendTimer(ctx context.Context, turnID uuid.UUID, additionalHours int) error {
	turn, err := s.store.GetTurn(ctx, turnID)
	if err != nil {
		return fmt.Errorf("getting turn: %w", err)
	}
	if !turn.TimeoutAt.Valid {
		return fmt.Errorf("no timeout set on turn %s", turnID)
	}

	newTimeout := turn.TimeoutAt.Time.Add(time.Duration(additionalHours) * time.Hour)
	_, err = s.store.UpdateTurnTimeout(ctx, refdata.UpdateTurnTimeoutParams{
		ID:        turnID,
		TimeoutAt: sql.NullTime{Time: newTimeout, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("updating turn timeout: %w", err)
	}
	return nil
}

// PauseCombatTimers freezes all timers for an encounter by clearing timeout_at on active turns.
func (s *Service) PauseCombatTimers(ctx context.Context, encounterID uuid.UUID) error {
	turns, err := s.store.ListActiveTurnsByEncounterID(ctx, encounterID)
	if err != nil {
		return fmt.Errorf("listing active turns: %w", err)
	}

	for _, turn := range turns {
		if _, err := s.store.ClearTurnTimeout(ctx, turn.ID); err != nil {
			return fmt.Errorf("clearing timeout for turn %s: %w", turn.ID, err)
		}
	}
	return nil
}

// ResumeCombatTimers re-sets timeout_at on active turns from now + timeoutHours.
func (s *Service) ResumeCombatTimers(ctx context.Context, encounterID uuid.UUID, timeoutHours int) error {
	turns, err := s.store.ListActiveTurnsByEncounterID(ctx, encounterID)
	if err != nil {
		return fmt.Errorf("listing active turns: %w", err)
	}

	newTimeout := time.Now().Add(time.Duration(timeoutHours) * time.Hour)
	for _, turn := range turns {
		if _, err := s.store.SetTurnTimeout(ctx, refdata.SetTurnTimeoutParams{
			ID:        turn.ID,
			TimeoutAt: sql.NullTime{Time: newTimeout, Valid: true},
		}); err != nil {
			return fmt.Errorf("setting timeout for turn %s: %w", turn.ID, err)
		}
	}
	return nil
}
