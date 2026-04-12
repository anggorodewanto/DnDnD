package discord

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestHelpHandler_NoArgs_ReturnsGeneralHelp(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	var respondedFlags discordgo.MessageFlags
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		respondedContent = resp.Data.Content
		respondedFlags = resp.Data.Flags
		return nil
	}

	handler := NewHelpHandler(mock)
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	if respondedFlags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral flag %d, got %d", discordgo.MessageFlagsEphemeral, respondedFlags)
	}

	// Should contain category headers
	for _, category := range []string{"Movement", "Combat", "Checks & Saves", "Communication", "Status & Inventory", "Character Management", "Utility"} {
		if !strings.Contains(respondedContent, category) {
			t.Errorf("expected general help to contain category %q", category)
		}
	}

	// Should contain some command names
	if !strings.Contains(respondedContent, "/move") {
		t.Error("expected general help to contain /move")
	}
	if !strings.Contains(respondedContent, "/attack") {
		t.Error("expected general help to contain /attack")
	}
}

func TestHelpHandler_AttackTopic_ReturnsAttackHelp(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	var respondedFlags discordgo.MessageFlags
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		respondedContent = resp.Data.Content
		respondedFlags = resp.Data.Flags
		return nil
	}

	handler := NewHelpHandler(mock)
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:  "topic",
					Type:  discordgo.ApplicationCommandOptionString,
					Value: "attack",
				},
			},
		},
	}
	handler.Handle(interaction)

	if respondedFlags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral flag, got %d", respondedFlags)
	}

	if !strings.Contains(respondedContent, "/attack — Attack a Target") {
		t.Error("expected attack help to contain title")
	}
	if !strings.Contains(respondedContent, "--gwm") {
		t.Error("expected attack help to contain --gwm flag")
	}
	if !strings.Contains(respondedContent, "Extra Attack") {
		t.Error("expected attack help to contain Extra Attack section")
	}
	if !strings.Contains(respondedContent, "Divine Smite prompt") {
		t.Error("expected attack help to contain Divine Smite tip")
	}
}

func TestHelpHandler_UnknownTopic_ReturnsError(t *testing.T) {
	mock := newTestMock()
	var respondedContent string
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		respondedContent = resp.Data.Content
		return nil
	}

	handler := NewHelpHandler(mock)
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:  "topic",
					Type:  discordgo.ApplicationCommandOptionString,
					Value: "nonexistent",
				},
			},
		},
	}
	handler.Handle(interaction)

	if !strings.Contains(respondedContent, "Unknown help topic") {
		t.Error("expected unknown topic error message")
	}
	if !strings.Contains(respondedContent, "nonexistent") {
		t.Error("expected error to include the topic name")
	}
}

func TestHelpHandler_AllSpecTopics_ReturnContent(t *testing.T) {
	topics := []string{
		"attack", "action", "ki", "rogue", "cleric", "paladin", "metamagic",
		"cast", "move", "check", "save", "rest", "equip", "inventory",
		"use", "give", "loot", "attune", "prepare", "retire", "register",
		"import", "create-character", "character", "recap", "distance",
		"whisper", "status", "done", "deathsave", "command", "reaction",
		"interact", "shove", "bonus", "fly", "undo", "help", "setup",
	}

	for _, topic := range topics {
		t.Run(topic, func(t *testing.T) {
			mock := newTestMock()
			var respondedContent string
			mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
				respondedContent = resp.Data.Content
				return nil
			}

			handler := NewHelpHandler(mock)
			interaction := &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "help",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  "topic",
							Type:  discordgo.ApplicationCommandOptionString,
							Value: topic,
						},
					},
				},
			}
			handler.Handle(interaction)

			if respondedContent == "" {
				t.Errorf("topic %q returned empty content", topic)
			}
			if strings.Contains(respondedContent, "Unknown help topic") {
				t.Errorf("topic %q was not recognized", topic)
			}
		})
	}
}

func TestHelpCommandDefinition_HasTopicOption(t *testing.T) {
	defs := CommandDefinitions()
	var helpCmd *discordgo.ApplicationCommand
	for _, cmd := range defs {
		if cmd.Name == "help" {
			helpCmd = cmd
			break
		}
	}
	if helpCmd == nil {
		t.Fatal("help command not found in CommandDefinitions")
	}

	if len(helpCmd.Options) == 0 {
		t.Fatal("help command should have an optional 'topic' parameter")
	}

	topicOpt := helpCmd.Options[0]
	if topicOpt.Name != "topic" {
		t.Errorf("expected option name 'topic', got %q", topicOpt.Name)
	}
	if topicOpt.Type != discordgo.ApplicationCommandOptionString {
		t.Errorf("expected string option type, got %d", topicOpt.Type)
	}
	if topicOpt.Required {
		t.Error("topic option should not be required")
	}
}

func TestSetHelpHandler_ReplacesStub(t *testing.T) {
	mock := newTestMock()
	bot := &Bot{session: mock}
	router := NewCommandRouter(bot, nil)

	var respondedContent string
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		respondedContent = resp.Data.Content
		return nil
	}

	// Before setting, should get stub response
	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	router.Handle(interaction)
	if !strings.Contains(respondedContent, "not yet implemented") {
		t.Error("expected stub response before SetHelpHandler")
	}

	// After setting, should get real help
	helpHandler := NewHelpHandler(mock)
	router.SetHelpHandler(helpHandler)

	respondedContent = ""
	router.Handle(interaction)
	if !strings.Contains(respondedContent, "Command Reference") {
		t.Error("expected real help response after SetHelpHandler")
	}
}

func TestHelpHandler_SpecTopics_MatchExactContent(t *testing.T) {
	tests := []struct {
		topic    string
		contains []string
	}{
		{"ki", []string{"Ki Abilities", "Martial Arts", "flurry-of-blows", "patient-defense", "step-of-the-wind", "Stunning Strike"}},
		{"rogue", []string{"Rogue Abilities", "Cunning Action", "Sneak Attack", "Uncanny Dodge", "Evasion"}},
		{"cleric", []string{"Cleric Abilities", "Channel Divinity", "turn-undead", "Domain spells"}},
		{"paladin", []string{"Paladin Abilities", "Divine Smite", "Lay on Hands", "Aura of Protection"}},
		{"metamagic", []string{"Metamagic", "Sorcery Point", "--careful", "--subtle", "--twinned", "font-of-magic"}},
		{"action", []string{"/action — Actions on Your Turn", "disengage", "dash", "dodge", "grapple", "Freeform actions"}},
	}

	for _, tc := range tests {
		t.Run(tc.topic, func(t *testing.T) {
			mock := newTestMock()
			var respondedContent string
			mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
				respondedContent = resp.Data.Content
				return nil
			}

			handler := NewHelpHandler(mock)
			interaction := &discordgo.Interaction{
				Type: discordgo.InteractionApplicationCommand,
				Data: discordgo.ApplicationCommandInteractionData{
					Name: "help",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name:  "topic",
							Type:  discordgo.ApplicationCommandOptionString,
							Value: tc.topic,
						},
					},
				},
			}
			handler.Handle(interaction)

			for _, s := range tc.contains {
				if !strings.Contains(respondedContent, s) {
					t.Errorf("topic %q: expected to contain %q", tc.topic, s)
				}
			}
		})
	}
}
