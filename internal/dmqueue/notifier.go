package dmqueue

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

// Status of a dm-queue item.
type Status string

const (
	StatusPending   Status = "pending"
	StatusCancelled Status = "cancelled"
	StatusResolved  Status = "resolved"
)

// ErrItemNotFound is returned by Cancel/Resolve for unknown item IDs.
var ErrItemNotFound = errors.New("dm-queue item not found")

// Item is a stored dm-queue entry.
type Item struct {
	ID         string
	Event      Event
	ChannelID  string
	MessageID  string
	PostedText string
	Status     Status
	Outcome    string
}

// Sender is the minimal Discord surface the Notifier needs.
// Implementations wrap MessageQueue or the raw Session.
type Sender interface {
	// Send posts a message to the channel. Returns the created message's ID.
	Send(channelID, content string) (messageID string, err error)
	// Edit replaces the content of an existing message.
	Edit(channelID, messageID, content string) error
}

// ChannelResolver maps a guild ID to the #dm-queue channel ID for that guild.
// Return "" if the guild has no configured dm-queue (posts become no-ops).
type ChannelResolver func(guildID string) string

// ResolvePathBuilder builds the dashboard URL path for a given item ID.
type ResolvePathBuilder func(itemID string) string

// Notifier is the dm-queue notification framework.
type Notifier interface {
	Post(ctx context.Context, e Event) (itemID string, err error)
	Cancel(ctx context.Context, itemID string, reason string) error
	Resolve(ctx context.Context, itemID string, outcome string) error
	Get(itemID string) (Item, bool)
	ListPending() []Item
}

// DefaultNotifier is an in-memory Notifier implementation suitable for
// single-process deployments. Persistence across restarts is out of
// scope for Phase 106a; upgrade to a DB-backed store if/when needed.
type DefaultNotifier struct {
	sender   Sender
	resolver ChannelResolver
	pathBldr ResolvePathBuilder

	mu    sync.Mutex
	items map[string]*Item
	order []string
}

// NewNotifier constructs a DefaultNotifier.
func NewNotifier(sender Sender, resolver ChannelResolver, pathBldr ResolvePathBuilder) *DefaultNotifier {
	return &DefaultNotifier{
		sender:   sender,
		resolver: resolver,
		pathBldr: pathBldr,
		items:    make(map[string]*Item),
	}
}

// Post formats an Event, sends it to the guild's #dm-queue, and persists
// an Item record. If no channel is configured for the guild, returns
// ("", nil) — treat as a silent no-op.
func (n *DefaultNotifier) Post(_ context.Context, e Event) (string, error) {
	channelID := n.resolver(e.GuildID)
	if channelID == "" {
		return "", nil
	}

	itemID := uuid.NewString()
	e.ResolvePath = n.pathBldr(itemID)
	content := FormatEvent(e)

	msgID, err := n.sender.Send(channelID, content)
	if err != nil {
		return "", err
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.items[itemID] = &Item{
		ID:         itemID,
		Event:      e,
		ChannelID:  channelID,
		MessageID:  msgID,
		PostedText: content,
		Status:     StatusPending,
	}
	n.order = append(n.order, itemID)
	return itemID, nil
}

// Cancel marks a pending item as cancelled and edits the Discord message
// with a strike-through "Cancelled by player" suffix.
func (n *DefaultNotifier) Cancel(_ context.Context, itemID, _ string) error {
	n.mu.Lock()
	item, ok := n.items[itemID]
	if !ok {
		n.mu.Unlock()
		return ErrItemNotFound
	}
	content := FormatCancelled(item.PostedText)
	item.Status = StatusCancelled
	channelID, msgID := item.ChannelID, item.MessageID
	n.mu.Unlock()

	return n.sender.Edit(channelID, msgID, content)
}

// Resolve marks a pending item as resolved and edits the Discord message
// with a ✅ prefix and the outcome summary.
func (n *DefaultNotifier) Resolve(_ context.Context, itemID, outcome string) error {
	n.mu.Lock()
	item, ok := n.items[itemID]
	if !ok {
		n.mu.Unlock()
		return ErrItemNotFound
	}
	content := FormatResolved(item.PostedText, outcome)
	item.Status = StatusResolved
	item.Outcome = outcome
	channelID, msgID := item.ChannelID, item.MessageID
	n.mu.Unlock()

	return n.sender.Edit(channelID, msgID, content)
}

// Get returns a copy of the item by ID.
func (n *DefaultNotifier) Get(itemID string) (Item, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	item, ok := n.items[itemID]
	if !ok {
		return Item{}, false
	}
	return *item, true
}

// ListPending returns all items currently in pending status, in post order.
func (n *DefaultNotifier) ListPending() []Item {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []Item
	for _, id := range n.order {
		item := n.items[id]
		if item.Status != StatusPending {
			continue
		}
		out = append(out, *item)
	}
	return out
}
