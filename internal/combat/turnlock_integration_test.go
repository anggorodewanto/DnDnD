package combat_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTurnLockTestDB sets up a test DB with a campaign, encounter, combatant, and active turn.
// Returns the db, queries, encounter_id, turn_id, campaign_id, and combatant_id.
type turnLockTestData struct {
	DB          *sql.DB
	Queries     *refdata.Queries
	CampaignID  uuid.UUID
	EncounterID uuid.UUID
	CombatantID uuid.UUID
	TurnID      uuid.UUID
	CharacterID uuid.UUID
	DMUserID    string
	PlayerUID   string
}

func setupTurnLockTest(t *testing.T) turnLockTestData {
	t.Helper()
	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	queries := refdata.New(db)
	ctx := context.Background()

	dmUserID := "dm-user-123"
	playerUID := "player-user-456"

	// Create campaign with known DM
	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "guild-1", dmUserID, "Test Campaign", "active")
	require.NoError(t, err)

	// Create character
	charID := uuid.New()
	_, err = db.Exec(`INSERT INTO characters (id, campaign_id, name, race, classes, level, ability_scores, hp_max, hp_current, ac, speed_ft, proficiency_bonus, hit_dice_remaining, languages) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		charID, campaignID, "Aragorn", "human", `[{"class":"fighter","level":5}]`, 5,
		`{"str":16,"dex":14,"con":14,"int":10,"wis":12,"cha":10}`,
		45, 45, 18, 30, 3, `[{"die":"d10","remaining":5}]`, `{Common}`)
	require.NoError(t, err)

	// Create player_character linking character to player's discord user
	_, err = db.Exec(`INSERT INTO player_characters (campaign_id, character_id, discord_user_id, status, created_via) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, charID, playerUID, "approved", "register")
	require.NoError(t, err)

	// Create encounter
	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "Lock Test Encounter",
		Status:     "active",
	})
	require.NoError(t, err)

	// Create combatant (PC)
	c, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
		EncounterID: enc.ID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		ShortID:     "AR",
		DisplayName: "Aragorn",
		HpMax:       45,
		HpCurrent:   45,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsNpc:       false,
	})
	require.NoError(t, err)

	// Create active turn
	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID: enc.ID,
		CombatantID: c.ID,
		RoundNumber: 1,
		Status:      "active",
	})
	require.NoError(t, err)

	// Set current turn on encounter
	_, err = queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            enc.ID,
		CurrentTurnID: uuid.NullUUID{UUID: turn.ID, Valid: true},
	})
	require.NoError(t, err)

	return turnLockTestData{
		DB:          db,
		Queries:     queries,
		CampaignID:  campaignID,
		EncounterID: enc.ID,
		CombatantID: c.ID,
		TurnID:      turn.ID,
		CharacterID: charID,
		DMUserID:    dmUserID,
		PlayerUID:   playerUID,
	}
}

// --- TDD Cycle 2: Advisory lock serializes concurrent operations ---

func TestIntegration_TurnLock_Serialization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	// Track execution order to verify serialization
	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tx, err := combat.AcquireTurnLock(ctx, td.DB, td.TurnID)
			require.NoError(t, err)

			// Record order of lock acquisition
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()

			// Hold the lock briefly
			time.Sleep(50 * time.Millisecond)

			err = tx.Commit()
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Len(t, order, 3, "all three goroutines should have acquired the lock")
}

// --- TDD Cycle 3: Rapid command queueing (second blocks, then proceeds) ---

func TestIntegration_TurnLock_RapidQueueing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	// First goroutine acquires the lock
	tx1, err := combat.AcquireTurnLock(ctx, td.DB, td.TurnID)
	require.NoError(t, err)

	var secondAcquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		tx2, err := combat.AcquireTurnLock(ctx, td.DB, td.TurnID)
		require.NoError(t, err)
		secondAcquired.Store(true)
		tx2.Commit()
	}()

	// Give goroutine time to start blocking
	time.Sleep(100 * time.Millisecond)
	assert.False(t, secondAcquired.Load(), "second goroutine should be blocked")

	// Release first lock
	tx1.Commit()
	wg.Wait()
	assert.True(t, secondAcquired.Load(), "second goroutine should have acquired lock after first released")
}

// --- TDD Cycle 4: Lock timeout ---

func TestIntegration_TurnLock_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	// Hold a lock using raw SQL so it persists
	holdTx, err := td.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer holdTx.Rollback()

	lockKey := int64(0)
	// Compute same key as AcquireTurnLock would
	idBytes := td.TurnID[:]
	lockKey = int64(uint64(idBytes[0])<<56 | uint64(idBytes[1])<<48 | uint64(idBytes[2])<<40 |
		uint64(idBytes[3])<<32 | uint64(idBytes[4])<<24 | uint64(idBytes[5])<<16 |
		uint64(idBytes[6])<<8 | uint64(idBytes[7]))
	_, err = holdTx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey)
	require.NoError(t, err)

	// Now try to acquire via AcquireTurnLock — should timeout
	start := time.Now()
	_, err = combat.AcquireTurnLock(ctx, td.DB, td.TurnID)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, combat.ErrLockTimeout)
	assert.GreaterOrEqual(t, elapsed, 4*time.Second, "should wait close to 5s before timing out")
	assert.Less(t, elapsed, 10*time.Second, "should not wait much longer than 5s")
}

// --- TDD Cycle 5: Out-of-turn rejection ---

func TestIntegration_TurnValidation_WrongUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	wrongUserID := "wrong-user-789"
	_, err := combat.ValidateTurnOwnership(ctx, td.DB, td.Queries, td.EncounterID, wrongUserID)
	require.Error(t, err)

	var notYourTurn *combat.ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn), "should return ErrNotYourTurn")
	assert.Contains(t, notYourTurn.Error(), "Aragorn")
	assert.Contains(t, notYourTurn.Error(), td.PlayerUID)
}

// --- TDD Cycle 6: Correct user passes validation ---

func TestIntegration_TurnValidation_CorrectUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	info, err := combat.ValidateTurnOwnership(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID)
	require.NoError(t, err)
	assert.Equal(t, td.TurnID, info.TurnID)
	assert.Equal(t, td.CombatantID, info.CombatantID)
	assert.Equal(t, td.PlayerUID, info.OwnerUserID)
	assert.Equal(t, td.DMUserID, info.DMUserID)
	assert.False(t, info.IsNPC)
}

// --- TDD Cycle 7: DM can always act ---

func TestIntegration_TurnValidation_DMBypass(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	info, err := combat.ValidateTurnOwnership(ctx, td.DB, td.Queries, td.EncounterID, td.DMUserID)
	require.NoError(t, err)
	assert.Equal(t, td.TurnID, info.TurnID)
	assert.Equal(t, td.DMUserID, info.DMUserID)
}

// --- TDD Cycle 8: DM acting on NPC turn ---

func TestIntegration_TurnValidation_NPC_DMOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	queries := refdata.New(db)
	ctx := context.Background()

	dmUserID := "dm-user-npc"
	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "guild-2", dmUserID, "NPC Test Campaign", "active")
	require.NoError(t, err)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "NPC Test",
		Status:     "active",
	})
	require.NoError(t, err)

	// NPC combatant (no character_id)
	npc, err := queries.CreateCombatant(ctx, refdata.CreateCombatantParams{
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
		IsNpc:       true,
	})
	require.NoError(t, err)

	turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
		EncounterID: enc.ID,
		CombatantID: npc.ID,
		RoundNumber: 1,
		Status:      "active",
	})
	require.NoError(t, err)

	_, err = queries.UpdateEncounterCurrentTurn(ctx, refdata.UpdateEncounterCurrentTurnParams{
		ID:            enc.ID,
		CurrentTurnID: uuid.NullUUID{UUID: turn.ID, Valid: true},
	})
	require.NoError(t, err)

	// DM should be allowed
	info, err := combat.ValidateTurnOwnership(ctx, db, queries, enc.ID, dmUserID)
	require.NoError(t, err)
	assert.True(t, info.IsNPC)

	// Player should be rejected
	_, err = combat.ValidateTurnOwnership(ctx, db, queries, enc.ID, "some-player")
	require.Error(t, err)
	var notYourTurn *combat.ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn))
}

// --- TDD Cycle 9: Different encounters don't block each other ---

func TestIntegration_TurnLock_DifferentEncountersDontBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	queries := refdata.New(db)
	ctx := context.Background()

	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "guild-1", "dm-1", "Multi Encounter Campaign", "active")
	require.NoError(t, err)

	// Create two encounters with separate turns
	var turnIDs [2]uuid.UUID
	for i := 0; i < 2; i++ {
		enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
			CampaignID: campaignID,
			Name:       "Encounter",
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
			IsNpc:       true,
		})
		require.NoError(t, err)

		turn, err := queries.CreateTurn(ctx, refdata.CreateTurnParams{
			EncounterID: enc.ID,
			CombatantID: c.ID,
			RoundNumber: 1,
			Status:      "active",
		})
		require.NoError(t, err)
		turnIDs[i] = turn.ID
	}

	// Acquire lock on first encounter
	tx1, err := combat.AcquireTurnLock(ctx, db, turnIDs[0])
	require.NoError(t, err)
	defer tx1.Rollback()

	// Should be able to immediately acquire lock on second encounter
	tx2, err := combat.AcquireTurnLock(ctx, db, turnIDs[1])
	require.NoError(t, err)
	defer tx2.Rollback()

	// Both acquired — different encounters don't block
	tx1.Commit()
	tx2.Commit()
}

// --- TDD Cycle 10: Exception commands bypass validation ---

func TestIsExemptCommand(t *testing.T) {
	assert.True(t, combat.IsExemptCommand("reaction"))
	assert.True(t, combat.IsExemptCommand("check"))
	assert.True(t, combat.IsExemptCommand("save"))
	assert.True(t, combat.IsExemptCommand("rest"))
	assert.False(t, combat.IsExemptCommand("attack"))
	assert.False(t, combat.IsExemptCommand("move"))
	assert.False(t, combat.IsExemptCommand("cast"))
	assert.False(t, combat.IsExemptCommand("done"))
	assert.False(t, combat.IsExemptCommand("deathsave"))
	assert.False(t, combat.IsExemptCommand("bonus"))
	assert.False(t, combat.IsExemptCommand("interact"))
	assert.False(t, combat.IsExemptCommand("action"))
}

// --- TDD Cycle 11: AcquireTurnLockWithValidation combines both ---

func TestIntegration_AcquireTurnLockWithValidation_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	tx, info, err := combat.AcquireTurnLockWithValidation(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID)
	require.NoError(t, err)
	defer tx.Rollback()

	assert.Equal(t, td.TurnID, info.TurnID)
	assert.Equal(t, td.CombatantID, info.CombatantID)

	err = tx.Commit()
	require.NoError(t, err)
}

func TestIntegration_AcquireTurnLockWithValidation_WrongUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	_, _, err := combat.AcquireTurnLockWithValidation(ctx, td.DB, td.Queries, td.EncounterID, "wrong-user")
	require.Error(t, err)

	var notYourTurn *combat.ErrNotYourTurn
	assert.True(t, errors.As(err, &notYourTurn))
}

// --- TDD Cycle 12: DM concurrent access with lock ---

func TestIntegration_TurnLock_DMConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	td := setupTurnLockTest(t)
	ctx := context.Background()

	// Player holds the lock
	tx1, _, err := combat.AcquireTurnLockWithValidation(ctx, td.DB, td.Queries, td.EncounterID, td.PlayerUID)
	require.NoError(t, err)

	var dmAcquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// DM validates (should pass) and then acquires lock (should block)
		tx2, info, err := combat.AcquireTurnLockWithValidation(ctx, td.DB, td.Queries, td.EncounterID, td.DMUserID)
		if err != nil {
			return
		}
		assert.Equal(t, td.DMUserID, info.DMUserID)
		dmAcquired.Store(true)
		tx2.Commit()
	}()

	time.Sleep(100 * time.Millisecond)
	assert.False(t, dmAcquired.Load(), "DM should be blocked by player's lock")

	tx1.Commit()
	wg.Wait()
	assert.True(t, dmAcquired.Load(), "DM should acquire lock after player releases")
}

// --- TDD Cycle 13: No active turn returns error ---

func TestIntegration_TurnValidation_NoActiveTurn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewMigratedTestDB(t, dbfs.Migrations)
	queries := refdata.New(db)
	ctx := context.Background()

	campaignID := uuid.New()
	_, err := db.Exec(`INSERT INTO campaigns (id, guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, "guild-1", "dm-1", "No Turn Campaign", "active")
	require.NoError(t, err)

	enc, err := queries.CreateEncounter(ctx, refdata.CreateEncounterParams{
		CampaignID: campaignID,
		Name:       "No Turn Test",
		Status:     "active",
	})
	require.NoError(t, err)

	_, err = combat.ValidateTurnOwnership(ctx, db, queries, enc.ID, "any-user")
	assert.ErrorIs(t, err, combat.ErrNoActiveTurn)
}
