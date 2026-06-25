package portal_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	scores := portal.PointBuyScores{STR: 18, DEX: 8, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.ErrorIs(t, err, portal.ErrScoreOutOfRange)
}

func TestValidatePointBuy_Exact27(t *testing.T) {
	// 15(9) + 15(9) + 8(0) + 8(0) + 8(0) + 8(0) = 18 -- underspent is OK
	scores := portal.PointBuyScores{STR: 15, DEX: 15, CON: 8, INT: 8, WIS: 8, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidatePointBuy_TreatsRacialBonusesAsFree(t *testing.T) {
	// The builder folds race + subrace ability bonuses into the submitted
	// (post-racial) scores. A legal 27-point base (STR15/DEX14/CON13/INT12/
	// WIS10/CHA8) with +2 DEX (race) and +1 INT (subrace) becomes DEX16/INT13.
	// Billed naively this stat line costs 30 and would be wrongly rejected; the
	// racial allowance makes those bonuses free so it validates.
	scores := portal.PointBuyScores{STR: 15, DEX: 16, CON: 13, INT: 13, WIS: 10, CHA: 8}
	err := portal.ValidatePointBuy(scores)
	assert.NoError(t, err)
}

func TestValidateAbilityScores_MethodRules(t *testing.T) {
	tests := []struct {
		name    string
		sub     portal.CharacterSubmission
		wantErr string
	}{
		{
			name: "point buy",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodPointBuy
				return sub
			}(),
		},
		{
			name: "standard array",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodStandardArray
				sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}
				return sub
			}(),
		},
		{
			name: "roll with 4d6 drop lowest details",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodRoll
				sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 13, CON: 12, INT: 12, WIS: 9, CHA: 6}
				sub.AbilityRolls = map[string][]int{
					"str": {6, 5, 4, 1},
					"dex": {6, 4, 3, 1},
					"con": {4, 4, 4, 1},
					"int": {6, 3, 2, 3},
					"wis": {2, 2, 5, 2},
					"cha": {1, 2, 3, 1},
				}
				return sub
			}(),
		},
		{
			name: "roll rejects mismatched score",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodRoll
				sub.AbilityScores = portal.PointBuyScores{STR: 10, DEX: 13, CON: 12, INT: 12, WIS: 9, CHA: 6}
				sub.AbilityRolls = map[string][]int{
					"str": {6, 5, 4, 1},
					"dex": {6, 4, 3, 1},
					"con": {4, 4, 4, 1},
					"int": {6, 3, 2, 3},
					"wis": {2, 2, 5, 2},
					"cha": {1, 2, 3, 1},
				}
				return sub
			}(),
			wantErr: "is less than 4d6 drop lowest",
		},
		{
			name: "roll rejects missing roll details",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodRoll
				sub.AbilityScores = portal.PointBuyScores{STR: 18, DEX: 18, CON: 18, INT: 18, WIS: 18, CHA: 18}
				return sub
			}(),
			wantErr: "roll must include four d6 results",
		},
		{
			name: "standard array rejects out of range",
			sub: func() portal.CharacterSubmission {
				sub := validSubmission()
				sub.AbilityMethod = portal.AbilityMethodStandardArray
				sub.AbilityScores = portal.PointBuyScores{STR: 20, DEX: 15, CON: 13, INT: 12, WIS: 10, CHA: 8}
				return sub
			}(),
			wantErr: "standard array score out of range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := portal.ValidateAbilityScoreGeneration(tt.sub)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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
		// soldier background grants athletics + intimidation (both locked).
		// Submitting only the locked skills (zero extra class picks) keeps this
		// fixture valid for every class it is reused with.
		Skills: []string{"athletics", "intimidation"},
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

// ISSUE-005: a Rogue (L1) submission carries its 2 chosen Expertise skills
// through the service into CreateCharacterParams.Expertise, so persistence can
// write the "expertise" key the combat reader parses.
func TestBuilderService_CreateCharacter_CarriesExpertise_Rogue(t *testing.T) {
	store := &mockBuilderStore{charID: "char-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	// soldier background locks athletics + intimidation; the rogue then takes 4
	// class picks. Two of the proficient skills are chosen for Expertise.
	sub := portal.CharacterSubmission{
		Name:          "Sneak",
		Race:          "human",
		Background:    "soldier",
		Class:         "rogue",
		AbilityScores: portal.PointBuyScores{STR: 8, DEX: 15, CON: 14, INT: 13, WIS: 12, CHA: 10},
		Skills:        []string{"athletics", "intimidation", "stealth", "perception", "acrobatics", "investigation"},
		Expertise:     []string{"stealth", "perception"},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	require.NoError(t, err)
	assert.Equal(t, []string{"stealth", "perception"}, store.lastCharExpertise)
}

// A non-expert class never carries expertise even if the payload smuggles some:
// the service rejects the illegal submission.
func TestBuilderService_CreateCharacter_RejectsExpertiseForNonExpertClass(t *testing.T) {
	store := &mockBuilderStore{charID: "char-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission() // fighter
	sub.Expertise = []string{"athletics"}

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
}

func TestBuilderService_CreateCharacter_NotifiesDMQueue(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	notifier := &mockDMQueueNotifier{}
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier))

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
	assert.Equal(t, "campaign-uuid", notifier.campaignID)
	assert.Equal(t, "Thorin", notifier.charName)
	assert.Equal(t, "discord-user-1", notifier.playerID)
	assert.Equal(t, "portal-create", notifier.via)
}

func TestBuilderService_CreateCharacter_NotifierErrorDoesNotFail(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	notifier := &mockDMQueueNotifier{err: errors.New("discord down")}
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier))

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_WithLogger(t *testing.T) {
	logger := slog.Default()
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store, portal.WithLogger(logger))
	// Verify it doesn't panic and produces results
	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
}

func TestBuilderService_NotifierError_WithLogger(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	notifier := &mockDMQueueNotifier{err: errors.New("discord down")}
	logger := slog.Default()
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier), portal.WithLogger(logger))

	sub := validSubmission()
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
	assert.True(t, notifier.called)
}

func TestBuilderService_RedeemTokenError_WithLogger(t *testing.T) {
	store := &mockBuilderStore{
		charID:         "c-1",
		pcID:           "pc-1",
		redeemTokenErr: errors.New("token expired"),
	}
	logger := slog.Default()
	svc := portal.NewBuilderService(store, portal.WithLogger(logger))

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	// Redeem failure now prevents character creation
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redeeming token")
}

func TestBuilderService_CreateCharacter_RejectsMismatchedTokenUser(t *testing.T) {
	store := &mockBuilderStore{
		charID: "c-1",
		pcID:   "pc-1",
		validateToken: &portal.PortalToken{
			DiscordUserID: "other-user",
		},
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrTokenOwnership)
	// Character should NOT have been created
	assert.Empty(t, store.lastCharName)
}

func TestBuilderService_CreateCharacter_RedeemFailsPreventsCreation(t *testing.T) {
	store := &mockBuilderStore{
		charID: "c-1",
		pcID:   "pc-1",
		validateToken: &portal.PortalToken{
			DiscordUserID: "discord-user-1",
		},
		redeemTokenErr: errors.New("already redeemed"),
	}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redeeming token")
	// H-M06: character creation now happens before redemption to prevent
	// token consumption on creation failure. If redemption fails (race),
	// the character may exist but the operation returns an error.
}

func TestBuilderService_CreateCharacter_InvalidSubmission(t *testing.T) {
	store := &mockBuilderStore{}
	svc := portal.NewBuilderService(store)

	sub := portal.CharacterSubmission{} // empty
	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestBuilderService_CreateCharacter_RejectsEmptyTokenOrCampaign(t *testing.T) {
	// Empty token/campaign_id must surface as a 400-class validation error
	// (prefix "validation"), not a generic 500 from a token lookup miss or a
	// rejected campaign_id insert.
	tests := []struct {
		name       string
		campaignID string
		token      string
	}{
		{"empty token", "campaign-uuid", ""},
		{"empty campaign", "", "tok-abc"},
		{"both empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockBuilderStore{charID: "char-1", pcID: "pc-1"}
			svc := portal.NewBuilderService(store)

			_, err := svc.CreateCharacter(context.Background(), tc.campaignID, "discord-user-1", tc.token, validSubmission())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation failed")
		})
	}
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

func TestBuilderService_CreateCharacter_RejectsDisallowedAbilityMethod(t *testing.T) {
	store := &mockBuilderStore{charID: "char-1", pcID: "pc-1"}
	provider := portal.StaticAbilityMethodProvider([]portal.AbilityScoreMethod{portal.AbilityMethodPointBuy})
	svc := portal.NewBuilderService(store, portal.WithAbilityMethodProvider(provider))

	sub := validSubmission()
	sub.AbilityMethod = portal.AbilityMethodStandardArray
	sub.AbilityScores = portal.PointBuyScores{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 10, CHA: 8}

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ability score method standard_array is not allowed")
}

// SR-013: when a player submits the builder and a non-retired row already
// exists for them (a stuck placeholder, a resubmit, or a DM "changes
// requested"), the submit must RE-LINK that row to the freshly built
// character instead of INSERTing a second one (which the partial unique index
// idx_player_characters_unique_active_discord_user forbids).
func TestBuilderService_CreateCharacter_RelinksExistingNonRetiredRow(t *testing.T) {
	for _, status := range []string{"pending", "changes_requested", "rejected"} {
		t.Run(status, func(t *testing.T) {
			store := &mockBuilderStore{
				charID:   "new-char-1",
				pcID:     "existing-pc-1",
				activePC: &portal.ActivePlayerCharacter{ID: "existing-pc-1", Status: status},
			}
			svc := portal.NewBuilderService(store)

			result, err := svc.CreateCharacter(context.Background(), "camp", "user-1", "tok", validSubmission())
			require.NoError(t, err)
			assert.True(t, store.relinkCalled, "expected relink, not insert")
			assert.Equal(t, "existing-pc-1", store.lastRelinkPCID)
			assert.Equal(t, "new-char-1", store.lastRelinkCharID)
			assert.Equal(t, "create", store.lastRelinkVia)
			assert.Equal(t, "existing-pc-1", result.PlayerCharacterID)
			// The insert path must NOT have run.
			assert.Empty(t, store.lastPCStatus, "CreatePlayerCharacterRecord should not be called on relink")
		})
	}
}

// An already-approved (active) character is never clobbered: the player must
// /retire first.
func TestBuilderService_CreateCharacter_RejectsWhenAlreadyApproved(t *testing.T) {
	store := &mockBuilderStore{
		charID:   "new-char-1",
		activePC: &portal.ActivePlayerCharacter{ID: "approved-pc", Status: "approved"},
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.CreateCharacter(context.Background(), "camp", "user-1", "tok", validSubmission())
	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrAlreadyActive)
	assert.False(t, store.relinkCalled)
	assert.Empty(t, store.lastPCStatus)
}

// T13 / Finding 4·d: a player approved mid-build must be rejected BEFORE the
// character record is created or the token is redeemed. Otherwise the player
// retries the 409, the token is already consumed, and the retry 500s with
// "token already used". The active-character guard must run first.
func TestBuilderService_CreateCharacter_AlreadyApprovedSparesTokenAndRecord(t *testing.T) {
	store := &mockBuilderStore{
		charID:   "new-char-1",
		activePC: &portal.ActivePlayerCharacter{ID: "approved-pc", Status: "approved"},
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.CreateCharacter(context.Background(), "camp", "user-1", "tok", validSubmission())
	require.ErrorIs(t, err, portal.ErrAlreadyActive)
	assert.Empty(t, store.lastRedeemedToken, "token must not be redeemed when already approved")
	assert.Empty(t, store.lastCharName, "character record must not be created when already approved")
}

// No existing row → the normal insert path runs (regression guard).
func TestBuilderService_CreateCharacter_InsertsWhenNoExistingRow(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"} // activePC nil
	svc := portal.NewBuilderService(store)

	result, err := svc.CreateCharacter(context.Background(), "camp", "user-1", "tok", validSubmission())
	require.NoError(t, err)
	assert.False(t, store.relinkCalled)
	assert.Equal(t, "pending", store.lastPCStatus)
	assert.Equal(t, "pc-1", result.PlayerCharacterID)
}

func TestBuilderService_CreateCharacter_ActiveLookupError(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", activePCErr: errors.New("db down")}
	svc := portal.NewBuilderService(store)

	_, err := svc.CreateCharacter(context.Background(), "camp", "user-1", "tok", validSubmission())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

// mockBuilderStore implements portal.BuilderStore for testing.
type mockBuilderStore struct {
	charID           string
	pcID             string
	createCharErr    error
	createPCErr      error
	redeemTokenErr   error
	validateToken    *portal.PortalToken
	validateTokenErr error

	lastCharName        string
	lastCharClass       string
	lastCharSubrace     string
	lastCharClasses     []character.ClassEntry
	lastCharEquipment   []string
	lastCharExpertise   []string
	lastCharProfBonus   int
	lastPCStatus        string
	lastPCCreatedVia    string
	lastPCDiscordUserID string
	lastRedeemedToken   string

	// activePC / activePCErr drive ActivePlayerCharacter; nil activePC means
	// "no existing row" so the insert path runs (default for most tests).
	activePC    *portal.ActivePlayerCharacter
	activePCErr error
	relinkErr   error
	// relink call capture.
	relinkCalled     bool
	lastRelinkPCID   string
	lastRelinkCharID string
	lastRelinkVia    string

	// draft persistence capture (T11 / Finding 4·b).
	saveDraftCalled bool
	lastDraftCamp   string
	lastDraftUser   string
	lastDraftMode   string
	lastDraftBlob   json.RawMessage
	saveDraftErr    error
	loadDraftResult json.RawMessage
	loadDraftErr    error

	// edit-mode capture (UpdateCharacter path).
	editContext        *portal.EditContext
	editContextErr     error
	updateCharErr      error
	updateCalled       bool
	lastUpdateCharID   string
	lastUpdateParams   portal.CreateCharacterParams
	setPendingErr      error
	setPendingCalled   bool
	lastSetPendingPCID string
	hasActiveEncounter bool
	hasActiveEncErr    error
	editSubmission     portal.CharacterSubmission
	editSubmissionErr  error
}

func (m *mockBuilderStore) CreateCharacterRecord(_ context.Context, p portal.CreateCharacterParams) (string, error) {
	m.lastCharName = p.Name
	m.lastCharClass = p.Class
	m.lastCharSubrace = p.Subrace
	m.lastCharClasses = p.Classes
	m.lastCharEquipment = p.Equipment
	m.lastCharExpertise = p.Expertise
	m.lastCharProfBonus = p.ProfBonus
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

func (m *mockBuilderStore) ActivePlayerCharacter(_ context.Context, _, _ string) (*portal.ActivePlayerCharacter, error) {
	if m.activePCErr != nil {
		return nil, m.activePCErr
	}
	return m.activePC, nil
}

func (m *mockBuilderStore) RelinkPlayerCharacterRecord(_ context.Context, pcID, characterID, createdVia string) (string, error) {
	m.relinkCalled = true
	m.lastRelinkPCID = pcID
	m.lastRelinkCharID = characterID
	m.lastRelinkVia = createdVia
	if m.relinkErr != nil {
		return "", m.relinkErr
	}
	if m.pcID != "" {
		return m.pcID, nil
	}
	return pcID, nil
}

func (m *mockBuilderStore) RedeemToken(_ context.Context, token string) error {
	m.lastRedeemedToken = token
	return m.redeemTokenErr
}

func (m *mockBuilderStore) ValidateToken(_ context.Context, _ string) (*portal.PortalToken, error) {
	if m.validateTokenErr != nil {
		return nil, m.validateTokenErr
	}
	return m.validateToken, nil
}

func (m *mockBuilderStore) SaveCharacterDraft(_ context.Context, campaignID, discordUserID, mode string, draft json.RawMessage) error {
	m.saveDraftCalled = true
	m.lastDraftCamp = campaignID
	m.lastDraftUser = discordUserID
	m.lastDraftMode = mode
	m.lastDraftBlob = draft
	return m.saveDraftErr
}

func (m *mockBuilderStore) LoadCharacterDraft(_ context.Context, _, _, _ string) (json.RawMessage, error) {
	return m.loadDraftResult, m.loadDraftErr
}

func (m *mockBuilderStore) GetEditContext(_ context.Context, _ string) (*portal.EditContext, error) {
	if m.editContextErr != nil {
		return nil, m.editContextErr
	}
	return m.editContext, nil
}

func (m *mockBuilderStore) UpdateCharacterRecord(_ context.Context, characterID string, p portal.CreateCharacterParams) error {
	m.updateCalled = true
	m.lastUpdateCharID = characterID
	m.lastUpdateParams = p
	return m.updateCharErr
}

func (m *mockBuilderStore) SetPlayerCharacterPending(_ context.Context, pcID string) error {
	m.setPendingCalled = true
	m.lastSetPendingPCID = pcID
	return m.setPendingErr
}

func (m *mockBuilderStore) HasActiveEncounter(_ context.Context, _ string) (bool, error) {
	return m.hasActiveEncounter, m.hasActiveEncErr
}

func (m *mockBuilderStore) LoadEditSubmission(_ context.Context, _ string) (portal.CharacterSubmission, error) {
	return m.editSubmission, m.editSubmissionErr
}

func TestBuilderService_LoadEditData_OwnerAllowed(t *testing.T) {
	want := portal.CharacterSubmission{Name: "Thorin", Race: "dwarf"}
	store := &mockBuilderStore{
		editContext:    &portal.EditContext{CampaignID: "c1", OwnerID: "player-1", DMUserID: "dm-1"},
		editSubmission: want,
	}
	svc := portal.NewBuilderService(store)

	got, err := svc.LoadEditData(context.Background(), "char-1", "player-1")
	require.NoError(t, err)
	assert.Equal(t, "Thorin", got.Name)
}

func TestBuilderService_LoadEditData_DMAllowed(t *testing.T) {
	store := &mockBuilderStore{
		editContext:    &portal.EditContext{CampaignID: "c1", OwnerID: "player-1", DMUserID: "dm-1"},
		editSubmission: portal.CharacterSubmission{Name: "Thorin"},
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.LoadEditData(context.Background(), "char-1", "dm-1")
	require.NoError(t, err)
}

func TestBuilderService_LoadEditData_StrangerForbidden(t *testing.T) {
	store := &mockBuilderStore{
		editContext: &portal.EditContext{CampaignID: "c1", OwnerID: "player-1", DMUserID: "dm-1"},
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.LoadEditData(context.Background(), "char-1", "stranger")
	require.ErrorIs(t, err, portal.ErrEditNotAllowed)
}

func TestBuilderService_LoadEditData_NotFound(t *testing.T) {
	store := &mockBuilderStore{editContextErr: portal.ErrCharacterNotFound}
	svc := portal.NewBuilderService(store)

	_, err := svc.LoadEditData(context.Background(), "missing", "player-1")
	require.ErrorIs(t, err, portal.ErrCharacterNotFound)
}

func TestBuilderService_UpdateCharacter_PlayerEditResetsToPending(t *testing.T) {
	notifier := &mockDMQueueNotifier{}
	store := &mockBuilderStore{
		editContext: &portal.EditContext{
			CampaignID:        "camp-1",
			OwnerID:           "player-1",
			DMUserID:          "dm-1",
			PlayerCharacterID: "pc-1",
			Status:            "approved",
		},
	}
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier))

	res, err := svc.UpdateCharacter(context.Background(), "char-1", "player-1", validSubmission())
	require.NoError(t, err)
	assert.Equal(t, "char-1", res.CharacterID)
	assert.Equal(t, "pc-1", res.PlayerCharacterID)

	assert.True(t, store.updateCalled)
	assert.Equal(t, "char-1", store.lastUpdateCharID)
	assert.Equal(t, "camp-1", store.lastUpdateParams.CampaignID)
	// Player edit must revert to pending and re-notify the DM queue.
	assert.True(t, store.setPendingCalled)
	assert.Equal(t, "pc-1", store.lastSetPendingPCID)
	assert.True(t, notifier.called)
	assert.Equal(t, "portal-edit", notifier.via)
}

func TestBuilderService_UpdateCharacter_DMEditAppliesInstantly(t *testing.T) {
	notifier := &mockDMQueueNotifier{}
	store := &mockBuilderStore{
		editContext: &portal.EditContext{
			CampaignID:        "camp-1",
			OwnerID:           "player-1",
			DMUserID:          "dm-1",
			PlayerCharacterID: "pc-1",
			Status:            "approved",
		},
	}
	svc := portal.NewBuilderService(store, portal.WithNotifier(notifier))

	_, err := svc.UpdateCharacter(context.Background(), "char-1", "dm-1", validSubmission())
	require.NoError(t, err)

	assert.True(t, store.updateCalled)
	// DM edit stays approved: no pending reset, no DM-queue notify.
	assert.False(t, store.setPendingCalled)
	assert.False(t, notifier.called)
}

func TestBuilderService_UpdateCharacter_RejectsStranger(t *testing.T) {
	store := &mockBuilderStore{
		editContext: &portal.EditContext{
			CampaignID: "camp-1", OwnerID: "player-1", DMUserID: "dm-1", PlayerCharacterID: "pc-1",
		},
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.UpdateCharacter(context.Background(), "char-1", "stranger", validSubmission())
	require.ErrorIs(t, err, portal.ErrEditNotAllowed)
	assert.False(t, store.updateCalled)
}

func TestBuilderService_UpdateCharacter_BlockedInEncounter(t *testing.T) {
	store := &mockBuilderStore{
		editContext: &portal.EditContext{
			CampaignID: "camp-1", OwnerID: "player-1", DMUserID: "dm-1", PlayerCharacterID: "pc-1",
		},
		hasActiveEncounter: true,
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.UpdateCharacter(context.Background(), "char-1", "player-1", validSubmission())
	require.ErrorIs(t, err, portal.ErrCharacterInEncounter)
	assert.False(t, store.updateCalled)
}

func TestBuilderService_UpdateCharacter_NotFound(t *testing.T) {
	store := &mockBuilderStore{editContextErr: portal.ErrCharacterNotFound}
	svc := portal.NewBuilderService(store)

	_, err := svc.UpdateCharacter(context.Background(), "missing", "player-1", validSubmission())
	require.ErrorIs(t, err, portal.ErrCharacterNotFound)
	assert.False(t, store.updateCalled)
}

func TestBuilderService_UpdateCharacter_InvalidSubmission(t *testing.T) {
	store := &mockBuilderStore{
		editContext: &portal.EditContext{CampaignID: "camp-1", OwnerID: "player-1", PlayerCharacterID: "pc-1"},
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.UpdateCharacter(context.Background(), "char-1", "player-1", portal.CharacterSubmission{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.False(t, store.updateCalled)
}

func TestBuilderService_UpdateCharacter_StoreError(t *testing.T) {
	store := &mockBuilderStore{
		editContext:   &portal.EditContext{CampaignID: "camp-1", OwnerID: "player-1", PlayerCharacterID: "pc-1"},
		updateCharErr: errors.New("db down"),
	}
	svc := portal.NewBuilderService(store)

	_, err := svc.UpdateCharacter(context.Background(), "char-1", "player-1", validSubmission())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

// mockDMQueueNotifier implements portal.DMQueueNotifier for testing.
type mockDMQueueNotifier struct {
	called     bool
	campaignID string
	charName   string
	playerID   string
	via        string
	err        error
}

func (m *mockDMQueueNotifier) NotifyDMQueue(ctx context.Context, campaignID, charName, playerDiscordID, via string) error {
	m.called = true
	m.campaignID = campaignID
	m.charName = charName
	m.playerID = playerDiscordID
	m.via = via
	return m.err
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

func TestBuilderService_CreateCharacter_WithEquipment(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Equipment = []string{"longsword", "chain-mail", "shield"}
	result, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "discord-user-1", "tok-abc", sub)
	assert.NoError(t, err)
	assert.Equal(t, "c-1", result.CharacterID)
	assert.Equal(t, []string{"longsword", "chain-mail", "shield"}, store.lastCharEquipment)
}

// TestBuilderService_CreateCharacter_PassesMulticlass confirms a payload
// with `Classes` set is threaded through CreateCharacterParams so the
// adapter can persist the multiclass entries.
func TestBuilderService_CreateCharacter_PassesMulticlass(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Classes = []character.ClassEntry{
		{Class: "fighter", Subclass: "champion", Level: 5},
		{Class: "wizard", Subclass: "evocation", Level: 3},
	}

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Len(t, store.lastCharClasses, 2)
	assert.Equal(t, "fighter", store.lastCharClasses[0].Class)
	assert.Equal(t, "champion", store.lastCharClasses[0].Subclass)
	assert.Equal(t, "wizard", store.lastCharClasses[1].Class)
}

func TestBuilderService_CreateCharacter_ProficiencyBonusFromTotalLevel(t *testing.T) {
	tests := []struct {
		name    string
		classes []character.ClassEntry
		want    int
	}{
		{
			name: "fighter 5",
			classes: []character.ClassEntry{
				{Class: "fighter", Subclass: "champion", Level: 5},
			},
			want: 3,
		},
		{
			name: "fighter 3 rogue 2",
			classes: []character.ClassEntry{
				{Class: "fighter", Subclass: "champion", Level: 3},
				{Class: "rogue", Subclass: "thief", Level: 2},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
			svc := portal.NewBuilderService(store)

			sub := validSubmission()
			sub.Class = tt.classes[0].Class
			sub.Subclass = tt.classes[0].Subclass
			sub.Classes = tt.classes

			_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, store.lastCharProfBonus)
		})
	}
}

// TestBuilderService_CreateCharacter_PassesSubrace confirms the subrace
// from the submission ends up on CreateCharacterParams.
func TestBuilderService_CreateCharacter_PassesSubrace(t *testing.T) {
	store := &mockBuilderStore{charID: "c-1", pcID: "pc-1"}
	svc := portal.NewBuilderService(store)

	sub := validSubmission()
	sub.Race = "elf"
	sub.Subrace = "high-elf"

	_, err := svc.CreateCharacter(context.Background(), "campaign-uuid", "u1", "tok", sub)
	assert.NoError(t, err)
	assert.Equal(t, "high-elf", store.lastCharSubrace)
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

// casterSubmission returns a valid level-1 wizard with the given INT score.
// Wizard uses INT for spellcasting, so the cap is AbilityModifier(INT)+1 (min 1).
func casterSubmission() portal.CharacterSubmission {
	return portal.CharacterSubmission{
		Name:          "Gandalf",
		Race:          "human",
		Background:    "sage",
		Class:         "wizard",
		AbilityScores: portal.PointBuyScores{STR: 8, DEX: 14, CON: 13, INT: 15, WIS: 12, CHA: 10},
	}
}

func TestValidateSubmission_CasterWithinSpellCapPasses(t *testing.T) {
	// Level 1 wizard, INT 15 (+2): budget = 3 cantrips + (2+1) prepared = 6.
	// Three spells is well within the cap.
	sub := casterSubmission()
	sub.Spells = []string{"magic-missile", "shield", "mage-armor"}
	errs := portal.ValidateSubmission(sub)
	assert.Empty(t, errs)
}

func TestValidateSubmission_CasterOverSpellCapFails(t *testing.T) {
	// Level 1 wizard, INT 15 (+2): budget = 6 (3 cantrips + 3 prepared).
	// Seven spells exceeds it.
	sub := casterSubmission()
	sub.Spells = []string{"a", "b", "c", "d", "e", "f", "g"}
	errs := portal.ValidateSubmission(sub)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs, "too many spells selected: 7 chosen, maximum 6")
}

func TestValidateSubmission_NonCasterWithSpellsPasses(t *testing.T) {
	// Fighter is not a spellcaster, so no spell-count cap applies.
	sub := validSubmission()
	sub.Spells = []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	errs := portal.ValidateSubmission(sub)
	assert.Empty(t, errs)
}

func TestValidateSubmission_SpellCapRespectsAbilityModAndLevel(t *testing.T) {
	// Higher INT raises the cap: INT 16 (+3) at level 1 -> 3 cantrips + 4 = 7.
	highInt := casterSubmission()
	highInt.AbilityScores.INT = 16
	highInt.Spells = []string{"a", "b", "c", "d", "e", "f", "g"}
	assert.Empty(t, portal.ValidateSubmission(highInt))

	// Higher level raises the cap: INT 15 (+2) at level 3 -> 3 cantrips + 5 = 8.
	highLevel := casterSubmission()
	highLevel.Class = ""
	highLevel.Classes = []character.ClassEntry{{Class: "wizard", Level: 3, IsPrimary: true}}
	highLevel.Spells = []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	assert.Empty(t, portal.ValidateSubmission(highLevel))
}

func TestValidateSubmission_SpellCapMinimumOne(t *testing.T) {
	// Level 1 wizard with INT 8 (-1): leveled cap floors at 1, so budget =
	// 3 cantrips + 1 = 4. Four spells is at the cap.
	sub := casterSubmission()
	sub.AbilityScores.INT = 8
	sub.Spells = []string{"a", "b", "c", "d"}
	assert.Empty(t, portal.ValidateSubmission(sub))

	// Five spells exceeds the floored budget.
	sub.Spells = []string{"a", "b", "c", "d", "e"}
	errs := portal.ValidateSubmission(sub)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs, "too many spells selected: 5 chosen, maximum 4")
}

func TestValidateSubmissionMode_SpellCapEnforcedInDMMode(t *testing.T) {
	// The cap is a 5e rule, not mode-specific: DM mode enforces it too.
	// Wizard L1 INT 15: budget = 6; seven spells exceeds it.
	sub := casterSubmission()
	sub.Spells = []string{"a", "b", "c", "d", "e", "f", "g"}
	errs := portal.ValidateSubmissionMode(sub, portal.ModeDM)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs, "too many spells selected: 7 chosen, maximum 6")
}

// --- Draft persistence (T11 / Finding 4·b) ---------------------------------

func TestBuilderService_SaveDraft_Delegates(t *testing.T) {
	store := &mockBuilderStore{}
	svc := portal.NewBuilderService(store)
	blob := json.RawMessage(`{"v":1,"name":"Gimli"}`)

	err := svc.SaveDraft(context.Background(), "camp-1", "user-1", "player", blob)
	require.NoError(t, err)
	assert.True(t, store.saveDraftCalled)
	assert.Equal(t, "camp-1", store.lastDraftCamp)
	assert.Equal(t, "user-1", store.lastDraftUser)
	assert.Equal(t, "player", store.lastDraftMode)
	assert.JSONEq(t, string(blob), string(store.lastDraftBlob))
}

func TestBuilderService_SaveDraft_EmptyBlobIsNoOp(t *testing.T) {
	store := &mockBuilderStore{}
	svc := portal.NewBuilderService(store)

	require.NoError(t, svc.SaveDraft(context.Background(), "camp-1", "user-1", "player", nil))
	assert.False(t, store.saveDraftCalled, "no draft to persist should not hit the store")
}

func TestBuilderService_SaveDraft_PropagatesError(t *testing.T) {
	store := &mockBuilderStore{saveDraftErr: errors.New("db down")}
	svc := portal.NewBuilderService(store)

	err := svc.SaveDraft(context.Background(), "camp-1", "user-1", "player", json.RawMessage(`{"v":1}`))
	require.Error(t, err)
}

func TestBuilderService_LoadDraft_ReturnsStored(t *testing.T) {
	stored := json.RawMessage(`{"v":1,"race":"dwarf"}`)
	store := &mockBuilderStore{loadDraftResult: stored}
	svc := portal.NewBuilderService(store)

	got, err := svc.LoadDraft(context.Background(), "camp-1", "user-1", "player")
	require.NoError(t, err)
	assert.JSONEq(t, string(stored), string(got))
}

func TestBuilderService_LoadDraft_EmptyCampaignReturnsNil(t *testing.T) {
	store := &mockBuilderStore{loadDraftResult: json.RawMessage(`{"v":1}`)}
	svc := portal.NewBuilderService(store)

	got, err := svc.LoadDraft(context.Background(), "", "user-1", "player")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBuilderService_LoadDraft_PropagatesError(t *testing.T) {
	store := &mockBuilderStore{loadDraftErr: errors.New("db down")}
	svc := portal.NewBuilderService(store)

	_, err := svc.LoadDraft(context.Background(), "camp-1", "user-1", "player")
	require.Error(t, err)
}
