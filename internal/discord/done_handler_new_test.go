package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Mock types for new DoneHandler dependencies ---

type mockDoneTurnAdvancer struct {
	advanceTurn func(ctx context.Context, encounterID uuid.UUID) (combat.TurnInfo, error)
}

func (m *mockDoneTurnAdvancer) AdvanceTurn(ctx context.Context, encounterID uuid.UUID) (combat.TurnInfo, error) {
	return m.advanceTurn(ctx, encounterID)
}

type mockDoneCampaignProvider struct {
	getCampaignByGuildID func(ctx context.Context, guildID string) (refdata.Campaign, error)
}

func (m *mockDoneCampaignProvider) GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error) {
	return m.getCampaignByGuildID(ctx, guildID)
}

type mockDonePlayerLookup struct {
	getPC func(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error)
}

func (m *mockDonePlayerLookup) GetPlayerCharacterByCharacter(ctx context.Context, arg refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	return m.getPC(ctx, arg)
}

type mockTurnNotifier struct {
	notifyTurnStart func(s Session, channelID string, content string)
	notifyAutoSkip  func(s Session, channelID string, content string)
}

func (m *mockTurnNotifier) NotifyTurnStart(s Session, channelID string, content string) {
	if m.notifyTurnStart != nil {
		m.notifyTurnStart(s, channelID, content)
	}
}

func (m *mockTurnNotifier) NotifyAutoSkip(s Session, channelID string, content string) {
	if m.notifyAutoSkip != nil {
		m.notifyAutoSkip(s, channelID, content)
	}
}

type mockCampaignSettingsProvider struct {
	getSettings func(ctx context.Context, encounterID uuid.UUID) (map[string]string, error)
}

func (m *mockCampaignSettingsProvider) GetChannelIDs(ctx context.Context, encounterID uuid.UUID) (map[string]string, error) {
	return m.getSettings(ctx, encounterID)
}

type mockImpactSummaryProvider struct {
	getImpactSummary func(ctx context.Context, encounterID, combatantID uuid.UUID) string
}

func (m *mockImpactSummaryProvider) GetImpactSummary(ctx context.Context, encounterID, combatantID uuid.UUID) string {
	return m.getImpactSummary(ctx, encounterID, combatantID)
}

func setupFullDoneHandler(sess *mockMoveSession) (*DoneHandler, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	nextCombatantID := uuid.New()
	campaignID := uuid.New()
	characterID := uuid.New()

	combatSvc := &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				CampaignID:    campaignID,
				Status:        "active",
				RoundNumber:   1,
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == combatantID {
				return refdata.Combatant{
					ID:          combatantID,
					CharacterID: uuid.NullUUID{UUID: characterID, Valid: true},
					PositionCol: "A",
					PositionRow: 1,
					IsAlive:     true,
					IsNpc:       false,
					DisplayName: "Aria",
				}, nil
			}
			return refdata.Combatant{
				ID:          nextCombatantID,
				PositionCol: "B",
				PositionRow: 2,
				IsAlive:     true,
				IsNpc:       true,
				DisplayName: "Goblin #1",
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: false},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	turnProv := &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:              turnID,
				CombatantID:     combatantID,
				ActionUsed:      true,
				BonusActionUsed: true,
				AttacksRemaining: 0,
			}, nil
		},
	}

	encProv := &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	turnAdvancer := &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{
				CombatantID: nextCombatantID,
				RoundNumber: 1,
				Skipped:     false,
			}, nil
		},
	}

	campProv := &mockDoneCampaignProvider{
		getCampaignByGuildID: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{
				ID:       campaignID,
				DmUserID: "dm-user",
			}, nil
		},
	}

	playerLookup := &mockDonePlayerLookup{
		getPC: func(_ context.Context, _ refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
			return refdata.PlayerCharacter{
				DiscordUserID: "user1",
			}, nil
		},
	}

	handler := NewDoneHandler(sess, combatSvc, turnProv, encProv)
	handler.SetTurnAdvancer(turnAdvancer)
	handler.SetCampaignProvider(campProv)
	handler.SetPlayerLookup(playerLookup)

	return handler, encounterID, turnID, combatantID, nextCombatantID
}

// --- TDD Cycle 5: Ownership validation - non-owner rejected ---

func TestDoneHandler_RejectsNonOwner(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "wrong-user"},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "done",
		},
	}
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Only the current turn's player or the DM can end this turn") {
		t.Errorf("expected ownership rejection, got: %s", content)
	}
}

// --- TDD Cycle 6: DM can end any turn ---

func TestDoneHandler_DMCanEndAnyTurn(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "dm-user"},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "done",
		},
	}
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn ended") {
		t.Errorf("expected turn ended message, got: %s", content)
	}
	if !strings.Contains(content, "Goblin #1") {
		t.Errorf("expected next combatant name, got: %s", content)
	}
}

// --- TDD Cycle 7: Unused resources show confirmation prompt ---

func TestDoneHandler_UnusedResourcesShowConfirmation(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	// Override turn to have unused bonus action
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:               uuid.New(),
				CombatantID:      uuid.New(),
				ActionUsed:       true,
				BonusActionUsed:  false,
				AttacksRemaining: 1,
			}, nil
		},
	}

	// Need to also update getCombatant to match the combatant ID from the turn
	combatantID := uuid.New()
	characterID := uuid.New()
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:               uuid.New(),
				CombatantID:      combatantID,
				ActionUsed:       true,
				BonusActionUsed:  false,
				AttacksRemaining: 1,
			}, nil
		},
	}
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				CharacterID: uuid.NullUUID{UUID: characterID, Valid: true},
				PositionCol: "A",
				PositionRow: 1,
				IsAlive:     true,
				IsNpc:       false,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, PositionCol: "A", PositionRow: 1, IsAlive: true},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "You still have") {
		t.Errorf("expected unused resources warning, got: %s", content)
	}
	if !strings.Contains(content, "1 attack") {
		t.Errorf("expected attack mention, got: %s", content)
	}
	if !strings.Contains(content, "Bonus action") {
		t.Errorf("expected bonus action mention, got: %s", content)
	}
	// Should have buttons
	if len(sess.lastResponse.Data.Components) == 0 {
		t.Error("expected confirmation buttons")
	}
}

// --- TDD Cycle 8: All resources spent — immediate end ---

func TestDoneHandler_AllResourcesSpent_ImmediateEnd(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn ended") {
		t.Errorf("expected immediate turn end, got: %s", content)
	}
	if !strings.Contains(content, "Goblin #1") {
		t.Errorf("expected next combatant, got: %s", content)
	}
}

// --- TDD Cycle 9: HandleDoneConfirm ---

func TestDoneHandler_HandleDoneConfirm(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _, _ := setupFullDoneHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user1"},
		},
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "done_confirm:" + encounterID.String(),
		},
	}

	handler.HandleDoneConfirm(interaction, encounterID)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn ended") {
		t.Errorf("expected turn ended, got: %s", content)
	}
}

// --- TDD Cycle 10: HandleDoneCancel ---

func TestDoneHandler_HandleDoneCancel(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
	}

	handler.HandleDoneCancel(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "cancelled") {
		t.Errorf("expected cancel message, got: %s", content)
	}
}

// --- TDD Cycle 11: AdvanceTurn error ---

func TestDoneHandler_AdvanceTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	handler.turnAdvancer = &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{}, errors.New("advance error")
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to advance turn") {
		t.Errorf("expected advance error, got: %s", content)
	}
}

// --- TDD Cycle 12: NPC turn — only DM can end ---

func TestDoneHandler_NPCTurn_NonDMRejected(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID, _ := setupFullDoneHandler(sess)

	// Override to make combatant an NPC
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "A",
				PositionRow: 1,
				IsAlive:     true,
				IsNpc:       true,
				DisplayName: "Goblin",
			}, nil
		},
		listCombatants: handler.combatService.(*mockMoveService).listCombatants,
		updateCombatantPos: handler.combatService.(*mockMoveService).updateCombatantPos,
	}

	interaction := makeDoneInteraction() // user1, not dm-user
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Only the current turn's player or the DM") {
		t.Errorf("expected ownership rejection for NPC turn, got: %s", content)
	}
}

// --- HandleDoneConfirm error getting encounter ---

func TestDoneHandler_HandleDoneConfirm_GetEncounterError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _, _ := setupFullDoneHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("db error")
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
	}

	handler.HandleDoneConfirm(interaction, encounterID)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get encounter") {
		t.Errorf("expected error, got: %s", content)
	}
}

// --- endTurn with no advancer (backward compat) ---

func TestDoneHandler_NoAdvancer_StubMessage(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)
	handler.turnAdvancer = nil

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not yet fully implemented") {
		t.Errorf("expected stub message, got: %s", content)
	}
}

// --- TDD Cycle 13: Router routes done_confirm buttons ---

func TestRouter_DoneConfirmRouting(t *testing.T) {
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

	sess := &mockMoveSession{}
	handler, encounterID, _, _, _ := setupFullDoneHandler(sess)
	// Use the mock session from the router's bot for the handler
	handler.session = mock
	router.SetDoneHandler(handler)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "done_confirm:" + encounterID.String(),
		},
	}
	router.Handle(interaction)

	if !strings.Contains(respondedContent, "Turn ended") {
		t.Errorf("expected turn ended from confirm routing, got: %s", respondedContent)
	}
}

func TestRouter_DoneCancelRouting(t *testing.T) {
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

	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)
	handler.session = mock
	router.SetDoneHandler(handler)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Data: discordgo.MessageComponentInteractionData{
			CustomID: "done_cancel",
		},
	}
	router.Handle(interaction)

	if !strings.Contains(respondedContent, "cancelled") {
		t.Errorf("expected cancel from routing, got: %s", respondedContent)
	}
}

// --- HandleDoneConfirm with advance error ---

func TestDoneHandler_HandleDoneConfirm_AdvanceError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _, _ := setupFullDoneHandler(sess)
	handler.turnAdvancer = &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{}, errors.New("advance error")
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
	}

	handler.HandleDoneConfirm(interaction, encounterID)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to advance turn") {
		t.Errorf("expected advance error, got: %s", content)
	}
}

// --- HandleDoneConfirm with combatant error ---

func TestDoneHandler_HandleDoneConfirm_CombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _, _ := setupFullDoneHandler(sess)

	unknownID := uuid.New()
	handler.turnAdvancer = &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{CombatantID: unknownID, RoundNumber: 2}, nil
		},
	}
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == unknownID {
				return refdata.Combatant{}, errors.New("not found")
			}
			return refdata.Combatant{ID: id, IsAlive: true, PositionCol: "A", PositionRow: 1}, nil
		},
		listCombatants: handler.combatService.(*mockMoveService).listCombatants,
		updateCombatantPos: handler.combatService.(*mockMoveService).updateCombatantPos,
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
	}

	handler.HandleDoneConfirm(interaction, encounterID)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "failed to get next combatant") {
		t.Errorf("expected combatant error, got: %s", content)
	}
}

// --- HandleDoneConfirm no advancer ---

func TestDoneHandler_HandleDoneConfirm_NoAdvancer(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _, _ := setupFullDoneHandler(sess)
	handler.turnAdvancer = nil

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
	}

	handler.HandleDoneConfirm(interaction, encounterID)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn ended") {
		t.Errorf("expected turn ended, got: %s", content)
	}
}

// --- HandleDoneConfirm with skipped combatant ---

func TestDoneHandler_HandleDoneConfirm_Skipped(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _, nextCombatantID := setupFullDoneHandler(sess)

	handler.turnAdvancer = &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{
				CombatantID: nextCombatantID,
				RoundNumber: 2,
				Skipped:     true,
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user1"},
		},
	}

	handler.HandleDoneConfirm(interaction, encounterID)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "skipped") {
		t.Errorf("expected skipped message, got: %s", content)
	}
	if !strings.Contains(content, "Goblin #1") {
		t.Errorf("expected next combatant name in skipped message, got: %s", content)
	}
}

// --- Turn-start notification sent to #your-turn channel ---

func TestDoneHandler_SendsTurnStartNotification(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	var sentChannel, sentContent string
	notifier := &mockTurnNotifier{
		notifyTurnStart: func(s Session, channelID string, content string) {
			sentChannel = channelID
			sentContent = content
		},
		notifyAutoSkip: func(s Session, channelID string, content string) {},
	}
	handler.SetTurnNotifier(notifier)

	// Set campaign settings provider that returns channel_ids
	handler.SetCampaignSettingsProvider(&mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, encounterID uuid.UUID) (map[string]string, error) {
			return map[string]string{"your-turn": "chan-your-turn", "combat-log": "chan-combat-log"}, nil
		},
	})

	// Set impact summary provider
	handler.SetImpactSummaryProvider(&mockImpactSummaryProvider{
		getImpactSummary: func(_ context.Context, encounterID, combatantID uuid.UUID) string {
			return ""
		},
	})

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if sentChannel != "chan-your-turn" {
		t.Errorf("expected notification to #your-turn channel, got: %s", sentChannel)
	}
	if !strings.Contains(sentContent, "Goblin #1") {
		t.Errorf("expected next combatant name in turn start notification, got: %s", sentContent)
	}
}

// --- No notification when providers not set ---

func TestDoneHandler_NoNotificationWithoutProviders(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)
	// Don't set any notification providers

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	// Should still end turn successfully
	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn ended") {
		t.Errorf("expected turn ended, got: %s", content)
	}
}

// --- Turn-start notification includes impact summary ---

func TestDoneHandler_TurnStartNotification_WithImpact(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	var sentContent string
	notifier := &mockTurnNotifier{
		notifyTurnStart: func(s Session, channelID string, content string) {
			sentContent = content
		},
		notifyAutoSkip: func(s Session, channelID string, content string) {},
	}
	handler.SetTurnNotifier(notifier)
	handler.SetCampaignSettingsProvider(&mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"your-turn": "chan-yt"}, nil
		},
	})
	handler.SetImpactSummaryProvider(&mockImpactSummaryProvider{
		getImpactSummary: func(_ context.Context, _, _ uuid.UUID) string {
			return "\u26a0\ufe0f Since your last turn: Orc hit you for 8 damage."
		},
	})

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if !strings.Contains(sentContent, "Since your last turn") {
		t.Errorf("expected impact summary in turn start notification, got: %s", sentContent)
	}
	if !strings.Contains(sentContent, "Orc hit you for 8 damage") {
		t.Errorf("expected impact details, got: %s", sentContent)
	}
}

// --- Auto-skip messages posted to #combat-log ---

func TestDoneHandler_PostsAutoSkipToCombatLog(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, nextCombatantID := setupFullDoneHandler(sess)

	handler.turnAdvancer = &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{
				CombatantID: nextCombatantID,
				RoundNumber: 1,
				SkippedCombatants: []combat.SkippedInfo{
					{DisplayName: "Stunned Fighter", ConditionName: "stunned"},
					{DisplayName: "Paralyzed Wizard", ConditionName: "paralyzed"},
				},
			}, nil
		},
	}

	var autoSkipMessages []string
	var autoSkipChannels []string
	notifier := &mockTurnNotifier{
		notifyTurnStart: func(s Session, channelID string, content string) {},
		notifyAutoSkip: func(s Session, channelID string, content string) {
			autoSkipChannels = append(autoSkipChannels, channelID)
			autoSkipMessages = append(autoSkipMessages, content)
		},
	}
	handler.SetTurnNotifier(notifier)
	handler.SetCampaignSettingsProvider(&mockCampaignSettingsProvider{
		getSettings: func(_ context.Context, _ uuid.UUID) (map[string]string, error) {
			return map[string]string{"your-turn": "chan-your-turn", "combat-log": "chan-combat-log"}, nil
		},
	})
	handler.SetImpactSummaryProvider(&mockImpactSummaryProvider{
		getImpactSummary: func(_ context.Context, _, _ uuid.UUID) string { return "" },
	})

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if len(autoSkipMessages) != 2 {
		t.Fatalf("expected 2 auto-skip messages, got %d", len(autoSkipMessages))
	}
	if autoSkipChannels[0] != "chan-combat-log" || autoSkipChannels[1] != "chan-combat-log" {
		t.Errorf("expected messages to combat-log channel, got: %v", autoSkipChannels)
	}
	if !strings.Contains(autoSkipMessages[0], "Stunned Fighter") {
		t.Errorf("expected first skip message about Stunned Fighter, got: %s", autoSkipMessages[0])
	}
	if !strings.Contains(autoSkipMessages[1], "Paralyzed Wizard") {
		t.Errorf("expected second skip message about Paralyzed Wizard, got: %s", autoSkipMessages[1])
	}
}

// --- DefaultTurnNotifier sends via ChannelMessageSend ---

func TestDefaultTurnNotifier_NotifyTurnStart(t *testing.T) {
	sess := &mockMoveSession{}
	notifier := &DefaultTurnNotifier{}

	var sentChannel, sentContent string
	origSend := sess.ChannelMessageSend
	_ = origSend
	// We need to capture what's sent - enhance mockMoveSession
	captureSess := &captureMoveSession{}
	notifier.NotifyTurnStart(captureSess, "chan-1", "Hello turn!")

	if captureSess.lastChannelID != "chan-1" {
		t.Errorf("expected channel chan-1, got: %s", captureSess.lastChannelID)
	}
	if captureSess.lastContent != "Hello turn!" {
		t.Errorf("expected content 'Hello turn!', got: %s", captureSess.lastContent)
	}
	_ = sentChannel
	_ = sentContent
}

func TestDefaultTurnNotifier_NotifyAutoSkip(t *testing.T) {
	captureSess := &captureMoveSession{}
	notifier := &DefaultTurnNotifier{}
	notifier.NotifyAutoSkip(captureSess, "chan-log", "Skip msg")

	if captureSess.lastChannelID != "chan-log" {
		t.Errorf("expected channel chan-log, got: %s", captureSess.lastChannelID)
	}
	if captureSess.lastContent != "Skip msg" {
		t.Errorf("expected content 'Skip msg', got: %s", captureSess.lastContent)
	}
}

// captureMoveSession captures ChannelMessageSend calls.
type captureMoveSession struct {
	mockMoveSession
	lastChannelID string
	lastContent   string
}

func (m *captureMoveSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	m.lastChannelID = channelID
	m.lastContent = content
	return nil, nil
}

// --- DefaultCampaignSettingsProvider ---

func TestDefaultCampaignSettingsProvider_ReturnsChannelIDs(t *testing.T) {
	encounterID := uuid.New()
	provider := &DefaultCampaignSettingsProvider{
		getCampaign: func(ctx context.Context, id uuid.UUID) (refdata.Campaign, error) {
			settings := `{"channel_ids":{"your-turn":"ch1","combat-log":"ch2"}}`
			return refdata.Campaign{
				Settings: pqtype.NullRawMessage{RawMessage: json.RawMessage(settings), Valid: true},
			}, nil
		},
	}

	channels, err := provider.GetChannelIDs(context.Background(), encounterID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channels["your-turn"] != "ch1" {
		t.Errorf("expected your-turn=ch1, got: %s", channels["your-turn"])
	}
	if channels["combat-log"] != "ch2" {
		t.Errorf("expected combat-log=ch2, got: %s", channels["combat-log"])
	}
}

// --- GetCombatant error after advance ---

func TestDoneHandler_AdvanceTurn_GetNextCombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _, _ := setupFullDoneHandler(sess)

	unknownID := uuid.New()
	handler.turnAdvancer = &mockDoneTurnAdvancer{
		advanceTurn: func(_ context.Context, _ uuid.UUID) (combat.TurnInfo, error) {
			return combat.TurnInfo{
				CombatantID: unknownID,
				RoundNumber: 2,
			}, nil
		},
	}
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == unknownID {
				return refdata.Combatant{}, errors.New("not found")
			}
			return refdata.Combatant{
				ID:          id,
				PositionCol: "A",
				PositionRow: 1,
				IsAlive:     true,
				IsNpc:       false,
				DisplayName: "Aria",
				CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
			}, nil
		},
		listCombatants: handler.combatService.(*mockMoveService).listCombatants,
		updateCombatantPos: handler.combatService.(*mockMoveService).updateCombatantPos,
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "failed to get next combatant") {
		t.Errorf("expected error about next combatant, got: %s", content)
	}
}
