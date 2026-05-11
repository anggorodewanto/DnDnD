package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- E-67-zone-anchor-follow: UpdateCombatantPosition invokes UpdateZoneAnchor ---

func TestUpdateCombatantPosition_InvokesZoneAnchorForFollowZones(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	zoneID := uuid.New()

	store := defaultMockStore()
	store.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          arg.ID,
			EncounterID: encounterID,
			PositionCol: arg.PositionCol,
			PositionRow: arg.PositionRow,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          id,
			EncounterID: encounterID,
			PositionCol: "C",
			PositionRow: 3,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		return []refdata.EncounterZone{
			{
				ID:                zoneID,
				EncounterID:       encounterID,
				SourceSpell:       "Spirit Guardians",
				Shape:             "circle",
				OriginCol:         "A",
				OriginRow:         1,
				Dimensions:        json.RawMessage(`{"radius_ft":15}`),
				AnchorMode:        "combatant",
				AnchorCombatantID: uuid.NullUUID{UUID: combatantID, Valid: true},
				ZoneType:          "damage",
				ZoneTriggers: pqtype.NullRawMessage{
					RawMessage: json.RawMessage(`[]`),
					Valid:      true,
				},
				TriggeredThisRound: pqtype.NullRawMessage{
					RawMessage: json.RawMessage(`{}`),
					Valid:      true,
				},
			},
		}, nil
	}

	updated := []refdata.UpdateEncounterZoneOriginParams{}
	store.updateEncounterZoneOriginFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneOriginParams) (refdata.EncounterZone, error) {
		updated = append(updated, arg)
		return refdata.EncounterZone{ID: arg.ID}, nil
	}

	svc := NewService(store)
	_, err := svc.UpdateCombatantPosition(context.Background(), combatantID, "D", 5, 0)
	require.NoError(t, err)

	require.Len(t, updated, 1, "Spirit Guardians anchor should follow on /move")
	assert.Equal(t, zoneID, updated[0].ID)
	assert.Equal(t, "D", updated[0].OriginCol)
	assert.Equal(t, int32(5), updated[0].OriginRow)
}

// --- E-67-zone-triggers: UpdateCombatantPosition runs CheckZoneTriggers(enter) ---

func TestUpdateCombatantPosition_FiresEnterTriggers(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	zoneID := uuid.New()

	store := defaultMockStore()
	store.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          arg.ID,
			EncounterID: encounterID,
			PositionCol: arg.PositionCol,
			PositionRow: arg.PositionRow,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{
			ID:          id,
			EncounterID: encounterID,
			PositionCol: "C",
			PositionRow: 3,
			Conditions:  json.RawMessage(`[]`),
		}, nil
	}
	store.listEncounterZonesByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.EncounterZone, error) {
		triggers, _ := json.Marshal([]ZoneTrigger{{Trigger: "enter", Effect: "damage"}})
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
	store.updateEncounterZoneTriggeredThisRoundFn = func(ctx context.Context, arg refdata.UpdateEncounterZoneTriggeredThisRoundParams) (refdata.EncounterZone, error) {
		return refdata.EncounterZone{ID: arg.ID}, nil
	}

	svc := NewService(store)
	_, results, err := svc.UpdateCombatantPositionWithTriggers(context.Background(), combatantID, "C", 3, 0)
	require.NoError(t, err)
	require.Len(t, results, 1, "moving onto Spirit Guardians origin should fire enter trigger")
	assert.Equal(t, "Spirit Guardians", results[0].SourceSpell)
	assert.Equal(t, "enter", results[0].Trigger)
	assert.Equal(t, "damage", results[0].Effect)
}

// --- E-67-zone-cleanup: maybeCreateSpellZone sets ExpiresAtRound ---

func TestSpellDurationRounds(t *testing.T) {
	tests := []struct {
		in        string
		wantRound int
		wantOk    bool
	}{
		{"1 round", 1, true},
		{"10 rounds", 10, true},
		{"1 minute", 10, true},
		{"10 minutes", 100, true},
		{"Concentration, up to 1 minute", 10, true},
		{"Concentration, up to 10 minutes", 100, true},
		{"1 hour", 600, true},
		{"Instantaneous", 0, false},
		{"Until dispelled", 0, false},
		{"Special", 0, false},
		{"", 0, false},
		{"garbage", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := SpellDurationRounds(tc.in)
			assert.Equal(t, tc.wantOk, ok)
			if tc.wantOk {
				assert.Equal(t, tc.wantRound, got)
			}
		})
	}
}

// --- E-67-zone-cleanup: AdvanceTurn round tick triggers CleanupExpiredZones + ResetZoneTriggersForRound ---

func TestAdvanceTurn_RoundAdvanceTriggersZoneCleanupAndReset(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, RoundNumber: 1, Status: "active"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:          combatantID,
				EncounterID: encounterID,
				IsAlive:     true,
				PositionCol: "A",
				PositionRow: 1,
				Conditions:  json.RawMessage(`[]`),
			},
		}, nil
	}
	// Every combatant already had a turn this round → forces round advance.
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{
			{ID: uuid.New(), CombatantID: combatantID, RoundNumber: arg.RoundNumber, Status: "completed"},
		}, nil
	}

	cleanupCalled := false
	store.deleteExpiredZonesFn = func(ctx context.Context, arg refdata.DeleteExpiredZonesParams) error {
		cleanupCalled = true
		assert.Equal(t, encounterID, arg.EncounterID)
		assert.Equal(t, int32(2), arg.ExpiresAtRound.Int32, "cleanup should pass new round number")
		return nil
	}
	resetCalled := false
	store.resetAllTriggeredThisRoundFn = func(ctx context.Context, eid uuid.UUID) error {
		resetCalled = true
		assert.Equal(t, encounterID, eid)
		return nil
	}

	svc := NewService(store)
	_, err := svc.AdvanceTurn(context.Background(), encounterID)
	require.NoError(t, err)
	assert.True(t, cleanupCalled, "round advance should invoke CleanupExpiredZones")
	assert.True(t, resetCalled, "round advance should invoke ResetZoneTriggersForRound")
}

// --- E-71-readied-action-expiry: createActiveTurn invokes ExpireReadiedActions ---

func TestCreateActiveTurn_ExpiresReadiedActions(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()
	declID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, RoundNumber: 2, Status: "active"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:          combatantID,
				EncounterID: encounterID,
				IsAlive:     true,
				IsNpc:       true,
				PositionCol: "A",
				PositionRow: 1,
				Conditions:  json.RawMessage(`[]`),
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil // combatant hasn't gone yet this round
	}
	cancelled := []uuid.UUID{}
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{
				ID:              declID,
				CombatantID:     combatantID,
				EncounterID:     encounterID,
				Description:     "Attack the next goblin to enter the room",
				Status:          "active",
				IsReadiedAction: true,
			},
		}, nil
	}
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		cancelled = append(cancelled, id)
		return refdata.ReactionDeclaration{ID: id, Status: "cancelled"}, nil
	}

	svc := NewService(store)
	info, err := svc.AdvanceTurn(context.Background(), encounterID)
	require.NoError(t, err)
	assert.Len(t, cancelled, 1, "active readied action should be cancelled at start of next turn")
	require.Len(t, info.ExpiryNotices, 1, "expiry notice should be surfaced via TurnInfo")
	assert.Contains(t, info.ExpiryNotices[0], "Attack the next goblin to enter the room")
}

// --- E-71-readied-action-expiry: spell readied actions clear concentration on expiry ---

func TestCreateActiveTurn_ReadiedSpellExpiryClearsConcentration(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	store := defaultMockStore()
	store.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, RoundNumber: 2, Status: "active"}, nil
	}
	store.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{
			{
				ID:          combatantID,
				EncounterID: encounterID,
				IsAlive:     true,
				IsNpc:       true,
				PositionCol: "A",
				PositionRow: 1,
				Conditions:  json.RawMessage(`[]`),
			},
		}, nil
	}
	store.listTurnsByEncounterAndRoundFn = func(ctx context.Context, arg refdata.ListTurnsByEncounterAndRoundParams) ([]refdata.Turn, error) {
		return []refdata.Turn{}, nil
	}
	store.listActiveReactionDeclarationsByCombatantFn = func(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error) {
		return []refdata.ReactionDeclaration{
			{
				ID:              uuid.New(),
				CombatantID:     combatantID,
				EncounterID:     encounterID,
				Description:     "Cast Hold Person on the shaman if he moves",
				Status:          "active",
				IsReadiedAction: true,
				SpellName:       sql.NullString{String: "Hold Person", Valid: true},
				SpellSlotLevel:  sql.NullInt32{Int32: 2, Valid: true},
			},
		}, nil
	}
	store.cancelReactionDeclarationFn = func(ctx context.Context, id uuid.UUID) (refdata.ReactionDeclaration, error) {
		return refdata.ReactionDeclaration{ID: id, Status: "cancelled"}, nil
	}
	cleared := false
	store.clearCombatantConcentrationFn = func(ctx context.Context, id uuid.UUID) error {
		cleared = true
		assert.Equal(t, combatantID, id)
		return nil
	}

	svc := NewService(store)
	info, err := svc.AdvanceTurn(context.Background(), encounterID)
	require.NoError(t, err)
	require.Len(t, info.ExpiryNotices, 1)
	assert.Contains(t, info.ExpiryNotices[0], "Concentration on Hold Person ended")
	assert.True(t, cleared, "readied-spell expiry should clear concentration columns")
}
