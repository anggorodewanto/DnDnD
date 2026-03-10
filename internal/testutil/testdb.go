package testutil

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestDBConnString spins up a throwaway PostgreSQL container and returns
// its connection string. The container is automatically terminated when the
// test completes. Use this when the caller manages its own connection.
func NewTestDBConnString(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

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
