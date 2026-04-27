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
		"INSERT INTO error_log",
		"command",
		"user_id",
		"summary",
	} {
		if !containsCI(q, want) {
			t.Errorf("INSERT SQL missing %q — got: %s", want, q)
		}
	}
	if containsCI(q, "action_log") {
		t.Errorf("INSERT SQL should target error_log, not action_log: %s", q)
	}
	if len(args) == 0 {
		t.Fatal("expected bound args")
	}
	foundCommand := false
	foundSummary := false
	foundUser := false
	for _, a := range args {
		s, _ := a.(string)
		if s == entry.Command {
			foundCommand = true
		}
		if s == entry.Summary {
			foundSummary = true
		}
		if s == entry.UserID {
			foundUser = true
		}
	}
	if !foundCommand {
		t.Errorf("expected command in args, got %v", args)
	}
	if !foundSummary {
		t.Errorf("expected summary in args, got %v", args)
	}
	if !foundUser {
		t.Errorf("expected user id in args, got %v", args)
	}
}

func TestBuildCountSinceQuery_ShapeAndArgs(t *testing.T) {
	since := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	q, args := buildCountSinceQuery(since)
	if !containsCI(q, "SELECT COUNT(") || !containsCI(q, "error_log") {
		t.Errorf("unexpected count query: %s", q)
	}
	if containsCI(q, "action_log") {
		t.Errorf("count query should target error_log, not action_log: %s", q)
	}
	if containsCI(q, "action_type") {
		t.Errorf("count query should not filter on action_type post-119: %s", q)
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
	if !containsCI(q, "FROM error_log") {
		t.Errorf("list query should select FROM error_log: %s", q)
	}
	if containsCI(q, "action_log") {
		t.Errorf("list query should not reference action_log post-119: %s", q)
	}
	for _, want := range []string{"command", "user_id", "summary", "created_at"} {
		if !containsCI(q, want) {
			t.Errorf("list query missing column %q — got: %s", want, q)
		}
	}
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
