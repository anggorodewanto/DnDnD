package ddbimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
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
	CreateFunc      func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error)
	GetByDdbURLFunc func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error)
	UpdateFunc      func(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error)
	pending         map[uuid.UUID]refdata.PendingDdbImport
}

func (m *mockCharStore) CreateCharacter(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
	return m.CreateFunc(ctx, params)
}

func (m *mockCharStore) GetCharacterByDdbURL(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
	if m.GetByDdbURLFunc != nil {
		return m.GetByDdbURLFunc(ctx, params)
	}
	return refdata.Character{}, sql.ErrNoRows
}

func (m *mockCharStore) UpdateCharacterFull(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, params)
	}
	return refdata.Character{}, nil
}

func (m *mockCharStore) UpsertPendingDDBImport(ctx context.Context, params refdata.UpsertPendingDDBImportParams) error {
	if m.pending == nil {
		m.pending = make(map[uuid.UUID]refdata.PendingDdbImport)
	}
	m.pending[params.ID] = refdata.PendingDdbImport{
		ID:          params.ID,
		CharacterID: params.CharacterID,
		ParamsJson:  params.ParamsJson,
		CreatedAt:   params.CreatedAt,
		UpdatedAt:   params.CreatedAt,
	}
	return nil
}

func (m *mockCharStore) GetPendingDDBImport(ctx context.Context, id uuid.UUID) (refdata.PendingDdbImport, error) {
	row, ok := m.pending[id]
	if !ok {
		return refdata.PendingDdbImport{}, sql.ErrNoRows
	}
	return row, nil
}

func (m *mockCharStore) DeletePendingDDBImport(ctx context.Context, id uuid.UUID) error {
	delete(m.pending, id)
	return nil
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

func wizardCureWoundsDDBJSON() []byte {
	return []byte(`{
		"data": {
			"name": "Mira",
			"race": {"fullName": "Human"},
			"classes": [{"definition": {"name": "Wizard", "hitDice": 6}, "level": 3}],
			"stats": [
				{"id": 1, "value": 8},
				{"id": 2, "value": 14},
				{"id": 3, "value": 12},
				{"id": 4, "value": 16},
				{"id": 5, "value": 10},
				{"id": 6, "value": 10}
			],
			"bonusStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"overrideStats": [{"id":1,"value":null},{"id":2,"value":null},{"id":3,"value":null},{"id":4,"value":null},{"id":5,"value":null},{"id":6,"value":null}],
			"baseHitPoints": 18,
			"bonusHitPoints": 0,
			"removedHitPoints": 0,
			"temporaryHitPoints": 0,
			"inventory": [],
			"currencies": {"gp": 0, "sp": 0, "cp": 0, "ep": 0, "pp": 0},
			"modifiers": {"race": [], "class": [], "background": [], "item": [], "feat": [], "condition": []},
			"spells": {
				"class": [{"definition": {"name": "Cure Wounds", "level": 1}}],
				"race": [],
				"item": [],
				"feat": []
			}
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
	if result.Preview == "" {
		t.Error("preview should not be empty")
	}
	if result.IsResync {
		t.Error("first import should not be a resync")
	}
	if result.PendingImportID == uuid.Nil {
		t.Fatal("first import must return a PendingImportID")
	}

	// Approve the import to actually create the character.
	approved, err := svc.ApproveImport(context.Background(), result.PendingImportID)
	if err != nil {
		t.Fatalf("ApproveImport: %v", err)
	}
	if approved.ID != charID {
		t.Errorf("character ID = %s, want %s", approved.ID, charID)
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

func TestService_Import_WizardCureWoundsAdvisoryAndPersistedTag(t *testing.T) {
	charID := uuid.New()
	campaignID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"
	var createdParams refdata.CreateCharacterParams

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return wizardCureWoundsDDBJSON(), nil
		},
	}
	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			createdParams = params
			return refdata.Character{ID: charID, CampaignID: campaignID, Name: params.Name, CharacterData: params.CharacterData}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// H-H04: Off-list detection is disabled (was producing false positives).
	// Cure Wounds is no longer flagged as off-list for wizards.

	// Approve to trigger CreateCharacter.
	if _, err := svc.ApproveImport(context.Background(), result.PendingImportID); err != nil {
		t.Fatalf("ApproveImport: %v", err)
	}

	var charData struct {
		Spells []SpellEntry `json:"spells"`
	}
	if err := json.Unmarshal(createdParams.CharacterData.RawMessage, &charData); err != nil {
		t.Fatalf("unmarshal persisted character_data: %v", err)
	}
	if len(charData.Spells) != 1 {
		t.Fatalf("expected 1 persisted spell, got %d", len(charData.Spells))
	}
	// With detection disabled, spell should NOT be tagged
	if charData.Spells[0].OffList {
		t.Fatalf("expected spell NOT tagged off-list (detection disabled), got %+v", charData.Spells[0])
	}
}

// TestService_Import_FirstImportStagesPending verifies that a brand-new import
// (no existing DDB-URL row) does NOT call CreateCharacter immediately. Instead
// it stages the import for DM approval, just like re-syncs.
func TestService_Import_FirstImportStagesPending(t *testing.T) {
	campaignID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) {
			return minimalDDBJSON(), nil
		},
	}

	createCalls := 0
	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			createCalls++
			return refdata.Character{ID: uuid.New(), CampaignID: campaignID, Name: params.Name}, nil
		},
		// No GetByDdbURLFunc → returns sql.ErrNoRows (first import)
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalls != 0 {
		t.Errorf("CreateCharacter must NOT be called on first import (requires DM approval); called %d times", createCalls)
	}
	if result.PendingImportID == uuid.Nil {
		t.Error("first import must return a non-nil PendingImportID for DM approval")
	}
	if result.IsResync {
		t.Error("first import should not be flagged as resync")
	}
}

// TestService_Import_ResyncStagesNotMutates is the regression test for the
// chunk-7 Phase-90 finding: re-sync must NOT call UpdateCharacterFull until
// the DM explicitly approves. Import returns a pending import id; the DB is
// untouched.
func TestService_Import_ResyncStagesNotMutates(t *testing.T) {
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

	updateCalls := 0
	createCalls := 0
	store := &mockCharStore{
		CreateFunc: func(ctx context.Context, params refdata.CreateCharacterParams) (refdata.Character, error) {
			createCalls++
			return refdata.Character{}, nil
		},
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
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
		UpdateFunc: func(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error) {
			updateCalls++
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
	if updateCalls != 0 {
		t.Errorf("UpdateCharacterFull must NOT be called before approval; called %d times", updateCalls)
	}
	if createCalls != 0 {
		t.Errorf("CreateCharacter must NOT be called on re-sync; called %d times", createCalls)
	}
	if result.PendingImportID == uuid.Nil {
		t.Error("re-sync with changes must return a non-nil PendingImportID")
	}
	if result.Character.ID != existingID {
		t.Errorf("Character should reflect existing record (no DB write yet); got ID %s, want %s", result.Character.ID, existingID)
	}
}

func TestService_ApproveImport_Success(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) { return minimalDDBJSON(), nil },
	}

	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 2}})

	updateCalls := 0
	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
			return refdata.Character{
				ID: existingID, CampaignID: campaignID, Name: "Old Name", Race: "Human",
				Level: 2, AbilityScores: scores, Classes: classes,
				DdbUrl: sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
		UpdateFunc: func(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error) {
			updateCalls++
			if params.ID != existingID {
				t.Errorf("update called with wrong ID: %s", params.ID)
			}
			return refdata.Character{ID: existingID, Name: params.Name}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if updateCalls != 0 {
		t.Fatalf("update must not run before Approve; got %d", updateCalls)
	}

	approved, err := svc.ApproveImport(context.Background(), result.PendingImportID)
	if err != nil {
		t.Fatalf("ApproveImport: %v", err)
	}
	if updateCalls != 1 {
		t.Errorf("UpdateCharacterFull should be called exactly once; got %d", updateCalls)
	}
	if approved.ID != existingID {
		t.Errorf("approved.ID = %s, want %s", approved.ID, existingID)
	}
}

func TestService_ApproveImport_PendingSurvivesRestart(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) { return minimalDDBJSON(), nil },
	}

	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 2}})

	updateCalls := 0
	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
			return refdata.Character{
				ID: existingID, CampaignID: campaignID, Name: "Old Name", Race: "Human",
				Level: 2, AbilityScores: scores, Classes: classes,
				DdbUrl: sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
		UpdateFunc: func(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error) {
			updateCalls++
			if params.ID != existingID {
				t.Errorf("update called with wrong ID: %s", params.ID)
			}
			return refdata.Character{ID: existingID, Name: params.Name}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.PendingImportID == uuid.Nil {
		t.Fatal("expected pending import id")
	}

	restarted := NewService(client, store)
	approved, err := restarted.ApproveImport(context.Background(), result.PendingImportID)
	if err != nil {
		t.Fatalf("ApproveImport after restart: %v", err)
	}
	if updateCalls != 1 {
		t.Errorf("UpdateCharacterFull should be called exactly once; got %d", updateCalls)
	}
	if approved.ID != existingID {
		t.Errorf("approved.ID = %s, want %s", approved.ID, existingID)
	}
}

func TestService_ApproveImport_NotFound(t *testing.T) {
	svc := NewService(&mockClient{}, &mockCharStore{})
	_, err := svc.ApproveImport(context.Background(), uuid.New())
	if err != ErrPendingImportNotFound {
		t.Errorf("got %v, want ErrPendingImportNotFound", err)
	}
}

func TestService_ApproveImport_AlreadyApproved(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) { return minimalDDBJSON(), nil },
	}
	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 2}})

	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
			return refdata.Character{
				ID: existingID, CampaignID: campaignID, Name: "Old Name", Race: "Human",
				AbilityScores: scores, Classes: classes,
				DdbUrl: sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
		UpdateFunc: func(ctx context.Context, params refdata.UpdateCharacterFullParams) (refdata.Character, error) {
			return refdata.Character{ID: existingID, Name: params.Name}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if _, err := svc.ApproveImport(context.Background(), result.PendingImportID); err != nil {
		t.Fatalf("first ApproveImport: %v", err)
	}
	_, err = svc.ApproveImport(context.Background(), result.PendingImportID)
	if err != ErrPendingImportNotFound {
		t.Errorf("second ApproveImport: got %v, want ErrPendingImportNotFound (consumed)", err)
	}
}

func TestService_ApproveImport_Expired(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) { return minimalDDBJSON(), nil },
	}
	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 2}})

	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
			return refdata.Character{
				ID: existingID, CampaignID: campaignID, Name: "Old Name", Race: "Human",
				AbilityScores: scores, Classes: classes,
				DdbUrl: sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
	}

	// Inject a clock so we can fast-forward past the TTL.
	now := time.Now()
	svc := NewServiceWithClock(client, store, func() time.Time { return now })
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Advance past the TTL.
	now = now.Add(pendingImportTTL + time.Second)

	_, err = svc.ApproveImport(context.Background(), result.PendingImportID)
	if err != ErrPendingImportNotFound {
		t.Errorf("expired entry: got %v, want ErrPendingImportNotFound", err)
	}
}

func TestService_DiscardImport(t *testing.T) {
	campaignID := uuid.New()
	existingID := uuid.New()
	ddbURL := "https://www.dndbeyond.com/characters/12345"

	client := &mockClient{
		FetchFunc: func(ctx context.Context, id string) ([]byte, error) { return minimalDDBJSON(), nil },
	}
	scores, _ := json.Marshal(character.AbilityScores{STR: 14, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8})
	classes, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 2}})

	store := &mockCharStore{
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
			return refdata.Character{
				ID: existingID, CampaignID: campaignID, Name: "Old Name", Race: "Human",
				AbilityScores: scores, Classes: classes,
				DdbUrl: sql.NullString{String: ddbURL, Valid: true},
			}, nil
		},
	}

	svc := NewService(client, store)
	result, err := svc.Import(context.Background(), campaignID, ddbURL)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	svc.DiscardImport(result.PendingImportID)
	if _, err := svc.ApproveImport(context.Background(), result.PendingImportID); err != ErrPendingImportNotFound {
		t.Errorf("after Discard: got %v, want ErrPendingImportNotFound", err)
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
		GetByDdbURLFunc: func(ctx context.Context, params refdata.GetCharacterByDdbURLParams) (refdata.Character, error) {
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
	if result.PendingImportID != uuid.Nil {
		t.Errorf("no-change re-sync must not stash a pending import; got %s", result.PendingImportID)
	}
}

func TestCharacterToParseResult_FeaturesAndSpells(t *testing.T) {
	features := []character.Feature{{Name: "Action Surge", Source: "Fighter", Level: 2}}
	featuresJSON, _ := json.Marshal(features)
	spells := []SpellEntry{{Name: "Fire Bolt", Level: 0, Source: "class"}}
	charData := map[string]interface{}{"spells": spells}
	charDataJSON, _ := json.Marshal(charData)

	c := &refdata.Character{
		Name:          "Test",
		Race:          "Human",
		Level:         5,
		HpMax:         44,
		HpCurrent:     39,
		Ac:            18,
		SpeedFt:       30,
		Features:      pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
		CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
	}

	pc := characterToParseResult(c)
	if len(pc.Features) != 1 || pc.Features[0].Name != "Action Surge" {
		t.Errorf("Features = %v, want [{Action Surge ...}]", pc.Features)
	}
	if len(pc.Spells) != 1 || pc.Spells[0].Name != "Fire Bolt" {
		t.Errorf("Spells = %v, want [{Fire Bolt ...}]", pc.Spells)
	}
}

func TestBuildCreateParams_FeaturesAndSpells(t *testing.T) {
	pc := validCharacter()
	pc.Features = []character.Feature{
		{Name: "Second Wind", Source: "Fighter", Level: 1, Description: "Regain HP"},
	}
	pc.Spells = []SpellEntry{
		{Name: "Fire Bolt", Level: 0, Source: "class"},
		{Name: "Magic Missile", Level: 1, Source: "class"},
	}

	params, err := buildCreateParams(uuid.New(), "https://dndbeyond.com/characters/1", pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Features should be in the Features field
	if !params.Features.Valid {
		t.Fatal("expected Features to be valid")
	}
	var features []character.Feature
	if err := json.Unmarshal(params.Features.RawMessage, &features); err != nil {
		t.Fatalf("unmarshal features: %v", err)
	}
	if len(features) != 1 || features[0].Name != "Second Wind" {
		t.Errorf("features = %v, want [{Second Wind ...}]", features)
	}

	// Spells should be in CharacterData
	if !params.CharacterData.Valid {
		t.Fatal("expected CharacterData to be valid")
	}
	var charData map[string]json.RawMessage
	if err := json.Unmarshal(params.CharacterData.RawMessage, &charData); err != nil {
		t.Fatalf("unmarshal character_data: %v", err)
	}
	spellsRaw, ok := charData["spells"]
	if !ok {
		t.Fatal("expected 'spells' key in character_data")
	}
	var spells []SpellEntry
	if err := json.Unmarshal(spellsRaw, &spells); err != nil {
		t.Fatalf("unmarshal spells: %v", err)
	}
	if len(spells) != 2 {
		t.Errorf("expected 2 spells, got %d", len(spells))
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
	result, err := svc.Import(context.Background(), campaignID, "https://www.dndbeyond.com/characters/12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Approve to trigger CreateCharacter and capture params.
	if _, err := svc.ApproveImport(context.Background(), result.PendingImportID); err != nil {
		t.Fatalf("ApproveImport: %v", err)
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
