package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// TestMoveHandler_ExplorationMode_SkipsTurnEconomy verifies that when /move is
// invoked in an exploration-mode encounter (Phase 110), the handler:
//   - does NOT require current_turn_id
//   - does NOT call the turn provider
//   - still runs pathfinding + wall validation
//   - directly updates the combatant's position.
func TestMoveHandler_ExplorationMode_SkipsTurnEconomy(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)

	encID := uuid.New()
	mapID := uuid.New()
	charID := uuid.New()

	// Override the combat service: encounter is exploration-mode with NO current_turn_id.
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:     encID,
				Status: "active",
				Mode:   "exploration",
				MapID:  uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
				PositionCol: "A",
				PositionRow: 1,
				IsAlive:     true,
				IsNpc:       false,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          combatantID,
					CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
					PositionCol: "A",
					PositionRow: 1,
					IsAlive:     true,
					IsNpc:       false,
				},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, nil
		},
	}

	// Fail the test if turnProvider.GetTurn is called -- exploration must skip it.
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			t.Fatalf("exploration /move must not call turnProvider.GetTurn")
			return refdata.Turn{}, nil
		},
	}

	// Route the user's active encounter to ours.
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encID, nil
		},
	}

	interaction := makeMoveInteraction("D1")
	handler.Handle(interaction)

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	content := sess.lastResponse.Data.Content
	if !strings.Contains(content, "Moved to D1") {
		t.Errorf("expected 'Moved to D1' confirmation, got: %s", content)
	}
	// No turn resources to report in exploration.
	if strings.Contains(content, "movement remaining") {
		t.Errorf("exploration must not show turn resources; got: %s", content)
	}
}

func TestMoveHandler_ExplorationMode_RejectsBlockedPath(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)

	encID := uuid.New()
	mapID := uuid.New()

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:     encID,
				Status: "active",
				Mode:   "exploration",
				MapID:  uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          combatantID,
				CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
				PositionCol: "A", PositionRow: 1, IsAlive: true,
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			// Put an NPC blocking every D-column destination.
			return []refdata.Combatant{
				{
					ID:          combatantID,
					CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true,
				},
				// Wall of NPCs at D1..D5 to guarantee no path to D1 (D1 is occupied).
				{ID: uuid.New(), PositionCol: "D", PositionRow: 1, IsAlive: true, IsNpc: true},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			t.Fatalf("position update must not happen when destination is blocked")
			return refdata.Combatant{}, nil
		},
	}
	handler.turnProvider = &mockMoveTurnProvider{
		getTurn: func(_ context.Context, _ uuid.UUID) (refdata.Turn, error) {
			t.Fatalf("exploration /move must not call turnProvider.GetTurn")
			return refdata.Turn{}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) {
			return encID, nil
		},
	}

	handler.Handle(makeMoveInteraction("D1"))

	if sess.lastResponse == nil {
		t.Fatal("expected response")
	}
	// pathfinding should report the destination is occupied.
	content := sess.lastResponse.Data.Content
	if !strings.Contains(strings.ToLower(content), "destination") && !strings.Contains(strings.ToLower(content), "occupied") && !strings.Contains(strings.ToLower(content), "cannot") && !strings.Contains(strings.ToLower(content), "path") {
		t.Errorf("expected blocked-path message, got: %s", content)
	}
}
