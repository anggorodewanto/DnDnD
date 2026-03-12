package database_test

import (
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/testutil"
)

func TestIntegration_ConnectAndMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)

	// Run migrations up
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Verify campaigns table exists by inserting a row
	_, err := db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4)`,
		"guild-123", "user-456", "Test Campaign", "active")
	if err != nil {
		t.Fatalf("failed to insert into campaigns: %v", err)
	}

	// Verify sessions table exists (UUID PK is auto-generated)
	_, err = db.Exec(`INSERT INTO sessions (discord_user_id, access_token, refresh_token) VALUES ($1, $2, $3)`,
		"user-456", "access-tok", "refresh-tok")
	if err != nil {
		t.Fatalf("failed to insert into sessions: %v", err)
	}

	// Verify campaigns row
	var name string
	if err := db.QueryRow(`SELECT name FROM campaigns WHERE guild_id = $1`, "guild-123").Scan(&name); err != nil {
		t.Fatalf("failed to query campaigns: %v", err)
	}
	if name != "Test Campaign" {
		t.Fatalf("expected name 'Test Campaign', got %q", name)
	}

	// Verify sessions row has UUID id
	var id, discordUserID string
	if err := db.QueryRow(`SELECT id::text, discord_user_id FROM sessions WHERE discord_user_id = $1`, "user-456").Scan(&id, &discordUserID); err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty UUID id for session")
	}
	if discordUserID != "user-456" {
		t.Fatalf("expected discord_user_id 'user-456', got %q", discordUserID)
	}
}

func TestIntegration_MigrateDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)

	// Run migrations up
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Roll back the encounters migration (most recent)
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (encounters) failed: %v", err)
	}

	// Roll back the encounter_templates migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (encounter_templates) failed: %v", err)
	}

	// Roll back the assets migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (assets) failed: %v", err)
	}

	// Roll back the maps migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (maps) failed: %v", err)
	}

	// Roll back the character_card_message_id migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (character_card_message_id) failed: %v", err)
	}

	// Roll back the player_characters migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (player_characters) failed: %v", err)
	}

	// Roll back the characters migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (characters) failed: %v", err)
	}

	// Roll back the creatures/magic_items migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (creatures/magic_items) failed: %v", err)
	}

	// Roll back the spells migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (spells) failed: %v", err)
	}

	// Roll back the classes/races/feats migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (classes/races/feats) failed: %v", err)
	}

	// Roll back the reference tables migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (reference tables) failed: %v", err)
	}

	// Roll back the sessions migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateDown (sessions) failed: %v", err)
	}

	// Sessions table should no longer exist
	_, err := db.Exec(`INSERT INTO sessions (discord_user_id, access_token, refresh_token) VALUES ($1, $2, $3)`,
		"user-456", "access-tok", "refresh-tok")
	if err == nil {
		t.Fatal("expected error inserting into dropped sessions table, got nil")
	}

	// Campaigns table should still exist
	_, err = db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4)`,
		"guild-123", "user-456", "Test Campaign", "active")
	if err != nil {
		t.Fatalf("campaigns table should still exist after rolling back only sessions: %v", err)
	}

	// Roll back campaigns migration
	if err := database.MigrateDown(db, dbfs.Migrations); err != nil {
		t.Fatalf("second MigrateDown failed: %v", err)
	}

	// Campaigns table should no longer exist
	_, err = db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4)`,
		"guild-999", "user-999", "Should Fail", "active")
	if err == nil {
		t.Fatal("expected error inserting into dropped campaigns table, got nil")
	}
}

func TestIntegration_CampaignsTableConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)

	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Insert first campaign
	_, err := db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4)`,
		"guild-unique", "user-1", "Campaign One", "active")
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	// guild_id should be unique — duplicate insert should fail
	_, err = db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4)`,
		"guild-unique", "user-2", "Campaign Two", "active")
	if err == nil {
		t.Fatal("expected unique constraint violation for duplicate guild_id, got nil")
	}

	// Verify UUID default works
	var id string
	if err := db.QueryRow(`SELECT id FROM campaigns WHERE guild_id = $1`, "guild-unique").Scan(&id); err != nil {
		t.Fatalf("failed to query id: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty UUID, got empty string")
	}

	// Verify timestamps are set
	var createdAt, updatedAt string
	if err := db.QueryRow(`SELECT created_at::text, updated_at::text FROM campaigns WHERE guild_id = $1`, "guild-unique").Scan(&createdAt, &updatedAt); err != nil {
		t.Fatalf("failed to query timestamps: %v", err)
	}
	if createdAt == "" || updatedAt == "" {
		t.Fatal("expected non-empty timestamps")
	}
}

func TestIntegration_SessionsTableDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)

	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Insert a session — UUID PK is auto-generated
	var sessionID string
	err := db.QueryRow(`INSERT INTO sessions (discord_user_id, access_token, refresh_token) VALUES ($1, $2, $3) RETURNING id::text`,
		"user-1", "at", "rt").Scan(&sessionID)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if sessionID == "" {
		t.Fatal("expected non-empty session UUID")
	}

	// Verify all timestamp defaults
	var createdAt, updatedAt, expiresAt string
	if err := db.QueryRow(`SELECT created_at::text, updated_at::text, expires_at::text FROM sessions WHERE id = $1::uuid`, sessionID).Scan(&createdAt, &updatedAt, &expiresAt); err != nil {
		t.Fatalf("failed to query timestamps: %v", err)
	}
	if createdAt == "" || updatedAt == "" || expiresAt == "" {
		t.Fatal("expected non-empty default timestamps")
	}
}
