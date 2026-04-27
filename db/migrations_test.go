package db

import (
	"strings"
	"testing"
)

// TestPhase119Migration_CreatesErrorLog asserts the Phase 119 migration creates
// the dedicated error_log table (split from action_log per the spec) with the
// expected columns and a created_at index for the dashboard's recent-errors
// panel.
func TestPhase119Migration_CreatesErrorLog(t *testing.T) {
	const name = "20260427120001_create_error_log.sql"
	b, err := Migrations.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	body := string(b)
	required := []string{
		"-- +goose Up",
		"-- +goose Down",
		"CREATE TABLE error_log",
		"command       TEXT NOT NULL",
		"user_id       TEXT",
		"summary       TEXT NOT NULL",
		"error_detail  JSONB",
		"created_at    TIMESTAMPTZ NOT NULL DEFAULT now()",
		"CREATE INDEX error_log_created_at_idx ON error_log (created_at DESC)",
		"DROP INDEX IF EXISTS error_log_created_at_idx",
		"DROP TABLE IF EXISTS error_log",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Errorf("migration %s missing %q", name, want)
		}
	}
}

// TestPhase119_ActionLogRelaxationGone asserts the Phase 112 NOT NULL
// relaxation migration is no longer present — Phase 119 keeps action_log
// combat-only and rewrites pre-production migration history.
func TestPhase119_ActionLogRelaxationGone(t *testing.T) {
	const name = "20260424120001_action_log_allow_error_nulls.sql"
	if _, err := Migrations.ReadFile("migrations/" + name); err == nil {
		t.Fatalf("migration %s should have been deleted in Phase 119 (errors moved to error_log)", name)
	}
}
