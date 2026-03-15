package discord

import (
	"strings"
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

func TestCommandRouter_IgnoresNonCommandNonComponentInteractions(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionModalSubmit,
	}

	router.Handle(interaction)

	if responded {
		t.Fatal("should not respond to non-command, non-component interactions")
	}
}

func TestCommandRouter_MoveConfirmButtonRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetMoveHandler(handler)

	confirmID := "move_confirm:" + turnID.String() + ":" + combatantID.String() + ":3:0:15"
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: confirmID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from move confirm handler")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Moved to D1") {
		t.Errorf("expected moved confirmation, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestCommandRouter_MoveCancelButtonRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, _ := setupMoveHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetMoveHandler(handler)

	cancelID := "move_cancel:" + turnID.String()
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: cancelID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from move cancel handler")
	}
	if sess.lastResponse.Data.Content != "Move cancelled." {
		t.Errorf("expected cancel message, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestCommandRouter_MoveConfirmInvalidData(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

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
	router.SetMoveHandler(handler)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "move_confirm:bad-data",
		},
	}

	router.Handle(interaction)

	if !strings.Contains(respondedContent, "Invalid move data") {
		t.Errorf("expected invalid move data message, got: %s", respondedContent)
	}
}

func TestCommandRouter_UnknownComponentID(t *testing.T) {
	mock := newTestMock()
	var responded bool
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		responded = true
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "unknown_button:123",
		},
	}

	router.Handle(interaction)

	// Unknown component IDs should be silently ignored (no crash)
	if responded {
		t.Fatal("should not respond to unknown component IDs")
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

func TestCommandRouter_FlyConfirmButtonRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetFlyHandler(handler)

	confirmID := "fly_confirm:" + turnID.String() + ":" + combatantID.String() + ":30:30"
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: confirmID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from fly confirm handler")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "30ft") {
		t.Errorf("expected altitude in confirmation, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestCommandRouter_FlyCancelButtonRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, _ := setupFlyHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetFlyHandler(handler)

	cancelID := "fly_cancel:" + turnID.String()
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: cancelID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from fly cancel handler")
	}
	if sess.lastResponse.Data.Content != "Fly cancelled." {
		t.Errorf("expected cancel message, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestCommandRouter_FlyConfirmInvalidData(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

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
	router.SetFlyHandler(handler)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "fly_confirm:bad-data",
		},
	}

	router.Handle(interaction)

	if !strings.Contains(respondedContent, "Invalid fly data") {
		t.Errorf("expected invalid fly data message, got: %s", respondedContent)
	}
}

func TestCommandRouter_ProneStandButtonRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetMoveHandler(handler)

	customID := "prone_stand:" + turnID.String() + ":" + combatantID.String() + ":3:0:30"
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: customID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from prone_stand handler")
	}
	// Should get a move confirmation (Stand & move) or an error — not silence
	content := sess.lastResponse.Data.Content
	if content == "" {
		t.Fatal("expected non-empty response content")
	}
}

func TestCommandRouter_ProneCrawlButtonRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetMoveHandler(handler)

	customID := "prone_crawl:" + turnID.String() + ":" + combatantID.String() + ":3:0:30"
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: customID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from prone_crawl handler")
	}
	content := sess.lastResponse.Data.Content
	if content == "" {
		t.Fatal("expected non-empty response content")
	}
}

func TestCommandRouter_MoveConfirmWithModeRoutes(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	mock := newTestMock()
	mock.InteractionRespondFunc = func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
		sess.InteractionRespond(interaction, resp)
		return nil
	}

	bot := NewBot(mock, "app-1", newTestLogger())
	router := NewCommandRouter(bot, nil)
	router.SetMoveHandler(handler)

	// Extended format: move_confirm:<turnID>:<combatantID>:<col>:<row>:<cost>:<mode>:<standCost>
	confirmID := "move_confirm:" + turnID.String() + ":" + combatantID.String() + ":3:0:10:stand_and_move:15"
	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: confirmID,
		},
	}

	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response from move confirm with mode handler")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Stood up") {
		t.Errorf("expected stand and move confirmation, got: %s", content)
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
	if !strings.Contains(respondedContent, "attack") {
		t.Errorf("expected response to mention 'attack', got: %s", respondedContent)
	}
}

func TestCommandRouter_SetSummonCommandHandler(t *testing.T) {
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

	// Create a real SummonCommandHandler with a mock service
	handler := &SummonCommandHandler{
		session: mock,
	}
	router.SetSummonCommandHandler(handler)

	interaction := &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "command",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "creature_id", Value: "FAM"},
				{Name: "action", Value: "done"},
			},
		},
		GuildID: "guild-1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
	}

	router.Handle(interaction)

	// Since there's no encounter service wired, it should respond with an error
	if respondedContent == "" {
		t.Fatal("expected a response from the SummonCommandHandler")
	}
}
