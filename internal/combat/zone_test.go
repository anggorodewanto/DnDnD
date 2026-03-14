package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// TDD Cycle 1: CreateZone creates a zone and returns ZoneInfo
func TestCreateZone(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	ms := defaultMockStore()
	ms.createEncounterZoneFn = func(ctx context.Context, arg refdata.CreateEncounterZoneParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{
			ID:                    uuid.New(),
			EncounterID:           arg.EncounterID,
			SourceCombatantID:     arg.SourceCombatantID,
			SourceSpell:           arg.SourceSpell,
			Shape:                 arg.Shape,
			OriginCol:             arg.OriginCol,
			OriginRow:             arg.OriginRow,
			Dimensions:            arg.Dimensions,
			AnchorMode:            arg.AnchorMode,
			ZoneType:              arg.ZoneType,
			OverlayColor:          arg.OverlayColor,
			MarkerIcon:            arg.MarkerIcon,
			RequiresConcentration: arg.RequiresConcentration,
			ExpiresAtRound:        arg.ExpiresAtRound,
			ZoneTriggers:          arg.ZoneTriggers,
			TriggeredThisRound:    arg.TriggeredThisRound,
		}, nil
	}

	svc := NewService(ms)
	input := CreateZoneInput{
		EncounterID:           encounterID,
		SourceCombatantID:     combatantID,
		SourceSpell:           "Fog Cloud",
		Shape:                 "circle",
		OriginCol:             "C",
		OriginRow:             3,
		Dimensions:            json.RawMessage(`{"radius_ft":20}`),
		AnchorMode:            "static",
		ZoneType:              "heavy_obscurement",
		OverlayColor:          "#808080",
		MarkerIcon:            "\u2601",
		RequiresConcentration: true,
		ExpiresAtRound:        sql.NullInt32{Int32: 10, Valid: true},
		Triggers:              []ZoneTrigger{},
	}

	info, err := svc.CreateZone(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, encounterID, info.EncounterID)
	assert.Equal(t, "Fog Cloud", info.SourceSpell)
	assert.Equal(t, "circle", info.Shape)
	assert.Equal(t, "C", info.OriginCol)
	assert.Equal(t, int32(3), info.OriginRow)
	assert.Equal(t, "static", info.AnchorMode)
	assert.Equal(t, "heavy_obscurement", info.ZoneType)
	assert.True(t, info.RequiresConcentration)
}

// TDD Cycle 2: DeleteZone removes a zone by ID
func TestDeleteZone(t *testing.T) {
	zoneID := uuid.New()
	deleted := false

	ms := defaultMockStore()
	ms.deleteEncounterZoneFn = func(ctx context.Context, id uuid.UUID) error {
		assert.Equal(t, zoneID, id)
		deleted = true
		return nil
	}

	svc := NewService(ms)
	err := svc.DeleteZone(context.Background(), zoneID)
	require.NoError(t, err)
	assert.True(t, deleted)
}

// TDD Cycle 3: CleanupConcentrationZones deletes concentration zones for a combatant
func TestCleanupConcentrationZones(t *testing.T) {
	combatantID := uuid.New()
	cleaned := false

	ms := defaultMockStore()
	ms.deleteConcentrationZonesByCombatantFn = func(ctx context.Context, id uuid.UUID) error {
		assert.Equal(t, combatantID, id)
		cleaned = true
		return nil
	}

	svc := NewService(ms)
	err := svc.CleanupConcentrationZones(context.Background(), combatantID)
	require.NoError(t, err)
	assert.True(t, cleaned)
}

// TDD Cycle 4: CleanupExpiredZones deletes expired zones
func TestCleanupExpiredZones(t *testing.T) {
	encounterID := uuid.New()
	cleaned := false

	ms := defaultMockStore()
	ms.deleteExpiredZonesFn = func(ctx context.Context, arg refdata.DeleteExpiredZonesParams) error {
		assert.Equal(t, encounterID, arg.EncounterID)
		assert.Equal(t, int32(5), arg.ExpiresAtRound.Int32)
		cleaned = true
		return nil
	}

	svc := NewService(ms)
	err := svc.CleanupExpiredZones(context.Background(), encounterID, 5)
	require.NoError(t, err)
	assert.True(t, cleaned)
}

// TDD Cycle 5: CleanupEncounterZones deletes all zones for an encounter
func TestCleanupEncounterZones(t *testing.T) {
	encounterID := uuid.New()
	cleaned := false

	ms := defaultMockStore()
	ms.deleteEncounterZonesByEncounterIDFn = func(ctx context.Context, id uuid.UUID) error {
		assert.Equal(t, encounterID, id)
		cleaned = true
		return nil
	}

	svc := NewService(ms)
	err := svc.CleanupEncounterZones(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, cleaned)
}

// TDD Cycle 6: UpdateZoneAnchor updates origin for combatant-anchored zones
func TestUpdateZoneAnchor(t *testing.T) {
	combatantID := uuid.New()
	zoneID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = nil // not used
	ms.listConcentrationZonesByCombatantFn = nil

	// We need a way to list zones by anchor_combatant_id - but we don't have that query.
	// Instead, UpdateZoneAnchor will list zones by encounter and filter.
	// Actually, we should list all zones for the encounter and filter by anchor_combatant_id.
	// But we don't know the encounter ID. Let's use a different approach:
	// The spec says anchor_combatant_id is set when anchor_mode = 'combatant'.
	// We'll need to add a query for listing by anchor combatant. But the spec only lists
	// the queries above. Let me re-read...
	// The spec says: UpdateZoneAnchor(ctx, combatantID, newCol, newRow)
	// We need to find zones anchored to this combatant. Let's add a listing approach.
	// For now, let's make the service list zones and filter in Go.

	// Actually, we can add a query: ListZonesByAnchorCombatant or we can
	// reuse ListConcentrationZonesByCombatant pattern. Let me just use
	// the existing queries creatively. The simplest: we'll need to know the encounter ID.
	// But the function signature only takes combatantID, col, row.
	// So we need to either look up the combatant to get the encounter ID then list zones,
	// or add a new sqlc query. Let me add a query.

	// For TDD purposes, let me adjust the approach: the service will
	// get combatant to find encounterID, list zones for that encounter,
	// filter by anchor_combatant_id, and update origins.

	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          combatantID,
			EncounterID: uuid.New(),
		}, nil
	}

	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, encounterID uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:                zoneID,
				AnchorMode:        "combatant",
				AnchorCombatantID: uuid.NullUUID{UUID: combatantID, Valid: true},
			},
			{
				ID:         uuid.New(),
				AnchorMode: "static",
			},
		}, nil
	}

	updatedZones := []uuid.UUID{}
	ms.updateEncounterZoneOriginFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error) {
		updatedZones = append(updatedZones, arg.ID)
		assert.Equal(t, "D", arg.OriginCol)
		assert.Equal(t, int32(5), arg.OriginRow)
		return refdata.EncounterZone{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	err := svc.UpdateZoneAnchor(context.Background(), combatantID, "D", 5)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{zoneID}, updatedZones, "only combatant-anchored zone should be updated")
}

// TDD Cycle 7: CheckZoneTriggers detects zone entry triggers
func TestCheckZoneTriggers_Enter(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	zoneID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		triggers, _ := json.Marshal([]ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
		})
		return []refdata.EncounterZone{
			{
				ID:          zoneID,
				EncounterID: encounterID,
				SourceSpell: "Spirit Guardians",
				Shape:       "circle",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"radius_ft":15}`),
				ZoneType:    "damage",
				ZoneTriggers: pqtype.NullRawMessage{
					RawMessage: triggers,
					Valid:      true,
				},
				TriggeredThisRound: pqtype.NullRawMessage{
					RawMessage: json.RawMessage(`{}`),
					Valid:      true,
				},
			},
		}, nil
	}

	ms.updateEncounterZoneTriggeredThisRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	// Combatant moves to C3 (0-based: col=2, row=2), which is the zone origin
	results, err := svc.CheckZoneTriggers(context.Background(), combatantID, 2, 2, encounterID, "enter")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Spirit Guardians", results[0].SourceSpell)
	assert.Equal(t, "damage", results[0].Effect)
}

// TDD Cycle 8: CheckZoneTriggers respects once-per-turn tracking
func TestCheckZoneTriggers_AlreadyTriggered(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	zoneID := uuid.New()

	ms := defaultMockStore()
	triggered := map[string]bool{combatantID.String(): true}
	triggeredJSON, _ := json.Marshal(triggered)

	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		triggers, _ := json.Marshal([]ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
		})
		return []refdata.EncounterZone{
			{
				ID:          zoneID,
				EncounterID: encounterID,
				SourceSpell: "Spirit Guardians",
				Shape:       "circle",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"radius_ft":15}`),
				ZoneType:    "damage",
				ZoneTriggers: pqtype.NullRawMessage{
					RawMessage: triggers,
					Valid:      true,
				},
				TriggeredThisRound: pqtype.NullRawMessage{
					RawMessage: triggeredJSON,
					Valid:      true,
				},
			},
		}, nil
	}

	svc := NewService(ms)
	results, err := svc.CheckZoneTriggers(context.Background(), combatantID, 2, 2, encounterID, "enter")
	require.NoError(t, err)
	assert.Empty(t, results, "already triggered this round, should not fire again")
}

// TDD Cycle 9: ResetZoneTriggersForRound resets all triggered_this_round
func TestResetZoneTriggersForRound(t *testing.T) {
	encounterID := uuid.New()
	reset := false

	ms := defaultMockStore()
	ms.resetAllTriggeredThisRoundFn = func(ctx context.Context, id uuid.UUID) error {
		assert.Equal(t, encounterID, id)
		reset = true
		return nil
	}

	svc := NewService(ms)
	err := svc.ResetZoneTriggersForRound(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, reset)
}

// TDD Cycle 10: ListZonesForEncounter returns all zones
func TestListZonesForEncounter(t *testing.T) {
	encounterID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{ID: uuid.New(), SourceSpell: "Fog Cloud", EncounterID: encounterID},
			{ID: uuid.New(), SourceSpell: "Darkness", EncounterID: encounterID},
		}, nil
	}

	svc := NewService(ms)
	zones, err := svc.ListZonesForEncounter(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Len(t, zones, 2)
	assert.Equal(t, "Fog Cloud", zones[0].SourceSpell)
	assert.Equal(t, "Darkness", zones[1].SourceSpell)
}

// TDD Cycle 11: CheckZoneTriggers returns empty for combatant outside zone
func TestCheckZoneTriggers_OutsideZone(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		triggers, _ := json.Marshal([]ZoneTrigger{
			{Trigger: "enter", Effect: "damage"},
		})
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				EncounterID: encounterID,
				SourceSpell: "Spirit Guardians",
				Shape:       "circle",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"radius_ft":15}`),
				ZoneType:    "damage",
				ZoneTriggers: pqtype.NullRawMessage{
					RawMessage: triggers,
					Valid:      true,
				},
				TriggeredThisRound: pqtype.NullRawMessage{
					RawMessage: json.RawMessage(`{}`),
					Valid:      true,
				},
			},
		}, nil
	}

	svc := NewService(ms)
	// Position far away from zone origin (C3 = col 2, row 2)
	results, err := svc.CheckZoneTriggers(context.Background(), combatantID, 20, 20, encounterID, "enter")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// Test CreateZone error path
func TestCreateZone_StoreError(t *testing.T) {
	ms := defaultMockStore()
	ms.createEncounterZoneFn = func(ctx context.Context, arg refdata.CreateEncounterZoneParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{}, fmt.Errorf("db error")
	}

	svc := NewService(ms)
	_, err := svc.CreateZone(context.Background(), CreateZoneInput{
		SourceSpell: "Fog Cloud",
		Triggers:    []ZoneTrigger{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating encounter zone")
}

// Test UpdateZoneAnchor error paths
func TestUpdateZoneAnchor_GetCombatantError(t *testing.T) {
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{}, fmt.Errorf("not found")
	}

	svc := NewService(ms)
	err := svc.UpdateZoneAnchor(context.Background(), uuid.New(), "A", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "getting combatant")
}

func TestUpdateZoneAnchor_ListZonesError(t *testing.T) {
	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, fmt.Errorf("list error")
	}

	svc := NewService(ms)
	err := svc.UpdateZoneAnchor(context.Background(), uuid.New(), "A", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "listing zones")
}

func TestUpdateZoneAnchor_UpdateOriginError(t *testing.T) {
	combatantID := uuid.New()
	ms := defaultMockStore()
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: combatantID, EncounterID: uuid.New()}, nil
	}
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:                uuid.New(),
				AnchorMode:        "combatant",
				AnchorCombatantID: uuid.NullUUID{UUID: combatantID, Valid: true},
			},
		}, nil
	}
	ms.updateEncounterZoneOriginFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{}, fmt.Errorf("update error")
	}

	svc := NewService(ms)
	err := svc.UpdateZoneAnchor(context.Background(), combatantID, "A", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "updating zone origin")
}

// Test ListZonesForEncounter error
func TestListZonesForEncounter_Error(t *testing.T) {
	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, fmt.Errorf("list error")
	}

	svc := NewService(ms)
	_, err := svc.ListZonesForEncounter(context.Background(), uuid.New())
	assert.Error(t, err)
}

// Test CheckZoneTriggers error paths
func TestCheckZoneTriggers_ListError(t *testing.T) {
	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return nil, fmt.Errorf("list error")
	}

	svc := NewService(ms)
	_, err := svc.CheckZoneTriggers(context.Background(), uuid.New(), 0, 0, uuid.New(), "enter")
	assert.Error(t, err)
}

func TestCheckZoneTriggers_UpdateTriggeredError(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		triggers, _ := json.Marshal([]ZoneTrigger{{Trigger: "enter", Effect: "damage"}})
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				SourceSpell: "Moonbeam",
				Shape:       "circle",
				OriginCol:   "A",
				OriginRow:   1,
				Dimensions:  json.RawMessage(`{"radius_ft":5}`),
				ZoneTriggers: pqtype.NullRawMessage{RawMessage: triggers, Valid: true},
				TriggeredThisRound: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{}`), Valid: true},
			},
		}, nil
	}
	ms.updateEncounterZoneTriggeredThisRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{}, fmt.Errorf("update error")
	}

	svc := NewService(ms)
	_, err := svc.CheckZoneTriggers(context.Background(), combatantID, 0, 0, encounterID, "enter")
	assert.Error(t, err)
}

// Test zoneAffectedTiles with different shapes
func TestZoneAffectedTiles_Square(t *testing.T) {
	zone := refdata.EncounterZone{
		Shape:      "square",
		OriginCol:  "B",
		OriginRow:  2,
		Dimensions: json.RawMessage(`{"side_ft":10}`),
	}
	tiles := zoneAffectedTiles(zone)
	assert.Len(t, tiles, 4) // 2x2 grid
}

func TestZoneAffectedTiles_Rectangle(t *testing.T) {
	zone := refdata.EncounterZone{
		Shape:      "rectangle",
		OriginCol:  "A",
		OriginRow:  1,
		Dimensions: json.RawMessage(`{"width_ft":10}`),
	}
	tiles := zoneAffectedTiles(zone)
	assert.NotEmpty(t, tiles)
}

func TestZoneAffectedTiles_Line(t *testing.T) {
	zone := refdata.EncounterZone{
		Shape:      "line",
		OriginCol:  "A",
		OriginRow:  1,
		Dimensions: json.RawMessage(`{"length_ft":30,"width_ft":5}`),
	}
	tiles := zoneAffectedTiles(zone)
	// Line defaults to just origin tile
	assert.Len(t, tiles, 1)
}

// Test zone with invalid triggers JSON (should be skipped gracefully)
func TestCheckZoneTriggers_InvalidTriggersJSON(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				SourceSpell: "Bad Zone",
				Shape:       "circle",
				OriginCol:   "A",
				OriginRow:   1,
				Dimensions:  json.RawMessage(`{"radius_ft":5}`),
				ZoneTriggers: pqtype.NullRawMessage{RawMessage: json.RawMessage(`invalid`), Valid: true},
				TriggeredThisRound: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{}`), Valid: true},
			},
		}, nil
	}

	svc := NewService(ms)
	results, err := svc.CheckZoneTriggers(context.Background(), combatantID, 0, 0, encounterID, "enter")
	require.NoError(t, err)
	assert.Empty(t, results) // invalid JSON triggers should be skipped
}

// TDD Cycle 12: CheckZoneTriggers with start_of_turn trigger type
func TestCheckZoneTriggers_StartOfTurn(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	ms := defaultMockStore()
	ms.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		triggers, _ := json.Marshal([]ZoneTrigger{
			{Trigger: "start_of_turn", Effect: "damage"},
		})
		return []refdata.EncounterZone{
			{
				ID:          uuid.New(),
				EncounterID: encounterID,
				SourceSpell: "Moonbeam",
				Shape:       "circle",
				OriginCol:   "C",
				OriginRow:   3,
				Dimensions:  json.RawMessage(`{"radius_ft":5}`),
				ZoneType:    "damage",
				ZoneTriggers: pqtype.NullRawMessage{
					RawMessage: triggers,
					Valid:      true,
				},
				TriggeredThisRound: pqtype.NullRawMessage{
					RawMessage: json.RawMessage(`{}`),
					Valid:      true,
				},
			},
		}, nil
	}

	ms.updateEncounterZoneTriggeredThisRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{ID: arg.ID}, nil
	}

	svc := NewService(ms)
	results, err := svc.CheckZoneTriggers(context.Background(), combatantID, 2, 2, encounterID, "start_of_turn")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Moonbeam", results[0].SourceSpell)
}
