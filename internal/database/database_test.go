package database

import (
	"database/sql"
	"testing"
	"testing/fstest"
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
	err := MigrateUp(nil, fstest.MapFS{})
	if err == nil {
		t.Fatal("expected error for nil db, got nil")
	}
	if err.Error() != "database connection must not be nil" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestMigrateUp_BadFS(t *testing.T) {
	db, _ := sql.Open("pgx", "postgres://invalid:5432/nonexistent")
	defer db.Close()

	// Use an empty FS — goose should fail because there's no "migrations" dir
	// or it will fail trying to connect to db
	emptyFS := fstest.MapFS{}
	err := MigrateUp(db, emptyFS)
	if err == nil {
		t.Fatal("expected error for bad migration FS, got nil")
	}
}

func TestMigrateDown_NilDB(t *testing.T) {
	err := MigrateDown(nil, fstest.MapFS{})
	if err == nil {
		t.Fatal("expected error for nil db, got nil")
	}
	if err.Error() != "database connection must not be nil" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestMigrateDown_BadFS(t *testing.T) {
	db, _ := sql.Open("pgx", "postgres://invalid:5432/nonexistent")
	defer db.Close()

	emptyFS := fstest.MapFS{}
	err := MigrateDown(db, emptyFS)
	if err == nil {
		t.Fatal("expected error for bad migration FS, got nil")
	}
}
