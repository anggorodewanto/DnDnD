package combat_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, *refdata.Queries) {
	t.Helper()
	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	return db, refdata.New(db)
}

func createTestCampaign(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		id, "guild-1", "user-1", "Test Campaign", "active")
	require.NoError(t, err)
	return id
}

func createTestMap(t *testing.T, db *sql.DB, campaignID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO maps (id, campaign_id, name, width_squares, height_squares, tiled_json) VALUES ($1, $2, $3, $4, $5, $6)`,
		id, campaignID, "Test Map", 10, 10, `{}`)
	require.NoError(t, err)
	return id
}

func createTestCharacter(t *testing.T, db *sql.DB, campaignID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		id, campaignID, "Aragorn", "human", `[{"class":"fighter","level":5}]`, 5,
		`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":10}`,
		45, 45, 18, 30, 3, `[{"die":"d10","remaining":5}]`, `{Common}`)
	require.NoError(t, err)
	return id
}

func createTestCreature(t *testing.T, db *sql.DB) string {
	t.Helper()
	_, err := db.Exec(`INSERT INTO creatures (id, name, size, type, ac, hp_formula, hp_average, speed, ability_scores, cr, attacks)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`,
		"goblin", "Goblin", "Small", "humanoid", 15, "2d6", 7,
		`{"walk":30}`, `{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`, "1/4", `[]`)
	require.NoError(t, err)
	return "goblin"
}

// --- Integration TDD Cycle 1: Migration creates all 4 tables ---

func TestIntegration_MigrationCreatesAllTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, _ := setupTestDB(t)
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	// Verify encounters table
	_, err := db.Exec(`INSERT INTO encounters (campaign_id, map_id, name, status) VALUES ($1, $2, $3, $4)`,
		campaignID, mapID, "Test Encounter", "preparing")
	require.NoError(t, err, "encounters table should exist")

	// Verify combatants table
	var encID uuid.UUID
	err = db.QueryRow(`SELECT id FROM encounters WHERE campaign_id = $1`, campaignID).Scan(&encID)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO combatants (encounter_id, short_id, display_name, hp_max, hp_current, ac, position_col, position_row) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		encID, "G1", "Goblin", 7, 7, 15, "A", 1)
	require.NoError(t, err, "combatants table should exist")

	// Verify turns table
	var combatantID uuid.UUID
	err = db.QueryRow(`SELECT id FROM combatants WHERE encounter_id = $1`, encID).Scan(&combatantID)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO turns (encounter_id, combatant_id, round_number, status, movement_remaining_ft) VALUES ($1, $2, $3, $4, $5)`,
		encID, combatantID, 1, "active", 30)
	require.NoError(t, err, "turns table should exist")

	// Verify action_log table
	var turnID uuid.UUID
	err = db.QueryRow(`SELECT id FROM turns WHERE encounter_id = $1`, encID).Scan(&turnID)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO action_log (turn_id, encounter_id, action_type, actor_id, before_state, after_state) VALUES ($1, $2, $3, $4, $5, $6)`,
		turnID, encID, "attack", combatantID, `{}`, `{}`)
	require.NoError(t, err, "action_log table should exist")
}

// --- Integration TDD Cycle 2: Encounter CRUD round-trip ---

func TestIntegration_EncounterCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	mapID := createTestMap(t, db, campaignID)

	// Create
	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		MapID:      uuid.NullUUID{UUID: mapID, Valid: true},
		Name:       "Goblin Ambush",
		DisplayName: sql.NullString{String: "The Dark Forest", Valid: true},
		Status:     "preparing",
	})
	require.NoError(t, err)
	assert.Equal(t, "Goblin Ambush", enc.Name)
	assert.Equal(t, "preparing", enc.Status)
	assert.Equal(t, int32(0), enc.RoundNumber)

	// Get
	got, err := queries.GetEncounter(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, enc.ID, got.ID)
	assert.Equal(t, "The Dark Forest", got.DisplayName.String)

	// List
	list, err := queries.ListEncountersByCampaignID(ctx, campaignID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update status
	updated, err := queries.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     enc.ID,
		Status: "active",
	})
	require.NoError(t, err)
	assert.Equal(t, "active", updated.Status)

	// Update round
	updated, err = queries.UpdateEncounterRound(ctx, refdata.UpdateEncounterRoundParams{
		ID:          enc.ID,
		RoundNumber: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), updated.RoundNumber)

	// Delete
	err = queries.DeleteEncounter(ctx, enc.ID)
	require.NoError(t, err)

	_, err = queries.GetEncounter(ctx, enc.ID)
	require.Error(t, err)
}

// --- Integration TDD Cycle 3: Combatant CRUD round-trip ---

func TestIntegration_CombatantCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	createTestCreature(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "preparing",
	})
	require.NoError(t, err)

	// Create combatant
	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID:   enc.ID,
		CreatureRefID: sql.NullString{String: "goblin", Valid: true},
		ShortID:       "G1",
		DisplayName:   "Goblin 1",
		HpMax:         7,
		HpCurrent:     7,
		Ac:            15,
		Conditions:    json.RawMessage(`[]`),
		PositionCol:   "A",
		PositionRow:   1,
		IsVisible:     true,
		IsAlive:       true,
		IsNpc:         true,
	})
	require.NoError(t, err)
	assert.Equal(t, "G1", c.ShortID)
	assert.Equal(t, int32(7), c.HpMax)

	// Get
	got, err := queries.GetCombatant(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)

	// List
	list, err := queries.ListCombatantsByEncounterID(ctx, enc.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update HP
	updated, err := queries.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID:        c.ID,
		HpCurrent: 3,
		TempHp:    0,
		IsAlive:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), updated.HpCurrent)

	// Update Position
	updated, err = queries.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
		ID:          c.ID,
		PositionCol: "C",
		PositionRow: 5,
		AltitudeFt:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, "C", updated.PositionCol)
	assert.Equal(t, int32(5), updated.PositionRow)
	assert.Equal(t, int32(10), updated.AltitudeFt)

	// Update Conditions
	updated, err = queries.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              c.ID,
		Conditions:      json.RawMessage(`["poisoned"]`),
		ExhaustionLevel: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), updated.ExhaustionLevel)

	// Update Death Saves
	updated, err = queries.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         c.ID,
		DeathSaves: pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"successes":1,"failures":0}`), Valid: true},
	})
	require.NoError(t, err)
	assert.True(t, updated.DeathSaves.Valid)

	// Update Initiative
	updated, err = queries.UpdateCombatantInitiative(ctx, refdata.UpdateCombatantInitiativeParams{
		ID:              c.ID,
		InitiativeRoll:  15,
		InitiativeOrder: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(15), updated.InitiativeRoll)

	// Delete
	err = queries.DeleteCombatant(ctx, c.ID)
	require.NoError(t, err)
}

// --- Integration TDD Cycle 4: Turn CRUD round-trip ---

func TestIntegration_TurnCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "active",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "G1",
		DisplayName: "Goblin 1",
		HpMax:       7,
		HpCurrent:   7,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	})
	require.NoError(t, err)

	// Create turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         c.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)
	assert.Equal(t, "active", turn.Status)
	assert.Equal(t, int32(30), turn.MovementRemainingFt)

	// Get
	got, err := queries.GetTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.Equal(t, turn.ID, got.ID)

	// Get active turn
	active, err := queries.GetActiveTurnByEncounterID(ctx, enc.ID)
	require.NoError(t, err)
	assert.Equal(t, turn.ID, active.ID)

	// List turns by round
	turns, err := queries.ListTurnsByEncounterAndRound(ctx, refdata.ListTurnsByEncounterAndRoundParams{
		EncounterID: enc.ID,
		RoundNumber: 1,
	})
	require.NoError(t, err)
	assert.Len(t, turns, 1)

	// Complete turn
	completed, err := queries.CompleteTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", completed.Status)
	assert.True(t, completed.CompletedAt.Valid)

	// Update encounter current turn (circular FK with deferred constraint)
	turn2, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         c.ID,
		RoundNumber:         2,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	updatedEnc, err := queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            enc.ID,
		CurrentTurnID: uuid.NullUUID{UUID: turn2.ID, Valid: true},
	})
	require.NoError(t, err)
	assert.Equal(t, turn2.ID, updatedEnc.CurrentTurnID.UUID)
}

// --- Integration TDD Cycle 5: ActionLog CRUD round-trip ---

func TestIntegration_ActionLogCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "active",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "G1",
		DisplayName: "Goblin 1",
		HpMax:       7,
		HpCurrent:   7,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         c.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	// Create action log
	logEntry, err := queries.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      turn.ID,
		EncounterID: enc.ID,
		ActionType:  "attack",
		ActorID:     c.ID,
		BeforeState: json.RawMessage(`{"hp":7}`),
		AfterState:  json.RawMessage(`{"hp":3}`),
		DiceRolls:   pqtype.NullRawMessage{RawMessage: json.RawMessage(`[{"type":"d20","result":15}]`), Valid: true},
	})
	require.NoError(t, err)
	assert.Equal(t, "attack", logEntry.ActionType)

	// List by encounter
	logs, err := queries.ListActionLogByEncounterID(ctx, enc.ID)
	require.NoError(t, err)
	assert.Len(t, logs, 1)

	// List by turn
	logs, err = queries.ListActionLogByTurnID(ctx, turn.ID)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

// --- Integration TDD Cycle 6: Status check constraint ---

func TestIntegration_EncounterStatusConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, _ := setupTestDB(t)
	campaignID := createTestCampaign(t, db)

	_, err := db.Exec(`INSERT INTO encounters (campaign_id, name, status) VALUES ($1, $2, $3)`,
		campaignID, "Bad", "invalid_status")
	require.Error(t, err, "should reject invalid status")
}

// --- Integration TDD Cycle 7: Combatant with character FK ---

func TestIntegration_CombatantWithCharacter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	charID := createTestCharacter(t, db, campaignID)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "preparing",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HpMax:       45,
		HpCurrent:   45,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		DeathSaves:  pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"successes":0,"failures":0}`), Valid: true},
		PositionCol: "D",
		PositionRow: 5,
		IsAlive:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, charID, c.CharacterID.UUID)
	assert.True(t, c.DeathSaves.Valid)
}

// --- Integration TDD Cycle 8: Cascade delete ---

func TestIntegration_CascadeDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "active",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "G1",
		DisplayName: "Goblin",
		HpMax:       7,
		HpCurrent:   7,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         c.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	_, err = queries.CreateActionLog(ctx, refdata.CreateActionLogParams{
		TurnID:      turn.ID,
		EncounterID: enc.ID,
		ActionType:  "move",
		ActorID:     c.ID,
		BeforeState: json.RawMessage(`{}`),
		AfterState:  json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Clear the current_turn_id before deleting to avoid circular FK issues
	_, err = queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            enc.ID,
		CurrentTurnID: uuid.NullUUID{},
	})
	require.NoError(t, err)

	// Delete encounter should cascade
	err = queries.DeleteEncounter(ctx, enc.ID)
	require.NoError(t, err)

	// Combatants should be gone
	list, err := queries.ListCombatantsByEncounterID(ctx, enc.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- Integration TDD Cycle 9: Turn status constraint ---

func TestIntegration_TurnStatusConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "active",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "G1",
		DisplayName: "Goblin",
		HpMax:       7,
		HpCurrent:   7,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	})
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO turns (encounter_id, combatant_id, round_number, status, movement_remaining_ft) VALUES ($1, $2, $3, $4, $5)`,
		enc.ID, c.ID, 1, "invalid_status", 30)
	require.Error(t, err, "should reject invalid turn status")
}

// --- Integration TDD Cycle 10: Skip turn ---

func TestIntegration_SkipTurn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "active",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "G1",
		DisplayName: "Goblin",
		HpMax:       7,
		HpCurrent:   7,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         c.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	skipped, err := queries.SkipTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.Equal(t, "skipped", skipped.Status)
	assert.True(t, skipped.CompletedAt.Valid)
}

// --- Integration TDD Cycle 11: UpdateTurnActions ---

func TestIntegration_UpdateTurnActions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Test",
		Status:     "active",
	})
	require.NoError(t, err)

	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		ShortID:     "G1",
		DisplayName: "Goblin",
		HpMax:       7,
		HpCurrent:   7,
		Ac:          15,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         c.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    2,
	})
	require.NoError(t, err)

	updated, err := queries.UpdateTurnActions(ctx, refdata.UpdateTurnActionsParams{
		ID:                  turn.ID,
		MovementRemainingFt: 15,
		ActionUsed:          true,
		BonusActionUsed:     false,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(15), updated.MovementRemainingFt)
	assert.True(t, updated.ActionUsed)
	assert.Equal(t, int32(1), updated.AttacksRemaining)
}

// --- Integration TDD Cycle 12: Condition auto-expiration round-trip ---

// testStoreAdapter wraps refdata.Queries to satisfy the combat.Store interface.
type testStoreAdapter struct {
	*refdata.Queries
}

func (a *testStoreAdapter) UpdateCharacterInventory(ctx context.Context, id uuid.UUID, inventory pqtype.NullRawMessage) error {
	return nil // stub for tests
}

func (a *testStoreAdapter) UpdateCombatantRage(ctx context.Context, arg refdata.UpdateCombatantRageParams) (refdata.Combatant, error) {
	return a.Queries.UpdateCombatantRage(ctx, arg)
}

func (a *testStoreAdapter) UpdateCombatantWildShape(ctx context.Context, arg refdata.UpdateCombatantWildShapeParams) (refdata.Combatant, error) {
	return a.Queries.UpdateCombatantWildShape(ctx, arg)
}

func (a *testStoreAdapter) GetArmor(ctx context.Context, id string) (refdata.Armor, error) {
	return a.Queries.GetArmor(ctx, id)
}

func (a *testStoreAdapter) UpdateCharacterFeatureUses(ctx context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
	return a.Queries.UpdateCharacterFeatureUses(ctx, arg)
}

func (a *testStoreAdapter) UpdateCharacterSpellSlots(ctx context.Context, arg refdata.UpdateCharacterSpellSlotsParams) (refdata.Character, error) {
	return a.Queries.UpdateCharacterSpellSlots(ctx, arg)
}

func TestIntegration_ConditionAutoExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	charID := createTestCharacter(t, db, campaignID)
	createTestCreature(t, db)

	svc := combat.NewService(&testStoreAdapter{queries})

	// Create encounter
	enc, err := svc.CreateEncounter(ctx, combat.CreateEncounterInput{
		CampaignID: campaignID,
		Name:       "Condition Expiration Test",
	})
	require.NoError(t, err)

	// Add a PC combatant (the source of the condition)
	source, err := svc.AddCombatant(ctx, enc.ID, combat.CombatantParams{
		CharacterID: charID.String(),
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HPMax:       45,
		HPCurrent:   45,
		AC:          18,
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		IsAlive:     true,
		IsNPC:       false,
	})
	require.NoError(t, err)

	// Add an NPC combatant (the target of the condition)
	target, err := svc.AddCombatant(ctx, enc.ID, combat.CombatantParams{
		CreatureRefID: "goblin",
		ShortID:       "G1",
		DisplayName:   "Goblin",
		HPMax:         7,
		HPCurrent:     7,
		AC:            15,
		PositionCol:   "C",
		PositionRow:   3,
		IsVisible:     true,
		IsAlive:       true,
		IsNPC:         true,
	})
	require.NoError(t, err)

	// Apply a condition with duration 2 rounds, started at round 1, expires at start of source's turn
	cond := combat.CombatCondition{
		Condition:         "frightened",
		DurationRounds:    2,
		StartedRound:      1,
		SourceCombatantID: source.ID.String(),
		ExpiresOn:         "start_of_turn",
	}
	updatedTarget, msgs, err := svc.ApplyCondition(ctx, target.ID, cond)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.True(t, combat.HasCondition(updatedTarget.Conditions, "frightened"))

	// Verify condition persisted in DB
	fetched, err := queries.GetCombatant(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, combat.HasCondition(fetched.Conditions, "frightened"))

	// Set up encounter as active at round 3 (condition should expire: started_round 1 + duration 2 = round 3)
	_, err = queries.UpdateEncounterStatus(ctx, refdata.UpdateEncounterStatusParams{
		ID:     enc.ID,
		Status: "active",
	})
	require.NoError(t, err)
	_, err = queries.UpdateEncounterRound(ctx, refdata.UpdateEncounterRoundParams{
		ID:          enc.ID,
		RoundNumber: 3,
	})
	require.NoError(t, err)

	// Create a turn for the source combatant
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         source.ID,
		RoundNumber:         3,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	// Process turn start with logging — should expire the condition
	expMsgs, err := svc.ProcessTurnStartWithLog(ctx, enc.ID, source, 3, turn.ID)
	require.NoError(t, err)
	require.Len(t, expMsgs, 1)
	assert.Contains(t, expMsgs[0], "frightened")
	assert.Contains(t, expMsgs[0], "Goblin")
	assert.Contains(t, expMsgs[0], "Aragorn")
	assert.Contains(t, expMsgs[0], "placed by")

	// Verify condition was removed from DB
	fetched, err = queries.GetCombatant(ctx, target.ID)
	require.NoError(t, err)
	assert.False(t, combat.HasCondition(fetched.Conditions, "frightened"))

	// Verify action_log was persisted
	logs, err := queries.ListActionLogByTurnID(ctx, turn.ID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "condition_expired", logs[0].ActionType)
	assert.Contains(t, logs[0].Description.String, "frightened")
	assert.Contains(t, logs[0].Description.String, "Goblin")
	assert.Contains(t, logs[0].Description.String, "Aragorn")
}

// --- Integration: Bardic Inspiration ---

func createTestBardCharacter(t *testing.T, db *sql.DB, campaignID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	featureUses, _ := json.Marshal(map[string]int{"bardic-inspiration": 3})
	_, err := db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages, feature_uses) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		id, campaignID, "Thorn", "half-elf", `[{"class":"Bard","level":5}]`, 5,
		`{"str":10,"dex":14,"con":12,"int":10,"wis":12,"cha":16}`,
		35, 35, 14, 30, 3, `[{"die":"d8","remaining":5}]`, `{Common}`,
		pqtype.NullRawMessage{RawMessage: featureUses, Valid: true})
	require.NoError(t, err)
	return id
}

// bardicInspirationFixture creates the common test setup shared by all bardic inspiration
// integration tests: a campaign, bard character+combatant, target character+combatant,
// an active turn for the bard, and a service instance.
type bardicInspirationFixture struct {
	queries    *refdata.Queries
	svc        *combat.Service
	bardCharID uuid.UUID
	bard       refdata.Combatant
	target     refdata.Combatant
	turn       refdata.Turn
}

func setupBardicInspirationFixture(t *testing.T) bardicInspirationFixture {
	t.Helper()

	db, queries := setupTestDB(t)
	ctx := context.Background()
	campaignID := createTestCampaign(t, db)
	bardCharID := createTestBardCharacter(t, db, campaignID)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Bardic Inspiration Test",
		Status:     "active",
	})
	require.NoError(t, err)

	bard, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: bardCharID, Valid: true},
		ShortID:     "TH",
		DisplayName: "Thorn",
		HpMax:       35,
		HpCurrent:   35,
		Ac:          14,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	targetCharID := createTestCharacter(t, db, campaignID)
	target, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: targetCharID, Valid: true},
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HpMax:       45,
		HpCurrent:   45,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "B",
		PositionRow: 1,
		IsVisible:   true,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID:         enc.ID,
		CombatantID:         bard.ID,
		RoundNumber:         1,
		Status:              "active",
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	})
	require.NoError(t, err)

	return bardicInspirationFixture{
		queries:    queries,
		svc:        combat.NewService(&testStoreAdapter{queries}),
		bardCharID: bardCharID,
		bard:       bard,
		target:     target,
		turn:       turn,
	}
}

func TestIntegration_BardicInspirationGrantFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupBardicInspirationFixture(t)
	ctx := context.Background()

	fixedNow := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	result, err := f.svc.GrantBardicInspiration(ctx, combat.BardicInspirationCommand{
		Bard:   f.bard,
		Target: f.target,
		Turn:   f.turn,
		Now:    fixedNow,
	})
	require.NoError(t, err)

	assert.Equal(t, "d8", result.Die) // level 5 bard => d8
	assert.Equal(t, 2, result.UsesLeft)
	assert.Contains(t, result.CombatLog, "Thorn")
	assert.Contains(t, result.CombatLog, "Aragorn")
	assert.Contains(t, result.CombatLog, "d8")
	assert.Contains(t, result.Notification, "d8")

	// Verify target combatant in DB has inspiration
	fetchedTarget, err := f.queries.GetCombatant(ctx, f.target.ID)
	require.NoError(t, err)
	assert.True(t, combat.CombatantHasBardicInspiration(fetchedTarget))
	assert.Equal(t, "d8", fetchedTarget.BardicInspirationDie.String)
	assert.Equal(t, "Thorn", fetchedTarget.BardicInspirationSource.String)
	assert.True(t, fetchedTarget.BardicInspirationGrantedAt.Valid)
	assert.Equal(t, fixedNow.Unix(), fetchedTarget.BardicInspirationGrantedAt.Time.Unix())

	// Verify bard's feature_uses decremented
	fetchedChar, err := f.queries.GetCharacter(ctx, f.bardCharID)
	require.NoError(t, err)
	var featureUses map[string]int
	err = json.Unmarshal(fetchedChar.FeatureUses.RawMessage, &featureUses)
	require.NoError(t, err)
	assert.Equal(t, 2, featureUses["bardic-inspiration"])

	// Verify turn had bonus action consumed
	fetchedTurn, err := f.queries.GetTurn(ctx, f.turn.ID)
	require.NoError(t, err)
	assert.True(t, fetchedTurn.BonusActionUsed)
}

func TestIntegration_BardicInspirationUseFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupBardicInspirationFixture(t)
	ctx := context.Background()

	// Grant inspiration first
	fixedNow := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	grantResult, err := f.svc.GrantBardicInspiration(ctx, combat.BardicInspirationCommand{
		Bard:   f.bard,
		Target: f.target,
		Turn:   f.turn,
		Now:    fixedNow,
	})
	require.NoError(t, err)
	assert.Equal(t, "d8", grantResult.Die)

	// Re-fetch target from DB (it now has inspiration)
	inspiredTarget, err := f.queries.GetCombatant(ctx, f.target.ID)
	require.NoError(t, err)
	require.True(t, combat.CombatantHasBardicInspiration(inspiredTarget))

	// Use Bardic Inspiration with a deterministic roller (always returns 5)
	roller := dice.NewRoller(func(max int) int { return 5 })
	useResult, err := f.svc.UseBardicInspiration(ctx, combat.UseBardicInspirationCommand{
		Combatant:     inspiredTarget,
		OriginalTotal: 12,
	}, roller)
	require.NoError(t, err)

	assert.Equal(t, 5, useResult.DieResult)
	assert.Equal(t, 17, useResult.NewTotal) // 12 + 5
	assert.Equal(t, "d8", useResult.Die)
	assert.Contains(t, useResult.CombatLog, "Aragorn")
	assert.Contains(t, useResult.CombatLog, "+5")
	assert.Contains(t, useResult.CombatLog, "d8")

	// Verify inspiration was cleared from DB
	fetchedTarget, err := f.queries.GetCombatant(ctx, f.target.ID)
	require.NoError(t, err)
	assert.False(t, combat.CombatantHasBardicInspiration(fetchedTarget))
	assert.False(t, fetchedTarget.BardicInspirationDie.Valid)
	assert.False(t, fetchedTarget.BardicInspirationSource.Valid)
	assert.False(t, fetchedTarget.BardicInspirationGrantedAt.Valid)
}

func TestIntegration_BardicInspirationExpirationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	f := setupBardicInspirationFixture(t)
	ctx := context.Background()

	// Grant inspiration with a timestamp 11 minutes ago
	elevenMinutesAgo := time.Now().Add(-11 * time.Minute)
	_, err := f.svc.GrantBardicInspiration(ctx, combat.BardicInspirationCommand{
		Bard:   f.bard,
		Target: f.target,
		Turn:   f.turn,
		Now:    elevenMinutesAgo,
	})
	require.NoError(t, err)

	// Verify target has inspiration in DB
	inspiredTarget, err := f.queries.GetCombatant(ctx, f.target.ID)
	require.NoError(t, err)
	require.True(t, combat.CombatantHasBardicInspiration(inspiredTarget))

	// Verify expiration is detected
	assert.True(t, combat.IsBardicInspirationExpired(inspiredTarget.BardicInspirationGrantedAt.Time, time.Now()))

	// Simulate clearing expired inspiration (what the system would do when detected)
	cleared := combat.ClearBardicInspirationFromCombatant(inspiredTarget)
	_, err = f.queries.UpdateCombatantBardicInspiration(ctx, refdata.UpdateCombatantBardicInspirationParams{
		ID:                         cleared.ID,
		BardicInspirationDie:       cleared.BardicInspirationDie,
		BardicInspirationSource:    cleared.BardicInspirationSource,
		BardicInspirationGrantedAt: cleared.BardicInspirationGrantedAt,
	})
	require.NoError(t, err)

	// Verify inspiration was cleared from DB
	fetchedTarget, err := f.queries.GetCombatant(ctx, f.target.ID)
	require.NoError(t, err)
	assert.False(t, combat.CombatantHasBardicInspiration(fetchedTarget))
	assert.False(t, fetchedTarget.BardicInspirationDie.Valid)
	assert.False(t, fetchedTarget.BardicInspirationSource.Valid)
	assert.False(t, fetchedTarget.BardicInspirationGrantedAt.Valid)
}
