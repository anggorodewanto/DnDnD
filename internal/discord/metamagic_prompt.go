package discord

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// MetamagicPromptPoster posts the three interactive metamagic prompts:
// Empowered (per-die reroll), Careful (per-creature protect), Heightened
// (per-creature target). The non-interactive metamagic options (Distant,
// Extended, Subtle, Quickened, Twinned) are resolved at validation time and
// have no prompt path. Each method posts one prompt and invokes onResolved
// exactly once — either with the chosen index or with Forfeited=true on the
// TTL expiry. Callers compose multi-selection (the max-N case) by re-posting
// after each callback fires.
type MetamagicPromptPoster struct {
	prompts *ReactionPromptStore
}

// NewMetamagicPromptPoster wires the in-memory prompt store.
func NewMetamagicPromptPoster(prompts *ReactionPromptStore) *MetamagicPromptPoster {
	return &MetamagicPromptPoster{prompts: prompts}
}

// EmpoweredPromptArgs are the per-invocation inputs for the Empowered Spell
// prompt — the raw damage dice and the per-cast reroll cap (CHA mod, min 1).
type EmpoweredPromptArgs struct {
	ChannelID  string
	SpellName  string
	DiceRolls  []int
	MaxRerolls int
}

// EmpoweredPromptResult carries the player's selected die index back. If
// Forfeited is true SelectedIndex is meaningless.
type EmpoweredPromptResult struct {
	SelectedIndex int
	Forfeited     bool
}

// PromptEmpowered posts one button per damage die. The button label includes
// the rolled value so the player can pick the worst-rolling dice for reroll.
// Caller composes "up to N rerolls" by re-posting after each onResolved.
func (p *MetamagicPromptPoster) PromptEmpowered(args EmpoweredPromptArgs, onResolved func(EmpoweredPromptResult)) error {
	if len(args.DiceRolls) == 0 {
		return fmt.Errorf("PromptEmpowered: at least one die roll required")
	}
	buttons := make([]ReactionPromptButton, 0, len(args.DiceRolls))
	for i, v := range args.DiceRolls {
		buttons = append(buttons, ReactionPromptButton{
			Label:  fmt.Sprintf("Die %d (%d)", i+1, v),
			Choice: strconv.Itoa(i),
		})
	}
	content := fmt.Sprintf("✨ **Empowered Spell** (%s): pick a die to reroll (up to %d total).", args.SpellName, args.MaxRerolls)
	return p.postIndexed(args.ChannelID, content, buttons, func(idx int, forfeit bool) {
		onResolved(EmpoweredPromptResult{SelectedIndex: idx, Forfeited: forfeit})
	})
}

// CarefulPromptArgs are the per-invocation inputs for the Careful Spell prompt
// — the AoE targets and the per-cast protection cap (CHA mod, min 1).
type CarefulPromptArgs struct {
	ChannelID    string
	SpellName    string
	TargetNames  []string
	MaxProtected int
}

// CarefulPromptResult carries the player's selected target back.
type CarefulPromptResult struct {
	SelectedIndex int
	Forfeited     bool
}

// PromptCareful posts one button per AoE target. The player picks who auto-
// succeeds on the save; caller composes the multi-pick by re-posting.
func (p *MetamagicPromptPoster) PromptCareful(args CarefulPromptArgs, onResolved func(CarefulPromptResult)) error {
	if len(args.TargetNames) == 0 {
		return fmt.Errorf("PromptCareful: at least one target name required")
	}
	buttons := buildTargetButtons(args.TargetNames)
	content := fmt.Sprintf("✨ **Careful Spell** (%s): pick a creature to auto-succeed (up to %d total).", args.SpellName, args.MaxProtected)
	return p.postIndexed(args.ChannelID, content, buttons, func(idx int, forfeit bool) {
		onResolved(CarefulPromptResult{SelectedIndex: idx, Forfeited: forfeit})
	})
}

// HeightenedPromptArgs are the per-invocation inputs for the Heightened Spell
// prompt — the candidate targets the disadvantage can land on.
type HeightenedPromptArgs struct {
	ChannelID   string
	SpellName   string
	TargetNames []string
}

// HeightenedPromptResult carries the player's selected target back.
type HeightenedPromptResult struct {
	SelectedIndex int
	Forfeited     bool
}

// PromptHeightened posts one button per candidate target. The selected target
// has disadvantage on the first save of the spell.
func (p *MetamagicPromptPoster) PromptHeightened(args HeightenedPromptArgs, onResolved func(HeightenedPromptResult)) error {
	if len(args.TargetNames) == 0 {
		return fmt.Errorf("PromptHeightened: at least one target name required")
	}
	buttons := buildTargetButtons(args.TargetNames)
	content := fmt.Sprintf("✨ **Heightened Spell** (%s): pick a target for disadvantage on the first save.", args.SpellName)
	return p.postIndexed(args.ChannelID, content, buttons, func(idx int, forfeit bool) {
		onResolved(HeightenedPromptResult{SelectedIndex: idx, Forfeited: forfeit})
	})
}

// postIndexed is the shared shape: post the prompt, parse the selected index
// from the choice string, and invoke the callback. forfeit=true means the
// timer fired and the player did not click.
func (p *MetamagicPromptPoster) postIndexed(channelID, content string, buttons []ReactionPromptButton, onResolved func(idx int, forfeit bool)) error {
	_, err := p.prompts.Post(ReactionPromptPostArgs{
		ChannelID: channelID,
		Content:   content,
		Buttons:   buttons,
		OnChoice: func(_ context.Context, _ *discordgo.Interaction, choice string) {
			idx, parseErr := strconv.Atoi(choice)
			if parseErr != nil {
				return
			}
			onResolved(idx, false)
		},
		OnForfeit: func(context.Context) {
			onResolved(0, true)
		},
	})
	return err
}

func buildTargetButtons(names []string) []ReactionPromptButton {
	buttons := make([]ReactionPromptButton, 0, len(names))
	for i, name := range names {
		buttons = append(buttons, ReactionPromptButton{
			Label:  name,
			Choice: strconv.Itoa(i),
		})
	}
	return buttons
}
