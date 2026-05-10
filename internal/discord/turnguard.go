package discord

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
)

// TurnGate gates a state-mutating slash command on turn ownership and the
// per-turn advisory lock (Phase 27). Handlers call AcquireAndRelease at the
// top, before any service write, so that:
//
//   - A player who does not own the active turn is rejected with
//     ErrNotYourTurn (the wrapped error explains whose turn it is).
//   - Two concurrent invocations on the same turn serialize through
//     pg_advisory_xact_lock, preventing a TOCTOU race that would otherwise
//     let two writers both pass the "is it my turn" check.
//
// The gate releases the lock as soon as validation succeeds — production
// handlers do their writes outside the held lock, but the contract still
// guarantees the wrong-owner gate has fired. A future patch can extend the
// gate to hold the tx across the write; this minimal form closes the
// chunk3 cross-cutting risk #2 (out-of-turn writes from Discord) without
// rewriting every handler's persistence layer.
type TurnGate interface {
	// AcquireAndRelease validates that discordUserID owns the active turn on
	// encounterID and acquires (then immediately releases) the advisory
	// lock. Returns the resolved TurnOwnerInfo on success. Errors are
	// pass-through from combat.AcquireTurnLockWithValidation: ErrNotYourTurn,
	// ErrNoActiveTurn, ErrLockTimeout, ErrTurnChanged, or wrapped DB errors.
	AcquireAndRelease(ctx context.Context, encounterID uuid.UUID, discordUserID string) (combat.TurnOwnerInfo, error)
}

// formatTurnGateError converts a TurnGate error into the user-visible
// ephemeral string. Centralized so /move, /fly and any future gated handler
// surface identical wording to players.
func formatTurnGateError(err error) string {
	if err == nil {
		return ""
	}
	var notYourTurn *combat.ErrNotYourTurn
	if errors.As(err, &notYourTurn) {
		return notYourTurn.Error()
	}
	if errors.Is(err, combat.ErrNoActiveTurn) {
		return "No active turn."
	}
	if errors.Is(err, combat.ErrLockTimeout) {
		return "The server is busy processing the active turn — please try again."
	}
	if errors.Is(err, combat.ErrTurnChanged) {
		return "It's no longer your turn."
	}
	return "Failed to validate turn ownership."
}
