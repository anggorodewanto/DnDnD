package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/ab/dndnd/internal/dice"
)

// Roll guardrails: a freeform roller is player-facing, so a stray
// `/roll 999999d999999` must not spin the process. These caps bound the work
// without getting in the way of real D&D rolls (the biggest legitimate roll —
// rolling a stat array — is 4d6, and the fattest die is a d100).
const (
	maxRollDiceCount = 100
	maxRollDieSides  = 1000
)

// rollExampleExprs is the single source of truth for the dice expressions
// advertised to players. Every entry is asserted parseable by
// TestAdvertisedRollExamples_AllParse, so the forms shown in help can never
// drift from what ParseExpression accepts (the /roll d20 regression, e967364).
var rollExampleExprs = []string{"1d20+4", "2d6", "d20", "4d6+2"}

// rollExamples is appended to error replies so a player who fat-fingers an
// expression sees valid forms immediately. Built from rollExampleExprs so the
// advertised forms stay in lockstep with the tested set.
var rollExamples = "try " + joinRollExamples(rollExampleExprs) + "."

// joinRollExamples renders the examples as a backtick-quoted, comma-separated
// list with an Oxford "or" before the last (e.g. "`a`, `b`, or `c`").
func joinRollExamples(exprs []string) string {
	quoted := make([]string, len(exprs))
	for i, e := range exprs {
		quoted[i] = "`" + e + "`"
	}
	if len(quoted) < 2 {
		return strings.Join(quoted, "")
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + ", or " + quoted[len(quoted)-1]
}

// rollUsageHelp is shown when no dice expression is supplied.
const rollUsageHelp = "🎲 Tell me what to roll — e.g. `/roll dice:1d20+4` or `/roll dice:2d6 reason:fire damage`."

// RollHandler handles the /roll slash command — a freeform dice roller that
// lets any player roll arbitrary dice ("real D&D feel") and posts the result
// to #roll-history for the whole table. Unlike /check it does NOT require a
// registered character: the campaign + character lookups only enrich the
// roller label (and route the #roll-history post); a player with no character
// still gets their roll under their Discord display name.
type RollHandler struct {
	session          Session
	roller           *dice.Roller
	campaignProvider CheckCampaignProvider
	characterLookup  CheckCharacterLookup
	rollLogger       dice.RollHistoryLogger
}

// HasRollLogger reports whether a non-nil dice.RollHistoryLogger has been
// wired. /roll's whole point is the #roll-history post, so the production-
// wiring test asserts this is true to catch a nil-logger silent no-op (the
// same Phase 18 regression guarded for /check and /save).
func (h *RollHandler) HasRollLogger() bool { return h.rollLogger != nil }

// NewRollHandler creates a RollHandler. campaignProvider, characterLookup, and
// rollLogger are all best-effort and may be nil — the roll itself never
// depends on them.
func NewRollHandler(
	session Session,
	roller *dice.Roller,
	campaignProvider CheckCampaignProvider,
	characterLookup CheckCharacterLookup,
	rollLogger dice.RollHistoryLogger,
) *RollHandler {
	return &RollHandler{
		session:          session,
		roller:           roller,
		campaignProvider: campaignProvider,
		characterLookup:  characterLookup,
		rollLogger:       rollLogger,
	}
}

// Handle processes a /roll interaction.
func (h *RollHandler) Handle(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	expr, reason := h.parseOptions(data.Options)
	expr = strings.TrimSpace(expr)
	reason = strings.TrimSpace(reason)

	if expr == "" {
		respondEphemeral(h.session, interaction, rollUsageHelp)
		return
	}

	parsed, err := dice.ParseExpression(expr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("🎲 Couldn't read `%s` — %s", expr, rollExamples))
		return
	}
	if msg, ok := rollWithinLimits(parsed); !ok {
		respondEphemeral(h.session, interaction, msg)
		return
	}

	// ParseExpression already succeeded, so Roll's internal re-parse can't
	// fail; guard defensively anyway so a future parser divergence degrades
	// to a friendly message rather than a blank reply.
	result, err := h.roller.Roll(expr)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("🎲 Couldn't read `%s` — %s", expr, rollExamples))
		return
	}

	roller := h.resolveRollerName(interaction)
	h.logRoll(roller, reason, result)
	respondEphemeral(h.session, interaction, formatRollResponse(roller, reason, result))
}

// parseOptions pulls the dice expression and optional reason from the command
// options.
func (h *RollHandler) parseOptions(opts []*discordgo.ApplicationCommandInteractionDataOption) (expr, reason string) {
	for _, opt := range opts {
		switch opt.Name {
		case "dice":
			expr = opt.StringValue()
		case "reason":
			reason = opt.StringValue()
		}
	}
	return expr, reason
}

// rollWithinLimits enforces the dice/sides caps. Returns a player-facing
// message and false when the expression exceeds a cap.
func rollWithinLimits(expr dice.Expression) (string, bool) {
	total := 0
	for _, g := range expr.Groups {
		if g.Sides > maxRollDieSides {
			return fmt.Sprintf("🎲 That die has too many sides — max d%d.", maxRollDieSides), false
		}
		total += g.Count
	}
	if total > maxRollDiceCount {
		return fmt.Sprintf("🎲 That's too many dice at once — max %d.", maxRollDiceCount), false
	}
	return "", true
}

// resolveRollerName returns the best label for the roller: the player's
// character name when it resolves, otherwise their Discord display name. The
// character name is also what routes the #roll-history post to the right
// campaign (the by-roller logger keys on it).
func (h *RollHandler) resolveRollerName(interaction *discordgo.Interaction) string {
	if name, ok := h.lookupCharacterName(interaction); ok {
		return name
	}
	return rollerDisplayName(interaction)
}

// lookupCharacterName best-effort resolves the invoking player's character
// name. ok=false on any missing wiring or lookup miss (the caller then falls
// back to the Discord display name).
func (h *RollHandler) lookupCharacterName(interaction *discordgo.Interaction) (string, bool) {
	if h.campaignProvider == nil || h.characterLookup == nil {
		return "", false
	}
	ctx := context.Background()
	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		return "", false
	}
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, discordUserID(interaction))
	if err != nil || char.Name == "" {
		return "", false
	}
	return char.Name, true
}

// rollerDisplayName derives a human label from the interaction when no
// character resolves: the guild nickname, then the username, then a generic
// fallback so the reply is never blank.
func rollerDisplayName(interaction *discordgo.Interaction) string {
	if interaction.Member != nil {
		if interaction.Member.Nick != "" {
			return interaction.Member.Nick
		}
		if interaction.Member.User != nil && interaction.Member.User.Username != "" {
			return interaction.Member.User.Username
		}
	}
	if interaction.User != nil && interaction.User.Username != "" {
		return interaction.User.Username
	}
	return "Someone"
}

// logRoll posts the roll to #roll-history when a logger is wired. Purpose
// defaults to "roll" so the audit line reads cleanly when the player gave no
// reason.
func (h *RollHandler) logRoll(roller, reason string, result dice.RollResult) {
	if h.rollLogger == nil {
		return
	}
	purpose := reason
	if purpose == "" {
		purpose = "roll"
	}
	_ = h.rollLogger.LogRoll(dice.RollLogEntry{
		DiceRolls:  result.Groups,
		Total:      result.Total,
		Expression: result.Expression,
		Roller:     roller,
		Purpose:    purpose,
		Breakdown:  result.Breakdown,
		Timestamp:  result.Timestamp,
	})
}

// formatRollResponse renders the ephemeral confirmation shown to the roller.
// result.Breakdown already ends in "= total", so the leading bold total is a
// quick read with the full breakdown underneath.
func formatRollResponse(roller, reason string, result dice.RollResult) string {
	if reason != "" {
		return fmt.Sprintf("🎲 **%s** — *%s*: **%d**\n`%s` → %s", roller, reason, result.Total, result.Expression, result.Breakdown)
	}
	return fmt.Sprintf("🎲 **%s** rolled **%d**\n`%s` → %s", roller, result.Total, result.Expression, result.Breakdown)
}
