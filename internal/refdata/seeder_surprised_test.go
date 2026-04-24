package refdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

// capturingDBTX records every ExecContext call's args so tests can
// inspect the (id, name, description, mechanical_effects) tuple that
// seedConditions would pass to UpsertCondition.
type capturingDBTX struct {
	calls [][]any
}

func (m *capturingDBTX) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	// Copy to avoid the caller reusing a slice.
	cp := make([]any, len(args))
	copy(cp, args)
	m.calls = append(m.calls, cp)
	return nil, nil
}

func (m *capturingDBTX) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error) {
	return nil, nil
}
func (m *capturingDBTX) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, nil
}
func (m *capturingDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) *sql.Row {
	return nil
}

// findConditionByID returns the arg tuple for a given condition ID
// (upsertCondition uses $1=id, $2=name, $3=description, $4=effects JSON).
func findConditionByID(calls [][]any, id string) []any {
	for _, c := range calls {
		if len(c) > 0 {
			if s, ok := c[0].(string); ok && s == id {
				return c
			}
		}
	}
	return nil
}

// TestSeedConditions_IncludesSurprised verifies that seedConditions emits
// an UpsertCondition call with id="surprised" so the dashboard and combat
// engine can reference the condition row.
func TestSeedConditions_IncludesSurprised(t *testing.T) {
	mock := &capturingDBTX{}
	q := New(mock)
	if err := seedConditions(context.Background(), q); err != nil {
		t.Fatalf("seedConditions failed: %v", err)
	}

	args := findConditionByID(mock.calls, "surprised")
	if args == nil {
		t.Fatalf("expected seedConditions to include id=surprised; calls=%d", len(mock.calls))
	}
	if len(args) < 4 {
		t.Fatalf("expected 4 args (id,name,description,effects), got %d", len(args))
	}

	name, _ := args[1].(string)
	if name != "Surprised" {
		t.Errorf("expected name=Surprised, got %q", name)
	}

	desc, _ := args[2].(string)
	if !strings.Contains(strings.ToLower(desc), "surprised") {
		t.Errorf("expected description to mention surprise, got %q", desc)
	}

	var effects []MechanicalEffect
	switch v := args[3].(type) {
	case json.RawMessage:
		if err := json.Unmarshal(v, &effects); err != nil {
			t.Fatalf("failed to unmarshal mechanical_effects: %v", err)
		}
	case []byte:
		if err := json.Unmarshal(v, &effects); err != nil {
			t.Fatalf("failed to unmarshal mechanical_effects: %v", err)
		}
	default:
		t.Fatalf("unexpected mechanical_effects type: %T", args[3])
	}

	// Surprised bars actions, bonus actions, reactions, and movement.
	expectedEffects := map[string]bool{
		"cant_take_actions":       false,
		"cant_take_bonus_actions": false,
		"cant_take_reactions":     false,
		"cant_move":               false,
	}
	for _, e := range effects {
		if _, ok := expectedEffects[e.EffectType]; ok {
			expectedEffects[e.EffectType] = true
		}
	}
	for k, found := range expectedEffects {
		if !found {
			t.Errorf("expected mechanical_effect %q to be present on surprised", k)
		}
	}
}

// TestConditionCount_IncludesSurprised pins the seeded-condition count at 16
// after surprised is added (blinded, charmed, deafened, exhaustion,
// frightened, grappled, incapacitated, invisible, paralyzed, petrified,
// poisoned, prone, restrained, stunned, unconscious, surprised).
func TestConditionCount_IncludesSurprised(t *testing.T) {
	if ConditionCount != 16 {
		t.Fatalf("expected ConditionCount=16 (after adding surprised), got %d", ConditionCount)
	}

	mock := &capturingDBTX{}
	q := New(mock)
	if err := seedConditions(context.Background(), q); err != nil {
		t.Fatalf("seedConditions failed: %v", err)
	}
	if len(mock.calls) != ConditionCount {
		t.Fatalf("expected %d upsertCondition calls, got %d", ConditionCount, len(mock.calls))
	}
}
