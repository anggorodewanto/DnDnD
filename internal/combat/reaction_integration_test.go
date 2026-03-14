package combat_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestEncounterAndCombatant is a helper that creates a campaign, map, encounter, and combatant.
func createTestEncounterAndCombatant(t *testing.T, db *sql.DB, queries *refdata.Queries) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "Test Encounter",
		Status:     "active",
		RoundNumber: 1,
	})
	require.NoError(t, err)

	comb, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HpMax:       45,
		HpCurrent:   45,
		Ac:          18,
		PositionCol: "A",
		PositionRow: 1,
		Conditions:  []byte(`[]`),
		IsAlive:     true,
		IsVisible:   true,
	})
	require.NoError(t, err)

	return enc.ID, comb.ID
}

// --- Integration: Reaction declaration CRUD ---

func TestIntegration_ReactionDeclaration_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(&testStoreAdapter{queries})

	// Declare two reactions
	decl1, err := svc.DeclareReaction(ctx, encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)
	assert.Equal(t, "active", decl1.Status)
	assert.Equal(t, "Shield if I get hit", decl1.Description)
	assert.Equal(t, encounterID, decl1.EncounterID)
	assert.Equal(t, combatantID, decl1.CombatantID)

	decl2, err := svc.DeclareReaction(ctx, encounterID, combatantID, "Counterspell if enemy casts")
	require.NoError(t, err)
	assert.Equal(t, "active", decl2.Status)

	// List active by encounter
	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	// List by combatant
	byCombatant, err := svc.ListReactionsByCombatant(ctx, combatantID, encounterID)
	require.NoError(t, err)
	assert.Len(t, byCombatant, 2)

	// Cancel specific by ID
	cancelled, err := svc.CancelReaction(ctx, decl1.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", cancelled.Status)

	// Verify only 1 active
	active, err = svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, decl2.ID, active[0].ID)
}

// --- Integration: Cancel by description ---

func TestIntegration_ReactionDeclaration_CancelByDescription(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(&testStoreAdapter{queries})

	_, err := svc.DeclareReaction(ctx, encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)
	_, err = svc.DeclareReaction(ctx, encounterID, combatantID, "Counterspell if enemy casts")
	require.NoError(t, err)

	// Cancel by description substring (case-insensitive)
	cancelled, err := svc.CancelReactionByDescription(ctx, combatantID, encounterID, "shield")
	require.NoError(t, err)
	assert.Equal(t, "cancelled", cancelled.Status)
	assert.Contains(t, cancelled.Description, "Shield")

	// Only Counterspell remains active
	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Contains(t, active[0].Description, "Counterspell")

	// No match returns error
	_, err = svc.CancelReactionByDescription(ctx, combatantID, encounterID, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active reaction matching")
}

// --- Integration: Cancel all ---

func TestIntegration_ReactionDeclaration_CancelAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(&testStoreAdapter{queries})

	_, err := svc.DeclareReaction(ctx, encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)
	_, err = svc.DeclareReaction(ctx, encounterID, combatantID, "Counterspell if enemy casts")
	require.NoError(t, err)

	err = svc.CancelAllReactions(ctx, combatantID, encounterID)
	require.NoError(t, err)

	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 0)
}

// --- Integration: Resolve reaction + one-per-round enforcement ---

func TestIntegration_ReactionDeclaration_Resolve(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(&testStoreAdapter{queries})

	// Create a turn for the combatant
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         encounterID,
		CombatantID:         combatantID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	// Update encounter current_turn
	_, err = queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            encounterID,
		CurrentTurnID: uuid.NullUUID{UUID: turn.ID, Valid: true},
	})
	require.NoError(t, err)

	// Declare reaction and resolve
	decl, err := svc.DeclareReaction(ctx, encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)

	resolved, err := svc.ResolveReaction(ctx, decl.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "used", resolved.Status)
	assert.True(t, resolved.UsedOnRound.Valid)
	assert.Equal(t, int32(1), resolved.UsedOnRound.Int32)

	// Verify turn reaction_used is true
	updatedTurn, err := queries.GetTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.True(t, updatedTurn.ReactionUsed)

	// Cannot resolve an already-used declaration
	_, err = svc.ResolveReaction(ctx, decl.ID, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

// --- Integration: Encounter end cleanup ---

func TestIntegration_ReactionDeclaration_EncounterEndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatantID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(&testStoreAdapter{queries})

	// Declare reactions
	_, err := svc.DeclareReaction(ctx, encounterID, combatantID, "Shield if I get hit")
	require.NoError(t, err)
	_, err = svc.DeclareReaction(ctx, encounterID, combatantID, "Counterspell if enemy casts")
	require.NoError(t, err)

	// End combat should clean up reactions
	_, err = svc.EndCombat(ctx, encounterID)
	require.NoError(t, err)

	// Verify reactions are deleted
	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 0)

	// All declarations should be gone
	all, err := svc.ListReactionsByCombatant(ctx, combatantID, encounterID)
	require.NoError(t, err)
	assert.Len(t, all, 0)
}

// --- Integration: Multiple combatants with independent reactions ---

func TestIntegration_ReactionDeclaration_MultipleCombatants(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	encounterID, combatant1ID := createTestEncounterAndCombatant(t, db, queries)
	svc := combat.NewService(&testStoreAdapter{queries})

	// Create second combatant
	comb2, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: encounterID,
		ShortID:     "LE",
		DisplayName: "Legolas",
		HpMax:       35,
		HpCurrent:   35,
		Ac:          16,
		PositionCol: "B",
		PositionRow: 2,
		Conditions:  []byte(`[]`),
		IsAlive:     true,
		IsVisible:   true,
	})
	require.NoError(t, err)

	// Each combatant declares reactions
	_, err = svc.DeclareReaction(ctx, encounterID, combatant1ID, "Shield if I get hit")
	require.NoError(t, err)
	_, err = svc.DeclareReaction(ctx, encounterID, comb2.ID, "OA if G1 moves away")
	require.NoError(t, err)

	// List all active should show both
	active, err := svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	// Cancel all for combatant1 only
	err = svc.CancelAllReactions(ctx, combatant1ID, encounterID)
	require.NoError(t, err)

	// Only combatant2's reaction remains
	active, err = svc.ListActiveReactions(ctx, encounterID)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, comb2.ID, active[0].CombatantID)
}
