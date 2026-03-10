package database

import (
	"database/sql"
	"testing"
)

func TestConnect_InvalidDSN(t *testing.T) {
	db, err := Connect("postgres://invalid:5432/nonexistent?connect_timeout=1")
	if err == nil {
		db.Close()
		t.Fatal("expected error for invalid DSN, got nil")
	}
}

func TestConnect_EmptyDSN(t *testing.T) {
	_, err := Connect("")
	if err == nil {
		t.Fatal("expected error for empty DSN, got nil")
	}
}

func TestMigrateUp_NilDB(t *testing.T) {
	err := MigrateUp(nil, "db/migrations")
	if err == nil {
		t.Fatal("expected error for nil db, got nil")
	}
}

func TestMigrateUp_BadDir(t *testing.T) {
	// Use an in-memory-like approach — we need a real DB for goose, but we can test the error path
	db, _ := sql.Open("pgx", "postgres://invalid:5432/nonexistent")
	defer db.Close()
	err := MigrateUp(db, "/nonexistent/migrations/dir")
	if err == nil {
		t.Fatal("expected error for bad migration dir, got nil")
	}
}

func TestMigrateDown_NilDB(t *testing.T) {
	err := MigrateDown(nil, "db/migrations")
	if err == nil {
		t.Fatal("expected error for nil db, got nil")
	}
}

func TestMigrateDown_BadDir(t *testing.T) {
	db, _ := sql.Open("pgx", "postgres://invalid:5432/nonexistent")
	defer db.Close()
	err := MigrateDown(db, "/nonexistent/migrations/dir")
	if err == nil {
		t.Fatal("expected error for bad migration dir, got nil")
	}
}
