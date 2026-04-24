package errorlog

import (
	"testing"
	"time"
)

func TestBuildInsertErrorQuery_ShapeAndArgs(t *testing.T) {
	entry := Entry{
		Command:   "cast",
		UserID:    "alice",
		Summary:   "DB timeout on /cast by @alice",
		CreatedAt: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
	}
	q, args := buildInsertErrorQuery(entry)

	for _, want := range []string{
		"INSERT INTO action_log",
		"action_type",
		"description",
		"before_state",
		"after_state",
	} {
		if !containsCI(q, want) {
			t.Errorf("INSERT SQL missing %q — got: %s", want, q)
		}
	}
	// action_type='error', description, and user id (inside before_state
	// JSON) should end up somewhere in args.
	if len(args) == 0 {
		t.Fatal("expected bound args")
	}
	foundError := false
	foundSummary := false
	foundUser := false
	for _, a := range args {
		s, _ := a.(string)
		if s == "error" {
			foundError = true
		}
		if s == entry.Summary {
			foundSummary = true
		}
		if containsCI(s, entry.UserID) {
			foundUser = true
		}
	}
	if !foundError {
		t.Errorf("expected action_type='error' in args, got %v", args)
	}
	if !foundSummary {
		t.Errorf("expected summary in args, got %v", args)
	}
	if !foundUser {
		t.Errorf("expected user id in args (inside JSON), got %v", args)
	}
}

func TestBuildCountSinceQuery_ShapeAndArgs(t *testing.T) {
	since := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	q, args := buildCountSinceQuery(since)
	if !containsCI(q, "SELECT COUNT(") || !containsCI(q, "action_log") {
		t.Errorf("unexpected count query: %s", q)
	}
	if !containsCI(q, "action_type") {
		t.Errorf("count query should filter action_type='error': %s", q)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg (since), got %d", len(args))
	}
	if args[0] != since {
		t.Errorf("expected since=%v arg, got %v", since, args[0])
	}
}

func TestBuildListRecentQuery_ShapeAndArgs(t *testing.T) {
	q, args := buildListRecentQuery(25)
	if !containsCI(q, "ORDER BY created_at DESC") {
		t.Errorf("list query should order newest first: %s", q)
	}
	if !containsCI(q, "LIMIT") {
		t.Errorf("list query should LIMIT: %s", q)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	if v, _ := args[0].(int); v != 25 {
		t.Errorf("expected limit=25, got %v", args[0])
	}
}

func TestNewPgStore_NilDBReturnsNil(t *testing.T) {
	if store := NewPgStore(nil); store != nil {
		t.Fatal("NewPgStore(nil) should return nil so callers fall back to MemoryStore")
	}
}
