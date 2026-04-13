package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// --- Mock types for WhisperHandler ---

type mockWhisperCampaignProvider struct {
	campaign refdata.Campaign
	err      error
}

func (m *mockWhisperCampaignProvider) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	return m.campaign, m.err
}

type mockWhisperCharacterLookup struct {
	char refdata.Character
	err  error
}

func (m *mockWhisperCharacterLookup) GetCharacterByCampaignAndDiscord(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
	return m.char, m.err
}

type mockWhisperPoster struct {
	calledPlayer    string
	calledMessage   string
	calledDiscordID string
	calledGuildID   string
	calledCampaignID string
	returnID        string
	returnErr       error
}

func (m *mockWhisperPoster) PostWhisper(_ context.Context, player, message, discordUserID, guildID, campaignID string) (string, error) {
	m.calledPlayer = player
	m.calledMessage = message
	m.calledDiscordID = discordUserID
	m.calledGuildID = guildID
	m.calledCampaignID = campaignID
	return m.returnID, m.returnErr
}

// --- Helpers ---

func whisperInteraction(guildID, userID, message string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: guildID,
		Member: &discordgo.Member{
			User: &discordgo.User{ID: userID},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "whisper",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "message", Value: message, Type: discordgo.ApplicationCommandOptionString},
			},
		},
	}
}

func runWhisperHandler(t *testing.T, h *WhisperHandler, i *discordgo.Interaction) string {
	t.Helper()
	var content string
	mock := h.session.(*MockSession)
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}
	h.Handle(i)
	return content
}

func TestWhisperHandler_NoCampaign(t *testing.T) {
	sess := &MockSession{}
	h := NewWhisperHandler(
		sess,
		&mockWhisperCampaignProvider{err: errors.New("not found")},
		&mockWhisperCharacterLookup{},
	)
	resp := runWhisperHandler(t, h, whisperInteraction("guild-1", "user-1", "hello DM"))
	if resp != "No campaign found for this server." {
		t.Errorf("unexpected response: %q", resp)
	}
}

func TestWhisperHandler_Success(t *testing.T) {
	sess := &MockSession{}
	campID := uuid.New()
	poster := &mockWhisperPoster{returnID: "item-42"}
	h := NewWhisperHandler(
		sess,
		&mockWhisperCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockWhisperCharacterLookup{char: refdata.Character{Name: "Aria"}},
	)
	h.SetNotifier(poster)

	resp := runWhisperHandler(t, h, whisperInteraction("guild-1", "user-1", "I search the drawer secretly"))

	if poster.calledPlayer != "Aria" {
		t.Errorf("player = %q want Aria", poster.calledPlayer)
	}
	if poster.calledMessage != "I search the drawer secretly" {
		t.Errorf("message = %q want 'I search the drawer secretly'", poster.calledMessage)
	}
	if poster.calledDiscordID != "user-1" {
		t.Errorf("discordID = %q want user-1", poster.calledDiscordID)
	}
	if poster.calledGuildID != "guild-1" {
		t.Errorf("guildID = %q want guild-1", poster.calledGuildID)
	}
	if poster.calledCampaignID != campID.String() {
		t.Errorf("campaignID = %q want %q", poster.calledCampaignID, campID.String())
	}
	want := "🤫 Whisper sent to the DM."
	if resp != want {
		t.Errorf("response = %q want %q", resp, want)
	}
}

func TestWhisperHandler_PosterError(t *testing.T) {
	sess := &MockSession{}
	campID := uuid.New()
	poster := &mockWhisperPoster{returnErr: errors.New("queue down")}
	h := NewWhisperHandler(
		sess,
		&mockWhisperCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockWhisperCharacterLookup{char: refdata.Character{Name: "Aria"}},
	)
	h.SetNotifier(poster)

	resp := runWhisperHandler(t, h, whisperInteraction("guild-1", "user-1", "hello"))
	if resp != "Failed to send whisper. Please try again." {
		t.Errorf("unexpected response: %q", resp)
	}
}

func TestWhisperHandler_NoPoster(t *testing.T) {
	sess := &MockSession{}
	campID := uuid.New()
	h := NewWhisperHandler(
		sess,
		&mockWhisperCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockWhisperCharacterLookup{char: refdata.Character{Name: "Aria"}},
	)
	// no SetNotifier call
	resp := runWhisperHandler(t, h, whisperInteraction("guild-1", "user-1", "hello"))
	if resp != "Whisper service is not available." {
		t.Errorf("unexpected response: %q", resp)
	}
}

func TestSetWhisperHandler_RoutesCommand(t *testing.T) {
	sess := &MockSession{}
	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)

	campID := uuid.New()
	poster := &mockWhisperPoster{returnID: "item-1"}
	h := NewWhisperHandler(
		sess,
		&mockWhisperCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockWhisperCharacterLookup{char: refdata.Character{Name: "Theron"}},
	)
	h.SetNotifier(poster)
	router.SetWhisperHandler(h)

	var content string
	sess.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}

	router.Handle(whisperInteraction("guild-1", "user-1", "secret msg"))

	if poster.calledPlayer != "Theron" {
		t.Errorf("expected whisper to route through WhisperHandler, got player=%q", poster.calledPlayer)
	}
	want := "🤫 Whisper sent to the DM."
	if content != want {
		t.Errorf("response = %q want %q", content, want)
	}
}

func TestWhisperHandler_NoCharacter(t *testing.T) {
	sess := &MockSession{}
	campID := uuid.New()
	h := NewWhisperHandler(
		sess,
		&mockWhisperCampaignProvider{campaign: refdata.Campaign{ID: campID}},
		&mockWhisperCharacterLookup{err: errors.New("not found")},
	)
	resp := runWhisperHandler(t, h, whisperInteraction("guild-1", "user-1", "hello DM"))
	if resp != "Could not find your character. Use `/register` first." {
		t.Errorf("unexpected response: %q", resp)
	}
}
