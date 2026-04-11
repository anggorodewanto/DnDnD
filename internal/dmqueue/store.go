package dmqueue

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// Store persists dm-queue items. Implementations may keep state in memory
// (MemoryStore, used by unit tests) or in PostgreSQL (PgStore, used in
// production via main.go wiring).
//
// Insert reserves an ID up-front so callers can build the resolve link
// (which embeds the ID) before the Discord message is sent. The returned
// Item carries the assigned ID and Pending status.
type Store interface {
	// Insert persists a new item in pending status. The Event's CampaignID
	// and GuildID are taken as-is; channelID/messageID are recorded so
	// later edits know which Discord message to mutate.
	Insert(ctx context.Context, id string, e Event, channelID, messageID, postedText string) (Item, error)
	// Get returns a copy of the item by ID.
	Get(ctx context.Context, id string) (Item, bool, error)
	// MarkResolved transitions a pending item to resolved with the given outcome.
	MarkResolved(ctx context.Context, id, outcome string) (Item, error)
	// MarkCancelled transitions a pending item to cancelled with the given reason.
	MarkCancelled(ctx context.Context, id, reason string) (Item, error)
	// ListPending returns all pending items in stable post order.
	ListPending(ctx context.Context) ([]Item, error)
}

// NewItemID returns a fresh UUID string for use as a dm-queue item ID.
func NewItemID() string { return uuid.NewString() }

// MemoryStore is an in-memory Store implementation suitable for unit tests
// and single-process deployments without persistence requirements.
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]*Item
	order []string
}

// NewMemoryStore constructs an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]*Item)}
}

// Insert persists a new pending item.
func (m *MemoryStore) Insert(_ context.Context, id string, e Event, channelID, messageID, postedText string) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := &Item{
		ID:         id,
		Event:      e,
		ChannelID:  channelID,
		MessageID:  messageID,
		PostedText: postedText,
		Status:     StatusPending,
	}
	m.items[id] = item
	m.order = append(m.order, id)
	return *item, nil
}

// Get returns a copy of the item by ID.
func (m *MemoryStore) Get(_ context.Context, id string) (Item, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok {
		return Item{}, false, nil
	}
	return *item, true, nil
}

// MarkResolved transitions an item to resolved.
func (m *MemoryStore) MarkResolved(_ context.Context, id, outcome string) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	item.Status = StatusResolved
	item.Outcome = outcome
	return *item, nil
}

// MarkCancelled transitions an item to cancelled.
func (m *MemoryStore) MarkCancelled(_ context.Context, id, reason string) (Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	item.Status = StatusCancelled
	if reason != "" {
		item.Outcome = reason
	}
	return *item, nil
}

// ListPending returns all pending items in post order.
func (m *MemoryStore) ListPending(_ context.Context) ([]Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Item
	for _, id := range m.order {
		item := m.items[id]
		if item.Status != StatusPending {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}
