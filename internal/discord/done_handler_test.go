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

func makeDoneInteraction() *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user1"},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "done",
		},
	}
}

func setupDoneHandler(sess *mockMoveSession) (*DoneHandler, uuid.UUID, uuid.UUID, uuid.UUID) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()

	combatSvc := &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "A",
				PositionRow: 1,
				IsAlive: true, HpCurrent: 10,
				IsNpc:       false,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, PositionCol: "A", PositionRow: 1, IsAlive: true, HpCurrent: 10, IsNpc: false},
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

	handler := NewDoneHandler(sess, combatSvc, turnProv, encProv)
	return handler, encounterID, turnID, combatantID
}

func TestDoneHandler_RejectsSharingTile(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupDoneHandler(sess)

	otherID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "A",
				PositionRow: 1,
				IsAlive: true, HpCurrent: 10,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, PositionCol: "A", PositionRow: 1, IsAlive: true},
				{ID: otherID, PositionCol: "A", PositionRow: 1, IsAlive: true, HpCurrent: 10, DisplayName: "Goblin"},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{ID: turnID, CombatantID: combatantID}, nil
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "can't end your turn") {
		t.Errorf("expected rejection message, got: %s", content)
	}
	if !strings.Contains(content, "Goblin") {
		t.Errorf("expected mention of Goblin, got: %s", content)
	}
}

func TestDoneHandler_AllowsEndTurnAlone(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDoneHandler(sess)

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	// Should NOT contain rejection message
	if strings.Contains(content, "can't end your turn") {
		t.Errorf("expected no rejection, got: %s", content)
	}
	// Should contain success/stub message for ending turn
	if !strings.Contains(content, "done") && !strings.Contains(content, "Turn ended") {
		t.Errorf("expected turn end message, got: %s", content)
	}
}

func TestDoneHandler_NoActiveEncounter(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDoneHandler(sess)
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active encounter") {
		t.Errorf("expected no active encounter message, got: %s", content)
	}
}

func TestDoneHandler_GetEncounterError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDoneHandler(sess)
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

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get encounter") {
		t.Errorf("expected encounter error message, got: %s", content)
	}
}

func TestDoneHandler_GetTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDoneHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("turn error")
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get turn") {
		t.Errorf("expected turn error message, got: %s", content)
	}
}

func TestDoneHandler_GetCombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDoneHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("combatant error")
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error message, got: %s", content)
	}
}

func TestDoneHandler_ListCombatantsError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupDoneHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: handler.combatService.(*mockMoveService).getCombatant,
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, errors.New("list error")
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to list combatants") {
		t.Errorf("expected list combatants error message, got: %s", content)
	}
}

func TestDoneHandler_NoActiveTurn(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _ := setupDoneHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{Valid: false},
			}, nil
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

	interaction := makeDoneInteraction()
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active turn") {
		t.Errorf("expected no active turn message, got: %s", content)
	}
}
