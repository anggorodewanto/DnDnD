// Package errorlog records internal errors for surfacing on the DM dashboard
// and logs them at ERROR level via slog. Phase 112 — Error Handling &
// Observability: every panic / DB timeout / Discord API failure funnels
// through Record so the DM dashboard badge (24h count) and error panel can
// present the same events that appear in structured stdout logs.
package errorlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Entry is a single recorded error. CreatedAt is filled in by the store on
// Record — callers pass an otherwise-complete Entry.
type Entry struct {
	CreatedAt time.Time
	// Command is the slash-command name (e.g. "cast") or a short route
	// identifier for non-command paths (e.g. "dashboard.errors").
	Command string
	// UserID is the Discord user ID, or "" for system-context errors (PNG
	// rendering, background goroutines) that have no associated player.
	UserID string
	// Summary is a short human-readable description suitable for the DM
	// panel. Use BuildSummary to produce it from (command, user, err).
	Summary string
	// Detail holds optional structured data (e.g. stack trace) as JSON,
	// stored in the error_detail JSONB column.
	Detail json.RawMessage
}

// Recorder is the minimum surface callers (panic recovery, /command error
// paths, dashboard handlers) need. Implementations must be safe for
// concurrent use.
type Recorder interface {
	Record(ctx context.Context, entry Entry) error
}

// Reader exposes the read side used by the dashboard badge + panel.
type Reader interface {
	CountSince(ctx context.Context, since time.Time) (int, error)
	ListRecent(ctx context.Context, limit int) ([]Entry, error)
}

// Store combines the write and read surfaces.
type Store interface {
	Recorder
	Reader
}

// MemoryStore is a thread-safe in-memory Store used by tests and by deploys
// without a configured error_log backend. It keeps every recorded entry in
// memory; production deploys should wrap or swap with PgStore which writes
// to the dedicated error_log table (Phase 119).
type MemoryStore struct {
	mu      sync.Mutex
	clock   func() time.Time
	entries []Entry
}

// NewMemoryStore returns an empty MemoryStore. Pass time.Now unless a test
// needs a deterministic clock.
func NewMemoryStore(clock func() time.Time) *MemoryStore {
	if clock == nil {
		clock = time.Now
	}
	return &MemoryStore{clock: clock}
}

// Record timestamps and appends the entry.
func (m *MemoryStore) Record(_ context.Context, entry Entry) error {
	entry.CreatedAt = m.clock()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return nil
}

// CountSince returns the number of entries with CreatedAt >= since.
func (m *MemoryStore) CountSince(_ context.Context, since time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, e := range m.entries {
		if !e.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

// ListRecent returns up to limit entries, most recent first.
func (m *MemoryStore) ListRecent(_ context.Context, limit int) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 {
		return nil, nil
	}
	n := len(m.entries)
	if limit > n {
		limit = n
	}
	out := make([]Entry, 0, limit)
	for i := n - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.entries[i])
	}
	return out, nil
}

// BuildSummary formats a short, DM-readable summary like
// "DB timeout on /cast by @alice" from a command name, user ID, and error.
// Either userID or err may be empty / nil.
func BuildSummary(command, userID string, err error) string {
	msg := "error"
	if err != nil {
		msg = err.Error()
	}
	if command == "" {
		command = "unknown"
	}
	if userID == "" {
		return fmt.Sprintf("%s on /%s", msg, command)
	}
	return fmt.Sprintf("%s on /%s by @%s", msg, command, userID)
}

// ErrNoRecorder is returned by NoopRecorder to signal recording is disabled.
// Callers that want hard-fail semantics can check for it; the default
// behavior in this phase is soft (log-only) so production paths should not
// propagate this error to players.
var ErrNoRecorder = errors.New("errorlog: no recorder configured")

// RecorderRef is a thread-safe swappable Recorder. It starts with an initial
// recorder and can be upgraded later (e.g., from MemoryStore to PgStore).
type RecorderRef struct {
	mu  sync.RWMutex
	rec Recorder
}

// NewRecorderRef creates a RecorderRef with the given initial recorder.
func NewRecorderRef(initial Recorder) *RecorderRef {
	return &RecorderRef{rec: initial}
}

// Record delegates to the current recorder.
func (r *RecorderRef) Record(ctx context.Context, entry Entry) error {
	r.mu.RLock()
	rec := r.rec
	r.mu.RUnlock()
	if rec == nil {
		return nil
	}
	return rec.Record(ctx, entry)
}

// Swap replaces the underlying recorder and returns the previous one.
func (r *RecorderRef) Swap(next Recorder) Recorder {
	r.mu.Lock()
	prev := r.rec
	r.rec = next
	r.mu.Unlock()
	return prev
}
