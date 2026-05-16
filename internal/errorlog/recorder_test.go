package errorlog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestMemoryStore_RecordAndCount(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := NewMemoryStore(clock)

	entry := Entry{
		Command: "cast",
		UserID:  "user-1",
		Summary: "DB timeout on /cast by user-1",
	}
	if err := store.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record: %v", err)
	}

	count, err := store.CountSince(context.Background(), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountSince: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Command != "cast" || entries[0].UserID != "user-1" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	if entries[0].CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt populated")
	}
}

func TestMemoryStore_CountSinceIgnoresOldEntries(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	current := now.Add(-48 * time.Hour)
	clock := func() time.Time { return current }
	store := NewMemoryStore(clock)

	// Record an entry 48h ago
	_ = store.Record(context.Background(), Entry{Command: "old"})

	// Advance clock to "now" and record another
	current = now
	_ = store.Record(context.Background(), Entry{Command: "fresh"})

	count, err := store.CountSince(context.Background(), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountSince: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1 (only fresh within 24h), got %d", count)
	}
}

func TestMemoryStore_ListRecentMostRecentFirst(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	current := now
	clock := func() time.Time { return current }
	store := NewMemoryStore(clock)

	_ = store.Record(context.Background(), Entry{Command: "first"})
	current = now.Add(1 * time.Minute)
	_ = store.Record(context.Background(), Entry{Command: "second"})
	current = now.Add(2 * time.Minute)
	_ = store.Record(context.Background(), Entry{Command: "third"})

	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Command != "third" {
		t.Fatalf("expected newest first, got %q", entries[0].Command)
	}
	if entries[2].Command != "first" {
		t.Fatalf("expected oldest last, got %q", entries[2].Command)
	}
}

func TestMemoryStore_ListRecentRespectsLimit(t *testing.T) {
	store := NewMemoryStore(time.Now)
	for i := 0; i < 5; i++ {
		_ = store.Record(context.Background(), Entry{Command: "cmd"})
	}
	entries, err := store.ListRecent(context.Background(), 3)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (limit), got %d", len(entries))
	}
}

func TestBuildSummary(t *testing.T) {
	err := errors.New("context deadline exceeded")
	got := BuildSummary("cast", "alice", err)
	if got == "" {
		t.Fatal("expected non-empty summary")
	}
	// Should include command, user ref, and error class
	for _, want := range []string{"cast", "alice", "context deadline"} {
		if !containsCI(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

func TestBuildSummary_NoUser(t *testing.T) {
	got := BuildSummary("render", "", errors.New("png generation failed"))
	if got == "" {
		t.Fatal("expected non-empty summary")
	}
	if !containsCI(got, "render") {
		t.Errorf("summary %q missing command", got)
	}
}

// containsCI is a tiny case-insensitive substring check for tests.
func containsCI(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// ASCII lower-case both and compare
	lower := func(r byte) byte {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if lower(s[i+j]) != lower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestMemoryStore_DetailFieldStoredAndReturned(t *testing.T) {
	store := NewMemoryStore(time.Now)
	detail := json.RawMessage(`{"stack":"goroutine 1 [running]:\nmain.main()"}`)

	err := store.Record(context.Background(), Entry{
		Command: "cast",
		UserID:  "user-1",
		Summary: "panic on /cast",
		Detail:  detail,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, err := store.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if string(entries[0].Detail) != string(detail) {
		t.Fatalf("Detail mismatch: got %q, want %q", entries[0].Detail, detail)
	}
}
