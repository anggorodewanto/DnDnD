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

// --- Mocks ---

type mockMoveService struct {
	getEncounter              func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
	getCombatant              func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error)
	listCombatants            func(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
	updateCombatantPos        func(ctx context.Context, id uuid.UUID, col string, row, alt int32) (refdata.Combatant, error)
	updateConditions func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error)
}

func (m *mockMoveService) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	return m.getEncounter(ctx, id)
}
func (m *mockMoveService) GetCombatant(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
	return m.getCombatant(ctx, id)
}
func (m *mockMoveService) ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error) {
	return m.listCombatants(ctx, encounterID)
}
func (m *mockMoveService) UpdateCombatantPosition(ctx context.Context, id uuid.UUID, col string, row, alt int32) (refdata.Combatant, error) {
	return m.updateCombatantPos(ctx, id, col, row, alt)
}
func (m *mockMoveService) UpdateCombatantConditions(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
	if m.updateConditions != nil {
		return m.updateConditions(ctx, arg)
	}
	return refdata.Combatant{}, nil
}

type mockMoveMapProvider struct {
	getByID func(ctx context.Context, id uuid.UUID) (refdata.Map, error)
}

func (m *mockMoveMapProvider) GetByID(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	return m.getByID(ctx, id)
}

type mockMoveTurnProvider struct {
	getTurn           func(ctx context.Context, id uuid.UUID) (refdata.Turn, error)
	updateTurnActions func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error)
}

func (m *mockMoveTurnProvider) GetTurn(ctx context.Context, id uuid.UUID) (refdata.Turn, error) {
	return m.getTurn(ctx, id)
}
func (m *mockMoveTurnProvider) UpdateTurnActions(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
	return m.updateTurnActions(ctx, arg)
}

type mockMoveEncounterProvider struct {
	// Phase 105: routed via the invoker's combatant entry. The mock retains a
	// guild-only func for legacy tests and a new user-aware func for
	// disambiguation tests; when the user-aware func is set it takes precedence.
	getActiveEncounterID    func(ctx context.Context, guildID string) (uuid.UUID, error)
	activeEncounterForUser  func(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

func (m *mockMoveEncounterProvider) ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error) {
	if m.activeEncounterForUser != nil {
		return m.activeEncounterForUser(ctx, guildID, discordUserID)
	}
	return m.getActiveEncounterID(ctx, guildID)
}

type mockMoveSession struct {
	lastResponse *discordgo.InteractionResponse
}

func (m *mockMoveSession) InteractionRespond(i *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	m.lastResponse = resp
	return nil
}
func (m *mockMoveSession) InteractionResponseEdit(*discordgo.Interaction, *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockMoveSession) UserChannelCreate(string) (*discordgo.Channel, error) { return nil, nil }
func (m *mockMoveSession) ChannelMessageSend(string, string) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockMoveSession) ChannelMessageSendComplex(string, *discordgo.MessageSend) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockMoveSession) ApplicationCommandBulkOverwrite(string, string, []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (m *mockMoveSession) ApplicationCommands(string, string) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}
func (m *mockMoveSession) ApplicationCommandDelete(string, string, string) error { return nil }
func (m *mockMoveSession) GuildChannels(string) ([]*discordgo.Channel, error)    { return nil, nil }
func (m *mockMoveSession) GuildChannelCreateComplex(string, discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	return nil, nil
}
func (m *mockMoveSession) ChannelMessageEdit(string, string, string) (*discordgo.Message, error) {
	return nil, nil
}
func (m *mockMoveSession) GetState() *discordgo.State { return nil }

// --- Helpers ---

func tiledJSON5x5() json.RawMessage {
	return json.RawMessage(`{
		"width": 5, "height": 5, "tilewidth": 48, "tileheight": 48,
		"layers": [
			{"name": "terrain", "type": "tilelayer", "width": 5, "height": 5,
			 "data": [1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1, 1,1,1,1,1]}
		],
		"tilesets": [{"firstgid": 1, "name": "base", "tiles": [{"id": 0, "type": "open_ground"}]}]
	}`)
}

func makeMoveInteraction(coord string) *discordgo.Interaction {
	return &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member: &discordgo.Member{
			User: &discordgo.User{ID: "user1"},
		},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "move",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "coordinate", Value: coord, Type: discordgo.ApplicationCommandOptionString},
			},
		},
	}
}

func setupMoveHandler(sess *mockMoveSession) (*MoveHandler, uuid.UUID, uuid.UUID, uuid.UUID) {
	encounterID := uuid.New()
	turnID := uuid.New()
	combatantID := uuid.New()
	mapID := uuid.New()

	combatSvc := &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{UUID: turnID, Valid: true},
				MapID:         uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "A",
				PositionRow: 1,
				IsAlive:     true,
				IsNpc:       false,
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

	mapProv := &mockMoveMapProvider{
		getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
			return refdata.Map{
				ID:            mapID,
				WidthSquares:  5,
				HeightSquares: 5,
				TiledJson:     tiledJSON5x5(),
			}, nil
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
			return refdata.Turn{}, nil
		},
	}

	encProv := &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encounterID, nil
		},
	}

	handler := NewMoveHandler(sess, combatSvc, mapProv, turnProv, encProv, nil)
	return handler, encounterID, turnID, combatantID
}

// --- Tests ---

func TestMoveHandler_ValidMove_ShowsConfirmation(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Move to D1") {
		t.Errorf("expected confirmation message, got: %s", content)
	}
	if !strings.Contains(content, "15ft") {
		t.Errorf("expected cost in message, got: %s", content)
	}
	// Should have buttons
	if len(sess.lastResponse.Data.Components) == 0 {
		t.Error("expected buttons in response")
	}
}

func TestMoveHandler_InvalidCoordinate(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	interaction := makeMoveInteraction("ZZZ")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	// ZZZ has no row number - should fail
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Invalid coordinate") {
		t.Errorf("expected invalid coordinate message, got: %s", content)
	}
}

func TestMoveHandler_NoActiveEncounter(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return uuid.Nil, errors.New("not found")
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active encounter") {
		t.Errorf("expected no active encounter message, got: %s", content)
	}
}

func TestMoveHandler_NotEnoughMovement(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	// Override turn to have only 5ft remaining
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 5,
			}, nil
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Not enough movement") {
		t.Errorf("expected not enough movement message, got: %s", content)
	}
}

func TestMoveHandler_SamePosition(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	interaction := makeMoveInteraction("A1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "already at") {
		t.Errorf("expected already at message, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirm(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	var updatedCol string
	var updatedRow int32
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, col string, row, _ int32) (refdata.Combatant, error) {
			updatedCol = col
			updatedRow = row
			return refdata.Combatant{}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15)

	if updatedCol != "D" {
		t.Errorf("expected position col D, got %s", updatedCol)
	}
	if updatedRow != 1 {
		t.Errorf("expected position row 1, got %d", updatedRow)
	}

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Moved to D1") {
		t.Errorf("expected moved confirmation, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveCancel(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleMoveCancel(interaction)

	content := sess.lastResponse.Data.Content
	if content != "Move cancelled." {
		t.Errorf("expected cancel message, got: %s", content)
	}
}

func TestParseMoveConfirmData(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	customID := "move_confirm:" + turnID.String() + ":" + combatantID.String() + ":3:0:15"

	gotTurnID, gotCombatantID, gotCol, gotRow, gotCost, err := ParseMoveConfirmData(customID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTurnID != turnID {
		t.Errorf("turn ID mismatch")
	}
	if gotCombatantID != combatantID {
		t.Errorf("combatant ID mismatch")
	}
	if gotCol != 3 || gotRow != 0 || gotCost != 15 {
		t.Errorf("expected (3,0,15), got (%d,%d,%d)", gotCol, gotRow, gotCost)
	}
}

func TestParseMoveConfirmData_Invalid(t *testing.T) {
	_, _, _, _, _, err := ParseMoveConfirmData("invalid")
	if err == nil {
		t.Error("expected error for invalid custom ID")
	}
}

func TestParseMoveConfirmData_MalformedTurnUUID(t *testing.T) {
	// Valid format (passes Sscanf %36s) but not a valid UUID
	customID := "move_confirm:zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz:00000000-0000-0000-0000-000000000001:3:0:15"
	_, _, _, _, _, err := ParseMoveConfirmData(customID)
	if err == nil {
		t.Fatal("expected error for malformed turn UUID")
	}
	if !strings.Contains(err.Error(), "invalid turn ID") {
		t.Errorf("expected 'invalid turn ID' error, got: %v", err)
	}
}

func TestParseMoveConfirmData_MalformedCombatantUUID(t *testing.T) {
	turnID := uuid.New()
	// Valid format (passes Sscanf %36s) but not a valid UUID
	customID := "move_confirm:" + turnID.String() + ":zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz:3:0:15"
	_, _, _, _, _, err := ParseMoveConfirmData(customID)
	if err == nil {
		t.Fatal("expected error for malformed combatant UUID")
	}
	if !strings.Contains(err.Error(), "invalid combatant ID") {
		t.Errorf("expected 'invalid combatant ID' error, got: %v", err)
	}
}

func TestMoveHandler_NoMap(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _ := setupMoveHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
				MapID:         uuid.NullUUID{Valid: false},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{PositionCol: "A", PositionRow: 1}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "no map") {
		t.Errorf("expected no map message, got: %s", content)
	}
}

func TestMoveHandler_NoActiveTurn(t *testing.T) {
	sess := &mockMoveSession{}
	handler, encounterID, _, _ := setupMoveHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:            encounterID,
				Status:        "active",
				CurrentTurnID: uuid.NullUUID{Valid: false}, // no active turn
				MapID:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
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

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "No active turn") {
		t.Errorf("expected no active turn message, got: %s", content)
	}
}

func TestMoveHandler_GetEncounterError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

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

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get encounter") {
		t.Errorf("expected encounter error message, got: %s", content)
	}
}

func TestMoveHandler_GetTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{}, errors.New("turn error")
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get turn") {
		t.Errorf("expected turn error message, got: %s", content)
	}
}

func TestMoveHandler_GetCombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
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

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error message, got: %s", content)
	}
}

func TestMoveHandler_MapLoadError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	handler.mapProvider = &mockMoveMapProvider{
		getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, errors.New("map error")
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to load map") {
		t.Errorf("expected map load error message, got: %s", content)
	}
}

func TestMoveHandler_ListCombatantsError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	origSvc := handler.combatService.(*mockMoveService)
	handler.combatService = &mockMoveService{
		getEncounter: origSvc.getEncounter,
		getCombatant: origSvc.getCombatant,
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, errors.New("list error")
		},
		updateCombatantPos: origSvc.updateCombatantPos,
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to list combatants") {
		t.Errorf("expected list combatants error message, got: %s", content)
	}
}

func TestMoveHandler_NoOptions(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    "move",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{},
		},
	}
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "provide a coordinate") {
		t.Errorf("expected provide coordinate message, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirm_TurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
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

	handler.HandleMoveConfirm(interaction, uuid.New(), uuid.New(), 3, 0, 15)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn no longer active") {
		t.Errorf("expected turn no longer active, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirm_NotEnoughMovement(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 5, // only 5ft remaining
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15) // needs 15ft

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Cannot move") {
		t.Errorf("expected cannot move, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirm_UpdateTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)
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

	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to update turn") {
		t.Errorf("expected turn update error, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirm_PositionUpdateError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) { return refdata.Encounter{}, nil },
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
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

	handler.HandleMoveConfirm(interaction, turnID, combatantID, 3, 0, 15)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to update position") {
		t.Errorf("expected position error, got: %s", content)
	}
}

func TestBuildOccupants(t *testing.T) {
	moverID := uuid.New()
	allyID := uuid.New()
	enemyID := uuid.New()
	deadID := uuid.New()

	mover := refdata.Combatant{ID: moverID, PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: false}
	all := []refdata.Combatant{
		mover,
		{ID: allyID, PositionCol: "B", PositionRow: 1, IsAlive: true, IsNpc: false},
		{ID: enemyID, PositionCol: "C", PositionRow: 1, IsAlive: true, IsNpc: true},
		{ID: deadID, PositionCol: "D", PositionRow: 1, IsAlive: false, IsNpc: true},
	}

	occupants := buildOccupants(all, mover)

	if len(occupants) != 2 {
		t.Fatalf("expected 2 occupants (excluding mover and dead), got %d", len(occupants))
	}

	// Ally (PC) should be IsAlly=true
	found := false
	for _, o := range occupants {
		if o.Col == 1 && o.Row == 0 {
			found = true
			if !o.IsAlly {
				t.Error("expected ally PC to be IsAlly=true")
			}
		}
	}
	if !found {
		t.Error("ally occupant not found")
	}
}

func TestBuildOccupants_InvalidPosition(t *testing.T) {
	mover := refdata.Combatant{ID: uuid.New(), PositionCol: "A", PositionRow: 1, IsAlive: true}
	all := []refdata.Combatant{
		mover,
		{ID: uuid.New(), PositionCol: "", PositionRow: 0, IsAlive: true}, // invalid position
	}
	occupants := buildOccupants(all, mover)
	if len(occupants) != 0 {
		t.Errorf("expected 0 occupants for invalid position, got %d", len(occupants))
	}
}

func TestMoveHandler_BadMapJSON(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	handler.mapProvider = &mockMoveMapProvider{
		getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
			return refdata.Map{
				TiledJson: json.RawMessage(`invalid json`),
			}, nil
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to parse map") {
		t.Errorf("expected parse error message, got: %s", content)
	}
}

func TestMoveHandler_SplitMovement(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupMoveHandler(sess)

	// Track position and movement updates
	currentCol := "A"
	var currentRow int32 = 1
	movementRemaining := int32(30)

	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: currentCol,
				PositionRow: currentRow,
				IsAlive:     true,
				IsNpc:       false,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, PositionCol: currentCol, PositionRow: currentRow, IsAlive: true, IsNpc: false},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, col string, row, _ int32) (refdata.Combatant, error) {
			currentCol = col
			currentRow = row
			return refdata.Combatant{}, nil
		},
	}

	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: movementRemaining,
			}, nil
		},
		updateTurnActions: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			movementRemaining = arg.MovementRemainingFt
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: movementRemaining,
			}, nil
		},
	}

	// First move: A1 -> C1 (2 tiles = 10ft), should show 20ft remaining
	interaction1 := makeMoveInteraction("C1")
	handler.Handle(interaction1)

	if sess.lastResponse == nil {
		t.Fatal("expected confirmation response for first move")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "10ft") {
		t.Errorf("expected 10ft cost for first move, got: %s", sess.lastResponse.Data.Content)
	}

	// Simulate confirming the first move
	confirmInteraction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleMoveConfirm(confirmInteraction, turnID, combatantID, 2, 0, 10)

	if sess.lastResponse == nil {
		t.Fatal("expected response after first move confirm")
	}
	if !strings.Contains(sess.lastResponse.Data.Content, "Moved to C1") {
		t.Errorf("expected moved to C1, got: %s", sess.lastResponse.Data.Content)
	}

	// Verify movement was deducted: should be 20ft remaining
	if movementRemaining != 20 {
		t.Fatalf("expected 20ft remaining after first move, got %d", movementRemaining)
	}

	// Second move: C1 -> E1 (2 tiles = 10ft), should use updated remaining (20ft)
	interaction2 := makeMoveInteraction("E1")
	handler.Handle(interaction2)

	if sess.lastResponse == nil {
		t.Fatal("expected confirmation response for second move")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "10ft remaining") {
		t.Errorf("expected 10ft remaining after second move confirmation, got: %s", content)
	}

	// Confirm the second move
	handler.HandleMoveConfirm(confirmInteraction, turnID, combatantID, 4, 0, 10)

	if movementRemaining != 10 {
		t.Errorf("expected 10ft remaining after second move, got %d", movementRemaining)
	}
}

// --- Phase 41: Prone movement handler tests ---

func setupProneMoveHandler(sess *mockMoveSession) (*MoveHandler, uuid.UUID, uuid.UUID, uuid.UUID) {
	handler, encounterID, turnID, combatantID := setupMoveHandler(sess)

	// Override combatant to be prone
	proneConditions, _ := json.Marshal([]map[string]interface{}{
		{"condition": "prone"},
	})

	handler.combatService = &mockMoveService{
		getEncounter: handler.combatService.(*mockMoveService).getEncounter,
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				PositionCol: "A",
				PositionRow: 1,
				IsAlive:     true,
				IsNpc:       false,
				Conditions:  proneConditions,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combatantID, PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: false, Conditions: proneConditions},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	return handler, encounterID, turnID, combatantID
}

func TestMoveHandler_ProneShowsChoicePrompt(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupProneMoveHandler(sess)

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "prone") {
		t.Errorf("expected prone prompt, got: %s", content)
	}
	// Should have Stand & Move and Crawl buttons
	if len(sess.lastResponse.Data.Components) == 0 {
		t.Error("expected buttons in response")
	}
}

func TestMoveHandler_ProneSkipsPromptIfAlreadyStood(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	// Override turn to have HasStoodThisTurn=true
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
				HasStoodThisTurn:    true,
			}, nil
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{}, nil
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	// Should show normal move confirmation, not prone prompt
	if strings.Contains(content, "prone") {
		t.Errorf("should skip prone prompt when already stood, got: %s", content)
	}
	if !strings.Contains(content, "Move to D1") {
		t.Errorf("expected normal move confirmation, got: %s", content)
	}
}

func TestMoveHandler_HandleProneStandAndMove(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleProneStandAndMove(interaction, turnID, combatantID, 3, 0, 30)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Stand & move") {
		t.Errorf("expected stand & move confirmation, got: %s", content)
	}
}

func TestMoveHandler_HandleProneCrawl(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleProneCrawl(interaction, turnID, combatantID, 3, 0)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Crawl") {
		t.Errorf("expected crawl confirmation, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirm_StandAndMove_RemovesProne(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	var updatedParams refdata.UpdateTurnActionsParams
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			updatedParams = arg
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: arg.MovementRemainingFt,
				HasStoodThisTurn:    arg.HasStoodThisTurn,
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	// Use stand_and_move mode — costFt is total (stand + path), standCost encoded in custom ID
	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 10, "stand_and_move", 15)

	if !updatedParams.HasStoodThisTurn {
		t.Error("expected HasStoodThisTurn to be true after stand_and_move")
	}
	// Total deduction: stand cost (15) + path cost (10) = 25
	expectedRemaining := int32(30) - 15 - 10
	if updatedParams.MovementRemainingFt != expectedRemaining {
		t.Errorf("expected %d remaining, got %d", expectedRemaining, updatedParams.MovementRemainingFt)
	}
}

func TestParseProneMoveData(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	customID := "prone_stand:" + turnID.String() + ":" + combatantID.String() + ":3:0:30"

	gotTurnID, gotCombatantID, gotCol, gotRow, gotMaxSpeed, err := ParseProneMoveData(customID, "prone_stand")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTurnID != turnID {
		t.Errorf("turn ID mismatch")
	}
	if gotCombatantID != combatantID {
		t.Errorf("combatant ID mismatch")
	}
	if gotCol != 3 || gotRow != 0 || gotMaxSpeed != 30 {
		t.Errorf("expected (3,0,30), got (%d,%d,%d)", gotCol, gotRow, gotMaxSpeed)
	}
}

func TestParseMoveConfirmWithModeData(t *testing.T) {
	turnID := uuid.New()
	combatantID := uuid.New()
	customID := "move_confirm:" + turnID.String() + ":" + combatantID.String() + ":3:0:10:stand_and_move:15"

	gotTurnID, gotCombatantID, gotCol, gotRow, gotCost, gotMode, gotStandCost, err := ParseMoveConfirmWithModeData(customID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTurnID != turnID || gotCombatantID != combatantID {
		t.Error("ID mismatch")
	}
	if gotCol != 3 || gotRow != 0 || gotCost != 10 {
		t.Errorf("expected (3,0,10), got (%d,%d,%d)", gotCol, gotRow, gotCost)
	}
	if gotMode != "stand_and_move" {
		t.Errorf("expected mode stand_and_move, got %q", gotMode)
	}
	if gotStandCost != 15 {
		t.Errorf("expected stand cost 15, got %d", gotStandCost)
	}
}

func TestMoveHandler_HandleProneStandAndMove_TurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupProneMoveHandler(sess)
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
	handler.HandleProneStandAndMove(interaction, uuid.New(), uuid.New(), 3, 0, 30)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn no longer active") {
		t.Errorf("expected turn error, got: %s", content)
	}
}

func TestMoveHandler_HandleProneStandAndMove_CombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, nil
		},
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

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleProneStandAndMove(interaction, turnID, combatantID, 3, 0, 30)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error, got: %s", content)
	}
}

func TestMoveHandler_HandleProneCrawl_TurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupProneMoveHandler(sess)
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
	handler.HandleProneCrawl(interaction, uuid.New(), uuid.New(), 3, 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn no longer active") {
		t.Errorf("expected turn error, got: %s", content)
	}
}

func TestMoveHandler_HandleProneCrawl_CombatantError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, nil
		},
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

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleProneCrawl(interaction, turnID, combatantID, 3, 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get combatant") {
		t.Errorf("expected combatant error, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_TurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupProneMoveHandler(sess)
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
	handler.HandleMoveConfirmWithMode(interaction, uuid.New(), uuid.New(), 3, 0, 10, "crawl", 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Turn no longer active") {
		t.Errorf("expected turn error, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_StandAndMove_RemovesProneCondition(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	var conditionsUpdated bool
	var updatedConditions json.RawMessage
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			proneConditions, _ := json.Marshal([]map[string]interface{}{
				{"condition": "prone"},
			})
			return refdata.Combatant{
				ID:          combatantID,
				Conditions:  proneConditions,
				DisplayName: "TestChar",
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
		updateConditions: func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			conditionsUpdated = true
			updatedConditions = arg.Conditions
			return refdata.Combatant{}, nil
		},
	}

	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: arg.MovementRemainingFt,
				HasStoodThisTurn:    arg.HasStoodThisTurn,
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}

	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 10, "stand_and_move", 15)

	if !conditionsUpdated {
		t.Fatal("expected conditions to be updated (prone removed)")
	}
	// Verify prone was removed from conditions
	if strings.Contains(string(updatedConditions), "prone") {
		t.Errorf("expected prone condition to be removed, but conditions still contain prone: %s", string(updatedConditions))
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_CrawlEmoji(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: arg.MovementRemainingFt,
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 20, "crawl", 0)
	content := sess.lastResponse.Data.Content
	// Should use bug emoji \U0001f41b, not whale \U0001f40b
	if strings.Contains(content, "\U0001f40b") {
		t.Errorf("crawl message uses whale emoji instead of bug emoji: %s", content)
	}
	if !strings.Contains(content, "\U0001f41b") {
		t.Errorf("expected bug emoji in crawl message, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_CrawlMode(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: arg.MovementRemainingFt,
			}, nil
		},
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 20, "crawl", 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Crawled to") {
		t.Errorf("expected crawl confirmation, got: %s", content)
	}
}

func TestParseProneMoveData_Invalid(t *testing.T) {
	_, _, _, _, _, err := ParseProneMoveData("invalid", "prone_stand")
	if err == nil {
		t.Error("expected error for invalid prone data")
	}
}

func TestParseMoveConfirmWithModeData_Invalid(t *testing.T) {
	_, _, _, _, _, _, _, err := ParseMoveConfirmWithModeData("invalid")
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestMoveHandler_HandleProneStandAndMove_MapError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	// Encounter has no valid map
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{MapID: uuid.NullUUID{Valid: false}}, nil
		},
		getCombatant: handler.combatService.(*mockMoveService).getCombatant,
		listCombatants: handler.combatService.(*mockMoveService).listCombatants,
		updateCombatantPos: handler.combatService.(*mockMoveService).updateCombatantPos,
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleProneStandAndMove(interaction, turnID, combatantID, 3, 0, 30)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get map") {
		t.Errorf("expected map error, got: %s", content)
	}
}

func TestMoveHandler_HandleProneCrawl_MapError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{MapID: uuid.NullUUID{Valid: false}}, nil
		},
		getCombatant: handler.combatService.(*mockMoveService).getCombatant,
		listCombatants: handler.combatService.(*mockMoveService).listCombatants,
		updateCombatantPos: handler.combatService.(*mockMoveService).updateCombatantPos,
	}

	interaction := &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "guild1",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "user1"}},
	}
	handler.HandleProneCrawl(interaction, turnID, combatantID, 3, 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to get map") {
		t.Errorf("expected map error, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_NotEnoughMovement(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)
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
	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 10, "stand_and_move", 15)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Cannot move") {
		t.Errorf("expected cannot move error, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_UpdateTurnError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)
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
	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 10, "crawl", 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to update turn") {
		t.Errorf("expected update error, got: %s", content)
	}
}

func TestMoveHandler_HandleMoveConfirmWithMode_PositionError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, turnID, combatantID := setupProneMoveHandler(sess)
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			return refdata.Turn{
				ID:                  turnID,
				CombatantID:         combatantID,
				MovementRemainingFt: 30,
			}, nil
		},
		updateTurnActions: func(_ context.Context, _ refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
			return refdata.Turn{MovementRemainingFt: 10}, nil
		},
	}
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) { return refdata.Encounter{}, nil },
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) { return refdata.Combatant{}, nil },
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
	handler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, 3, 0, 10, "crawl", 0)
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Failed to update position") {
		t.Errorf("expected position error, got: %s", content)
	}
}

func TestMoveHandler_OutOfBounds(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	// Z99 is way out of bounds for a 5x5 grid
	interaction := makeMoveInteraction("Z99")
	handler.Handle(interaction)

	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "out of bounds") {
		t.Errorf("expected out of bounds message, got: %s", content)
	}
}
