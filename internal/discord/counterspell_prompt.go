package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
)

// CounterspellService is the slice of *combat.Service the Counterspell prompt
// poster needs. *combat.Service satisfies it structurally; tests inject a mock.
type CounterspellService interface {
	TriggerCounterspell(ctx context.Context, declarationID uuid.UUID, enemySpellName string, enemyCastLevel int, isSubtle bool) (combat.CounterspellPrompt, error)
	ResolveCounterspell(ctx context.Context, declarationID uuid.UUID, slotLevel int) (combat.CounterspellResult, error)
	PassCounterspell(ctx context.Context, declarationID uuid.UUID) (combat.CounterspellResult, error)
	ForfeitCounterspell(ctx context.Context, declarationID uuid.UUID) (combat.CounterspellResult, error)
}

// CounterspellPromptPoster bridges the Counterspell service to a Discord
// prompt: it triggers the prompt on the service, posts the slot-level buttons
// (plus a [Pass] button) to the player's channel, and routes the click — or
// the 30s timeout — back to the matching resolve / pass / forfeit service
// method. Posting the in-Discord prompt is required for med-29 (the
// underlying mechanics already landed in d6c755a).
type CounterspellPromptPoster struct {
	svc     CounterspellService
	prompts *ReactionPromptStore
	session Session
}

// NewCounterspellPromptPoster wires the service, the in-memory prompt store,
// and the session for posting follow-up combat-log messages.
func NewCounterspellPromptPoster(svc CounterspellService, prompts *ReactionPromptStore, session Session) *CounterspellPromptPoster {
	return &CounterspellPromptPoster{svc: svc, prompts: prompts, session: session}
}

// CounterspellPromptArgs collects the per-call inputs the poster needs.
type CounterspellPromptArgs struct {
	DeclarationID  uuid.UUID
	EnemySpellName string
	EnemyCastLevel int
	IsSubtle       bool
	ChannelID      string
}

// Trigger calls TriggerCounterspell on the service. On success the slot-level
// buttons + [Pass] are posted to ChannelID. On ErrSubtleSpellNotCounterspellable
// a one-line info message is posted instead. Any other error bubbles up.
func (p *CounterspellPromptPoster) Trigger(ctx context.Context, args CounterspellPromptArgs) error {
	prompt, err := p.svc.TriggerCounterspell(ctx, args.DeclarationID, args.EnemySpellName, args.EnemyCastLevel, args.IsSubtle)
	if errors.Is(err, combat.ErrSubtleSpellNotCounterspellable) {
		_, _ = p.session.ChannelMessageSend(args.ChannelID, fmt.Sprintf("\U0001f910 %s was cast subtly — Counterspell cannot trigger.", args.EnemySpellName))
		return nil
	}
	if err != nil {
		return err
	}

	buttons := make([]ReactionPromptButton, 0, len(prompt.AvailableSlots)+1)
	for _, lvl := range prompt.AvailableSlots {
		buttons = append(buttons, ReactionPromptButton{
			Label:  fmt.Sprintf("Slot %d", lvl),
			Choice: strconv.Itoa(lvl),
		})
	}
	buttons = append(buttons, ReactionPromptButton{
		Label:  "Pass",
		Choice: "pass",
		Style:  discordgo.SecondaryButton,
	})

	declID := prompt.DeclarationID
	content := fmt.Sprintf("✨ **Counterspell?** %s is casting **%s**. Choose a slot or pass.", prompt.CasterName, prompt.EnemySpellName)
	_, postErr := p.prompts.Post(ReactionPromptPostArgs{
		ChannelID: args.ChannelID,
		Content:   content,
		Buttons:   buttons,
		OnChoice: func(c context.Context, _ *discordgo.Interaction, choice string) {
			if choice == "pass" {
				p.resolvePass(c, args.ChannelID, declID)
				return
			}
			lvl, parseErr := strconv.Atoi(choice)
			if parseErr != nil {
				return
			}
			p.resolveSlot(c, args.ChannelID, declID, lvl)
		},
		OnForfeit: func(c context.Context) {
			p.resolveForfeit(c, args.ChannelID, declID)
		},
	})
	return postErr
}

func (p *CounterspellPromptPoster) resolveSlot(ctx context.Context, channelID string, declID uuid.UUID, lvl int) {
	res, err := p.svc.ResolveCounterspell(ctx, declID, lvl)
	if err != nil {
		_, _ = p.session.ChannelMessageSend(channelID, fmt.Sprintf("Counterspell resolve failed: %v", err))
		return
	}
	_, _ = p.session.ChannelMessageSend(channelID, combat.FormatCounterspellLog(res))
}

func (p *CounterspellPromptPoster) resolvePass(ctx context.Context, channelID string, declID uuid.UUID) {
	res, err := p.svc.PassCounterspell(ctx, declID)
	if err != nil {
		_, _ = p.session.ChannelMessageSend(channelID, fmt.Sprintf("Counterspell pass failed: %v", err))
		return
	}
	_, _ = p.session.ChannelMessageSend(channelID, combat.FormatCounterspellLog(res))
}

func (p *CounterspellPromptPoster) resolveForfeit(ctx context.Context, channelID string, declID uuid.UUID) {
	res, err := p.svc.ForfeitCounterspell(ctx, declID)
	if err != nil {
		return
	}
	_, _ = p.session.ChannelMessageSend(channelID, combat.FormatCounterspellLog(res))
}

// Compile-time interface satisfaction check — *combat.Service implements
// CounterspellService.
var _ CounterspellService = (*combat.Service)(nil)
