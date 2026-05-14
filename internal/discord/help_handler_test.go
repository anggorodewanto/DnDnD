package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// runHelpCommand invokes the help handler with the given topic (empty for no topic)
// and returns the response content and flags.
func runHelpCommand(topic string) (content string, flags discordgo.MessageFlags) {
	mock := newTestMock()
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		flags = resp.Data.Flags
		return nil
	}

	var opts []*discordgo.ApplicationCommandInteractionDataOption
	if topic != "" {
		opts = []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "topic", Type: discordgo.ApplicationCommandOptionString, Value: topic},
		}
	}

	handler := NewHelpHandler(mock)
	handler.Handle(&discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "help",
			Options: opts,
		},
	})
	return content, flags
}

func TestHelpHandler_NoArgs_ReturnsGeneralHelp(t *testing.T) {
	content, flags := runHelpCommand("")

	if flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral flag %d, got %d", discordgo.MessageFlagsEphemeral, flags)
	}

	for _, category := range []string{"Movement", "Combat", "Checks & Saves", "Communication", "Status & Inventory", "Character Management", "Utility"} {
		if !strings.Contains(content, category) {
			t.Errorf("expected general help to contain category %q", category)
		}
	}

	if !strings.Contains(content, "/move") {
		t.Error("expected general help to contain /move")
	}
	if !strings.Contains(content, "/attack") {
		t.Error("expected general help to contain /attack")
	}
}

func TestHelpHandler_AttackTopic_ReturnsAttackHelp(t *testing.T) {
	content, flags := runHelpCommand("attack")

	if flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("expected ephemeral flag, got %d", flags)
	}

	for _, want := range []string{"/attack — Attack a Target", "--gwm", "Extra Attack", "Divine Smite prompt"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected attack help to contain %q", want)
		}
	}
}

func TestHelpHandler_UnknownTopic_ReturnsError(t *testing.T) {
	content, _ := runHelpCommand("nonexistent")

	if !strings.Contains(content, "Unknown help topic") {
		t.Error("expected unknown topic error message")
	}
	if !strings.Contains(content, "nonexistent") {
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
			content, _ := runHelpCommand(topic)

			if content == "" {
				t.Errorf("topic %q returned empty content", topic)
			}
			if strings.Contains(content, "Unknown help topic") {
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
			content, _ := runHelpCommand(tc.topic)

			for _, s := range tc.contains {
				if !strings.Contains(content, s) {
					t.Errorf("topic %q: expected to contain %q", tc.topic, s)
				}
			}
		})
	}
}

// mockHelpEncounterProvider implements HelpEncounterProvider for tests.
type mockHelpEncounterProvider struct {
	encounterID uuid.UUID
	encounter   refdata.Encounter
	err         error
}

func (m *mockHelpEncounterProvider) ActiveEncounterForUser(_ context.Context, _, _ string) (uuid.UUID, error) {
	return m.encounterID, m.err
}

func (m *mockHelpEncounterProvider) GetEncounter(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
	return m.encounter, m.err
}

func TestHelpHandler_CombatContext_ShowsCombatTips(t *testing.T) {
	mock := newTestMock()
	var content string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}

	encID := uuid.New()
	provider := &mockHelpEncounterProvider{
		encounterID: encID,
		encounter:   refdata.Encounter{ID: encID, Mode: "combat"},
	}

	handler := NewHelpHandler(mock)
	handler.SetEncounterProvider(provider)
	handler.Handle(&discordgo.Interaction{
		GuildID: "guild-1",
		Type:    discordgo.InteractionApplicationCommand,
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	})

	if !strings.Contains(content, "/move") {
		t.Error("expected combat tips to mention /move")
	}
	if !strings.Contains(content, "Context Tips") {
		t.Error("expected combat context tips section header")
	}
	if strings.Contains(content, "exploration") {
		t.Error("should not contain exploration tips during combat")
	}
}

func TestHelpHandler_ExplorationContext_ShowsExplorationTips(t *testing.T) {
	mock := newTestMock()
	var content string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}

	encID := uuid.New()
	provider := &mockHelpEncounterProvider{
		encounterID: encID,
		encounter:   refdata.Encounter{ID: encID, Mode: "exploration"},
	}

	handler := NewHelpHandler(mock)
	handler.SetEncounterProvider(provider)
	handler.Handle(&discordgo.Interaction{
		GuildID: "guild-1",
		Type:    discordgo.InteractionApplicationCommand,
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	})

	if !strings.Contains(content, "Context Tips") {
		t.Error("expected exploration context tips section header")
	}
	if !strings.Contains(content, "/action") {
		t.Error("expected exploration tips to mention /action")
	}
}

func TestHelpHandler_NoEncounter_NoContextTips(t *testing.T) {
	mock := newTestMock()
	var content string
	mock.InteractionRespondFunc = func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		content = resp.Data.Content
		return nil
	}

	provider := &mockHelpEncounterProvider{
		err: errors.New("no active encounter"),
	}

	handler := NewHelpHandler(mock)
	handler.SetEncounterProvider(provider)
	handler.Handle(&discordgo.Interaction{
		GuildID: "guild-1",
		Type:    discordgo.InteractionApplicationCommand,
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "help",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	})

	if strings.Contains(content, "Context Tips") {
		t.Error("should not show context tips when no encounter is active")
	}
	// Should still show general help
	if !strings.Contains(content, "Command Reference") {
		t.Error("expected general help content")
	}
}
