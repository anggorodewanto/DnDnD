package database

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrateUp runs all pending migrations in the given directory.
func MigrateUp(db *sql.DB, migrationsDir string) error {
	if db == nil {
		return fmt.Errorf("database connection must not be nil")
	}

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("running migrations up: %w", err)
	}

	return nil
}

// MigrateDown rolls back the most recent migration.
func MigrateDown(db *sql.DB, migrationsDir string) error {
	if db == nil {
		return fmt.Errorf("database connection must not be nil")
	}

	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Down(db, migrationsDir); err != nil {
		return fmt.Errorf("running migration down: %w", err)
	}

	return nil
}
