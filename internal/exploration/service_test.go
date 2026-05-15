package exploration_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/exploration"
	"github.com/ab/dndnd/internal/refdata"
)

// fakeStore is an in-memory implementation of exploration.Store for service tests.
type fakeStore struct {
	createExplorationErr error
	createdEncounter     refdata.Encounter

	encounters map[uuid.UUID]refdata.Encounter
	combatants map[uuid.UUID][]refdata.Combatant
	characters map[uuid.UUID]refdata.Character
	maps       map[uuid.UUID]refdata.Map

	// update hooks
	updateStatusCalls []refdata.UpdateEncounterStatusParams
	updateModeCalls   []refdata.UpdateEncounterModeParams
	positionCalls     []refdata.UpdateCombatantPositionParams

	// counters
	addCombatantCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		encounters: map[uuid.UUID]refdata.Encounter{},
		combatants: map[uuid.UUID][]refdata.Combatant{},
		characters: map[uuid.UUID]refdata.Character{},
		maps:       map[uuid.UUID]refdata.Map{},
	}
}

func (f *fakeStore) CreateExplorationEncounter(ctx context.Context, arg refdata.CreateExplorationEncounterParams) (refdata.Encounter, error) {
	if f.createExplorationErr != nil {
		return refdata.Encounter{}, f.createExplorationErr
	}
	enc := refdata.Encounter{
		ID:          uuid.New(),
		CampaignID:  arg.CampaignID,
		MapID:       arg.MapID,
		Name:        arg.Name,
		DisplayName: arg.DisplayName,
		Status:      "active",
		Mode:        "exploration",
	}
	f.createdEncounter = enc
	f.encounters[enc.ID] = enc
	return enc, nil
}

func (f *fakeStore) GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
	enc, ok := f.encounters[id]
	if !ok {
		return refdata.Encounter{}, errors.New("encounter not found")
	}
	return enc, nil
}

func (f *fakeStore) UpdateEncounterStatus(ctx context.Context, arg refdata.UpdateEncounterStatusParams) (refdata.Encounter, error) {
	f.updateStatusCalls = append(f.updateStatusCalls, arg)
	enc, ok := f.encounters[arg.ID]
	if !ok {
		return refdata.Encounter{}, errors.New("encounter not found")
	}
	enc.Status = arg.Status
	f.encounters[arg.ID] = enc
	return enc, nil
}

func (f *fakeStore) UpdateEncounterMode(ctx context.Context, arg refdata.UpdateEncounterModeParams) (refdata.Encounter, error) {
	f.updateModeCalls = append(f.updateModeCalls, arg)
	enc, ok := f.encounters[arg.ID]
	if !ok {
		return refdata.Encounter{}, errors.New("encounter not found")
	}
	enc.Mode = arg.Mode
	f.encounters[arg.ID] = enc
	return enc, nil
}

func (f *fakeStore) GetMapByIDUnchecked(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
	m, ok := f.maps[id]
	if !ok {
		return refdata.Map{}, errors.New("map not found")
	}
	return m, nil
}

func (f *fakeStore) GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
	c, ok := f.characters[id]
	if !ok {
		return refdata.Character{}, errors.New("character not found")
	}
	return c, nil
}

func (f *fakeStore) CreateCombatant(ctx context.Context, arg refdata.CreateCombatantParams) (refdata.Combatant, error) {
	f.addCombatantCalls++
	cb := refdata.Combatant{
		ID:          uuid.New(),
		EncounterID: arg.EncounterID,
		CharacterID: arg.CharacterID,
		ShortID:     arg.ShortID,
		DisplayName: arg.DisplayName,
		PositionCol: arg.PositionCol,
		PositionRow: arg.PositionRow,
		HpMax:       arg.HpMax,
		HpCurrent:   arg.HpCurrent,
		Ac:          arg.Ac,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  []byte(`[]`),
	}
	f.combatants[arg.EncounterID] = append(f.combatants[arg.EncounterID], cb)
	return cb, nil
}

func (f *fakeStore) ListCombatantsByEncounterID(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
	return f.combatants[encID], nil
}

func (f *fakeStore) UpdateCombatantPosition(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
	f.positionCalls = append(f.positionCalls, arg)
	for encID, list := range f.combatants {
		for i, c := range list {
			if c.ID != arg.ID {
				continue
			}
			list[i].PositionCol = arg.PositionCol
			list[i].PositionRow = arg.PositionRow
			list[i].AltitudeFt = arg.AltitudeFt
			f.combatants[encID] = list
			return list[i], nil
		}
	}
	return refdata.Combatant{}, errors.New("combatant not found")
}

// ---------- Tests ----------

func buildMapWithPlayerZone(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"width":      10,
		"height":     10,
		"tilewidth":  48,
		"tileheight": 48,
		"layers": []map[string]any{
			{
				"name": "spawn_zones",
				"type": "objectgroup",
				"objects": []map[string]any{
					{"id": 1, "x": 96.0, "y": 144.0, "width": 96.0, "height": 96.0, "type": "player"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestStartExploration_CreatesEncounterWithExplorationMode(t *testing.T) {
	store := newFakeStore()
	mapID := uuid.New()
	campID := uuid.New()
	store.maps[mapID] = refdata.Map{
		ID:        mapID,
		TiledJson: buildMapWithPlayerZone(t),
	}
	svc := exploration.NewService(store)

	out, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:   campID,
		MapID:        mapID,
		Name:         "The Forest Clearing",
		CharacterIDs: nil,
	})
	if err != nil {
		t.Fatalf("StartExploration: %v", err)
	}
	if out.Encounter.Mode != "exploration" {
		t.Errorf("encounter mode = %q, want exploration", out.Encounter.Mode)
	}
	if out.Encounter.Status != "active" {
		t.Errorf("encounter status = %q, want active", out.Encounter.Status)
	}
	if out.Encounter.CampaignID != campID {
		t.Errorf("campaign_id mismatch")
	}
	if !out.Encounter.MapID.Valid || out.Encounter.MapID.UUID != mapID {
		t.Errorf("map_id mismatch: %+v", out.Encounter.MapID)
	}
}

func TestStartExploration_PlacesPCsAtSpawnZones(t *testing.T) {
	store := newFakeStore()
	mapID := uuid.New()
	campID := uuid.New()
	store.maps[mapID] = refdata.Map{ID: mapID, TiledJson: buildMapWithPlayerZone(t)}

	charA := uuid.New()
	charB := uuid.New()
	store.characters[charA] = refdata.Character{ID: charA, Name: "Aragorn", HpMax: 40, HpCurrent: 40, Ac: 16, SpeedFt: 30}
	store.characters[charB] = refdata.Character{ID: charB, Name: "Bilbo", HpMax: 20, HpCurrent: 20, Ac: 12, SpeedFt: 25}

	svc := exploration.NewService(store)

	out, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:   campID,
		MapID:        mapID,
		Name:         "Forest",
		CharacterIDs: []uuid.UUID{charA, charB},
	})
	if err != nil {
		t.Fatalf("StartExploration: %v", err)
	}
	if len(out.PCs) != 2 {
		t.Fatalf("got %d placed PCs, want 2", len(out.PCs))
	}
	// Zone covers tiles (2,3)-(3,3)-(4,3)-(2,4)-(3,4)-(4,4)-(2,5)-(3,5)-(4,5).
	// Actually zone is 2x2 at (2,3): tiles (2,3),(3,3),(2,4),(3,4).
	// Wait: x=96/48=2, y=144/48=3, w=96/48=2, h=96/48=2 => tiles (2,3),(3,3),(2,4),(3,4).
	// Assignment order: charA -> (2,3), charB -> (3,3).
	if out.PCs[charA].Col != "C" || out.PCs[charA].Row != 4 {
		t.Errorf("charA at %s%d, want C4", out.PCs[charA].Col, out.PCs[charA].Row)
	}
	if out.PCs[charB].Col != "D" || out.PCs[charB].Row != 4 {
		t.Errorf("charB at %s%d, want D4", out.PCs[charB].Col, out.PCs[charB].Row)
	}
	if store.addCombatantCalls != 2 {
		t.Errorf("addCombatant called %d times, want 2", store.addCombatantCalls)
	}
}

func TestStartExploration_MapWithoutPlayerZoneStillSucceedsIfNoPCs(t *testing.T) {
	store := newFakeStore()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{
		ID:        mapID,
		TiledJson: json.RawMessage(`{"width":5,"height":5,"tilewidth":48,"tileheight":48,"layers":[]}`),
	}
	svc := exploration.NewService(store)

	_, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:   uuid.New(),
		MapID:        mapID,
		Name:         "empty",
		CharacterIDs: nil,
	})
	if err != nil {
		t.Fatalf("StartExploration: %v", err)
	}
}

func TestStartExploration_MapWithoutPlayerZoneFailsIfPCsProvided(t *testing.T) {
	store := newFakeStore()
	mapID := uuid.New()
	store.maps[mapID] = refdata.Map{
		ID:        mapID,
		TiledJson: json.RawMessage(`{"width":5,"height":5,"tilewidth":48,"tileheight":48,"layers":[]}`),
	}
	charA := uuid.New()
	store.characters[charA] = refdata.Character{ID: charA, Name: "A"}
	svc := exploration.NewService(store)

	_, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID:   uuid.New(),
		MapID:        mapID,
		Name:         "empty",
		CharacterIDs: []uuid.UUID{charA},
	})
	if !errors.Is(err, exploration.ErrNoPlayerSpawnZones) {
		t.Fatalf("expected ErrNoPlayerSpawnZones, got %v", err)
	}
}

func TestStartExploration_MapNotFound(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	_, err := svc.StartExploration(context.Background(), exploration.StartInput{
		CampaignID: uuid.New(),
		MapID:      uuid.New(),
		Name:       "x",
	})
	if err == nil {
		t.Fatal("expected error when map not found")
	}
}

func TestEndExploration_SetsStatusCompleted(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{ID: encID, Mode: "exploration", Status: "active"}

	err := svc.EndExploration(context.Background(), encID)
	if err != nil {
		t.Fatalf("EndExploration: %v", err)
	}
	if len(store.updateStatusCalls) != 1 || store.updateStatusCalls[0].Status != "completed" {
		t.Errorf("expected one completed status update; got %+v", store.updateStatusCalls)
	}
}

func TestEndExploration_RejectsNonExplorationEncounter(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{ID: encID, Mode: "combat", Status: "active"}

	err := svc.EndExploration(context.Background(), encID)
	if !errors.Is(err, exploration.ErrEncounterNotExploration) {
		t.Fatalf("expected ErrEncounterNotExploration, got %v", err)
	}
}

func TestTransitionToCombat_CapturesCurrentPositions(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{ID: encID, Mode: "exploration", Status: "active"}

	charA := uuid.New()
	charB := uuid.New()
	store.combatants[encID] = []refdata.Combatant{
		{ID: uuid.New(), EncounterID: encID, CharacterID: uuid.NullUUID{UUID: charA, Valid: true}, PositionCol: "A", PositionRow: 1},
		{ID: uuid.New(), EncounterID: encID, CharacterID: uuid.NullUUID{UUID: charB, Valid: true}, PositionCol: "B", PositionRow: 2},
	}

	positions, err := svc.CapturePositions(context.Background(), encID)
	if err != nil {
		t.Fatalf("CapturePositions: %v", err)
	}
	if len(positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(positions))
	}
	if positions[charA] != (combat.Position{Col: "A", Row: 1}) {
		t.Errorf("charA: %+v", positions[charA])
	}
	if positions[charB] != (combat.Position{Col: "B", Row: 2}) {
		t.Errorf("charB: %+v", positions[charB])
	}
}

func TestCapturePositions_NonExplorationRejected(t *testing.T) {
	store := newFakeStore()
	svc := exploration.NewService(store)
	encID := uuid.New()
	store.encounters[encID] = refdata.Encounter{ID: encID, Mode: "combat"}
	_, err := svc.CapturePositions(context.Background(), encID)
	if !errors.Is(err, exploration.ErrEncounterNotExploration) {
		t.Fatalf("expected ErrEncounterNotExploration, got %v", err)
	}
}

func TestApplyPositionOverrides(t *testing.T) {
	base := map[uuid.UUID]combat.Position{}
	a := uuid.New()
	b := uuid.New()
	base[a] = combat.Position{Col: "A", Row: 1}
	base[b] = combat.Position{Col: "B", Row: 2}

	overrides := map[uuid.UUID]combat.Position{
		a: {Col: "Z", Row: 9},
	}
	got := exploration.ApplyPositionOverrides(base, overrides)
	if got[a] != (combat.Position{Col: "Z", Row: 9}) {
		t.Errorf("override for a not applied: %+v", got[a])
	}
	if got[b] != (combat.Position{Col: "B", Row: 2}) {
		t.Errorf("b should be unchanged: %+v", got[b])
	}
}
