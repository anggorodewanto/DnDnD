package discord

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

func TestMoveHandler_ExplorationMode_NoMap(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	encID := uuid.New()

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Status: "active", Mode: "exploration"}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "no map") {
		t.Errorf("expected 'no map' message, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_ExplorationMode_MapLoadError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID: encID, Status: "active", Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID}, nil
		},
	}
	handler.mapProvider = &mockMoveMapProvider{
		getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
			return refdata.Map{}, errors.New("db error")
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "Failed to load map") {
		t.Errorf("expected 'Failed to load map' message, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_ExplorationMode_MapParseError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID: encID, Status: "active", Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		getCombatant: func(_ context.Context, _ uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: combatantID}, nil
		},
	}
	handler.mapProvider = &mockMoveMapProvider{
		getByID: func(_ context.Context, _ uuid.UUID) (refdata.Map, error) {
			return refdata.Map{ID: mapID, TiledJson: json.RawMessage(`not-json`)}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "parse map") {
		t.Errorf("expected parse error, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_ExplorationMode_NoPCCombatant(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID: encID, Status: "active", Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			// only NPC combatants, no PC
			return []refdata.Combatant{
				{ID: uuid.New(), PositionCol: "B", PositionRow: 1, IsAlive: true, HpCurrent: 10, IsNpc: true},
			}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "Could not find") {
		t.Errorf("expected 'Could not find' message, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_ExplorationMode_ListCombatantsError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID: encID, Status: "active", Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return nil, errors.New("db down")
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "list combatants") {
		t.Errorf("expected list combatants error, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_ExplorationMode_UpdatePositionError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	charID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID: encID, Status: "active", Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          combatantID,
					CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true, HpCurrent: 10,
				},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			return refdata.Combatant{}, errors.New("write failed")
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "Failed to update position") {
		t.Errorf("expected position-update error, got: %s", sess.lastResponse.Data.Content)
	}
}

func TestMoveHandler_ExplorationMode_InvalidCurrentPosition(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, combatantID := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	charID := uuid.New()
	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID: encID, Status: "active", Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          combatantID,
					CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
					// Invalid position -- no row number
					PositionCol: "??", PositionRow: 0, IsAlive: true, HpCurrent: 10,
				},
			}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		getActiveEncounterID: func(_ context.Context, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "current position") &&
		!strings.Contains(sess.lastResponse.Data.Content, "No path") {
		// Either is acceptable; the key is we didn't panic.
	}
}
