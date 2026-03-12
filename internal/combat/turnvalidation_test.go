package combat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

type mockQuerier struct {
	activeTurn    refdata.Turn
	activeTurnErr error

	combatant    refdata.Combatant
	combatantErr error

	campaign    refdata.Campaign
	campaignErr error

	playerChar    refdata.PlayerCharacter
	playerCharErr error
}

func (m *mockQuerier) GetActiveTurnByEncounterID(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
	return m.activeTurn, m.activeTurnErr
}

func (m *mockQuerier) GetCombatant(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
	return m.combatant, m.combatantErr
}

func (m *mockQuerier) GetCampaignByEncounterID(_ context.Context, _ uuid.UUID) (refdata.Campaign, error) {
	return m.campaign, m.campaignErr
}

func (m *mockQuerier) GetPlayerCharacterByCharacter(_ context.Context, _ refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	return m.playerChar, m.playerCharErr
}

func (m *mockQuerier) WithTx(_ *sql.Tx) *refdata.Queries {
	return nil
}

// --- Tests for isLockTimeoutError ---

func TestIsLockTimeoutError_NilError(t *testing.T) {
	assert.False(t, isLockTimeoutError(nil))
}

func TestIsLockTimeoutError_PQErrorCode55P03(t *testing.T) {
	pqErr := &pgconn.PgError{Code: "55P03"}
	assert.True(t, isLockTimeoutError(pqErr))
}

func TestIsLockTimeoutError_PQErrorOtherCode(t *testing.T) {
	pqErr := &pgconn.PgError{Code: "23505"} // unique violation
	assert.False(t, isLockTimeoutError(pqErr))
}

func TestIsLockTimeoutError_WrappedPQError(t *testing.T) {
	pqErr := &pgconn.PgError{Code: "55P03"}
	wrapped := errors.New("outer: " + pqErr.Error())
	// Plain errors.New wrapping won't work with errors.As
	assert.False(t, isLockTimeoutError(wrapped))

	// But fmt.Errorf %w wrapping will
	wrappedFmt := fmt.Errorf("acquiring lock: %w", pqErr)
	assert.True(t, isLockTimeoutError(wrappedFmt))
}

func TestIsLockTimeoutError_NonPQError(t *testing.T) {
	assert.False(t, isLockTimeoutError(errors.New("some other error")))
}

// --- Tests for ErrNotYourTurn ---

func TestErrNotYourTurn_Error(t *testing.T) {
	err := &ErrNotYourTurn{
		CurrentCharacterName: "Gandalf",
		CurrentDiscordUserID: "user-123",
	}
	msg := err.Error()
	assert.Contains(t, msg, "Gandalf")
	assert.Contains(t, msg, "user-123")
	assert.Contains(t, msg, "not your turn")
}

// --- Tests for ErrTurnChanged ---

func TestErrTurnChanged(t *testing.T) {
	assert.EqualError(t, ErrTurnChanged, "it's no longer your turn")
}

// --- Tests for IsExemptCommand edge cases ---

func TestIsExemptCommand_EmptyString(t *testing.T) {
	assert.False(t, IsExemptCommand(""))
}

// --- Tests for ValidateTurnOwnership error paths (mocked) ---

func baseMock() *mockQuerier {
	turnID := uuid.New()
	combatantID := uuid.New()
	charID := uuid.New()
	campaignID := uuid.New()

	return &mockQuerier{
		activeTurn: refdata.Turn{
			ID:          turnID,
			CombatantID: combatantID,
			Status:      "active",
		},
		combatant: refdata.Combatant{
			ID:          combatantID,
			CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
			DisplayName: "Aragorn",
			IsNpc:       false,
		},
		campaign: refdata.Campaign{
			ID:       campaignID,
			DmUserID: "dm-user",
		},
		playerChar: refdata.PlayerCharacter{
			DiscordUserID: "player-user",
		},
	}
}

func TestValidateTurnOwnership_GetActiveTurnError(t *testing.T) {
	m := baseMock()
	m.activeTurnErr = errors.New("db connection lost")

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting active turn")
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestValidateTurnOwnership_GetActiveTurnNoRows(t *testing.T) {
	m := baseMock()
	m.activeTurnErr = sql.ErrNoRows

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	assert.ErrorIs(t, err, ErrNoActiveTurn)
}

func TestValidateTurnOwnership_GetCombatantError(t *testing.T) {
	m := baseMock()
	m.combatantErr = errors.New("combatant query failed")

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
	assert.Contains(t, err.Error(), "combatant query failed")
}

func TestValidateTurnOwnership_GetCampaignByEncounterIDError(t *testing.T) {
	m := baseMock()
	m.campaignErr = errors.New("campaign query failed")

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting campaign")
	assert.Contains(t, err.Error(), "campaign query failed")
}

func TestValidateTurnOwnership_GetPlayerCharacterGeneralError(t *testing.T) {
	m := baseMock()
	m.playerCharErr = errors.New("player character db error")

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting player character")
	assert.Contains(t, err.Error(), "player character db error")
}

func TestValidateTurnOwnership_GetPlayerCharacterNoRows(t *testing.T) {
	m := baseMock()
	m.playerCharErr = sql.ErrNoRows

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	var notYourTurn *ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn))
	assert.Equal(t, "dm-user", notYourTurn.CurrentDiscordUserID)
}

func TestValidateTurnOwnership_DMCanAlwaysAct(t *testing.T) {
	m := baseMock()
	info, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "dm-user")
	require.NoError(t, err)
	assert.Equal(t, m.activeTurn.ID, info.TurnID)
	assert.Equal(t, "dm-user", info.DMUserID)
}

func TestValidateTurnOwnership_NPCTurn_NonDMRejected(t *testing.T) {
	m := baseMock()
	m.combatant.IsNpc = true

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	var notYourTurn *ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn))
}

func TestValidateTurnOwnership_NoCharacterID_NonDMRejected(t *testing.T) {
	m := baseMock()
	m.combatant.CharacterID = uuid.NullUUID{Valid: false}

	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.Error(t, err)
	var notYourTurn *ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn))
}

func TestValidateTurnOwnership_CorrectPlayer(t *testing.T) {
	m := baseMock()
	info, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "player-user")
	require.NoError(t, err)
	assert.Equal(t, "player-user", info.OwnerUserID)
	assert.Equal(t, m.activeTurn.ID, info.TurnID)
}

func TestValidateTurnOwnership_WrongPlayer(t *testing.T) {
	m := baseMock()
	_, err := ValidateTurnOwnership(context.Background(), m, uuid.New(), "wrong-user")
	require.Error(t, err)
	var notYourTurn *ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn))
	assert.Equal(t, "player-user", notYourTurn.CurrentDiscordUserID)
}

type mockTxBeginner struct {
	tx  *sql.Tx
	err error
}

func (m *mockTxBeginner) BeginTx(_ context.Context, _ *sql.TxOptions) (*sql.Tx, error) {
	return m.tx, m.err
}

func TestAcquireTurnLock_BeginTxError(t *testing.T) {
	db := &mockTxBeginner{err: errors.New("connection refused")}
	_, err := AcquireTurnLock(context.Background(), db, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beginning transaction")
	assert.Contains(t, err.Error(), "connection refused")
}

// --- Tests for AcquireTurnLockWithValidation error propagation ---

func TestAcquireTurnLockWithValidation_ValidationError(t *testing.T) {
	m := baseMock()
	m.activeTurnErr = errors.New("validation db error")

	_, _, err := AcquireTurnLockWithValidation(context.Background(), nil, m, uuid.New(), "player-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting active turn")
}

func TestAcquireTurnLockWithValidation_BeginTxError(t *testing.T) {
	m := baseMock()
	db := &mockTxBeginner{err: errors.New("db pool exhausted")}

	_, _, err := AcquireTurnLockWithValidation(context.Background(), db, m, uuid.New(), "player-user")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beginning transaction")
}
