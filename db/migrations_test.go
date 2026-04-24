package db

import (
	"strings"
	"testing"
)

// TestPhase112Migration_ActionLogAllowsErrorNulls asserts the Phase 112
// migration makes turn_id, encounter_id, and actor_id nullable on action_log
// so errors recorded outside of combat (e.g. /register, /import, dashboard
// requests) can be stored with action_type='error' per the spec.
func TestPhase112Migration_ActionLogAllowsErrorNulls(t *testing.T) {
	const name = "20260424120001_action_log_allow_error_nulls.sql"
	b, err := Migrations.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	body := string(b)
	required := []string{
		"ALTER TABLE action_log ALTER COLUMN turn_id DROP NOT NULL",
		"ALTER TABLE action_log ALTER COLUMN encounter_id DROP NOT NULL",
		"ALTER TABLE action_log ALTER COLUMN actor_id DROP NOT NULL",
		"-- +goose Up",
		"-- +goose Down",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Errorf("migration %s missing %q", name, want)
		}
	}
}

// TestPhase112Migration_DownRestoresNotNull asserts the down migration
// restores the NOT NULL constraints (after clearing error rows) so the
// migration is reversible.
func TestPhase112Migration_DownRestoresNotNull(t *testing.T) {
	const name = "20260424120001_action_log_allow_error_nulls.sql"
	b, err := Migrations.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	body := string(b)
	required := []string{
		"DELETE FROM action_log WHERE action_type = 'error'",
		"ALTER TABLE action_log ALTER COLUMN turn_id SET NOT NULL",
		"ALTER TABLE action_log ALTER COLUMN encounter_id SET NOT NULL",
		"ALTER TABLE action_log ALTER COLUMN actor_id SET NOT NULL",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Errorf("migration %s missing down clause %q", name, want)
		}
	}
}
