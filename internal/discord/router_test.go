package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCommandRouter_RoutesToStubForNewCommands(t *testing.T) {
	stubCommands := []string{
		"move", "fly", "attack", "cast", "bonus", "action", "shove",
		"interact", "done", "deathsave", "command", "reaction", "check",
		"save", "rest", "whisper", "status", "equip", "undo", "inventory",
		"use", "give", "loot", "attune", "unattune", "prepare", "retire",
		"register", "import", "create-character", "character", "recap",
		"distance", "help",
	}

	for _, name := range stubCommands {
		t.Run(name, func(t *testing.T) {
			mock := newTestMock()
			var respondedContent string
			var respondedFlags discordgo.MessageFlags
			mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
				if resp.Data != nil {
					respondedContent = resp.Data.Content
					respondedFlags = resp.Data.Flags
				}
				return nil
			}

			bot := NewBot(mock, "app-1", newTestLogger())
			router := NewCommandRouter(bot, nil)

			interaction := &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: name,
				},
			}

			router.Handle(interaction)

			if respondedContent == "" {
				t.Fatal("expected a response, got empty string")
			}
			if respondedFlags != discordgo.MessageFlagsEphemeral {
				t.Errorf("expected ephemeral flag, got %d", respondedFlags)
			}
		})
	}
}

func TestCommandRouter_RoutesToSetupHandler(t *testing.T) {
	mock := newTestMock()
	var deferredResponse bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Type == discordgo.InteractionResponseDeferredChannelMessageWithSource {
			deferredResponse = true
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	campaignLookup := &mockCampaignLookup{}
	setupHandler := NewSetupHandler(bot, campaignLookup)
	router := NewCommandRouter(bot, setupHandler)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "setup",
		},
		GuildID: "guild-1",
	}

	router.Handle(interaction)

	if !deferredResponse {
		t.Fatal("expected setup to route to SetupHandler (deferred response)")
	}
}

func TestCommandRouter_IgnoresNonCommandInteractions(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionMessageComponent,
	}

	router.Handle(interaction)

	if responded {
		t.Fatal("should not respond to non-command interactions")
	}
}

func TestCommandRouter_UnknownCommandRespondsWithError(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "nonexistent-command",
		},
	}

	router.Handle(interaction)

	if respondedContent == "" {
		t.Fatal("expected error response for unknown command")
	}
}

func TestCommandRouter_StubResponseIncludesCommandName(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		if resp.Data != nil {
			respondedContent = resp.Data.Content
		}
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "attack",
		},
	}

	router.Handle(interaction)

	if respondedContent == "" {
		t.Fatal("expected response content")
	}
	// Should mention the command name
	if !contains(respondedContent, "attack") {
		t.Errorf("expected response to mention 'attack', got: %s", respondedContent)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
