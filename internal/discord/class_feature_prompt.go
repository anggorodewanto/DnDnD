package discord

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// ClassFeaturePromptPoster posts the four class-feature reaction prompts
// driven by ProcessorResult.ResourceTriggers (combat/effect.go:374):
//
//   - Stunning Strike    (Monk, after melee hit, 1 Ki to attempt)
//   - Divine Smite       (Paladin, after melee hit, pick a spell slot)
//   - Uncanny Dodge      (Rogue, after taking damage, halve)
//   - Bardic Inspiration (holder, 30s usage window on attack/save/check)
//
// Each Prompt* method posts one prompt and invokes onResolved exactly once,
// either with the chosen result or with Forfeited=true on TTL expiry. The
// actual mechanic application (deducting Ki, rolling smite, halving damage)
// is left to the caller's closure so this layer stays free of service deps.
type ClassFeaturePromptPoster struct {
	prompts *ReactionPromptStore
}

// NewClassFeaturePromptPoster wires the in-memory prompt store.
func NewClassFeaturePromptPoster(prompts *ReactionPromptStore) *ClassFeaturePromptPoster {
	return &ClassFeaturePromptPoster{prompts: prompts}
}

// StunningStrikePromptArgs are the per-invocation inputs for the Stunning
// Strike prompt.
type StunningStrikePromptArgs struct {
	ChannelID    string
	AttackerName string
	TargetName   string
	KiAvailable  int
}

// StunningStrikePromptResult signals whether the monk spent Ki or the prompt
// forfeited (timed out).
type StunningStrikePromptResult struct {
	UseKi     bool
	Forfeited bool
}

// PromptStunningStrike posts [Use Ki] / [Skip] after a Monk melee hit.
func (p *ClassFeaturePromptPoster) PromptStunningStrike(args StunningStrikePromptArgs, onResolved func(StunningStrikePromptResult)) error {
	buttons := []ReactionPromptButton{
		{Label: fmt.Sprintf("Use Ki (%d left)", args.KiAvailable), Choice: "use"},
		{Label: "Skip", Choice: "skip", Style: discordgo.SecondaryButton},
	}
	content := fmt.Sprintf("⚡ **Stunning Strike?** %s hit %s. Spend 1 ki to attempt to stun?", args.AttackerName, args.TargetName)
	return p.postBinary(args.ChannelID, content, buttons, func(choice string, forfeit bool) {
		onResolved(StunningStrikePromptResult{UseKi: choice == "use", Forfeited: forfeit})
	})
}

// DivineSmitePromptArgs are the per-invocation inputs for the Divine Smite
// prompt.
type DivineSmitePromptArgs struct {
	ChannelID      string
	AttackerName   string
	TargetName     string
	AvailableSlots []int
}

// DivineSmitePromptResult is the player's choice — either a slot level or
// Skip / Forfeit.
type DivineSmitePromptResult struct {
	UseSlot   bool
	SlotLevel int
	Forfeited bool
}

// PromptDivineSmite posts one button per available slot level + a [Skip]
// after a Paladin melee hit.
func (p *ClassFeaturePromptPoster) PromptDivineSmite(args DivineSmitePromptArgs, onResolved func(DivineSmitePromptResult)) error {
	if len(args.AvailableSlots) == 0 {
		return fmt.Errorf("PromptDivineSmite: no available slots — at least one required")
	}
	buttons := make([]ReactionPromptButton, 0, len(args.AvailableSlots)+1)
	for _, lvl := range args.AvailableSlots {
		buttons = append(buttons, ReactionPromptButton{
			Label:  fmt.Sprintf("Slot %d", lvl),
			Choice: strconv.Itoa(lvl),
		})
	}
	buttons = append(buttons, ReactionPromptButton{
		Label:  "Skip",
		Choice: "skip",
		Style:  discordgo.SecondaryButton,
	})
	content := fmt.Sprintf("⚡ **Divine Smite?** %s hit %s. Pick a slot or skip.", args.AttackerName, args.TargetName)
	_, err := p.prompts.Post(ReactionPromptPostArgs{
		ChannelID: args.ChannelID,
		Content:   content,
		Buttons:   buttons,
		OnChoice: func(_ context.Context, _ *discordgo.Interaction, choice string) {
			if choice == "skip" {
				onResolved(DivineSmitePromptResult{})
				return
			}
			lvl, parseErr := strconv.Atoi(choice)
			if parseErr != nil {
				return
			}
			onResolved(DivineSmitePromptResult{UseSlot: true, SlotLevel: lvl})
		},
		OnForfeit: func(context.Context) {
			onResolved(DivineSmitePromptResult{Forfeited: true})
		},
	})
	return err
}

// UncannyDodgePromptArgs are the per-invocation inputs for the Uncanny Dodge
// prompt.
type UncannyDodgePromptArgs struct {
	ChannelID      string
	DefenderName   string
	AttackerName   string
	IncomingDamage int
}

// UncannyDodgePromptResult — Halve true means apply ApplyUncannyDodge.
type UncannyDodgePromptResult struct {
	Halve     bool
	Forfeited bool
}

// PromptUncannyDodge posts [Halve] / [Take Full] after a Rogue takes damage.
func (p *ClassFeaturePromptPoster) PromptUncannyDodge(args UncannyDodgePromptArgs, onResolved func(UncannyDodgePromptResult)) error {
	buttons := []ReactionPromptButton{
		{Label: fmt.Sprintf("Halve (%d → %d)", args.IncomingDamage, args.IncomingDamage/2), Choice: "halve"},
		{Label: "Take Full", Choice: "full", Style: discordgo.SecondaryButton},
	}
	content := fmt.Sprintf("🌀 **Uncanny Dodge?** %s hit by %s for %d damage. Halve via reaction?", args.DefenderName, args.AttackerName, args.IncomingDamage)
	return p.postBinary(args.ChannelID, content, buttons, func(choice string, forfeit bool) {
		onResolved(UncannyDodgePromptResult{Halve: choice == "halve", Forfeited: forfeit})
	})
}

// BardicInspirationPromptArgs are the per-invocation inputs for the BI usage
// prompt (30 s window per spec line 1102).
type BardicInspirationPromptArgs struct {
	ChannelID  string
	HolderName string
	Die        string
	Context    string // e.g. "attack roll", "saving throw"
}

// BardicInspirationPromptResult — UseDie true means consume the BI die.
type BardicInspirationPromptResult struct {
	UseDie    bool
	Forfeited bool
}

// PromptBardicInspiration posts [Use Die] / [Keep] with the 30 s window.
// The prompt store's DefaultReactionPromptTTL already matches the 30 s spec.
func (p *ClassFeaturePromptPoster) PromptBardicInspiration(args BardicInspirationPromptArgs, onResolved func(BardicInspirationPromptResult)) error {
	buttons := []ReactionPromptButton{
		{Label: fmt.Sprintf("Use %s", args.Die), Choice: "use"},
		{Label: "Keep", Choice: "keep", Style: discordgo.SecondaryButton},
	}
	content := fmt.Sprintf("🎵 **Bardic Inspiration?** %s — add %s to your %s?", args.HolderName, args.Die, args.Context)
	return p.postBinary(args.ChannelID, content, buttons, func(choice string, forfeit bool) {
		onResolved(BardicInspirationPromptResult{UseDie: choice == "use", Forfeited: forfeit})
	})
}

// postBinary is the shared two-button post + callback shape.
func (p *ClassFeaturePromptPoster) postBinary(channelID, content string, buttons []ReactionPromptButton, onResolved func(choice string, forfeit bool)) error {
	_, err := p.prompts.Post(ReactionPromptPostArgs{
		ChannelID: channelID,
		Content:   content,
		Buttons:   buttons,
		OnChoice: func(_ context.Context, _ *discordgo.Interaction, choice string) {
			onResolved(choice, false)
		},
		OnForfeit: func(context.Context) {
			onResolved("", true)
		},
	})
	return err
}
