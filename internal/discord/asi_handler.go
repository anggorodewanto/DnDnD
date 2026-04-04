package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/ab/dndnd/internal/character"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

const (
	asiChoicePrefix  = "asi_choice"
	asiSelectPrefix  = "asi_select"
	asiApprovePrefix = "asi_approve"
	asiDenyPrefix    = "asi_deny"
)

// abilityOrder defines the standard display order for ability scores.
var abilityOrder = []struct {
	Key   string
	Label string
}{
	{"str", "STR"},
	{"dex", "DEX"},
	{"con", "CON"},
	{"int", "INT"},
	{"wis", "WIS"},
	{"cha", "CHA"},
}

// BuildASIPromptComponents creates the Discord button components for the ASI/Feat choice prompt.
// Returns three buttons: +2 to One Score, +1 to Two Scores, Choose a Feat.
func BuildASIPromptComponents(charID uuid.UUID) []discordgo.MessageComponent {
	prefix := fmt.Sprintf("%s:%s", asiChoicePrefix, charID.String())
	return []discordgo.MessageComponent{
		&discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "+2 to One Score",
					Style:    discordgo.PrimaryButton,
					CustomID: prefix + ":plus2",
					Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f4aa"},
				},
				discordgo.Button{
					Label:    "+1 to Two Scores",
					Style:    discordgo.PrimaryButton,
					CustomID: prefix + ":plus1plus1",
					Emoji:    &discordgo.ComponentEmoji{Name: "\u2696\ufe0f"},
				},
				discordgo.Button{
					Label:    "Choose a Feat",
					Style:    discordgo.SecondaryButton,
					CustomID: prefix + ":feat",
					Emoji:    &discordgo.ComponentEmoji{Name: "\U0001f4d6"},
				},
			},
		},
	}
}

// ParseASIChoiceCustomID parses a custom ID like "asi_choice:<charID>:<type>".
func ParseASIChoiceCustomID(customID string) (uuid.UUID, string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 || parts[0] != asiChoicePrefix {
		return uuid.Nil, "", fmt.Errorf("invalid ASI choice custom ID: %s", customID)
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid character ID: %w", err)
	}
	return charID, parts[2], nil
}

// ParseASISelectCustomID parses a custom ID like "asi_select:<charID>:<type>".
func ParseASISelectCustomID(customID string) (uuid.UUID, string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 || parts[0] != asiSelectPrefix {
		return uuid.Nil, "", fmt.Errorf("invalid ASI select custom ID: %s", customID)
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid character ID: %w", err)
	}
	return charID, parts[2], nil
}

// BuildAbilitySelectMenu creates a select menu for choosing ability scores to increase.
// For "plus2", shows each ability with current -> new (+2) values. Excludes abilities at 20.
// For "plus1plus1", shows each ability with current -> new (+1) values. Excludes abilities at 20.
func BuildAbilitySelectMenu(charID uuid.UUID, asiType string, scores character.AbilityScores) []discordgo.MessageComponent {
	customID := fmt.Sprintf("%s:%s:%s", asiSelectPrefix, charID.String(), asiType)
	bonus := 2
	if asiType == "plus1plus1" {
		bonus = 1
	}

	var options []discordgo.SelectMenuOption
	for _, ab := range abilityOrder {
		current := scores.Get(ab.Key)
		if current >= 20 {
			continue
		}
		newVal := current + bonus
		if newVal > 20 {
			newVal = 20
		}
		options = append(options, discordgo.SelectMenuOption{
			Label: fmt.Sprintf("%s (%d -> %d)", ab.Label, current, newVal),
			Value: ab.Key,
		})
	}

	maxValues := 1
	if asiType == "plus1plus1" {
		maxValues = 2
	}
	minValues := func() *int { v := maxValues; return &v }()

	return []discordgo.MessageComponent{
		&discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    customID,
					Placeholder: "Select ability score(s)",
					MinValues:   minValues,
					MaxValues:   maxValues,
					Options:     options,
				},
			},
		},
	}
}

// BuildDMApprovalComponents creates approve/deny buttons for the DM queue.
func BuildDMApprovalComponents(charID uuid.UUID) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		&discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Approve",
					Style:    discordgo.SuccessButton,
					CustomID: fmt.Sprintf("%s:%s", asiApprovePrefix, charID.String()),
					Emoji:    &discordgo.ComponentEmoji{Name: "\u2705"},
				},
				discordgo.Button{
					Label:    "Deny",
					Style:    discordgo.DangerButton,
					CustomID: fmt.Sprintf("%s:%s", asiDenyPrefix, charID.String()),
					Emoji:    &discordgo.ComponentEmoji{Name: "\u274c"},
				},
			},
		},
	}
}

// FormatDMQueueASIMessage formats the DM queue message for an ASI/Feat choice.
func FormatDMQueueASIMessage(characterName, classInfo, choiceDescription string) string {
	return fmt.Sprintf("\U0001f393 **ASI/Feat** -- %s (%s) chose: %s", characterName, classInfo, choiceDescription)
}

// ParseDMApprovalCustomID parses a custom ID like "asi_approve:<charID>" or "asi_deny:<charID>".
func ParseDMApprovalCustomID(customID string) (uuid.UUID, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 2 {
		return uuid.Nil, fmt.Errorf("invalid DM approval custom ID: %s", customID)
	}
	if parts[0] != asiApprovePrefix && parts[0] != asiDenyPrefix {
		return uuid.Nil, fmt.Errorf("invalid DM approval prefix: %s", parts[0])
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid character ID: %w", err)
	}
	return charID, nil
}

// ASIChoiceData holds the data for an ASI choice to be approved.
type ASIChoiceData struct {
	Type     string   `json:"type"`
	Ability  string   `json:"ability,omitempty"`
	Ability2 string   `json:"ability2,omitempty"`
	FeatID   string   `json:"feat_id,omitempty"`
}

// ASICharacterData holds the character data needed for ASI interactions.
type ASICharacterData struct {
	ID            uuid.UUID
	Name          string
	DiscordUserID string
	Scores        character.AbilityScores
	ClassInfo     string
}

// ASIService is the interface the ASI handler uses to interact with the level-up service.
type ASIService interface {
	ApproveASI(ctx context.Context, charID uuid.UUID, choice ASIChoiceData) error
	DenyASI(ctx context.Context, charID uuid.UUID, reason string) error
	GetCharacterForASI(ctx context.Context, charID uuid.UUID) (*ASICharacterData, error)
}

// PendingASIChoice holds a player's ASI/Feat choice awaiting DM approval.
type PendingASIChoice struct {
	CharID      uuid.UUID
	ASIType     string
	Abilities   []string
	FeatID      string
	PlayerID    string
	Description string
}

// ASIHandler processes Discord component interactions for the ASI/Feat flow.
type ASIHandler struct {
	session     Session
	service     ASIService
	dmQueueFunc func(guildID string) string

	mu      sync.RWMutex
	pending map[uuid.UUID]PendingASIChoice
}

// NewASIHandler creates a new ASIHandler.
func NewASIHandler(session Session, service ASIService, dmQueueFunc func(guildID string) string) *ASIHandler {
	return &ASIHandler{
		session:     session,
		service:     service,
		dmQueueFunc: dmQueueFunc,
		pending:     make(map[uuid.UUID]PendingASIChoice),
	}
}

func (h *ASIHandler) storePendingChoice(charID uuid.UUID, choice PendingASIChoice) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pending[charID] = choice
}

func (h *ASIHandler) getPendingChoice(charID uuid.UUID) (PendingASIChoice, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.pending[charID]
	return c, ok
}

func (h *ASIHandler) removePendingChoice(charID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.pending, charID)
}

// HandleASIChoice handles a button click for +2/+1+1/feat selection.
func (h *ASIHandler) HandleASIChoice(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, asiType, err := ParseASIChoiceCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid ASI choice: %v", err))
		return
	}

	charData, err := h.service.GetCharacterForASI(context.Background(), charID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not load character data.")
		return
	}

	if asiType == "feat" {
		// Feat path: for now, respond with placeholder (feat select menu is a future enhancement)
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Feat selection for **%s** is not yet available. Please use the dashboard or ask your DM.", charData.Name),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// ASI path: show ability score select menu
	components := BuildAbilitySelectMenu(charID, asiType, charData.Scores)
	label := "+2 to one ability score"
	if asiType == "plus1plus1" {
		label = "+1 to two ability scores"
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("\U0001f3af Select %s for **%s**:", label, charData.Name),
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// HandleASISelect handles the ability score select menu submission.
func (h *ASIHandler) HandleASISelect(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, asiType, err := ParseASISelectCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid selection: %v", err))
		return
	}

	abilities := data.Values
	if len(abilities) == 0 {
		respondEphemeral(h.session, interaction, "No abilities selected.")
		return
	}

	charData, err := h.service.GetCharacterForASI(context.Background(), charID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not load character data.")
		return
	}

	// Build description for DM queue
	description := buildASIDescription(asiType, abilities, charData.Scores)

	// Acknowledge the interaction
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("\u23f3 Your choice (%s) has been sent to the DM for approval.", description),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// Store pending choice
	playerID := discordUserID(interaction)
	pending := PendingASIChoice{
		CharID:      charID,
		ASIType:     asiType,
		Abilities:   abilities,
		PlayerID:    playerID,
		Description: description,
	}
	h.storePendingChoice(charID, pending)

	// Post to DM queue
	if h.dmQueueFunc == nil {
		return
	}
	channelID := h.dmQueueFunc(interaction.GuildID)
	if channelID == "" {
		return
	}

	msg := FormatDMQueueASIMessage(charData.Name, charData.ClassInfo, description)
	components := BuildDMApprovalComponents(charID)

	_, err = h.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:    msg,
		Components: components,
	})
	if err != nil {
		slog.Error("failed to post ASI choice to DM queue", "error", err, "character", charData.Name)
	}
}

// HandleDMApprove handles the DM clicking the Approve button.
func (h *ASIHandler) HandleDMApprove(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, err := ParseDMApprovalCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid approval data: %v", err))
		return
	}

	pending, ok := h.getPendingChoice(charID)
	if !ok {
		respondEphemeral(h.session, interaction, "No pending ASI choice found for this character.")
		return
	}

	// Acknowledge
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	// Build the choice data
	choice := ASIChoiceData{Type: pending.ASIType}
	if len(pending.Abilities) > 0 {
		choice.Ability = pending.Abilities[0]
	}
	if len(pending.Abilities) > 1 {
		choice.Ability2 = pending.Abilities[1]
	}
	if pending.FeatID != "" {
		choice.FeatID = pending.FeatID
	}

	err = h.service.ApproveASI(context.Background(), charID, choice)
	if err != nil {
		slog.Error("failed to approve ASI", "error", err, "character_id", charID)
		content := fmt.Sprintf("\u274c Approval failed: %v", err)
		empty := []discordgo.MessageComponent{}
		_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content:    &content,
			Components: &empty,
		})
		return
	}

	h.removePendingChoice(charID)

	// Update the DM queue message
	content := fmt.Sprintf("\u2705 Approved: %s", pending.Description)
	empty := []discordgo.MessageComponent{}
	_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Components: &empty,
	})

	// Notify the player
	h.notifyPlayer(pending.PlayerID, fmt.Sprintf("\u2705 Your ASI choice (%s) has been approved!", pending.Description))
}

// HandleDMDeny handles the DM clicking the Deny button.
func (h *ASIHandler) HandleDMDeny(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, err := ParseDMApprovalCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid denial data: %v", err))
		return
	}

	pending, ok := h.getPendingChoice(charID)
	if !ok {
		respondEphemeral(h.session, interaction, "No pending ASI choice found for this character.")
		return
	}

	// Acknowledge
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	reason := "DM denied the choice. Please select again."
	err = h.service.DenyASI(context.Background(), charID, reason)
	if err != nil {
		slog.Error("failed to deny ASI", "error", err, "character_id", charID)
	}

	h.removePendingChoice(charID)

	// Update the DM queue message
	content := fmt.Sprintf("\u274c Denied: %s", pending.Description)
	empty := []discordgo.MessageComponent{}
	_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Components: &empty,
	})
}

func (h *ASIHandler) notifyPlayer(playerDiscordID, message string) {
	ch, err := h.session.UserChannelCreate(playerDiscordID)
	if err != nil {
		slog.Error("failed to create DM channel for player", "error", err, "player", playerDiscordID)
		return
	}
	_, err = h.session.ChannelMessageSend(ch.ID, message)
	if err != nil {
		slog.Error("failed to send DM to player", "error", err, "player", playerDiscordID)
	}
}

// buildASIDescription creates a human-readable description of the ASI choice.
func buildASIDescription(asiType string, abilities []string, scores character.AbilityScores) string {
	if asiType == "plus2" && len(abilities) >= 1 {
		ab := abilities[0]
		current := scores.Get(ab)
		newVal := current + 2
		if newVal > 20 {
			newVal = 20
		}
		return fmt.Sprintf("+2 %s (%d -> %d)", strings.ToUpper(ab), current, newVal)
	}
	if asiType == "plus1plus1" && len(abilities) >= 2 {
		ab1, ab2 := abilities[0], abilities[1]
		c1, c2 := scores.Get(ab1), scores.Get(ab2)
		n1, n2 := c1+1, c2+1
		if n1 > 20 {
			n1 = 20
		}
		if n2 > 20 {
			n2 = 20
		}
		return fmt.Sprintf("+1 %s (%d -> %d), +1 %s (%d -> %d)",
			strings.ToUpper(ab1), c1, n1, strings.ToUpper(ab2), c2, n2)
	}
	return "unknown choice"
}
