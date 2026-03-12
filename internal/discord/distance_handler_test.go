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

func makeDistanceInteraction(target string, target2 ...string) *discordgo.Interaction {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "target", Value: target, Type: discordgo.ApplicationCommandOptionString},
	}
	if len(target2) > 0 {
		opts = append(opts, &discordgo.ApplicationCommandInteractionDataOption{
			Name: "target2", Value: target2[0], Type: discordgo.ApplicationCommandOptionString,
		})
	}
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "distance",
			Options: opts,
		},
	}
}

func setupDistanceHandler(sess *mockMoveSession) (*DistanceHandler, uuid.UUID, uuid.UUID, uuid.UUID) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()

	combatants := []refdata.Combatant{
		{ID: combatantID, ShortID: "AR", DisplayName: "Aria", PositionCol: "A", PositionRow: 1, AltitudeFt: 0},
		{ShortID: "G1", DisplayName: "Goblin #1", PositionCol: "F", PositionRow: 1, AltitudeFt: 0},
	}

	combatSvc := &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			for _, c := range combatants {
				if c.ID == id {
					return c, nil
				}
			}
			return refdata.Combatant{}, errors.New("not found")
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return combatants, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	turnProv := &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:          turnID,
				CombatantID: combatantID,
			}, nil
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{}, nil
		},
	}

	encProv := &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewDistanceHandler(sess, combatSvc, turnProv, encProv)
	return handler, encounterID, turnID, combatantID
}

func TestDistanceHandler_WiredViaRouter(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	bot := &Bot{session: sess}
	router := NewCommandRouter(bot, nil)
	router.SetDistanceHandler(handler)

	interaction := makeDistanceInteraction("G1")
	router.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "25ft") {
		t.Errorf("expected 25ft distance, got: %s", content)
	}
}

func TestDistanceHandler_NoActiveEncounter(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active encounter") {
		t.Errorf("expected no active encounter, got: %s", content)
	}
}

func TestDistanceHandler_GetEncounterError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("db error")
		},
		getCombatant:       func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get encounter") {
		t.Errorf("expected encounter error, got: %s", content)
	}
}

func TestDistanceHandler_ListCombatantsError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)
	origSvc := handler.combatService.(*mockMoveService)
	handler.combatService = &mockMoveService{
		getEncounter:       origSvc.getEncounter,
		getCombatant:       origSvc.getCombatant,
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, errors.New("db error") },
		updateCombatantPos: origSvc.updateCombatantPos,
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to list combatants") {
		t.Errorf("expected list error, got: %s", content)
	}
}

func TestDistanceHandler_NoActiveTurn(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _ := setupDistanceHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{Valid: false},
			}, nil
		},
		getCombatant:       func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active turn") {
		t.Errorf("expected no active turn, got: %s", content)
	}
}

func TestDistanceHandler_GetTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("turn error")
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{}, nil
		},
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get turn") {
		t.Errorf("expected turn error, got: %s", content)
	}
}

func TestDistanceHandler_GetCombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)
	origSvc := handler.combatService.(*mockMoveService)
	handler.combatService = &mockMoveService{
		getEncounter: origSvc.getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("combatant error")
		},
		listCombatants:     origSvc.listCombatants,
		updateCombatantPos: origSvc.updateCombatantPos,
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error, got: %s", content)
	}
}

func TestDistanceHandler_TargetNotFound(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	interaction := makeDistanceInteraction("ZZ")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not found") {
		t.Errorf("expected not found error, got: %s", content)
	}
}

func TestDistanceHandler_TwoTargets_FirstNotFound(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	interaction := makeDistanceInteraction("ZZ", "AR")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not found") {
		t.Errorf("expected not found error, got: %s", content)
	}
}

func TestDistanceHandler_TwoTargets_SecondNotFound(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	interaction := makeDistanceInteraction("G1", "ZZ")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "not found") {
		t.Errorf("expected not found error, got: %s", content)
	}
}

func TestDistanceHandler_NoOptions(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "distance",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "provide a target") {
		t.Errorf("expected provide target message, got: %s", content)
	}
}

func TestDistanceHandler_3DDistance(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupDistanceHandler(sess)

	// Override combatants with altitude
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == combatantID {
				return refdata.Combatant{
					ID: combatantID, ShortID: "AR", DisplayName: "Aria",
					PositionCol: "A", PositionRow: 1, AltitudeFt: 0,
				}, nil
			}
			return refdata.Combatant{}, errors.New("not found")
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, ShortID: "AR", DisplayName: "Aria", PositionCol: "A", PositionRow: 1, AltitudeFt: 0},
				{ShortID: "G1", DisplayName: "Goblin #1", PositionCol: "A", PositionRow: 1, AltitudeFt: 30},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	// Same position but 30ft altitude difference => 30ft
	if !strings.Contains(content, "30ft") {
		t.Errorf("expected 30ft for altitude distance, got: %s", content)
	}
}

func TestDistanceHandler_TwoTargets(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	interaction := makeDistanceInteraction("G1", "AR")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	// G1 at F1 (col=5), AR at A1 (col=0) => 25ft
	if !strings.Contains(content, "25ft") {
		t.Errorf("expected 25ft distance, got: %s", content)
	}
	if !strings.Contains(content, "Goblin #1 (G1)") {
		t.Errorf("expected first target in message, got: %s", content)
	}
	if !strings.Contains(content, "Aria (AR)") {
		t.Errorf("expected second target in message, got: %s", content)
	}
}

func TestDistanceHandler_SingleTarget(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDistanceHandler(sess)

	interaction := makeDistanceInteraction("G1")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	// Aria at A1 (col=0, row=0), Goblin at F1 (col=5, row=0) => 5*5=25ft
	if !strings.Contains(content, "25ft") {
		t.Errorf("expected 25ft distance, got: %s", content)
	}
	if !strings.Contains(content, "Goblin #1") {
		t.Errorf("expected target name in message, got: %s", content)
	}
	// Should be ephemeral
	if sess.lastResponse.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Error("expected ephemeral response")
	}
}
