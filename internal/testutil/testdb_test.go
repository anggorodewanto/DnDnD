package testutil

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNewTestDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := NewTestDB(t)

	// Verify the connection works
	var result int
	if err := db.QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("failed to query test database: %v", err)
	}
	if result != 1 {
		t.Fatalf("expected 1, got %d", result)
	}
}

func TestNewTestDBConnString(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	connStr := NewTestDBConnString(t)
	if connStr == "" {
		t.Fatal("expected non-empty connection string")
	}

	// Verify we can connect using the returned string
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}
}
