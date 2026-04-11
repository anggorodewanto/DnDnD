package dmqueue

import (
	"context"
	"errors"
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

// DefaultNotifier is the standard Notifier implementation. It composes a
// Store (in-memory or DB-backed) with a Sender, ChannelResolver, and
// ResolvePathBuilder. Construct it with NewNotifier (in-memory default) or
// NewNotifierWithStore (caller-provided Store, e.g. PgStore).
type DefaultNotifier struct {
	sender   Sender
	resolver ChannelResolver
	pathBldr ResolvePathBuilder
	store    Store
}

// NewNotifier constructs a DefaultNotifier backed by an in-memory store.
// Use NewNotifierWithStore for production where persistence is required.
func NewNotifier(sender Sender, resolver ChannelResolver, pathBldr ResolvePathBuilder) *DefaultNotifier {
	return NewNotifierWithStore(sender, resolver, pathBldr, NewMemoryStore())
}

// NewNotifierWithStore constructs a DefaultNotifier with the given Store.
func NewNotifierWithStore(sender Sender, resolver ChannelResolver, pathBldr ResolvePathBuilder, store Store) *DefaultNotifier {
	return &DefaultNotifier{
		sender:   sender,
		resolver: resolver,
		pathBldr: pathBldr,
		store:    store,
	}
}

// Post formats an Event, sends it to the guild's #dm-queue, and persists
// an Item record. If no channel is configured for the guild, returns
// ("", nil) — treat as a silent no-op.
func (n *DefaultNotifier) Post(ctx context.Context, e Event) (string, error) {
	channelID := n.resolver(e.GuildID)
	if channelID == "" {
		return "", nil
	}

	itemID := NewItemID()
	e.ResolvePath = n.pathBldr(itemID)
	content := FormatEvent(e)

	msgID, err := n.sender.Send(channelID, content)
	if err != nil {
		return "", err
	}

	if _, err := n.store.Insert(ctx, itemID, e, channelID, msgID, content); err != nil {
		return "", err
	}
	return itemID, nil
}

// Cancel marks a pending item as cancelled and edits the Discord message
// with a strike-through "Cancelled by player" suffix.
func (n *DefaultNotifier) Cancel(ctx context.Context, itemID, reason string) error {
	item, ok, err := n.store.Get(ctx, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotFound
	}
	if _, err := n.store.MarkCancelled(ctx, itemID, reason); err != nil {
		return err
	}
	content := FormatCancelled(item.PostedText)
	return n.sender.Edit(item.ChannelID, item.MessageID, content)
}

// Resolve marks a pending item as resolved and edits the Discord message
// with a checkmark prefix and the outcome summary.
func (n *DefaultNotifier) Resolve(ctx context.Context, itemID, outcome string) error {
	item, ok, err := n.store.Get(ctx, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotFound
	}
	if _, err := n.store.MarkResolved(ctx, itemID, outcome); err != nil {
		return err
	}
	content := FormatResolved(item.PostedText, outcome)
	return n.sender.Edit(item.ChannelID, item.MessageID, content)
}

// Get returns a copy of the item by ID. The store is consulted with a
// background context; callers needing cancellation should use the store
// directly.
func (n *DefaultNotifier) Get(itemID string) (Item, bool) {
	item, ok, err := n.store.Get(context.Background(), itemID)
	if err != nil {
		return Item{}, false
	}
	return item, ok
}

// ListPending returns all items currently in pending status, in post order.
func (n *DefaultNotifier) ListPending() []Item {
	items, err := n.store.ListPending(context.Background())
	if err != nil {
		return nil
	}
	return items
}
