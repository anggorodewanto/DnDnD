package discord

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
)

// --- Mock implementations ---

type mockRegistrationService struct {
	RegisterFunc func(ctx context.Context, campaignID uuid.UUID, discordUserID, characterName string) (*registration.RegisterResult, error)
	ImportFunc   func(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error)
	CreateFunc   func(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error)
	GetStatusFunc func(ctx context.Context, campaignID uuid.UUID, discordUserID string) (*refdata.PlayerCharacter, error)
}

func (m *mockRegistrationService) Register(ctx context.Context, campaignID uuid.UUID, discordUserID, characterName string) (*registration.RegisterResult, error) {
	return m.RegisterFunc(ctx, campaignID, discordUserID, characterName)
}

func (m *mockRegistrationService) Import(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error) {
	return m.ImportFunc(ctx, campaignID, discordUserID, characterID)
}

func (m *mockRegistrationService) Create(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error) {
	return m.CreateFunc(ctx, campaignID, discordUserID, characterID)
}

func (m *mockRegistrationService) GetStatus(ctx context.Context, campaignID uuid.UUID, discordUserID string) (*refdata.PlayerCharacter, error) {
	return m.GetStatusFunc(ctx, campaignID, discordUserID)
}

type mockCampaignProvider struct {
	GetCampaignByGuildIDFunc func(ctx context.Context, guildID string) (refdata.Campaign, error)
}

func (m *mockCampaignProvider) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return m.GetCampaignByGuildIDFunc(ctx, guildID)
}

type mockCharacterCreator struct {
	CreatePlaceholderFunc func(ctx context.Context, campaignID uuid.UUID, name string, ddbURL string) (refdata.Character, error)
}

func (m *mockCharacterCreator) CreatePlaceholder(ctx context.Context, campaignID uuid.UUID, name string, ddbURL string) (refdata.Character, error) {
	return m.CreatePlaceholderFunc(ctx, campaignID, name, ddbURL)
}

// --- Test helpers ---

func testCampaignID() uuid.UUID {
	return uuid.MustParse("11111111-1111-1111-1111-111111111111")
}

func testCharacterID() uuid.UUID {
	return uuid.MustParse("22222222-2222-2222-2222-222222222222")
}

func testPCID() uuid.UUID {
	return uuid.MustParse("33333333-3333-3333-3333-333333333333")
}

// responseCapture sets up a MockSession to capture interaction response content and flags.
type responseCapture struct {
	Content string
	Flags   discordgo.MessageFlags
}

func captureResponse(mock *MockSession) *responseCapture {
	rc := &responseCapture{}
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			rc.Content = resp.Data.Content
			rc.Flags = resp.Data.Flags
		}
		return nil
	}
	return rc
}

func newMockCampaignProvider() *mockCampaignProvider {
	return &mockCampaignProvider{
		GetCampaignByGuildIDFunc: func(ctx context.Context, guildID string) (refdata.Campaign, error) {
			return refdata.Campaign{
				ID:       testCampaignID(),
				GuildID:  guildID,
				DmUserID: "dm-user-1",
				Name:     "Test Campaign",
			}, nil
		},
	}
}

func newMockRegService() *mockRegistrationService {
	return &mockRegistrationService{
		GetStatusFunc: func(ctx context.Context, campaignID uuid.UUID, discordUserID string) (*refdata.PlayerCharacter, error) {
			return nil, fmt.Errorf("not found")
		},
	}
}

func staticDMQueueFunc(channelID string) func(string) string {
	return func(_ string) string { return channelID }
}

func staticDMUserFunc(dmUserID string) func(string) string {
	return func(_ string) string { return dmUserID }
}

func makeInteraction(commandName string, userID string, guildID string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    commandName,
			Options: options,
		},
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
	}
}

func stringOption(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

// --- /register tests ---

func TestRegisterHandler_ExactMatch_SendsConfirmationAndPostsDMQueue(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	var dmQueueChannelID, dmQueueMessage string
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		dmQueueChannelID = channelID
		dmQueueMessage = content
		return &discordgo.Message{}, nil
	}

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status: registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{
				ID:     testPCID(),
				Status: "pending",
			},
		}, nil
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc("dm-queue-chan-1"), staticDMUserFunc("dm-user-1"))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	if !strings.Contains(rc.Content, "Registration submitted") {
		t.Errorf("expected confirmation message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "Thorn") {
		t.Errorf("expected character name in message, got: %s", rc.Content)
	}
	if rc.Flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral, got %d", rc.Flags)
	}

	if dmQueueChannelID != "dm-queue-chan-1" {
		t.Errorf("expected dm-queue channel, got: %s", dmQueueChannelID)
	}
	if !strings.Contains(dmQueueMessage, "Thorn") {
		t.Errorf("expected character name in dm-queue msg, got: %s", dmQueueMessage)
	}
	if !strings.Contains(dmQueueMessage, "player-1") {
		t.Errorf("expected player ID in dm-queue msg, got: %s", dmQueueMessage)
	}
	if !strings.Contains(dmQueueMessage, "/register") {
		t.Errorf("expected via method in dm-queue msg, got: %s", dmQueueMessage)
	}
}

func TestRegisterHandler_FuzzyMatch_SuggestsNames(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status:      registration.ResultFuzzyMatch,
			Suggestions: []string{"Thorn"},
		}, nil
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thron")))

	if !strings.Contains(rc.Content, "No character named") {
		t.Errorf("expected no-match message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "Thorn") {
		t.Errorf("expected suggestion, got: %s", rc.Content)
	}
}

func TestRegisterHandler_NoMatch_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status: registration.ResultNoMatch,
		}, nil
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Unknown")))

	if !strings.Contains(rc.Content, "No character named") {
		t.Errorf("expected no-match message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "No close matches") {
		t.Errorf("expected no-matches notice, got: %s", rc.Content)
	}
}

func TestRegisterHandler_NoCampaign_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	campProv := &mockCampaignProvider{
		GetCampaignByGuildIDFunc: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, fmt.Errorf("not found")
		},
	}

	handler := NewRegisterHandler(mock, newMockRegService(), campProv, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	if !strings.Contains(rc.Content, "No campaign found") {
		t.Errorf("expected no-campaign error, got: %s", rc.Content)
	}
}

func TestRegisterHandler_EmptyName_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	handler := NewRegisterHandler(mock, newMockRegService(), newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("register", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "character name") {
		t.Errorf("expected name required error, got: %s", rc.Content)
	}
}

// --- /import tests ---

func TestImportHandler_CreatesPlaceholderAndPendingRecord(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	var dmQueueMessage string
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		dmQueueMessage = content
		return &discordgo.Message{}, nil
	}

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID(), Name: "Imported (https://dndbeyond.com/char/123)"}, nil
		},
	}

	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{
			ID:         testPCID(),
			Status:     "pending",
			CreatedVia: "import",
		}, nil
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), charCreator, staticDMQueueFunc("dm-queue-chan-1"), staticDMUserFunc("dm-user-1"))
	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://dndbeyond.com/characters/12345")))

	if !strings.Contains(rc.Content, "Registration submitted") {
		t.Errorf("expected confirmation, got: %s", rc.Content)
	}
	if rc.Flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral, got %d", rc.Flags)
	}
	if !strings.Contains(dmQueueMessage, "/import") {
		t.Errorf("expected import via in dm-queue, got: %s", dmQueueMessage)
	}
}

func TestImportHandler_NoURL_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	handler := NewImportHandler(mock, newMockRegService(), newMockCampaignProvider(), nil, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("import", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "D&D Beyond URL") {
		t.Errorf("expected URL error, got: %s", rc.Content)
	}
}

// --- /create-character tests ---

func TestCreateCharacterHandler_CreatesRecordAndReturnsPortalLink(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	var dmQueueMessage string
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		dmQueueMessage = content
		return &discordgo.Message{}, nil
	}

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID(), Name: "New Character"}, nil
		},
	}

	regService := newMockRegService()
	regService.CreateFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{
			ID:         testPCID(),
			Status:     "pending",
			CreatedVia: "create",
		}, nil
	}

	tokenFunc := func(_ uuid.UUID, _ string) string { return "test-token-123" }

	handler := NewCreateCharacterHandler(mock, regService, newMockCampaignProvider(), charCreator, staticDMQueueFunc("dm-queue-chan-1"), staticDMUserFunc("dm-user-1"), tokenFunc)
	handler.Handle(makeInteraction("create-character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "Registration submitted") {
		t.Errorf("expected confirmation, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "test-token-123") {
		t.Errorf("expected portal token in response, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "24 hours") {
		t.Errorf("expected expiry notice, got: %s", rc.Content)
	}
	if rc.Flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral, got %d", rc.Flags)
	}
	if !strings.Contains(dmQueueMessage, "/create-character") {
		t.Errorf("expected create-character via in dm-queue, got: %s", dmQueueMessage)
	}
}

// --- Status-aware response tests ---

func TestStatusCheckResponse_Pending(t *testing.T) {
	pc := &refdata.PlayerCharacter{
		Status:    "pending",
		CreatedAt: time.Now().Add(-30 * time.Minute),
	}
	msg := StatusCheckResponse(pc, "Thorn")
	if !strings.Contains(msg, "pending DM approval") {
		t.Errorf("expected pending message, got: %s", msg)
	}
	if !strings.Contains(msg, "Thorn") {
		t.Errorf("expected character name, got: %s", msg)
	}
}

func TestStatusCheckResponse_ChangesRequested(t *testing.T) {
	pc := &refdata.PlayerCharacter{
		Status:     "changes_requested",
		DmFeedback: sql.NullString{String: "Please fix your backstory", Valid: true},
	}
	msg := StatusCheckResponse(pc, "Thorn")
	if !strings.Contains(msg, "DM requested changes") {
		t.Errorf("expected changes message, got: %s", msg)
	}
	if !strings.Contains(msg, "Please fix your backstory") {
		t.Errorf("expected feedback, got: %s", msg)
	}
}

func TestStatusCheckResponse_Approved_ReturnsEmpty(t *testing.T) {
	pc := &refdata.PlayerCharacter{Status: "approved"}
	msg := StatusCheckResponse(pc, "Thorn")
	if msg != "" {
		t.Errorf("expected empty for approved, got: %s", msg)
	}
}

func TestNoRegistrationMessage_Content(t *testing.T) {
	if !strings.Contains(NoRegistrationMessage, "/create-character") {
		t.Errorf("expected /create-character in message: %s", NoRegistrationMessage)
	}
	if !strings.Contains(NoRegistrationMessage, "/import") {
		t.Errorf("expected /import in message: %s", NoRegistrationMessage)
	}
	if !strings.Contains(NoRegistrationMessage, "/register") {
		t.Errorf("expected /register in message: %s", NoRegistrationMessage)
	}
}

// --- Status-aware stub handler tests ---

func TestStatusAwareStubHandler_UnregisteredPlayer_ShowsNoCharMessage(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	regService := newMockRegService()

	handler := NewStatusAwareStubHandler(mock, "attack", regService, newMockCampaignProvider(), nil)
	handler.Handle(makeInteraction("attack", "player-1", "guild-1", stringOption("target", "G2")))

	if !strings.Contains(rc.Content, "No character found") {
		t.Errorf("expected no-character message, got: %s", rc.Content)
	}
}

func TestStatusAwareStubHandler_PendingPlayer_ShowsCharacterName(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	charID := testCharacterID()
	regService := newMockRegService()
	regService.GetStatusFunc = func(_ context.Context, _ uuid.UUID, _ string) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{
			Status:      "pending",
			CharacterID: charID,
			CreatedAt:   time.Now().Add(-2 * time.Hour),
		}, nil
	}

	nameResolver := func(_ context.Context, id uuid.UUID) (string, error) {
		if id == charID {
			return "Thorn", nil
		}
		return "", fmt.Errorf("not found")
	}

	handler := NewStatusAwareStubHandler(mock, "attack", regService, newMockCampaignProvider(), nameResolver)
	handler.Handle(makeInteraction("attack", "player-1", "guild-1", stringOption("target", "G2")))

	if !strings.Contains(rc.Content, "pending DM approval") {
		t.Errorf("expected pending status, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "Thorn") {
		t.Errorf("expected character name 'Thorn' in status message, got: %s", rc.Content)
	}
}

func TestStatusAwareStubHandler_ApprovedPlayer_ShowsStubMessage(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	regService := newMockRegService()
	regService.GetStatusFunc = func(_ context.Context, _ uuid.UUID, _ string) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{Status: "approved"}, nil
	}

	handler := NewStatusAwareStubHandler(mock, "attack", regService, newMockCampaignProvider(), nil)
	handler.Handle(makeInteraction("attack", "player-1", "guild-1", stringOption("target", "G2")))

	if !strings.Contains(rc.Content, "not yet implemented") {
		t.Errorf("expected stub message for approved player, got: %s", rc.Content)
	}
}

func TestStatusAwareStubHandler_ChangesRequested_ShowsFeedback(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	regService := newMockRegService()
	regService.GetStatusFunc = func(_ context.Context, _ uuid.UUID, _ string) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{
			Status:     "changes_requested",
			DmFeedback: sql.NullString{String: "Fix your backstory", Valid: true},
		}, nil
	}

	handler := NewStatusAwareStubHandler(mock, "attack", regService, newMockCampaignProvider(), nil)
	handler.Handle(makeInteraction("attack", "player-1", "guild-1", stringOption("target", "G2")))

	if !strings.Contains(rc.Content, "DM requested changes") {
		t.Errorf("expected changes-requested message, got: %s", rc.Content)
	}
	if !strings.Contains(rc.Content, "Fix your backstory") {
		t.Errorf("expected feedback in message, got: %s", rc.Content)
	}
}

// --- Utility tests ---

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		d      time.Duration
		expect string
	}{
		{30 * time.Second, "just now"},
		{1 * time.Minute, "1 minute ago"},
		{45 * time.Minute, "45 minutes ago"},
		{1 * time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{25 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3 days ago"},
	}

	for _, tc := range tests {
		t.Run(tc.expect, func(t *testing.T) {
			got := formatRelativeTime(tc.d)
			if got != tc.expect {
				t.Errorf("formatRelativeTime(%v) = %q, want %q", tc.d, got, tc.expect)
			}
		})
	}
}

func TestTruncateURL(t *testing.T) {
	short := "https://short.url"
	if truncateURL(short, 40) != short {
		t.Errorf("short URL should not be truncated")
	}

	long := "https://www.dndbeyond.com/characters/1234567890"
	result := truncateURL(long, 30)
	if len(result) > 30 {
		t.Errorf("truncated URL too long: %d", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected ... suffix, got: %s", result)
	}
}

func TestGeneratePortalToken_Format(t *testing.T) {
	campID := testCampaignID()
	token := GeneratePortalToken(campID, "player-1")
	if !strings.HasPrefix(token, campID.String()[:8]) {
		t.Errorf("token should start with campaign ID prefix, got: %s", token)
	}
	if !strings.Contains(token, "player-1") {
		t.Errorf("token should contain player ID, got: %s", token)
	}
}

func TestInteractionUserID_FromMember(t *testing.T) {
	interaction := &discordgo.Interaction{
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user-123"},
		},
	}
	if interactionUserID(interaction) != "user-123" {
		t.Errorf("expected user-123")
	}
}

func TestInteractionUserID_FromUser(t *testing.T) {
	interaction := &discordgo.Interaction{
		User: &discordgo.User{ID: "user-456"},
	}
	if interactionUserID(interaction) != "user-456" {
		t.Errorf("expected user-456")
	}
}

func TestInteractionUserID_NoUser(t *testing.T) {
	interaction := &discordgo.Interaction{}
	if interactionUserID(interaction) != "" {
		t.Errorf("expected empty string")
	}
}

// --- Router integration tests ---

func TestCommandRouter_RegisterHandlerRoutesCorrectly(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		return &discordgo.Message{}, nil
	}

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status: registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{
				ID:     testPCID(),
				Status: "pending",
			},
		}, nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.handlers["register"] = NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc("dm-queue-1"), staticDMUserFunc("dm-user-1"))

	router.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	if !strings.Contains(rc.Content, "Registration submitted") {
		t.Errorf("expected registration handler response, got: %s", rc.Content)
	}
}

func TestRegisterHandler_ServiceError_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return nil, fmt.Errorf("database error")
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	if !strings.Contains(rc.Content, "Registration error") {
		t.Errorf("expected error message, got: %s", rc.Content)
	}
}

func TestImportHandler_NoCampaign_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	campProv := &mockCampaignProvider{
		GetCampaignByGuildIDFunc: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, fmt.Errorf("not found")
		},
	}

	handler := NewImportHandler(mock, newMockRegService(), campProv, nil, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://dndbeyond.com/characters/123")))

	if !strings.Contains(rc.Content, "No campaign found") {
		t.Errorf("expected campaign error, got: %s", rc.Content)
	}
}

func TestCreateCharacterHandler_NoCampaign_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	campProv := &mockCampaignProvider{
		GetCampaignByGuildIDFunc: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, fmt.Errorf("not found")
		},
	}

	handler := NewCreateCharacterHandler(mock, newMockRegService(), campProv, nil, staticDMQueueFunc(""), staticDMUserFunc(""), nil)
	handler.Handle(makeInteraction("create-character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "No campaign found") {
		t.Errorf("expected campaign error, got: %s", rc.Content)
	}
}

func TestImportHandler_CharCreatorError_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{}, fmt.Errorf("db error")
		},
	}

	handler := NewImportHandler(mock, newMockRegService(), newMockCampaignProvider(), charCreator, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://dndbeyond.com/characters/123")))

	if !strings.Contains(rc.Content, "Import error") {
		t.Errorf("expected import error, got: %s", rc.Content)
	}
}

func TestCreateCharacterHandler_CharCreatorError_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{}, fmt.Errorf("db error")
		},
	}

	handler := NewCreateCharacterHandler(mock, newMockRegService(), newMockCampaignProvider(), charCreator, staticDMQueueFunc(""), staticDMUserFunc(""), nil)
	handler.Handle(makeInteraction("create-character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "Error creating character") {
		t.Errorf("expected char creation error, got: %s", rc.Content)
	}
}

func TestImportHandler_RegServiceError_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID()}, nil
		},
	}

	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return nil, fmt.Errorf("duplicate registration")
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), charCreator, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://dndbeyond.com/characters/123")))

	if !strings.Contains(rc.Content, "Import error") {
		t.Errorf("expected import error, got: %s", rc.Content)
	}
}

func TestCreateCharacterHandler_RegServiceError_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID()}, nil
		},
	}

	regService := newMockRegService()
	regService.CreateFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return nil, fmt.Errorf("duplicate")
	}

	handler := NewCreateCharacterHandler(mock, regService, newMockCampaignProvider(), charCreator, staticDMQueueFunc(""), staticDMUserFunc(""), nil)
	handler.Handle(makeInteraction("create-character", "player-1", "guild-1"))

	if !strings.Contains(rc.Content, "Error") {
		t.Errorf("expected error, got: %s", rc.Content)
	}
}

func TestImportHandler_DMQueueNotSent_WhenChannelIDEmpty(t *testing.T) {
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	var channelMessageSent bool
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		channelMessageSent = true
		return &discordgo.Message{}, nil
	}

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID()}, nil
		},
	}
	regService := newMockRegService()
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"}, nil
	}

	handler := NewImportHandler(mock, regService, newMockCampaignProvider(), charCreator, staticDMQueueFunc(""), staticDMUserFunc(""))
	interaction := makeInteraction("import", "player-1", "guild-1", stringOption("ddb-url", "https://dndbeyond.com/characters/123"))
	handler.Handle(interaction)

	if channelMessageSent {
		t.Error("should not send to dm-queue when channel ID is empty")
	}
}

func TestCreateCharacterHandler_DMQueueNotSent_WhenChannelIDEmpty(t *testing.T) {
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		return nil
	}
	var channelMessageSent bool
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		channelMessageSent = true
		return &discordgo.Message{}, nil
	}

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID()}, nil
		},
	}
	regService := newMockRegService()
	regService.CreateFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"}, nil
	}

	handler := NewCreateCharacterHandler(mock, regService, newMockCampaignProvider(), charCreator, staticDMQueueFunc(""), staticDMUserFunc(""), func(_ uuid.UUID, _ string) string { return "token" })
	interaction := makeInteraction("create-character", "player-1", "guild-1")
	handler.Handle(interaction)

	if channelMessageSent {
		t.Error("should not send to dm-queue when channel ID is empty")
	}
}

func TestStatusCheckResponse_UnknownStatus_ReturnsEmpty(t *testing.T) {
	pc := &refdata.PlayerCharacter{Status: "some_future_status"}
	msg := StatusCheckResponse(pc, "Thorn")
	if msg != "" {
		t.Errorf("expected empty for unknown status, got: %s", msg)
	}
}

func TestGameCommandStatusCheck_PendingWithResolver_ShowsCharacterName(t *testing.T) {
	charID := testCharacterID()
	regService := newMockRegService()
	regService.GetStatusFunc = func(_ context.Context, _ uuid.UUID, _ string) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{
			Status:      "pending",
			CharacterID: charID,
			CreatedAt:   time.Now().Add(-5 * time.Minute),
		}, nil
	}

	nameResolver := func(_ context.Context, id uuid.UUID) (string, error) {
		if id == charID {
			return "Thorn", nil
		}
		return "", fmt.Errorf("not found")
	}

	msg := GameCommandStatusCheck(context.Background(), regService, newMockCampaignProvider(), nameResolver, "guild-1", "user-1")
	if !strings.Contains(msg, "Thorn") {
		t.Errorf("expected character name 'Thorn', got: %s", msg)
	}
	if !strings.Contains(msg, "pending DM approval") {
		t.Errorf("expected pending message, got: %s", msg)
	}
}

func TestGameCommandStatusCheck_PendingWithNilResolver_FallsBack(t *testing.T) {
	regService := newMockRegService()
	regService.GetStatusFunc = func(_ context.Context, _ uuid.UUID, _ string) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{
			Status:    "pending",
			CreatedAt: time.Now().Add(-5 * time.Minute),
		}, nil
	}

	msg := GameCommandStatusCheck(context.Background(), regService, newMockCampaignProvider(), nil, "guild-1", "user-1")
	if !strings.Contains(msg, "Your character") {
		t.Errorf("expected fallback name 'Your character', got: %s", msg)
	}
}

func TestGameCommandStatusCheck_NoCampaign_ReturnsEmpty(t *testing.T) {
	campProv := &mockCampaignProvider{
		GetCampaignByGuildIDFunc: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, fmt.Errorf("not found")
		},
	}
	msg := GameCommandStatusCheck(context.Background(), newMockRegService(), campProv, nil, "guild-1", "user-1")
	if msg != "" {
		t.Errorf("expected empty when no campaign, got: %s", msg)
	}
}

func TestDMQueueNotSent_WhenChannelIDEmpty(t *testing.T) {
	mock := newTestMock()
	captureResponse(mock)

	var channelMessageSent bool
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		channelMessageSent = true
		return &discordgo.Message{}, nil
	}

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status:          registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"},
		}, nil
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc("dm-1"))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	if channelMessageSent {
		t.Error("should not send to dm-queue when channel ID is empty")
	}
}

func TestStatusCheckResponse_Rejected(t *testing.T) {
	pc := &refdata.PlayerCharacter{Status: "rejected"}
	msg := StatusCheckResponse(pc, "Thorn")
	if !strings.Contains(msg, "rejected") {
		t.Errorf("expected rejected message, got: %s", msg)
	}
}

func TestNewCommandRouter_WithRegDeps_WiresRealHandlers(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		return &discordgo.Message{}, nil
	}

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status: registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{
				ID:     testPCID(),
				Status: "pending",
			},
		}, nil
	}

	charCreator := &mockCharacterCreator{
		CreatePlaceholderFunc: func(_ context.Context, _ uuid.UUID, _ string, _ string) (refdata.Character, error) {
			return refdata.Character{ID: testCharacterID()}, nil
		},
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	deps := &RegistrationDeps{
		RegService:   regService,
		CampaignProv: newMockCampaignProvider(),
		CharCreator:  charCreator,
		DMQueueFunc:  staticDMQueueFunc("dm-queue-1"),
		DMUserFunc:   staticDMUserFunc("dm-user-1"),
	}
	router := NewCommandRouter(bot, nil, deps)

	router.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))
	if !strings.Contains(rc.Content, "Registration submitted") {
		t.Errorf("expected real register handler, got: %s", rc.Content)
	}

	rc.Content = ""
	router.Handle(makeInteraction("attack", "player-1", "guild-1", stringOption("target", "G2")))
	if !strings.Contains(rc.Content, "No character found") {
		t.Errorf("expected status-aware response for unregistered player, got: %s", rc.Content)
	}
}

func TestNewCommandRouter_WithoutRegDeps_UsesStubs(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	if !strings.Contains(rc.Content, "not yet implemented") {
		t.Errorf("expected stub handler without deps, got: %s", rc.Content)
	}
}
