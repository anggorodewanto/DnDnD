package errorlog

import (
	"context"
	"testing"
	"time"
)

func TestNewRecorderRef_DelegatesToInitial(t *testing.T) {
	store := NewMemoryStore(time.Now)
	ref := NewRecorderRef(store)

	err := ref.Record(context.Background(), Entry{Command: "test", Summary: "hello"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	count, _ := store.CountSince(context.Background(), time.Now().Add(-time.Hour))
	if count != 1 {
		t.Fatalf("expected 1 entry in underlying store, got %d", count)
	}
}

func TestRecorderRef_NilRecorder(t *testing.T) {
	ref := NewRecorderRef(nil)
	err := ref.Record(context.Background(), Entry{Command: "x"})
	if err != nil {
		t.Fatalf("expected nil error for nil recorder, got %v", err)
	}
}

func TestRecorderRef_Swap(t *testing.T) {
	store1 := NewMemoryStore(time.Now)
	store2 := NewMemoryStore(time.Now)
	ref := NewRecorderRef(store1)

	prev := ref.Swap(store2)
	if prev != store1 {
		t.Fatal("Swap should return previous recorder")
	}

	// New records go to store2
	_ = ref.Record(context.Background(), Entry{Command: "after"})
	count1, _ := store1.CountSince(context.Background(), time.Now().Add(-time.Hour))
	count2, _ := store2.CountSince(context.Background(), time.Now().Add(-time.Hour))
	if count1 != 0 {
		t.Fatalf("store1 should have 0 entries after swap, got %d", count1)
	}
	if count2 != 1 {
		t.Fatalf("store2 should have 1 entry after swap, got %d", count2)
	}
}

func TestNewMemoryStore_NilClockDefaultsToTimeNow(t *testing.T) {
	store := NewMemoryStore(nil)
	_ = store.Record(context.Background(), Entry{Command: "x"})
	entries, _ := store.ListRecent(context.Background(), 1)
	if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	}
	if entries[0].CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt from default clock")
	}
}

func TestMemoryStore_ListRecentZeroLimit(t *testing.T) {
	store := NewMemoryStore(time.Now)
	_ = store.Record(context.Background(), Entry{Command: "x"})
	entries, err := store.ListRecent(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListRecent(0): %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil for limit<=0, got %v", entries)
	}
}

func TestBuildSummary_EmptyCommand(t *testing.T) {
	got := BuildSummary("", "bob", nil)
	if !containsCI(got, "unknown") {
		t.Errorf("expected 'unknown' for empty command, got %q", got)
	}
}

func TestBuildSummary_NilError(t *testing.T) {
	got := BuildSummary("move", "alice", nil)
	if !containsCI(got, "error") {
		t.Errorf("expected 'error' placeholder for nil err, got %q", got)
	}
	if !containsCI(got, "move") {
		t.Errorf("expected command in summary, got %q", got)
	}
}
