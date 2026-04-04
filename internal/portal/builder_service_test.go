package portal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
)

func TestValidatePointBuy_AllEights(t *testing.T) {
	scores := portal.PointBuyScores{STR: 8, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidatePointBuy_Standard(t *testing.T) {
	// 15(9) + 14(7) + 13(5) + 12(4) + 10(2) + 8(0) = 27
	scores := portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidatePointBuy_TooManyPoints(t *testing.T) {
	scores := portal.PointBuyScores{STR: 15, DEX: 15, CON: 15, INT: 15, WIS: 15, CHA: 15}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrPointBuyOverspent)
}

func TestValidatePointBuy_ScoreTooLow(t *testing.T) {
	scores := portal.PointBuyScores{STR: 7, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrScoreOutOfRange)
}

func TestValidatePointBuy_ScoreTooHigh(t *testing.T) {
	scores := portal.PointBuyScores{STR: 16, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrScoreOutOfRange)
}

func TestValidatePointBuy_Exact27(t *testing.T) {
	// 15(9) + 15(9) + 8(0) + 8(0) + 8(0) + 8(0) = 18 -- underspent is OK
	scores := portal.PointBuyScores{STR: 15, DEX: 15, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidateSubmission_EmptyName(t *testing.T) {
	sub := validSubmission()
	sub.Name = ""
	errs := portal.ValidateSubmission(sub)
	assert.Contains(t, errs, "name is required")
}

func TestValidateSubmission_EmptyRace(t *testing.T) {
	sub := validSubmission()
	sub.Race = ""
	errs := portal.ValidateSubmission(sub)
	assert.Contains(t, errs, "race is required")
}

func TestValidateSubmission_EmptyClass(t *testing.T) {
	sub := validSubmission()
	sub.Class = ""
	errs := portal.ValidateSubmission(sub)
	assert.Contains(t, errs, "class is required")
}

func TestValidateSubmission_InvalidPointBuy(t *testing.T) {
	sub := validSubmission()
	sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 15, CON: 15, INT: 15, WIS: 15, CHA: 15}
	errs := portal.ValidateSubmission(sub)
	assert.NotEmpty(t, errs)
}

func TestValidateSubmission_Valid(t *testing.T) {
	sub := validSubmission()
	errs := portal.ValidateSubmission(sub)
	assert.Empty(t, errs)
}

func validSubmission() portal.CharacterSubmission {
	return portal.CharacterSubmission{
		Name:          "Thorin",
		Race:          "dwarf",
		Background:    "soldier",
		Class:         "fighter",
		AbilityScores: portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8},
		Skills:        []string{"athletics", "perception"},
	}
}

func TestBuilderService_CreateCharacter_Valid(t *testing.T) {
	store := &mockBuilderStore{
		charID: "char-uuid-123",
		pcID:   "pc-uuid-456",
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "char-uuid-123", result.CharacterID)
	assert.Equal(t, "pc-uuid-456", result.PlayerCharacterID)

	// Verify store was called correctly
	assert.Equal(t, "Thorin", store.lastCharName)
	assert.Equal(t, "fighter", store.lastCharClass)
	assert.Equal(t, "pending", store.lastPCStatus)
	assert.Equal(t, "create", store.lastPCCreatedVia)
	assert.Equal(t, "discord-user-1", store.lastPCDiscordUserID)
}

func TestBuilderService_CreateCharacter_InvalidSubmission(t *testing.T) {
	store := &mockBuilderStore{}
	svc := portal.NewBuilderService(store)

	sub := portal.CharacterSubmission{} // empty
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestBuilderService_CreateCharacter_StoreError(t *testing.T) {
	store := &mockBuilderStore{
		createCharErr: errors.New("db down"),
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestBuilderService_CreateCharacter_RedeemToken(t *testing.T) {
	store := &mockBuilderStore{
		charID: "char-1",
		pcID:   "pc-1",
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "tok-abc", store.lastRedeemedToken)
}

// mockBuilderStore implements portal.BuilderStore for testing.
type mockBuilderStore struct {
	charID        string
	pcID          string
	createCharErr error
	createPCErr   error

	lastCharName        string
	lastCharClass       string
	lastPCStatus        string
	lastPCCreatedVia    string
	lastPCDiscordUserID string
	lastRedeemedToken   string
}

func (m *mockBuilderStore) CreateCharacterRecord(_ context.Context, p portal.CreateCharacterParams) (string, error) {
	m.lastCharName = p.Name
	m.lastCharClass = p.Class
	if m.createCharErr != nil {
		return "", m.createCharErr
	}
	return m.charID, nil
}

func (m *mockBuilderStore) CreatePlayerCharacterRecord(_ context.Context, p portal.CreatePlayerCharacterParams) (string, error) {
	m.lastPCStatus = p.Status
	m.lastPCCreatedVia = p.CreatedVia
	m.lastPCDiscordUserID = p.DiscordUserID
	if m.createPCErr != nil {
		return "", m.createPCErr
	}
	return m.pcID, nil
}

func (m *mockBuilderStore) RedeemToken(_ context.Context, token string) error {
	m.lastRedeemedToken = token
	return nil
}

func TestBuilderService_CreateCharacter_PCStoreError(t *testing.T) {
	store := &mockBuilderStore{
		charID:      "char-1",
		createPCErr: errors.New("pc db error"),
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pc db error")
}

func TestBuilderService_CreateCharacter_Barbarian(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "barbarian"
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_CreateCharacter_Wizard(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "wizard"
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_CreateCharacter_Paladin(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "paladin"
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
}

func TestBuilderService_CreateCharacter_Rogue(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Class = "rogue"
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
}

func TestPointBuyCost(t *testing.T) {
	tests := []struct {
		score int
		cost  int
	}{
		{8, 0},
		{9, 1},
		{10, 2},
		{11, 3},
		{12, 4},
		{13, 5},
		{14, 7},
		{15, 9},
	}
	for _, tt := range tests {
		cost, err := portal.PointBuyCost(tt.score)
		assert.NoError(t, err)
		assert.Equal(t, tt.cost, cost, "score %d", tt.score)
	}
}
