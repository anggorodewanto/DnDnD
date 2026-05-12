package combat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ErrNotYourTurn is returned when a player tries to act on a turn that isn't theirs.
type ErrNotYourTurn struct {
	CurrentCharacterName string
	CurrentDiscordUserID string
}

func (e *ErrNotYourTurn) Error() string {
	return fmt.Sprintf("It's not your turn. Current turn: **%s** (@%s)", e.CurrentCharacterName, e.CurrentDiscordUserID)
}

// ErrNoActiveTurn is returned when there is no active turn for the encounter.
var ErrNoActiveTurn = errors.New("no active turn for this encounter")

// TurnValidationQuerier abstracts the query methods needed for turn validation,
// allowing mock implementations for testing.
type TurnValidationQuerier interface {
	GetActiveTurnByEncounterID(ctx context.Context, encounterID uuid.UUID) (refdata.Turn, error)
	GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	GetCampaignByEncounterID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
	GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
	WithTx(tx *sql.Tx) *refdata.Queries
}

// TurnOwnerInfo holds information about the current turn owner for validation.
type TurnOwnerInfo struct {
	TurnID       uuid.UUID
	CombatantID  uuid.UUID
	CharacterID  uuid.NullUUID
	DisplayName  string
	OwnerUserID  string // discord_user_id of the character owner, empty for NPCs
	DMUserID     string // dm_user_id of the campaign
	IsNPC        bool
}

// ValidateTurnOwnership checks if the given discord user is allowed to act on the current turn.
// Returns TurnOwnerInfo on success. Returns ErrNotYourTurn if the user doesn't own this turn.
// The DM (campaign.dm_user_id) can always act. NPCs can only be controlled by the DM.
func ValidateTurnOwnership(ctx context.Context, queries TurnValidationQuerier, encounterID uuid.UUID, discordUserID string) (TurnOwnerInfo, error) {
	turn, err := queries.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TurnOwnerInfo{}, ErrNoActiveTurn
		}
		return TurnOwnerInfo{}, fmt.Errorf("getting active turn: %w", err)
	}

	combatant, err := queries.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return TurnOwnerInfo{}, fmt.Errorf("getting combatant: %w", err)
	}

	campaign, err := queries.GetCampaignByEncounterID(ctx, encounterID)
	if err != nil {
		return TurnOwnerInfo{}, fmt.Errorf("getting campaign: %w", err)
	}

	info := TurnOwnerInfo{
		TurnID:      turn.ID,
		CombatantID: combatant.ID,
		CharacterID: combatant.CharacterID,
		DisplayName: combatant.DisplayName,
		DMUserID:    campaign.DmUserID,
		IsNPC:       combatant.IsNpc,
	}

	// DM can always act
	if discordUserID == campaign.DmUserID {
		return info, nil
	}

	// NPC turn: only DM can act
	if combatant.IsNpc || !combatant.CharacterID.Valid {
		return TurnOwnerInfo{}, &ErrNotYourTurn{
			CurrentCharacterName: combatant.DisplayName,
			CurrentDiscordUserID: campaign.DmUserID,
		}
	}

	// PC turn: look up the player_character to find the owner
	pc, err := queries.GetPlayerCharacterByCharacter(ctx, refdata.GetPlayerCharacterByCharacterParams{
		CampaignID:  campaign.ID,
		CharacterID: combatant.CharacterID.UUID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Character not linked to any player - only DM can act
			return TurnOwnerInfo{}, &ErrNotYourTurn{
				CurrentCharacterName: combatant.DisplayName,
				CurrentDiscordUserID: campaign.DmUserID,
			}
		}
		return TurnOwnerInfo{}, fmt.Errorf("getting player character: %w", err)
	}

	info.OwnerUserID = pc.DiscordUserID

	if discordUserID != pc.DiscordUserID {
		return TurnOwnerInfo{}, &ErrNotYourTurn{
			CurrentCharacterName: combatant.DisplayName,
			CurrentDiscordUserID: pc.DiscordUserID,
		}
	}

	return info, nil
}

// ErrTurnChanged is returned when the active turn changed between validation and lock acquisition.
var ErrTurnChanged = errors.New("it's no longer your turn")

// RunUnderTurnLock validates ownership, acquires the per-turn advisory
// lock, runs fn while the lock is still held, then commits the tx (releasing
// the lock per pg_advisory_xact_lock semantics). If fn returns an error the
// tx is rolled back and fn's error propagates verbatim; validation / lock
// errors are returned BEFORE fn runs and are the same combat.Err* sentinels
// as AcquireTurnLockWithValidation.
//
// fn's context carries the lock-holding *sql.Tx (via ContextWithTx). Callers
// that want their writes to share the lock-holding tx call TxFromContext and
// pass the result to refdata.Queries.WithTx; callers that don't opt in still
// get serialization — peers block at the pg_advisory_xact_lock acquire until
// our tx commits/rolls back. (F-4)
func RunUnderTurnLock(ctx context.Context, db TxBeginner, queries TurnValidationQuerier, encounterID uuid.UUID, discordUserID string, fn func(ctx context.Context) error) (TurnOwnerInfo, error) {
	if fn == nil {
		return TurnOwnerInfo{}, fmt.Errorf("RunUnderTurnLock: fn must not be nil")
	}
	tx, info, err := AcquireTurnLockWithValidation(ctx, db, queries, encounterID, discordUserID)
	if err != nil {
		return TurnOwnerInfo{}, err
	}

	txCtx := ContextWithTx(ctx, tx)
	runErr := func() (runErr error) {
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("RunUnderTurnLock: fn panicked: %v", r)
			}
		}()
		return fn(txCtx)
	}()

	if runErr != nil {
		_ = tx.Rollback()
		return TurnOwnerInfo{}, runErr
	}
	if commitErr := tx.Commit(); commitErr != nil {
		return TurnOwnerInfo{}, commitErr
	}
	return info, nil
}

// AcquireTurnLockWithValidation validates turn ownership and acquires the advisory lock.
// After acquiring the lock, it re-validates that the turn hasn't changed (TOCTOU protection).
// Returns the transaction for the caller to use (commit/rollback).
// Returns ErrNotYourTurn if the user doesn't own the turn.
// Returns ErrTurnChanged if the turn changed after validation (e.g., DM ended the turn).
// Returns ErrLockTimeout if the lock can't be acquired within 5 seconds.
func AcquireTurnLockWithValidation(ctx context.Context, db TxBeginner, queries TurnValidationQuerier, encounterID uuid.UUID, discordUserID string) (*sql.Tx, TurnOwnerInfo, error) {
	info, err := ValidateTurnOwnership(ctx, queries, encounterID, discordUserID)
	if err != nil {
		return nil, TurnOwnerInfo{}, err
	}

	tx, err := AcquireTurnLock(ctx, db, info.TurnID)
	if err != nil {
		return nil, TurnOwnerInfo{}, err
	}

	// Re-validate within the transaction to prevent TOCTOU race.
	// The DM could have ended this turn between our initial validation and lock acquisition.
	txQueries := queries.WithTx(tx)
	currentTurn, err := txQueries.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, TurnOwnerInfo{}, ErrTurnChanged
		}
		return nil, TurnOwnerInfo{}, fmt.Errorf("re-validating active turn: %w", err)
	}

	if currentTurn.ID != info.TurnID {
		tx.Rollback()
		return nil, TurnOwnerInfo{}, ErrTurnChanged
	}

	return tx, info, nil
}

// IsExemptCommand returns true if the command type is exempt from advisory
// turn-lock acquisition. The exempt set is the spec's read-only / off-turn
// commands: /reaction (declared during another's turn), /check, /save, /rest,
// and /distance (purely informational — no DB writes). Exempt commands MAY
// still call ValidateTurnOwnership softly to gate by encounter membership,
// but they SHOULD NOT take the per-turn advisory lock so the active player
// is not blocked from moving while a peer is just measuring range.
func IsExemptCommand(commandType string) bool {
	switch commandType {
	case "reaction", "check", "save", "rest", "distance":
		return true
	default:
		return false
	}
}
