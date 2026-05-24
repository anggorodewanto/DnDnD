package discord

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
)

// captureFull records the entire InteractionResponse so tests can assert on
// response Type, modal CustomID, and message components (buttons / text inputs).
type fullCapture struct {
	resp *discordgo.InteractionResponse
}

func captureFull(mock *MockSession) *fullCapture {
	fc := &fullCapture{}
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		fc.resp = resp
		return nil
	}
	return fc
}

// buttonCustomIDs flattens every Button CustomID across the response's action rows.
func buttonCustomIDs(resp *discordgo.InteractionResponse) []string {
	var ids []string
	if resp == nil || resp.Data == nil {
		return ids
	}
	for _, row := range resp.Data.Components {
		ar, ok := row.(discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, c := range ar.Components {
			if btn, ok := c.(discordgo.Button); ok {
				ids = append(ids, btn.CustomID)
			}
		}
	}
	return ids
}

func makeComponentInteraction(customID, userID, guildID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		Data:    discordgo.MessageComponentInteractionData{CustomID: customID},
		GuildID: guildID,
		Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
	}
}

// makeModalInteraction builds a modal-submit interaction with a single text
// input. Components use pointer types to mirror how discordgo unmarshals real
// modal submissions off the wire.
func makeModalInteraction(customID, inputID, value, userID, guildID string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type: discordgo.InteractionModalSubmit,
		Data: discordgo.ModalSubmitInteractionData{
			CustomID: customID,
			Components: []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					&discordgo.TextInput{CustomID: inputID, Value: value},
				}},
			},
		},
		GuildID: guildID,
		Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
	}
}

// --- /register chooser (no name) ---

func TestRegisterHandler_NoName_ShowsChoiceButtons(t *testing.T) {
	mock := newTestMock()
	fc := captureFull(mock)

	handler := NewRegisterHandler(mock, newMockRegService(), newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.Handle(makeInteraction("register", "player-1", "guild-1"))

	require.NotNil(t, fc.resp)
	assert.Equal(t, discordgo.InteractionResponseChannelMessageWithSource, fc.resp.Type)
	assert.Equal(t, discordgo.MessageFlagsEphemeral, fc.resp.Data.Flags)

	ids := buttonCustomIDs(fc.resp)
	assert.ElementsMatch(t, []string{regChoiceClaim, regChoiceBuild, regChoiceImport}, ids)

	// Hint text mentions all three paths.
	assert.Contains(t, fc.resp.Data.Content, "Claim")
	assert.Contains(t, fc.resp.Data.Content, "Build")
	assert.Contains(t, fc.resp.Data.Content, "Import")
}

func TestRegisterHandler_WithName_StillRegistersByName(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) { return &discordgo.Message{}, nil }

	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status:          registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"},
		}, nil
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc("dm-1"))
	handler.Handle(makeInteraction("register", "player-1", "guild-1", stringOption("name", "Thorn")))

	assert.Contains(t, rc.Content, "Registration submitted")
	assert.Contains(t, rc.Content, "Thorn")
}

// --- claim modal ---

func TestRegisterHandler_ShowClaimModal_OpensModalWithNameInput(t *testing.T) {
	mock := newTestMock()
	fc := captureFull(mock)

	handler := NewRegisterHandler(mock, newMockRegService(), newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.ShowClaimModal(makeComponentInteraction(regChoiceClaim, "player-1", "guild-1"))

	require.NotNil(t, fc.resp)
	assert.Equal(t, discordgo.InteractionResponseModal, fc.resp.Type)
	assert.Equal(t, regModalClaim, fc.resp.Data.CustomID)
	assert.Equal(t, regModalNameInput, firstTextInputID(fc.resp))
}

func TestRegisterHandler_HandleClaimSubmit_RunsRegisterByName(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) { return &discordgo.Message{}, nil }

	var gotName string
	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, name string) (*registration.RegisterResult, error) {
		gotName = name
		return &registration.RegisterResult{
			Status:          registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"},
		}, nil
	}

	handler := NewRegisterHandler(mock, regService, newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc("dm-1"))
	handler.HandleClaimSubmit(makeModalInteraction(regModalClaim, regModalNameInput, "Thorn", "player-1", "guild-1"))

	assert.Equal(t, "Thorn", gotName)
	assert.Contains(t, rc.Content, "Registration submitted")
	assert.Contains(t, rc.Content, "Thorn")
}

func TestRegisterHandler_HandleClaimSubmit_EmptyName_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	handler := NewRegisterHandler(mock, newMockRegService(), newMockCampaignProvider(), staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.HandleClaimSubmit(makeModalInteraction(regModalClaim, regModalNameInput, "", "player-1", "guild-1"))

	assert.Contains(t, rc.Content, "name")
}

// --- import modal ---

func TestImportHandler_ShowImportModal_OpensModalWithURLInput(t *testing.T) {
	mock := newTestMock()
	fc := captureFull(mock)

	handler := NewImportHandler(mock, newMockRegService(), newMockCampaignProvider(), nil, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.ShowImportModal(makeComponentInteraction(regChoiceImport, "player-1", "guild-1"))

	require.NotNil(t, fc.resp)
	assert.Equal(t, discordgo.InteractionResponseModal, fc.resp.Type)
	assert.Equal(t, regModalImport, fc.resp.Data.CustomID)
	assert.Equal(t, regModalURLInput, firstTextInputID(fc.resp))
}

func TestImportHandler_HandleImportSubmit_RunsImport(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) { return &discordgo.Message{}, nil }

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
	handler.HandleImportSubmit(makeModalInteraction(regModalImport, regModalURLInput, "https://dndbeyond.com/characters/123", "player-1", "guild-1"))

	assert.Contains(t, rc.Content, "Registration submitted")
}

func TestImportHandler_HandleImportSubmit_EmptyURL_ShowsError(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)

	handler := NewImportHandler(mock, newMockRegService(), newMockCampaignProvider(), nil, staticDMQueueFunc(""), staticDMUserFunc(""))
	handler.HandleImportSubmit(makeModalInteraction(regModalImport, regModalURLInput, "", "player-1", "guild-1"))

	assert.Contains(t, rc.Content, "D&D Beyond URL")
}

// --- router component + modal dispatch ---

func newChoiceRouter(t *testing.T, mock *MockSession) *CommandRouter {
	t.Helper()
	regService := newMockRegService()
	regService.RegisterFunc = func(_ context.Context, _ uuid.UUID, _, _ string) (*registration.RegisterResult, error) {
		return &registration.RegisterResult{
			Status:          registration.ResultExactMatch,
			PlayerCharacter: &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"},
		}, nil
	}
	regService.ImportFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"}, nil
	}
	regService.CreateFunc = func(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID) (*refdata.PlayerCharacter, error) {
		return &refdata.PlayerCharacter{ID: testPCID(), Status: "pending"}, nil
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
		DMQueueFunc:  staticDMQueueFunc(""),
		DMUserFunc:   staticDMUserFunc("dm-1"),
		TokenFunc:    func(_ uuid.UUID, _ string) (string, error) { return "tok-1", nil },
	}
	return NewCommandRouter(bot, nil, deps)
}

func TestCommandRouter_RegChoiceBuild_RunsCreateCharacter(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) { return &discordgo.Message{}, nil }

	router := newChoiceRouter(t, mock)
	router.Handle(makeComponentInteraction(regChoiceBuild, "player-1", "guild-1"))

	assert.Contains(t, rc.Content, "Character Builder")
	assert.Contains(t, rc.Content, "tok-1")
}

func TestCommandRouter_RegChoiceClaim_ShowsModal(t *testing.T) {
	mock := newTestMock()
	fc := captureFull(mock)

	router := newChoiceRouter(t, mock)
	router.Handle(makeComponentInteraction(regChoiceClaim, "player-1", "guild-1"))

	require.NotNil(t, fc.resp)
	assert.Equal(t, discordgo.InteractionResponseModal, fc.resp.Type)
	assert.Equal(t, regModalClaim, fc.resp.Data.CustomID)
}

func TestCommandRouter_RegChoiceImport_ShowsModal(t *testing.T) {
	mock := newTestMock()
	fc := captureFull(mock)

	router := newChoiceRouter(t, mock)
	router.Handle(makeComponentInteraction(regChoiceImport, "player-1", "guild-1"))

	require.NotNil(t, fc.resp)
	assert.Equal(t, discordgo.InteractionResponseModal, fc.resp.Type)
	assert.Equal(t, regModalImport, fc.resp.Data.CustomID)
}

func TestCommandRouter_ModalSubmit_Claim_RunsRegister(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) { return &discordgo.Message{}, nil }

	router := newChoiceRouter(t, mock)
	router.Handle(makeModalInteraction(regModalClaim, regModalNameInput, "Thorn", "player-1", "guild-1"))

	assert.Contains(t, rc.Content, "Registration submitted")
	assert.Contains(t, rc.Content, "Thorn")
}

func TestCommandRouter_ModalSubmit_Import_RunsImport(t *testing.T) {
	mock := newTestMock()
	rc := captureResponse(mock)
	mock.ChannelMessageSendFunc = func(_, _ string) (*discordgo.Message, error) { return &discordgo.Message{}, nil }

	router := newChoiceRouter(t, mock)
	router.Handle(makeModalInteraction(regModalImport, regModalURLInput, "https://dndbeyond.com/characters/123", "player-1", "guild-1"))

	assert.Contains(t, rc.Content, "Registration submitted")
}

// firstTextInputID returns the CustomID of the first text input found in a
// modal response, or "" if none.
func firstTextInputID(resp *discordgo.InteractionResponse) string {
	if resp == nil || resp.Data == nil {
		return ""
	}
	for _, row := range resp.Data.Components {
		ar, ok := row.(discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, c := range ar.Components {
			if ti, ok := c.(discordgo.TextInput); ok {
				return ti.CustomID
			}
		}
	}
	return ""
}

// modalTextValue must read values out of pointer-typed components the way
// discordgo delivers them from the wire.
func TestModalTextValue_ReadsPointerComponents(t *testing.T) {
	in := makeModalInteraction(regModalClaim, regModalNameInput, "Aria", "p", "g")
	assert.Equal(t, "Aria", modalTextValue(in, regModalNameInput))
	assert.Equal(t, "", modalTextValue(in, "nonexistent"))
}
