package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

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
