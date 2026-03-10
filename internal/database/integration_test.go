//go:build integration

package database_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/testutil"
)

func projectRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestIntegration_ConnectAndMigrate(t *testing.T) {
	db := testutil.NewTestDB(t)

	migrationsDir := filepath.Join(projectRoot(), "db", "migrations")

	// Run migrations up
	if err := database.MigrateUp(db, migrationsDir); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Verify campaigns table exists by inserting a row
	_, err := db.Exec(`INSERT INTO campaigns (guild_id, dm_user_id, name, status) VALUES ($1, $2, $3, $4)`,
		"guild-123", "user-456", "Test Campaign", "active")
	if err != nil {
		t.Fatalf("failed to insert into campaigns: %v", err)
	}

	// Verify sessions table exists
	_, err = db.Exec(`INSERT INTO sessions (session_id, discord_user_id, access_token, refresh_token) VALUES ($1, $2, $3, $4)`,
		"sess-abc", "user-456", "access-tok", "refresh-tok")
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

	// Verify sessions row
	var discordUserID string
	if err := db.QueryRow(`SELECT discord_user_id FROM sessions WHERE session_id = $1`, "sess-abc").Scan(&discordUserID); err != nil {
		t.Fatalf("failed to query sessions: %v", err)
	}
	if discordUserID != "user-456" {
		t.Fatalf("expected discord_user_id 'user-456', got %q", discordUserID)
	}
}

func TestIntegration_MigrateDown(t *testing.T) {
	db := testutil.NewTestDB(t)

	migrationsDir := filepath.Join(projectRoot(), "db", "migrations")

	// Run migrations up
	if err := database.MigrateUp(db, migrationsDir); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Roll back the sessions migration (most recent)
	if err := database.MigrateDown(db, migrationsDir); err != nil {
		t.Fatalf("MigrateDown failed: %v", err)
	}

	// Sessions table should no longer exist
	_, err := db.Exec(`INSERT INTO sessions (session_id, discord_user_id, access_token, refresh_token) VALUES ($1, $2, $3, $4)`,
		"sess-abc", "user-456", "access-tok", "refresh-tok")
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
	if err := database.MigrateDown(db, migrationsDir); err != nil {
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
	db := testutil.NewTestDB(t)

	migrationsDir := filepath.Join(projectRoot(), "db", "migrations")

	if err := database.MigrateUp(db, migrationsDir); err != nil {
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
	db := testutil.NewTestDB(t)

	migrationsDir := filepath.Join(projectRoot(), "db", "migrations")

	if err := database.MigrateUp(db, migrationsDir); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	// Insert a session and verify defaults
	_, err := db.Exec(`INSERT INTO sessions (session_id, discord_user_id, access_token, refresh_token) VALUES ($1, $2, $3, $4)`,
		"sess-defaults", "user-1", "at", "rt")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var createdAt, expiresAt string
	if err := db.QueryRow(`SELECT created_at::text, expires_at::text FROM sessions WHERE session_id = $1`, "sess-defaults").Scan(&createdAt, &expiresAt); err != nil {
		t.Fatalf("failed to query timestamps: %v", err)
	}
	if createdAt == "" || expiresAt == "" {
		t.Fatal("expected non-empty default timestamps")
	}
}
