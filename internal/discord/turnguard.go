package discord

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
)

// TurnGate gates a state-mutating slash command on turn ownership and the
// per-turn advisory lock (Phase 27). Handlers call one of the gate methods
// before any service write, so that:
//
//   - A player who does not own the active turn is rejected with
//     ErrNotYourTurn (the wrapped error explains whose turn it is).
//   - Two concurrent invocations on the same turn serialize through
//     pg_advisory_xact_lock, preventing a TOCTOU race that would otherwise
//     let two writers both pass the "is it my turn" check.
//
// AcquireAndRelease releases the lock as soon as validation succeeds; it is
// a read-only validator suitable for prompts / pre-confirmation flows where
// no DB write follows immediately. AcquireAndRun (F-4) holds the lock for
// the entire duration of a caller-supplied write callback so two concurrent
// handlers cannot interleave writes against the same turn — peers BLOCK at
// the lock acquisition until the holder's callback returns and the tx
// commits/rolls back, eliminating the lost-update race even when the
// callback's writes are committed via separate pooled connections.
type TurnGate interface {
	// AcquireAndRelease validates that discordUserID owns the active turn on
	// encounterID and acquires (then immediately releases) the advisory
	// lock. Returns the resolved TurnOwnerInfo on success. Errors are
	// pass-through from combat.AcquireTurnLockWithValidation: ErrNotYourTurn,
	// ErrNoActiveTurn, ErrLockTimeout, ErrTurnChanged, or wrapped DB errors.
	//
	// Prefer AcquireAndRun for any flow that performs a DB write after the
	// gate fires — AcquireAndRelease is intended for read-only validators
	// and pre-confirmation prompts (e.g. /move's initial Handle path which
	// only renders a Confirm/Cancel button, doing no writes).
	AcquireAndRelease(ctx context.Context, encounterID uuid.UUID, discordUserID string) (combat.TurnOwnerInfo, error)

	// AcquireAndRun validates turn ownership, acquires the per-turn advisory
	// lock, runs fn while the lock is still held, then commits the tx (which
	// releases the lock per pg_advisory_xact_lock semantics). If fn returns
	// an error the tx is rolled back and fn's error is returned verbatim;
	// validation/lock errors are returned BEFORE fn runs and propagate the
	// same combat.Err* sentinels as AcquireAndRelease.
	//
	// fn receives a context that the implementation MAY enrich with a
	// transaction handle so callers can opt into running their writes on the
	// lock-holding tx (use combat.TxFromContext to extract). Callers that do
	// NOT opt in still benefit from serialization: peers block at the lock
	// acquire and cannot interleave their writes against the same turn.
	AcquireAndRun(ctx context.Context, encounterID uuid.UUID, discordUserID string, fn func(ctx context.Context) error) (combat.TurnOwnerInfo, error)
}

// errAlreadyResponded is a sentinel returned by an AcquireAndRun callback
// when it already sent an ephemeral response to the user (e.g. a domain
// validation failure like "not enough movement"). The caller checks for
// this sentinel and skips the generic turn-gate translation, avoiding a
// double-response on the interaction.
var errAlreadyResponded = errors.New("already responded")

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
