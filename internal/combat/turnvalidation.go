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
func ValidateTurnOwnership(ctx context.Context, db *sql.DB, queries *refdata.Queries, encounterID uuid.UUID, discordUserID string) (TurnOwnerInfo, error) {
	// Get the active turn
	turn, err := queries.GetActiveTurnByEncounterID(ctx, encounterID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TurnOwnerInfo{}, ErrNoActiveTurn
		}
		return TurnOwnerInfo{}, fmt.Errorf("getting active turn: %w", err)
	}

	// Get the combatant
	combatant, err := queries.GetCombatant(ctx, turn.CombatantID)
	if err != nil {
		return TurnOwnerInfo{}, fmt.Errorf("getting combatant: %w", err)
	}

	// Get the campaign (for DM check)
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

// AcquireTurnLockWithValidation validates turn ownership and acquires the advisory lock.
// Returns the transaction for the caller to use (commit/rollback).
// Returns ErrNotYourTurn if the user doesn't own the turn.
// Returns ErrLockTimeout if the lock can't be acquired within 5 seconds.
func AcquireTurnLockWithValidation(ctx context.Context, db *sql.DB, queries *refdata.Queries, encounterID uuid.UUID, discordUserID string) (*sql.Tx, TurnOwnerInfo, error) {
	info, err := ValidateTurnOwnership(ctx, db, queries, encounterID, discordUserID)
	if err != nil {
		return nil, TurnOwnerInfo{}, err
	}

	tx, err := AcquireTurnLock(ctx, db, info.TurnID)
	if err != nil {
		return nil, TurnOwnerInfo{}, err
	}

	return tx, info, nil
}

// IsExemptCommand returns true if the command type is exempt from turn validation
// and lock acquisition (/reaction, /check, /save, /rest).
func IsExemptCommand(commandType string) bool {
	switch commandType {
	case "reaction", "check", "save", "rest":
		return true
	default:
		return false
	}
}
