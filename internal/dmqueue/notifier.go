package dmqueue

import (
	"context"
	"errors"
	"fmt"
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

// ErrNotWhisperItem is returned by ResolveWhisper when the target item is
// not a KindPlayerWhisper entry. Callers must use the regular Resolve path
// for non-whisper kinds.
var ErrNotWhisperItem = errors.New("dm-queue item is not a whisper")

// ErrWhisperTargetMissing is returned by ResolveWhisper when the whisper
// item has no whisper_target_discord_user_id in its ExtraMetadata.
var ErrWhisperTargetMissing = errors.New("whisper item missing target discord user id")

// ErrWhisperDelivererMissing is returned by ResolveWhisper when no
// WhisperReplyDeliverer has been wired onto the notifier.
var ErrWhisperDelivererMissing = errors.New("whisper reply deliverer not wired")

// ErrNotSkillCheckNarrationItem is returned by ResolveSkillCheckNarration
// when the target item is not a KindSkillCheckNarration entry.
var ErrNotSkillCheckNarrationItem = errors.New("dm-queue item is not a skill check narration")

// ErrSkillCheckChannelMissing is returned by ResolveSkillCheckNarration
// when the item has no skill_check_channel_id in its ExtraMetadata.
var ErrSkillCheckChannelMissing = errors.New("skill check narration item missing channel id")

// ErrSkillCheckNarrationDelivererMissing is returned by
// ResolveSkillCheckNarration when no SkillCheckNarrationDeliverer is wired.
var ErrSkillCheckNarrationDelivererMissing = errors.New("skill check narration deliverer not wired")

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
	// ResolveWhisper delivers a DM reply to the whispering player's
	// Discord user id (stashed in the item's ExtraMetadata) and then
	// marks the item resolved with replyText as the outcome. Returns
	// ErrNotWhisperItem for non-whisper kinds.
	ResolveWhisper(ctx context.Context, itemID string, replyText string) error
	// ResolveSkillCheckNarration delivers the DM's narration text as a
	// non-ephemeral follow-up message in the channel where the player
	// invoked /check (channel id stashed in ExtraMetadata) and then marks
	// the item resolved with the narration as its outcome. Returns
	// ErrNotSkillCheckNarrationItem for non-narration kinds.
	ResolveSkillCheckNarration(ctx context.Context, itemID string, narration string) error
	Get(itemID string) (Item, bool)
	ListPending() []Item
}

// WhisperReplyDeliverer delivers a DM reply text to a Discord user.
// *discord.DirectMessenger satisfies this shape.
type WhisperReplyDeliverer interface {
	SendDirectMessage(discordUserID, body string) ([]string, error)
}

// SkillCheckNarrationDeliverer delivers a non-ephemeral follow-up message
// to a Discord channel — used by ResolveSkillCheckNarration so the DM's
// narration text reaches the originating channel where /check was invoked.
// A thin wrapper over Session.ChannelMessageSend in the discord package
// satisfies this shape.
type SkillCheckNarrationDeliverer interface {
	DeliverSkillCheckNarration(channelID, body string) error
}

// DefaultNotifier is the standard Notifier implementation. It composes a
// Store (in-memory or DB-backed) with a Sender, ChannelResolver, and
// ResolvePathBuilder. Construct it with NewNotifier (in-memory default) or
// NewNotifierWithStore (caller-provided Store, e.g. PgStore).
type DefaultNotifier struct {
	sender              Sender
	resolver            ChannelResolver
	pathBldr            ResolvePathBuilder
	store               Store
	deliverer           WhisperReplyDeliverer
	narrationDeliverer  SkillCheckNarrationDeliverer
}

// SetWhisperDeliverer wires the DM-to-player deliverer used by
// ResolveWhisper. Passing nil disables whisper resolution; call sites that
// do not need whispers (e.g. unit tests, headless integration tests) may
// leave it unset.
func (n *DefaultNotifier) SetWhisperDeliverer(d WhisperReplyDeliverer) {
	n.deliverer = d
}

// SetSkillCheckNarrationDeliverer wires the channel-follow-up deliverer
// used by ResolveSkillCheckNarration. Passing nil disables narration
// resolution; call sites that do not need /check gating may leave it unset.
func (n *DefaultNotifier) SetSkillCheckNarrationDeliverer(d SkillCheckNarrationDeliverer) {
	n.narrationDeliverer = d
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
	return n.sender.Edit(item.ChannelID, item.MessageID, FormatCancelled(item.PostedText))
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
	return n.sender.Edit(item.ChannelID, item.MessageID, FormatResolved(item.PostedText, outcome))
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

// PostNarrativeTeleport posts a KindNarrativeTeleport event for the given
// caster and spell. This is a framework helper the future /cast command
// handler will invoke — until then it lets the dashboard render narrative
// teleport items end-to-end with a fake sender.
func (n *DefaultNotifier) PostNarrativeTeleport(ctx context.Context, caster, spellName, guildID, campaignID string) (string, error) {
	return n.Post(ctx, Event{
		Kind:       KindNarrativeTeleport,
		PlayerName: caster,
		Summary:    fmt.Sprintf("casts %s (narrative resolution)", spellName),
		GuildID:    guildID,
		CampaignID: campaignID,
	})
}

// PostWhisper posts a KindPlayerWhisper event for a player's /whisper
// message. The target player's Discord user ID is stashed in ExtraMetadata
// so ResolveWhisper can deliver the DM reply later.
func (n *DefaultNotifier) PostWhisper(ctx context.Context, player, message, discordUserID, guildID, campaignID string) (string, error) {
	return n.Post(ctx, Event{
		Kind:       KindPlayerWhisper,
		PlayerName: player,
		Summary:    message,
		GuildID:    guildID,
		CampaignID: campaignID,
		ExtraMetadata: map[string]string{
			WhisperTargetDiscordUserIDKey: discordUserID,
		},
	})
}

// ResolveWhisper delivers the DM's reply to the whispering player as a
// Discord DM, then marks the queue item resolved with the reply text. The
// DM is sent first: if delivery fails the item stays pending so the DM can
// retry after fixing the underlying issue (bot offline, DMs closed, etc).
func (n *DefaultNotifier) ResolveWhisper(ctx context.Context, itemID, replyText string) error {
	item, ok, err := n.store.Get(ctx, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotFound
	}
	if item.Event.Kind != KindPlayerWhisper {
		return ErrNotWhisperItem
	}
	if n.deliverer == nil {
		return ErrWhisperDelivererMissing
	}
	targetUserID := item.Event.ExtraMetadata[WhisperTargetDiscordUserIDKey]
	if targetUserID == "" {
		return ErrWhisperTargetMissing
	}
	if _, err := n.deliverer.SendDirectMessage(targetUserID, replyText); err != nil {
		return fmt.Errorf("delivering whisper reply: %w", err)
	}
	return n.Resolve(ctx, itemID, replyText)
}

// ResolveSkillCheckNarration delivers the DM's narration to the originating
// channel as a non-ephemeral follow-up that mentions the invoking player
// alongside the rolled total, then marks the queue item resolved with the
// narration as its outcome. The follow-up is delivered first: if delivery
// fails the item stays pending so the DM can retry.
func (n *DefaultNotifier) ResolveSkillCheckNarration(ctx context.Context, itemID, narration string) error {
	item, ok, err := n.store.Get(ctx, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrItemNotFound
	}
	if item.Event.Kind != KindSkillCheckNarration {
		return ErrNotSkillCheckNarrationItem
	}
	if n.narrationDeliverer == nil {
		return ErrSkillCheckNarrationDelivererMissing
	}
	channelID := item.Event.ExtraMetadata[SkillCheckChannelIDKey]
	if channelID == "" {
		return ErrSkillCheckChannelMissing
	}

	body := FormatSkillCheckNarrationFollowup(item.Event, narration)
	if err := n.narrationDeliverer.DeliverSkillCheckNarration(channelID, body); err != nil {
		return fmt.Errorf("delivering skill check narration: %w", err)
	}
	return n.Resolve(ctx, itemID, narration)
}

// ListPending returns all items currently in pending status, in post order.
func (n *DefaultNotifier) ListPending() []Item {
	items, err := n.store.ListPending(context.Background())
	if err != nil {
		return nil
	}
	return items
}
