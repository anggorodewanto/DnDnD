package errorlog_test

import (
	"context"
	"testing"
	"time"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/database"
	"github.com/ab/dndnd/internal/errorlog"
	"github.com/ab/dndnd/internal/testutil"
)

// TestPgStore_RoundTrip exercises the full PgStore read/write path through a
// real PostgreSQL container. Phase 112 migration 20260424120001 drops the
// NOT NULL constraints so errors with no encounter context can be stored.
func TestPgStore_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	store := errorlog.NewPgStore(db)
	if store == nil {
		t.Fatal("expected non-nil PgStore")
	}

	ctx := context.Background()
	err := store.Record(ctx, errorlog.Entry{
		Command: "cast",
		UserID:  "alice",
		Summary: "DB timeout on /cast by @alice",
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Second error without user (e.g. background PNG render).
	err = store.Record(ctx, errorlog.Entry{
		Command: "render",
		Summary: "PNG generation failed for encounter X",
	})
	if err != nil {
		t.Fatalf("Record (no user): %v", err)
	}

	count, err := store.CountSince(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountSince: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}

	entries, err := store.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Most recent first → render was second recorded.
	if entries[0].Command != "render" {
		t.Errorf("expected newest=render first, got %+v", entries[0])
	}
	if entries[1].Command != "cast" || entries[1].UserID != "alice" {
		t.Errorf("unexpected older entry: %+v", entries[1])
	}
	if entries[0].CreatedAt.IsZero() {
		t.Error("expected CreatedAt populated from DB default")
	}
}

// TestPgStore_OldEntriesExcludedFromCount verifies CountSince respects the
// since timestamp and does not report errors older than 24h.
func TestPgStore_OldEntriesExcludedFromCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testutil.NewTestDB(t)
	if err := database.MigrateUp(db, dbfs.Migrations); err != nil {
		t.Fatalf("MigrateUp failed: %v", err)
	}

	store := errorlog.NewPgStore(db)
	ctx := context.Background()

	// Record one error (now).
	if err := store.Record(ctx, errorlog.Entry{Command: "cast", Summary: "recent"}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Backdate that row so CountSince(now-24h) excludes it.
	if _, err := db.Exec(`UPDATE action_log SET created_at = now() - interval '48 hours' WHERE action_type = 'error'`); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	count, err := store.CountSince(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountSince: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0 for old error, got %d", count)
	}
}
