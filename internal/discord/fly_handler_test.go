package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

func makeFlyInteraction(altitude int) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user1"},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "fly",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "altitude", Value: float64(altitude), Type: discordgo.ApplicationCommandOptionInteger},
			},
		},
	}
}

func setupFlyHandler(sess *mockMoveSession) (*FlyHandler, uuid.UUID, uuid.UUID, uuid.UUID) {
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
				AltitudeFt:  0,
				Conditions:  json.RawMessage(`[{"condition":"fly_speed"}]`),
				IsAlive: true, HpCurrent: 10,
				IsNpc:       false,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	turnProv := &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{MovementRemainingFt: 0}, nil
		},
	}

	encProv := &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewFlyHandler(sess, combatSvc, turnProv, encProv)
	return handler, encounterID, turnID, combatantID
}

func TestFlyHandler_ValidAscend_ShowsConfirmation(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

	interaction := makeFlyInteraction(30)
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "30ft altitude") {
		t.Errorf("expected altitude in message, got: %s", content)
	}
	if !strings.Contains(content, "30ft") {
		t.Errorf("expected cost in message, got: %s", content)
	}
	if len(sess.lastResponse.Data.Components) == 0 {
		t.Error("expected buttons in response")
	}
}

func TestFlyHandler_NotEnoughMovement(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 10,
			}, nil
		},
	}

	interaction := makeFlyInteraction(50)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Not enough movement") {
		t.Errorf("expected not enough movement, got: %s", content)
	}
}

func TestFlyHandler_NegativeAltitude(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

	interaction := makeFlyInteraction(-10)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "cannot be negative") {
		t.Errorf("expected negative altitude error, got: %s", content)
	}
}

func TestFlyHandler_SameAltitude(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

	// Combatant already at 0, flying to 0
	interaction := makeFlyInteraction(0)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Already at") {
		t.Errorf("expected already at altitude message, got: %s", content)
	}
}

func TestFlyHandler_NoActiveEncounter(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
	}

	interaction := makeFlyInteraction(30)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active encounter") {
		t.Errorf("expected no active encounter, got: %s", content)
	}
}

func TestFlyHandler_NoActiveTurn(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _ := setupFlyHandler(sess)
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

	interaction := makeFlyInteraction(30)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active turn") {
		t.Errorf("expected no active turn, got: %s", content)
	}
}

func TestFlyHandler_NoOptions(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "fly",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "provide an altitude") {
		t.Errorf("expected provide altitude message, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyConfirm(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	var updatedAlt int32
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) { return refdata.Encounter{}, nil },
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{PositionCol: "A", PositionRow: 1, AltitudeFt: 0}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, alt int32) (refdata.Combatant, error) {
			updatedAlt = alt
			return refdata.Combatant{}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 30)

	if updatedAlt != 30 {
		t.Errorf("expected altitude 30, got %d", updatedAlt)
	}

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "30ft") {
		t.Errorf("expected altitude in confirmation, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyCancel(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyCancel(interaction)

	content := sess.lastResponse.Data.Content
	if content != "Fly cancelled." {
		t.Errorf("expected cancel message, got: %s", content)
	}
}

func TestParseFlyConfirmData(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	customID := "fly_confirm:" + turnID.String() + ":" + combatantID.String() + ":30:30"

	gotTurnID, gotCombatantID, gotAlt, gotCost, err := ParseFlyConfirmData(customID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTurnID != turnID {
		t.Errorf("turn ID mismatch")
	}
	if gotCombatantID != combatantID {
		t.Errorf("combatant ID mismatch")
	}
	if gotAlt != 30 || gotCost != 30 {
		t.Errorf("expected (30, 30), got (%d, %d)", gotAlt, gotCost)
	}
}

func TestParseFlyConfirmData_Invalid(t *testing.T) {
	_, _, _, _, err := ParseFlyConfirmData("invalid")
	if err == nil {
		t.Error("expected error for invalid custom ID")
	}
}

func TestFlyHandler_HandleFlyConfirm_TurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("turn gone")
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, uuid.New(), uuid.New(), 30, 30)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn no longer active") {
		t.Errorf("expected turn no longer active, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyConfirm_UpdateTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("update error")
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 30)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to update turn") {
		t.Errorf("expected turn update error, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyConfirm_PositionUpdateError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) { return refdata.Encounter{}, nil },
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{PositionCol: "A", PositionRow: 1}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("pos error")
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 30)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to update position") {
		t.Errorf("expected position error, got: %s", content)
	}
}

func TestFlyHandler_GetEncounterError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, errors.New("db error")
		},
		getCombatant:       func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
	}

	interaction := makeFlyInteraction(30)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get encounter") {
		t.Errorf("expected encounter error, got: %s", content)
	}
}

func TestFlyHandler_GetTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("turn error")
		},
	}

	interaction := makeFlyInteraction(30)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get turn") {
		t.Errorf("expected turn error, got: %s", content)
	}
}

func TestFlyHandler_GetCombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupFlyHandler(sess)
	origSvc := handler.combatService.(*mockMoveService)
	handler.combatService = &mockMoveService{
		getEncounter: origSvc.getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("combatant error")
		},
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
	}

	interaction := makeFlyInteraction(30)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyConfirm_NotEnoughMovement(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 5,
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 30) // needs 30ft, only 5 remaining

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Cannot fly") {
		t.Errorf("expected cannot fly, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyConfirm_GetCombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) { return refdata.Encounter{}, nil },
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("combatant gone")
		},
		listCombatants:     func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, turnID, combatantID, 30, 30)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error, got: %s", content)
	}
}

func TestFlyHandler_HandleFlyConfirm_DescendToGround(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) { return refdata.Encounter{}, nil },
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{PositionCol: "A", PositionRow: 1, AltitudeFt: 30}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) { return nil, nil },
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleFlyConfirm(interaction, turnID, combatantID, 0, 30)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "ground level") {
		t.Errorf("expected ground level in message, got: %s", content)
	}
}

func TestParseFlyConfirmData_MalformedTurnUUID(t *testing.T) {
	customID := "fly_confirm:zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz:00000000-0000-0000-0000-000000000001:30:30"
	_, _, _, _, err := ParseFlyConfirmData(customID)
	if err == nil {
		t.Fatal("expected error for malformed turn UUID")
	}
	if !strings.Contains(err.Error(), "invalid turn ID") {
		t.Errorf("expected 'invalid turn ID' error, got: %v", err)
	}
}

func TestParseFlyConfirmData_MalformedCombatantUUID(t *testing.T) {
	turnID := uuid.New()
	customID := "fly_confirm:" + turnID.String() + ":zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz:30:30"
	_, _, _, _, err := ParseFlyConfirmData(customID)
	if err == nil {
		t.Fatal("expected error for malformed combatant UUID")
	}
	if !strings.Contains(err.Error(), "invalid combatant ID") {
		t.Errorf("expected 'invalid combatant ID' error, got: %v", err)
	}
}

func TestFlyHandler_Descend(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupFlyHandler(sess)

	// Override combatant to be at altitude 30
	origSvc := handler.combatService.(*mockMoveService)
	handler.combatService = &mockMoveService{
		getEncounter: origSvc.getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "A",
				PositionRow: 1,
				AltitudeFt:  30,
				Conditions:  json.RawMessage(`[{"condition":"fly_speed"}]`),
				IsAlive: true, HpCurrent: 10,
			}, nil
		},
		listCombatants:     origSvc.listCombatants,
		updateCombatantPos: origSvc.updateCombatantPos,
	}
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{}, nil
		},
	}

	interaction := makeFlyInteraction(0)
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "ground level") {
		t.Errorf("expected ground level message, got: %s", content)
	}
}
