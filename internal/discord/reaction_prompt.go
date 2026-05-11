package discord

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// reactionPromptPrefix is the CustomID namespace owned by ReactionPromptStore.
// Every button it posts uses the layout <prefix>:<promptID>:<choice> so the
// router can route the click back without each call site registering its own
// component prefix.
const reactionPromptPrefix = "rxprompt"

// DefaultReactionPromptTTL mirrors the spec's 30-second reaction-prompt
// window (see spec line 1102 / Bardic Inspiration usage timeout). Counterspell
// and metamagic prompts share the same default; callers that want a different
// timeout build a store with NewReactionPromptStoreWithTTL.
const DefaultReactionPromptTTL = 30 * time.Second

// ErrReactionPromptNotFound is returned when a button click refers to a prompt
// that has already been consumed, never existed, or expired.
var ErrReactionPromptNotFound = errors.New("reaction prompt not found or expired")

// ReactionPromptChoice fires when the player clicks one of the buttons. The
// raw interaction is passed so the handler can edit the original message or
// post its own follow-up; choice carries the button's Choice string.
type ReactionPromptChoice func(ctx context.Context, interaction *discordgo.Interaction, choice string)

// ReactionPromptForfeit fires when the TTL elapses without a click. Nil to
// skip the forfeit path entirely (e.g. metamagic prompts that simply default
// to "use the original cast" on timeout).
type ReactionPromptForfeit func(ctx context.Context)

// ReactionPromptButton is one button on a reaction prompt.
type ReactionPromptButton struct {
	Label  string
	Choice string                // serialized into CustomID
	Style  discordgo.ButtonStyle // zero -> PrimaryButton
}

// ReactionPromptPostArgs collects the inputs for Post.
type ReactionPromptPostArgs struct {
	ChannelID string
	Content   string
	Buttons   []ReactionPromptButton
	OnChoice  ReactionPromptChoice
	OnForfeit ReactionPromptForfeit
}

type pendingReactionPrompt struct {
	onChoice ReactionPromptChoice
	timer    *time.Timer
}

// ReactionPromptStore tracks active reaction prompts and routes button clicks
// back to the registered handler.
//
// The state is in-process memory. Bot restarts drop the pending entries; that
// is acceptable for these prompts because the surrounding mechanics already
// fail safe — Counterspell ForfeitCounterspell, Stunning Strike no-op, Smite
// "no smite this hit" — none of which need persistence beyond the encounter.
// Mirrors the ddbimport ApproveImport pattern.
type ReactionPromptStore struct {
	session Session
	ttl     time.Duration

	mu      sync.Mutex
	pending map[uuid.UUID]*pendingReactionPrompt
}

// NewReactionPromptStore creates a store with DefaultReactionPromptTTL.
func NewReactionPromptStore(session Session) *ReactionPromptStore {
	return NewReactionPromptStoreWithTTL(session, DefaultReactionPromptTTL)
}

// NewReactionPromptStoreWithTTL creates a store with a custom TTL. Tests use
// this to fast-forward forfeit behavior.
func NewReactionPromptStoreWithTTL(session Session, ttl time.Duration) *ReactionPromptStore {
	if ttl <= 0 {
		ttl = DefaultReactionPromptTTL
	}
	return &ReactionPromptStore{
		session: session,
		ttl:     ttl,
		pending: make(map[uuid.UUID]*pendingReactionPrompt),
	}
}

// Post sends a message to the channel with the prompt buttons, registers the
// callbacks, and starts the forfeit timer. The returned promptID is informational
// — callers do not need to track it unless they want to Cancel it.
func (s *ReactionPromptStore) Post(args ReactionPromptPostArgs) (uuid.UUID, error) {
	if args.OnChoice == nil {
		return uuid.Nil, fmt.Errorf("ReactionPromptStore.Post: OnChoice is required")
	}
	if len(args.Buttons) == 0 {
		return uuid.Nil, fmt.Errorf("ReactionPromptStore.Post: at least one button required")
	}

	promptID := uuid.New()
	if _, err := s.session.ChannelMessageSendComplex(args.ChannelID, &discordgo.MessageSend{
		Content:    args.Content,
		Components: buildPromptComponents(promptID, args.Buttons),
	}); err != nil {
		return uuid.Nil, fmt.Errorf("sending reaction prompt: %w", err)
	}

	entry := &pendingReactionPrompt{onChoice: args.OnChoice}
	entry.timer = time.AfterFunc(s.ttl, func() {
		s.mu.Lock()
		if _, ok := s.pending[promptID]; !ok {
			s.mu.Unlock()
			return
		}
		delete(s.pending, promptID)
		s.mu.Unlock()
		if args.OnForfeit != nil {
			args.OnForfeit(context.Background())
		}
	})

	s.mu.Lock()
	s.pending[promptID] = entry
	s.mu.Unlock()
	return promptID, nil
}

// HandleComponent processes a button click. Returns true if the customID was
// in the reaction-prompt namespace (so the router knows the click was claimed).
// A claimed-but-stale click (timer already won) returns true and does nothing.
func (s *ReactionPromptStore) HandleComponent(interaction *discordgo.Interaction) bool {
	data, ok := interaction.Data.(discordgo.MessageComponentInteractionData)
	if !ok {
		return false
	}
	if !strings.HasPrefix(data.CustomID, reactionPromptPrefix+":") {
		return false
	}
	rest := strings.TrimPrefix(data.CustomID, reactionPromptPrefix+":")
	idPart, choice, found := strings.Cut(rest, ":")
	if !found {
		return true
	}
	promptID, err := uuid.Parse(idPart)
	if err != nil {
		return true
	}

	s.mu.Lock()
	entry, ok := s.pending[promptID]
	if !ok {
		s.mu.Unlock()
		return true
	}
	delete(s.pending, promptID)
	s.mu.Unlock()

	entry.timer.Stop()
	entry.onChoice(context.Background(), interaction, choice)
	return true
}

// Cancel removes a pending prompt and stops its forfeit timer. Use this when
// the same logical action has been resolved through another code path (e.g. a
// DM cancelled the encounter before the timeout elapsed).
func (s *ReactionPromptStore) Cancel(promptID uuid.UUID) {
	s.mu.Lock()
	entry, ok := s.pending[promptID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.pending, promptID)
	s.mu.Unlock()
	entry.timer.Stop()
}

func buildPromptComponents(promptID uuid.UUID, buttons []ReactionPromptButton) []discordgo.MessageComponent {
	row := make([]discordgo.MessageComponent, 0, len(buttons))
	for _, b := range buttons {
		style := b.Style
		if style == 0 {
			style = discordgo.PrimaryButton
		}
		row = append(row, discordgo.Button{
			Label:    b.Label,
			Style:    style,
			CustomID: fmt.Sprintf("%s:%s:%s", reactionPromptPrefix, promptID.String(), b.Choice),
		})
	}
	return []discordgo.MessageComponent{discordgo.ActionsRow{Components: row}}
}
