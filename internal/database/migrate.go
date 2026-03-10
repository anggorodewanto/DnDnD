package database

import (
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

func prepareGoose(db *sql.DB, migrationsFS fs.FS) error {
	if db == nil {
		return fmt.Errorf("database connection must not be nil")
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	return nil
}

// MigrateUp runs all pending migrations using the provided filesystem.
func MigrateUp(db *sql.DB, migrationsFS fs.FS) error {
	if err := prepareGoose(db, migrationsFS); err != nil {
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations up: %w", err)
	}

	return nil
}

// MigrateDown rolls back the most recent migration using the provided filesystem.
func MigrateDown(db *sql.DB, migrationsFS fs.FS) error {
	if err := prepareGoose(db, migrationsFS); err != nil {
		return err
	}

	if err := goose.Down(db, "migrations"); err != nil {
		return fmt.Errorf("running migration down: %w", err)
	}

	return nil
}
