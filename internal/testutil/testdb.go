package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPostgresContainer starts a postgres testcontainer and returns it with
// its connection string. Callers are responsible for terminating the container.
func startPostgresContainer(ctx context.Context) (testcontainers.Container, string, error) {
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("get connection string: %w", err)
	}

	return container, connStr, nil
}

// NewTestDBConnString spins up a throwaway PostgreSQL container and returns
// its connection string. The container is automatically terminated when the
// test completes. Use this when the caller manages its own connection.
func NewTestDBConnString(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, connStr, err := startPostgresContainer(ctx)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	return connStr
}

// NewTestDB spins up a throwaway PostgreSQL container and returns
// an *sql.DB connected to it. The container is automatically terminated
// when the test completes.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	connStr := NewTestDBConnString(t)

	db, err := database.Connect(connStr)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// Deprecated: Use SharedTestDB.AcquireDB instead. This function starts a
// separate container per call and is significantly slower.
//
// NewMigratedTestDB spins up a throwaway PostgreSQL container, connects to it,
// and runs all migrations. Returns a ready-to-query *sql.DB.
func NewMigratedTestDB(t *testing.T, migrations fs.FS) *sql.DB {
	t.Helper()

	db := NewTestDB(t)
	if err := database.MigrateUp(db, migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}
	return db
}

// MutableTables are user-data tables that get truncated between tests.
var MutableTables = []string{
	"campaigns",
	"sessions",
	"characters",
	"player_characters",
	"maps",
	"assets",
	"encounter_templates",
	"encounters",
	"combatants",
	"turns",
	"action_log",
	"encounter_zones",
	"reaction_declarations",
	"pending_saves",
	"pending_actions",
	"pending_checks",
	"pending_asi",
	"loot_pool_items",
	"loot_pools",
	"shop_items",
	"shops",
	"portal_tokens",
	"narration_posts",
	"narration_templates",
	"dm_player_messages",
	"dm_queue_items",
	"error_log",
}

// ReferenceTables are seeded with ON CONFLICT DO NOTHING and preserved across tests.
var ReferenceTables = []string{
	"weapons",
	"armor",
	"conditions_ref",
	"classes",
	"races",
	"feats",
	"spells",
	"creatures",
	"magic_items",
}

// homebrewRefdataTables are reference tables that may contain campaign-scoped
// homebrew rows. Those rows must be deleted between tests, but SRD rows
// (campaign_id IS NULL) must be preserved.
var homebrewRefdataTables = []string{
	"creatures",
	"magic_items",
	"spells",
	"weapons",
	"races",
	"feats",
	"classes",
}

// orderedDeleteTables lists mutable user-data tables in dependency-safe
// deletion order. Encounters is deleted before its children (turns,
// combatants, action_log, etc.) because the encounters table has a
// DEFERRABLE FK to turns(current_turn_id) that would otherwise be violated
// mid-statement. The remaining child tables CASCADE from encounters, but
// we still issue explicit DELETEs for them to keep behaviour deterministic.
var orderedDeleteTables = []string{
	"dm_player_messages",
	"portal_tokens",
	"shop_items",
	"shops",
	"loot_pool_items",
	"loot_pools",
	"pending_actions",
	"dm_queue_items",
	"pending_saves",
	"reaction_declarations",
	"encounter_zones",
	"action_log",
	"error_log",
	"encounters",
	"turns",
	"combatants",
	"encounter_templates",
	"player_characters",
	"characters",
	"maps",
	"assets",
	"campaigns",
	"sessions",
}

// TruncateUserTables removes all per-test mutable rows while preserving
// seeded SRD reference data. It deletes campaign-scoped homebrew rows from
// reference tables first (so the campaigns row they reference can be removed),
// then deletes mutable tables in FK-dependency order.
func TruncateUserTables(db *sql.DB) error {
	for _, table := range homebrewRefdataTables {
		if _, err := db.Exec("DELETE FROM " + table + " WHERE campaign_id IS NOT NULL"); err != nil {
			return fmt.Errorf("delete homebrew %s: %w", table, err)
		}
	}
	for _, table := range orderedDeleteTables {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}
	return nil
}

// SharedTestDB manages a single PostgreSQL container shared across all tests
// in a package. The container is lazily started on the first AcquireDB call.
type SharedTestDB struct {
	db         *sql.DB
	container  testcontainers.Container
	migrations fs.FS
	once       sync.Once
	initErr    error
}

// NewSharedTestDB creates a SharedTestDB that will lazily start a container
// using the given migrations FS. Call Teardown() in TestMain after m.Run().
func NewSharedTestDB(migrations fs.FS) *SharedTestDB {
	return &SharedTestDB{migrations: migrations}
}

// AcquireDB returns a *sql.DB connected to the shared container. On the first
// call it lazily starts the container and runs migrations. On subsequent calls
// it truncates mutable tables so each test gets a clean slate.
func (s *SharedTestDB) AcquireDB(t *testing.T) *sql.DB {
	t.Helper()

	s.once.Do(func() {
		s.initErr = s.start()
	})
	if s.initErr != nil {
		t.Fatalf("SharedTestDB init failed: %v", s.initErr)
	}

	if err := TruncateUserTables(s.db); err != nil {
		t.Fatalf("TruncateUserTables failed: %v", err)
	}

	return s.db
}

func (s *SharedTestDB) start() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, connStr, err := startPostgresContainer(ctx)
	if err != nil {
		return err
	}
	s.container = container

	db, err := database.Connect(connStr)
	if err != nil {
		return fmt.Errorf("connect to test database: %w", err)
	}
	s.db = db

	if err := database.MigrateUp(db, s.migrations); err != nil {
		return fmt.Errorf("MigrateUp: %w", err)
	}

	return nil
}

// Teardown terminates the shared container and closes the DB connection.
func (s *SharedTestDB) Teardown() {
	if s.db != nil {
		s.db.Close()
	}
	if s.container != nil {
		s.container.Terminate(context.Background()) //nolint:errcheck
	}
}
