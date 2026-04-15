package discord

import (
	"context"
	"errors"
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

// TestMoveHandler_ExplorationMode_MultiPC_ResolvesInvokerCorrectly verifies that
// when multiple PCs share an exploration encounter, /move moves the PC owned
// by the invoking Discord user -- not the first PC in the list (Phase 110 it2).
func TestMoveHandler_ExplorationMode_MultiPC_ResolvesInvokerCorrectly(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)

	encID := uuid.New()
	mapID := uuid.New()
	campID := uuid.New()

	// PC A belongs to user-a (first in list); PC B belongs to user-b (invoker).
	// If the bug is present, the first PC (A) gets moved instead of B.
	charAID := uuid.New()
	charBID := uuid.New()
	combAID := uuid.New()
	combBID := uuid.New()

	moved := struct {
		id  uuid.UUID
		col string
		row int32
	}{}

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{
				ID:     encID,
				Status: "active",
				Mode:   "exploration",
				MapID:  uuid.NullUUID{UUID: mapID, Valid: true},
			}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{
					ID:          combAID,
					CharacterID: uuid.NullUUID{UUID: charAID, Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true, IsNpc: false,
				},
				{
					ID:          combBID,
					CharacterID: uuid.NullUUID{UUID: charBID, Valid: true},
					PositionCol: "B", PositionRow: 1, IsAlive: true, IsNpc: false,
				},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, id uuid.UUID, col string, row, _ int32) (refdata.Combatant, error) {
			moved.id = id
			moved.col = col
			moved.row = row
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
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) {
			return encID, nil
		},
	}

	// New: wire campaign + character lookup so handler can resolve user-b -> charBID.
	handler.campaignProv = &mockMoveCampaignProvider{
		getCampaign: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: campID}, nil
		},
	}
	handler.characterLookup = &mockMoveCharacterLookup{
		getChar: func(_ context.Context, _ uuid.UUID, discordUserID string) (refdata.Character, error) {
			if discordUserID == "user-b" {
				return refdata.Character{ID: charBID}, nil
			}
			return refdata.Character{ID: charAID}, nil
		},
	}

	interaction := makeMoveInteraction("D1")
	interaction.Member.User.ID = "user-b"

	handler.Handle(interaction)

	if moved.id != combBID {
		t.Fatalf("expected combatant B (%s) to move, got %s", combBID, moved.id)
	}
	if moved.col != "D" || moved.row != 1 {
		t.Errorf("expected move to D1, got %s%d", moved.col, moved.row)
	}
}


// TestMoveHandler_ExplorationMode_MultiPC_FallsBackWhenLookupMissing verifies
// resolveExplorationMover gracefully falls back to the first PC when the
// campaign/character lookup is unwired (dev-mode, single-PC deployments).
func TestMoveHandler_ExplorationMode_MultiPC_FallsBackWhenLookupMissing(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	combAID := uuid.New()
	combBID := uuid.New()

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true}}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combAID, CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true},
				{ID: combBID, CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "B", PositionRow: 1, IsAlive: true},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, id uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			if id != combAID {
				t.Fatalf("expected fallback to first PC (%s), got %s", combAID, id)
			}
			return refdata.Combatant{}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encID, nil },
	}
	// campaignProv and characterLookup are nil -> fallback path.
	handler.campaignProv = nil
	handler.characterLookup = nil

	handler.Handle(makeMoveInteraction("D1"))
}

// TestMoveHandler_ExplorationMode_MultiPC_FallsBackOnCampaignError covers the
// branch where GetCampaignByGuildID errors.
func TestMoveHandler_ExplorationMode_MultiPC_FallsBackOnCampaignError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	combAID := uuid.New()

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true}}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combAID, CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true},
				{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "B", PositionRow: 1, IsAlive: true},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, id uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			if id != combAID {
				t.Fatalf("expected fallback to first PC, got %s", id)
			}
			return refdata.Combatant{}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.campaignProv = &mockMoveCampaignProvider{
		getCampaign: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("campaign lookup failed")
		},
	}
	handler.characterLookup = &mockMoveCharacterLookup{
		getChar: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			t.Fatalf("character lookup should not be called when campaign lookup fails")
			return refdata.Character{}, nil
		},
	}
	handler.Handle(makeMoveInteraction("D1"))
}

// TestMoveHandler_ExplorationMode_MultiPC_FallsBackOnCharacterError covers the
// branch where GetCharacterByCampaignAndDiscord errors.
func TestMoveHandler_ExplorationMode_MultiPC_FallsBackOnCharacterError(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()
	combAID := uuid.New()

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true}}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: combAID, CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true},
				{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "B", PositionRow: 1, IsAlive: true},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, id uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			if id != combAID {
				t.Fatalf("expected fallback to first PC, got %s", id)
			}
			return refdata.Combatant{}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.campaignProv = &mockMoveCampaignProvider{
		getCampaign: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: uuid.New()}, nil
		},
	}
	handler.characterLookup = &mockMoveCharacterLookup{
		getChar: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			return refdata.Character{}, errors.New("not found")
		},
	}
	handler.Handle(makeMoveInteraction("D1"))
}

// TestMoveHandler_ExplorationMode_MultiPC_NoMatchingCharacter covers the
// branch where the invoker's character has no combatant in the encounter.
func TestMoveHandler_ExplorationMode_MultiPC_NoMatchingCharacter(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	encID := uuid.New()
	mapID := uuid.New()

	handler.combatService = &mockMoveService{
		getEncounter: func(_ context.Context, _ uuid.UUID) (refdata.Encounter, error) {
			return refdata.Encounter{ID: encID, Mode: "exploration",
				MapID: uuid.NullUUID{UUID: mapID, Valid: true}}, nil
		},
		listCombatants: func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "A", PositionRow: 1, IsAlive: true},
				{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true},
					PositionCol: "B", PositionRow: 1, IsAlive: true},
			}, nil
		},
		updateCombatantPos: func(_ context.Context, _ uuid.UUID, _ string, _, _ int32) (refdata.Combatant, error) {
			t.Fatalf("no update should happen when no combatant matches invoker")
			return refdata.Combatant{}, nil
		},
	}
	handler.encounterProvider = &mockMoveEncounterProvider{
		activeEncounterForUser: func(_ context.Context, _, _ string) (uuid.UUID, error) { return encID, nil },
	}
	handler.campaignProv = &mockMoveCampaignProvider{
		getCampaign: func(_ context.Context, _ string) (refdata.Campaign, error) {
			return refdata.Campaign{ID: uuid.New()}, nil
		},
	}
	handler.characterLookup = &mockMoveCharacterLookup{
		getChar: func(_ context.Context, _ uuid.UUID, _ string) (refdata.Character, error) {
			// A charID that does not match any combatant.
			return refdata.Character{ID: uuid.New()}, nil
		},
	}
	handler.Handle(makeMoveInteraction("D1"))
	if !strings.Contains(sess.lastResponse.Data.Content, "Could not find") {
		t.Errorf("expected 'Could not find' message, got: %s", sess.lastResponse.Data.Content)
	}
}

// TestMoveHandler_SetCharacterLookup verifies the setter wires the dependency.
func TestMoveHandler_SetCharacterLookup(t *testing.T) {
	sess := &mockMoveSession{}
	handler, _, _, _ := setupMoveHandler(sess)
	if handler.characterLookup != nil {
		t.Fatalf("expected nil characterLookup initially")
	}
	lookup := &mockMoveCharacterLookup{}
	handler.SetCharacterLookup(lookup)
	if handler.characterLookup != lookup {
		t.Errorf("SetCharacterLookup did not wire the dependency")
	}
}
