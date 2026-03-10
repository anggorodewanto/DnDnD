package testutil

import (
	"testing"
)

func TestNewTestDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := NewTestDB(t)

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
}
