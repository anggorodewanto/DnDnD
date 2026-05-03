// Package discordfake provides a goroutine-safe fake implementation of the
// discord.Session interface that records every outbound interaction into an
// ordered transcript and lets tests inject inbound interactions on demand.
//
// The fake is the foundation of the Phase 120 end-to-end harness: it lets
// black-box scenario tests drive the full stack (HTTP + Discord router +
// Postgres + dashboard) without ever touching the live Discord gateway.
package discordfake

import (
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/discord"
)

// Compile-time guarantee that *Fake satisfies discord.Session so harness
// callers can plug it in wherever the production session interface is used.
var _ discord.Session = (*Fake)(nil)

// Kind tags each transcript entry by the surface it was emitted on.
type Kind int

const (
	// KindChannelMessage covers ChannelMessageSend and ChannelMessageSendComplex.
	KindChannelMessage Kind = iota + 1
	// KindInteractionResponse covers InteractionRespond.
	KindInteractionResponse
	// KindInteractionEdit covers InteractionResponseEdit.
	KindInteractionEdit
	// KindChannelMessageEdit covers ChannelMessageEdit.
	KindChannelMessageEdit
	// KindDirectMessage covers UserChannelCreate followed by a send.
	KindDirectMessage
)

// String renders a Kind in a stable form for golden-style assertions.
func (k Kind) String() string {
	switch k {
	case KindChannelMessage:
		return "channel_message"
	case KindInteractionResponse:
		return "interaction_response"
	case KindInteractionEdit:
		return "interaction_edit"
	case KindChannelMessageEdit:
		return "channel_message_edit"
	case KindDirectMessage:
		return "direct_message"
	default:
		return "unknown"
	}
}

// Entry is a single recorded outbound message / interaction.
type Entry struct {
	Kind          Kind
	ChannelID     string
	MessageID     string
	InteractionID string
	Content       string
	Embeds        []*discordgo.MessageEmbed
	Components    []discordgo.MessageComponent
	Ephemeral     bool
	Timestamp     time.Time
}

// ErrWaitTimeout is returned by WaitFor when no entry matches the predicate
// within the supplied timeout.
var ErrWaitTimeout = errors.New("discordfake: WaitFor timed out before any entry matched")

// Fake implements the subset of discord.Session that the bot uses, recording
// every outbound call and letting tests inject inbound interactions through
// SetInteractionHandler + InjectInteraction.
type Fake struct {
	mu             sync.Mutex
	cond           *sync.Cond
	transcript     []Entry
	guildChannels  map[string][]*discordgo.Channel
	dmChannels     map[string]*discordgo.Channel
	state          *discordgo.State
	handler        func(*discordgo.Interaction)
	clock          func() time.Time
	dmAutoIncr     int
}

// New constructs an empty Fake. Tests are expected to seed any guild channels
// they need with AddGuildChannel before driving traffic through the bot.
func New() *Fake {
	f := &Fake{
		guildChannels: make(map[string][]*discordgo.Channel),
		dmChannels:    make(map[string]*discordgo.Channel),
		clock:         time.Now,
		state: &discordgo.State{
			Ready: discordgo.Ready{User: &discordgo.User{ID: "bot-1"}},
		},
	}
	f.cond = sync.NewCond(&f.mu)
	return f
}

// SetClock overrides the timestamp source used in transcript entries. Useful
// for deterministic golden-file assertions.
func (f *Fake) SetClock(c func() time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clock = c
}

// SetInteractionHandler registers the function that InjectInteraction will
// invoke. The CommandRouter's Handle method is the canonical handler.
func (f *Fake) SetInteractionHandler(h func(*discordgo.Interaction)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handler = h
}

// InjectInteraction synchronously delivers an interaction to the registered
// handler. Returns immediately when no handler is set so harness setup
// ordering bugs surface as missing recorded responses rather than panics.
func (f *Fake) InjectInteraction(i *discordgo.Interaction) {
	f.mu.Lock()
	h := f.handler
	f.mu.Unlock()
	if h == nil {
		return
	}
	h(i)
}

// AddGuildChannel registers a channel under the given guild ID so the bot's
// GuildChannels probes (e.g. the #dm-queue resolver) find it.
func (f *Fake) AddGuildChannel(guildID string, ch *discordgo.Channel) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.guildChannels[guildID] = append(f.guildChannels[guildID], ch)
}

// Transcript returns a snapshot copy of every recorded entry in order.
func (f *Fake) Transcript() []Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Entry, len(f.transcript))
	copy(out, f.transcript)
	return out
}

// Reset clears the transcript. Channel registrations and the interaction
// handler are preserved so a single Fake can be reused across phases.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transcript = nil
}

// WaitFor blocks until an entry satisfying predicate appears in the
// transcript, or returns ErrWaitTimeout. Entries appended before WaitFor was
// called are also considered, so callers do not need to race against the
// handler goroutine.
func (f *Fake) WaitFor(predicate func(Entry) bool, timeout time.Duration) (Entry, error) {
	deadline := f.clock().Add(timeout)
	f.mu.Lock()
	defer f.mu.Unlock()
	for {
		for i := range f.transcript {
			if predicate(f.transcript[i]) {
				return f.transcript[i], nil
			}
		}
		now := f.clock()
		if !now.Before(deadline) {
			return Entry{}, ErrWaitTimeout
		}
		// Use a short timed wait so the predicate is re-evaluated even when
		// no broadcast occurs (e.g. no further entries arrive).
		f.timedWait(deadline.Sub(now))
	}
}

// timedWait releases the mutex and waits until either a broadcast arrives or
// the supplied duration elapses. The mutex is reacquired before returning.
func (f *Fake) timedWait(d time.Duration) {
	timer := time.AfterFunc(d, func() {
		f.mu.Lock()
		f.cond.Broadcast()
		f.mu.Unlock()
	})
	f.cond.Wait()
	timer.Stop()
}

func (f *Fake) appendEntry(e Entry) {
	f.mu.Lock()
	if e.Timestamp.IsZero() {
		e.Timestamp = f.clock()
	}
	f.transcript = append(f.transcript, e)
	f.cond.Broadcast()
	f.mu.Unlock()
}

// --- discord.Session implementation ---

// UserChannelCreate returns (and caches) a synthetic DM channel for the
// recipient. Subsequent ChannelMessageSend calls against that channel ID are
// recorded as KindDirectMessage to make DM flows easy to assert on.
func (f *Fake) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	f.mu.Lock()
	if ch, ok := f.dmChannels[recipientID]; ok {
		f.mu.Unlock()
		return ch, nil
	}
	f.dmAutoIncr++
	ch := &discordgo.Channel{
		ID:            "dm-" + recipientID,
		Type:          discordgo.ChannelTypeDM,
		Recipients:    []*discordgo.User{{ID: recipientID}},
	}
	f.dmChannels[recipientID] = ch
	f.mu.Unlock()
	return ch, nil
}

// ChannelMessageSend records a plain channel send.
func (f *Fake) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	kind := f.kindForChannel(channelID)
	f.appendEntry(Entry{
		Kind:      kind,
		ChannelID: channelID,
		Content:   content,
	})
	return &discordgo.Message{ID: "m-" + channelID, ChannelID: channelID, Content: content}, nil
}

// ChannelMessageSendComplex records a complex channel send (with embeds /
// components).
func (f *Fake) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	kind := f.kindForChannel(channelID)
	entry := Entry{
		Kind:      kind,
		ChannelID: channelID,
	}
	if data != nil {
		entry.Content = data.Content
		entry.Embeds = data.Embeds
		entry.Components = data.Components
	}
	f.appendEntry(entry)
	return &discordgo.Message{ID: "m-" + channelID, ChannelID: channelID, Content: entry.Content}, nil
}

// kindForChannel returns KindDirectMessage for synthetic DM channels created
// via UserChannelCreate, KindChannelMessage otherwise.
func (f *Fake) kindForChannel(channelID string) Kind {
	f.mu.Lock()
	for _, ch := range f.dmChannels {
		if ch.ID == channelID {
			f.mu.Unlock()
			return KindDirectMessage
		}
	}
	f.mu.Unlock()
	return KindChannelMessage
}

// ApplicationCommandBulkOverwrite is a no-op recorder.
func (f *Fake) ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return cmds, nil
}

// ApplicationCommands returns nil so the bot's drift check sees an empty
// remote slate during e2e runs.
func (f *Fake) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return nil, nil
}

// ApplicationCommandDelete is a no-op recorder.
func (f *Fake) ApplicationCommandDelete(appID, guildID, cmdID string) error {
	return nil
}

// GuildChannels returns the channels registered with AddGuildChannel.
func (f *Fake) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*discordgo.Channel, len(f.guildChannels[guildID]))
	copy(out, f.guildChannels[guildID])
	return out, nil
}

// GuildChannelCreateComplex synthesizes a channel and registers it under the
// given guild so subsequent GuildChannels probes find it.
func (f *Fake) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	ch := &discordgo.Channel{
		ID:   "ch-" + guildID + "-" + data.Name,
		Name: data.Name,
	}
	f.AddGuildChannel(guildID, ch)
	return ch, nil
}

// InteractionRespond records the response and tags ephemeral if the data
// flags include MessageFlagsEphemeral.
func (f *Fake) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	entry := Entry{
		Kind: KindInteractionResponse,
	}
	if interaction != nil {
		entry.InteractionID = interaction.ID
		entry.ChannelID = interaction.ChannelID
	}
	if resp != nil && resp.Data != nil {
		entry.Content = resp.Data.Content
		entry.Embeds = resp.Data.Embeds
		entry.Components = resp.Data.Components
		if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
			entry.Ephemeral = true
		}
	}
	f.appendEntry(entry)
	return nil
}

// InteractionResponseEdit records the edit.
func (f *Fake) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	entry := Entry{
		Kind: KindInteractionEdit,
	}
	if interaction != nil {
		entry.InteractionID = interaction.ID
		entry.ChannelID = interaction.ChannelID
	}
	if newresp != nil {
		if newresp.Content != nil {
			entry.Content = *newresp.Content
		}
		if newresp.Embeds != nil {
			entry.Embeds = *newresp.Embeds
		}
		if newresp.Components != nil {
			entry.Components = *newresp.Components
		}
	}
	f.appendEntry(entry)
	return &discordgo.Message{}, nil
}

// ChannelMessageEdit records the edit.
func (f *Fake) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	f.appendEntry(Entry{
		Kind:      KindChannelMessageEdit,
		ChannelID: channelID,
		MessageID: messageID,
		Content:   content,
	})
	return &discordgo.Message{ID: messageID, ChannelID: channelID, Content: content}, nil
}

// GetState returns a stable State stub; tests can swap it via SetState.
func (f *Fake) GetState() *discordgo.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

// SetState overrides the *discordgo.State returned by GetState.
func (f *Fake) SetState(s *discordgo.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = s
}
