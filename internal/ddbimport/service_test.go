package ddbimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
)

// mockClient implements Client for testing.
type mockClient struct {
	FetchFunc func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockClient) FetchCharacter(ctx context.Context, id string) ([]byte, error) {
	return m.FetchFunc(ctx, id)
}

// mockCharStore implements CharacterStore for testing.
type mockCharStore struct {
	CreateFunc           func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error)
	GetByDdbURLFunc      func(ctx context.Context, campaignID uuid.UUID, ddbURL string) (refdata.Character, error)
	UpdateFunc           func(ctx context.Context, id uuid.UUID, params refdata.CreateCharacterParams) (refdata.Character, error)
}

func (m *mockCharStore) CreateCharacterFull(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
	return m.CreateFunc(ctx, params)
}

func (m *mockCharStore) GetCharacterByDdbURL(ctx context.Context, campaignID uuid.UUID, ddbURL string) (refdata.Character, error) {
	if m.GetByDdbURLFunc != nil {
		return m.GetByDdbURLFunc(ctx, campaignID, ddbURL)
	}
	return refdata.Character{}, sql.ErrNoRows
}

func (m *mockCharStore) UpdateCharacterFull(ctx context.Context, id uuid.UUID, params refdata.CreateCharacterParams) (refdata.Character, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, params)
	}
	return refdata.Character{}, nil
}

func minimalDDBJSON() []byte {
	return []byte(`{
		"data": {
			"name": "Test Hero",
			"race": {"fullName": "Human"},
			"classes": [{"definition": {"name": "Fighter", "hitDice": 10}, "level": 3}],
			"stats": [
				{"id": 1, "value": 16},
				{"id": 2, "value": 14},
				{"id": 3, "value": 15},
				{"id": 4, "value": 10},
				{"id": 5, "value": 12},
				{"id": 6, "value": 8}
			],
			"bonusStats": [
				{"id": 1, "value": null},
				{"id": 2, "value": null},
				{"id": 3, "value": null},
				{"id": 4, "value": null},
				{"id": 5, "value": null},
				{"id": 6, "value": null}
			],
			"overrideStats": [
				{"id": 1, "value": null},
				{"id": 2, "value": null},
				{"id": 3, "value": null},
				{"id": 4, "value": null},
				{"id": 5, "value": null},
				{"id": 6, "value": null}
			],
			"baseHitPoints": 28,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 10, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)
}

func TestService_Import_Success(t *testing.T) {
	campaignID := uuid.New()
	charID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			if id != "12345" {
				t.Errorf("unexpected character ID: %s", id)
			}
			return minimalDDBJSON(), nil
		},
	}

	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			if params.Name != "Test Hero" {
				t.Errorf("expected name 'Test Hero', got %q", params.Name)
			}
			return refdata.Character{
				ID:         charID,
				CampaignID: campaignID,
				Name:       params.Name,
			}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Character.ID != charID {
		t.Errorf("character ID = %s, want %s", result.Character.ID, charID)
	}
	if result.Preview == "" {
		t.Error("preview should not be empty")
	}
	if result.IsResync {
		t.Error("first import should not be a resync")
	}
}

func TestService_Import_InvalidURL(t *testing.T) {
	svc := NewService(&mockClient{}, &mockCharStore{})
	_, err := svc.Import(context.Background(), uuid.New(), "not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestService_Import_FetchError(t *testing.T) {
	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return nil, fmt.Errorf("network error")
		},
	}
	svc := NewService(client, &mockCharStore{})
	_, err := svc.Import(context.Background(), uuid.New(), "https://www.dndbeyond.com/characters/12345")
	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
}

func TestService_Import_StructuralValidationError(t *testing.T) {
	// JSON with level 0 (invalid)
	badJSON := []byte(`{
		"data": {
			"name": "Bad",
			"race": {"fullName": "Elf"},
			"classes": [],
			"stats": [
				{"id": 1, "value": 10},
				{"id": 2, "value": 10},
				{"id": 3, "value": 10},
				{"id": 4, "value": 10},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"overrideStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"baseHitPoints": 10,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return badJSON, nil
		},
	}
	svc := NewService(client, &mockCharStore{})
	_, err := svc.Import(context.Background(), uuid.New(), "https://www.dndbeyond.com/characters/12345")
	if err == nil {
		t.Fatal("expected error for structural validation failure")
	}
}

func TestService_Import_WithWarnings(t *testing.T) {
	// JSON with STR 22
	highStatJSON := []byte(`{
		"data": {
			"name": "Strong",
			"race": {"fullName": "Human"},
			"classes": [{"definition": {"name": "Fighter", "hitDice": 10}, "level": 5}],
			"stats": [
				{"id": 1, "value": 22},
				{"id": 2, "value": 14},
				{"id": 3, "value": 15},
				{"id": 4, "value": 10},
				{"id": 5, "value": 12},
				{"id": 6, "value": 8}
			],
			"bonusStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"overrideStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"baseHitPoints": 44,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 10, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {"class": [], "race": [], "item": [], "feat": []}
		}
	}`)

	charID := uuid.New()
	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return highStatJSON, nil
		},
	}
	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			return refdata.Character{ID: charID, Name: params.Name}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), uuid.New(), "https://www.dndbeyond.com/characters/12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for STR 22")
	}
	if !strings.Contains(result.Preview, "STR 22") {
		t.Errorf("preview should contain warning about STR 22:\n%s", result.Preview)
	}
}

func TestService_Import_Resync(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return minimalDDBJSON(), nil
		},
	}

	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 2}})

	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, cID uuid.UUID, url string) (refdata.Character, error) {
			return refdata.Character{
				ID:            existingID,
				CampaignID:    campaignID,
				Name:          "Old Name",
				Race:          "Human",
				Level:         2,
				AbilityScores: scores,
				Classes:       classes,
				HpMax:         20,
				HpCurrent:     20,
				Ac:            10,
				SpeedFt:       30,
				DdbUrl:        sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
		UpdateFunc: func(ctx context.Context, id uuid.UUID, params refdata.CreateCharacterParams) (refdata.Character, error) {
			if id != existingID {
				t.Errorf("update called with wrong ID: %s", id)
			}
			return refdata.Character{ID: existingID, Name: params.Name}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsResync {
		t.Error("re-import should be flagged as resync")
	}
	if len(result.Changes) == 0 {
		t.Error("expected changes in re-sync diff")
	}
	if result.Character.ID != existingID {
		t.Errorf("should update existing character, got ID %s", result.Character.ID)
	}
}

func TestService_Import_ResyncNoChanges(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	fetchJSON := minimalDDBJSON()

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return fetchJSON, nil
		},
	}

	// Parse the JSON to get the exact values
	parsed, _ := ParseDDBJSON(fetchJSON)
	scores, _ := json.Marshal(parsed.AbilityScores)
	classes, _ := json.Marshal(parsed.Classes)

	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, cID uuid.UUID, url string) (refdata.Character, error) {
			return refdata.Character{
				ID:            existingID,
				CampaignID:    campaignID,
				Name:          parsed.Name,
				Race:          parsed.Race,
				Level:         int32(parsed.Level),
				AbilityScores: scores,
				Classes:       classes,
				HpMax:         int32(parsed.HPMax),
				HpCurrent:     int32(parsed.HPCurrent),
				Ac:            int32(parsed.AC),
				SpeedFt:       int32(parsed.SpeedFt),
				Gold:          int32(parsed.Gold),
				DdbUrl:        sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsResync {
		t.Error("should be flagged as resync")
	}
	if len(result.Changes) != 0 {
		t.Errorf("expected no changes, got %v", result.Changes)
	}
}

// Test that the service builds CreateCharacterParams correctly
func TestService_Import_CharacterParams(t *testing.T) {
	campaignID := uuid.New()
	charID := uuid.New()

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return minimalDDBJSON(), nil
		},
	}

	var capturedParams refdata.CreateCharacterParams
	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			capturedParams = params
			return refdata.Character{ID: charID, CampaignID: campaignID, Name: params.Name}, nil
		},
	}

	svc := NewService(client, store)
	_, err := svc.Import(context.Background(), campaignID, "https://www.dndbeyond.com/characters/12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedParams.CampaignID != campaignID {
		t.Errorf("CampaignID = %s, want %s", capturedParams.CampaignID, campaignID)
	}
	if capturedParams.Name != "Test Hero" {
		t.Errorf("Name = %q, want %q", capturedParams.Name, "Test Hero")
	}
	if capturedParams.Race != "Human" {
		t.Errorf("Race = %q, want %q", capturedParams.Race, "Human")
	}
	if capturedParams.Level != 3 {
		t.Errorf("Level = %d, want 3", capturedParams.Level)
	}
	if capturedParams.HpMax != 28 {
		t.Errorf("HpMax = %d, want 28", capturedParams.HpMax)
	}
	if !capturedParams.DdbUrl.Valid || capturedParams.DdbUrl.String != "https://www.dndbeyond.com/characters/12345" {
		t.Errorf("DdbUrl = %v, want valid with URL", capturedParams.DdbUrl)
	}

	// Check ability scores
	var as character.AbilityScores
	if err := json.Unmarshal(capturedParams.AbilityScores, &as); err != nil {
		t.Fatalf("unmarshal ability scores: %v", err)
	}
	if as.STR != 16 {
		t.Errorf("STR = %d, want 16", as.STR)
	}

	// Check inventory (NullRawMessage)
	if capturedParams.Inventory.Valid {
		var items []character.InventoryItem
		if err := json.Unmarshal(capturedParams.Inventory.RawMessage, &items); err != nil {
			t.Fatalf("unmarshal inventory: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 inventory items, got %d", len(items))
		}
	}
}
