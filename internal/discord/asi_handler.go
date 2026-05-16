package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/ab/dndnd/internal/character"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

const (
	asiChoicePrefix     = "asi_choice"
	asiSelectPrefix     = "asi_select"
	asiApprovePrefix    = "asi_approve"
	asiDenyPrefix       = "asi_deny"
	asiFeatSelectPrefix = "asi_feat_select"
	asiFeatChoicePrefix = "asi_feat_choice"
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

// parseThreePartCustomID parses a custom ID like "prefix:<charID>:<type>" and validates the prefix.
func parseThreePartCustomID(customID, wantPrefix string) (uuid.UUID, string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 || parts[0] != wantPrefix {
		return uuid.Nil, "", fmt.Errorf("invalid %s custom ID: %s", wantPrefix, customID)
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid character ID: %w", err)
	}
	return charID, parts[2], nil
}

// ParseASIChoiceCustomID parses a custom ID like "asi_choice:<charID>:<type>".
func ParseASIChoiceCustomID(customID string) (uuid.UUID, string, error) {
	return parseThreePartCustomID(customID, asiChoicePrefix)
}

// ParseASISelectCustomID parses a custom ID like "asi_select:<charID>:<type>".
func ParseASISelectCustomID(customID string) (uuid.UUID, string, error) {
	return parseThreePartCustomID(customID, asiSelectPrefix)
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
	minValues := &maxValues

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
	Type        string              `json:"type"`
	Ability     string              `json:"ability,omitempty"`
	Ability2    string              `json:"ability2,omitempty"`
	FeatID      string              `json:"feat_id,omitempty"`
	FeatChoices map[string][]string `json:"feat_choices,omitempty"`
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

// FeatOption is the minimum feat info the ASI handler needs to render a
// select-menu option and post the player's choice to #dm-queue. (med-36)
type FeatOption struct {
	ID          string
	Name        string
	Description string
}

// FeatLister enumerates feats a character may legally pick. Implementations
// run prerequisite checks server-side (CheckFeatPrerequisites) and return
// only eligible entries. (med-36 / Phase 89)
type FeatLister interface {
	ListEligibleFeats(ctx context.Context, charID uuid.UUID) ([]FeatOption, error)
}

// PendingASIChoice holds a player's ASI/Feat choice awaiting DM approval.
type PendingASIChoice struct {
	CharID      uuid.UUID           `json:"char_id"`
	ASIType     string              `json:"asi_type"`
	Abilities   []string            `json:"abilities,omitempty"`
	FeatID      string              `json:"feat_id,omitempty"`
	FeatChoices map[string][]string `json:"feat_choices,omitempty"`
	PlayerID    string              `json:"player_id"`
	Description string              `json:"description"`
}

// ASIPendingStore persists pending ASI/Feat choices so a process restart
// does not drop in-flight DM approvals (F-89d). The handler keeps an
// in-memory cache for speed; the store is the durable source of truth.
// All methods are safe to leave returning errors — the handler logs and
// continues with the in-memory copy so a transient DB blip doesn't lose
// the prompt outright.
type ASIPendingStore interface {
	Save(ctx context.Context, choice PendingASIChoice) error
	Delete(ctx context.Context, charID uuid.UUID) error
	List(ctx context.Context) ([]PendingASIChoice, error)
}

// ASIHandler processes Discord component interactions for the ASI/Feat flow.
type ASIHandler struct {
	session     Session
	service     ASIService
	dmQueueFunc func(guildID string) string
	dmUserFunc  func(guildID string) string
	// med-36 / Phase 89: optional feat lister so the "Choose a Feat"
	// button posts a real Discord SelectMenu populated with eligible
	// feats. Nil falls back to the historical "not yet available" stub.
	featLister FeatLister

	// F-89d: durable store for pending ASI/Feat choices so a process
	// restart preserves in-flight DM approvals. Nil keeps the
	// historical in-memory-only behavior.
	pendingStore ASIPendingStore

	mu      sync.RWMutex
	pending map[uuid.UUID]PendingASIChoice
}

// SetFeatLister wires the eligible-feats source so the "Choose a Feat" button
// posts a select menu instead of the placeholder stub. Nil keeps the stub.
func (h *ASIHandler) SetFeatLister(l FeatLister) { h.featLister = l }

// SetDMUserFunc wires the DM user lookup so HandleDMApprove/HandleDMDeny
// reject interactions from non-DM users. Nil skips the check (backwards compat).
func (h *ASIHandler) SetDMUserFunc(f func(guildID string) string) { h.dmUserFunc = f }

// isDMUser returns true if dmUserFunc is nil (backwards compat) or the
// interacting user matches the campaign's DM for the guild.
func (h *ASIHandler) isDMUser(interaction *discordgo.Interaction) bool {
	if h.dmUserFunc == nil {
		return true
	}
	dmID := h.dmUserFunc(interaction.GuildID)
	return dmID == "" || discordUserID(interaction) == dmID
}

// SetPendingStore wires the durable pending-choice store. F-89d: when wired,
// every pending choice is upserted to the store on accept and deleted on
// approve/deny. Call HydratePending after wiring to rehydrate from the DB
// after a restart. Nil keeps the prior in-memory-only behavior so tests
// built before F-89d landed keep passing unchanged.
func (h *ASIHandler) SetPendingStore(s ASIPendingStore) { h.pendingStore = s }

// HydratePending repopulates the in-memory pending map from the durable
// store. Call once at startup after SetPendingStore. Best-effort: a store
// failure is logged but doesn't fail the boot — the handler still works,
// it just behaves as if there were no in-flight prompts before restart.
// (F-89d)
func (h *ASIHandler) HydratePending(ctx context.Context) error {
	if h.pendingStore == nil {
		return nil
	}
	rows, err := h.pendingStore.List(ctx)
	if err != nil {
		slog.Error("failed to hydrate pending ASI choices", "error", err)
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range rows {
		h.pending[c.CharID] = c
	}
	return nil
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
	h.pending[charID] = choice
	h.mu.Unlock()
	if h.pendingStore == nil {
		return
	}
	if err := h.pendingStore.Save(context.Background(), choice); err != nil {
		slog.Error("failed to persist pending ASI choice", "error", err, "character_id", charID)
	}
}

func (h *ASIHandler) getPendingChoice(charID uuid.UUID) (PendingASIChoice, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.pending[charID]
	return c, ok
}

func (h *ASIHandler) removePendingChoice(charID uuid.UUID) error {
	if h.pendingStore != nil {
		if err := h.pendingStore.Delete(context.Background(), charID); err != nil {
			slog.Error("failed to delete pending ASI choice", "error", err, "character_id", charID)
			return err
		}
	}

	h.mu.Lock()
	delete(h.pending, charID)
	h.mu.Unlock()
	return nil
}

// MarshalPendingASIChoice serialises a pending choice to the JSON form the
// pending_asi.snapshot_json column carries. Exposed for adapter tests.
func MarshalPendingASIChoice(c PendingASIChoice) ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalPendingASIChoice deserialises a pending choice snapshot.
func UnmarshalPendingASIChoice(raw []byte) (PendingASIChoice, error) {
	var c PendingASIChoice
	err := json.Unmarshal(raw, &c)
	return c, err
}

// validateASIOwner checks that the interacting user owns the character.
func (h *ASIHandler) validateASIOwner(interaction *discordgo.Interaction, charData *ASICharacterData) error {
	if discordUserID(interaction) != charData.DiscordUserID {
		respondEphemeral(h.session, interaction, "⛔ This is not your character.")
		return fmt.Errorf("owner mismatch")
	}
	return nil
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

	if h.validateASIOwner(interaction, charData) != nil {
		return
	}

	if asiType == "feat" {
		h.handleFeatChoice(interaction, charData)
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

	if h.validateASIOwner(interaction, charData) != nil {
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
	if !h.isDMUser(interaction) {
		respondEphemeral(h.session, interaction, "Only the DM can approve ASI choices.")
		return
	}

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
	if len(pending.FeatChoices) > 0 {
		choice.FeatChoices = pending.FeatChoices
	}

	if err := h.removePendingChoice(charID); err != nil {
		content := fmt.Sprintf("\u274c Approval failed: could not clear pending ASI choice: %v", err)
		_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	err = h.service.ApproveASI(context.Background(), charID, choice)
	if err != nil {
		h.storePendingChoice(charID, pending)
		slog.Error("failed to approve ASI", "error", err, "character_id", charID)
		content := fmt.Sprintf("\u274c Approval failed: %v", err)
		empty := []discordgo.MessageComponent{}
		_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
			Content:    &content,
			Components: &empty,
		})
		return
	}

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
	if !h.isDMUser(interaction) {
		respondEphemeral(h.session, interaction, "Only the DM can deny ASI choices.")
		return
	}

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

	_ = h.removePendingChoice(charID)

	// Update the DM queue message
	content := fmt.Sprintf("\u274c Denied: %s", pending.Description)
	empty := []discordgo.MessageComponent{}
	_, _ = h.session.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{
		Content:    &content,
		Components: &empty,
	})
}

// handleFeatChoice resolves the eligible feat list and posts a select menu.
// Falls back to the historical "not yet available" stub when no FeatLister
// is wired (preserves test deploys without the lister). med-36 / Phase 89.
func (h *ASIHandler) handleFeatChoice(interaction *discordgo.Interaction, charData *ASICharacterData) {
	if h.featLister == nil {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Feat selection for **%s** is not yet available. Please use the dashboard or ask your DM.", charData.Name),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	feats, err := h.featLister.ListEligibleFeats(context.Background(), charData.ID)
	if err != nil || len(feats) == 0 {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("No feats available for **%s** right now.", charData.Name),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	components := buildFeatSelectMenu(charData.ID, feats)
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("\U0001f4d6 Pick a feat for **%s**:", charData.Name),
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// buildFeatSelectMenu renders up to 25 eligible feats as a Discord select
// menu (Discord caps at 25 options per menu — pagination is a follow-up).
func buildFeatSelectMenu(charID uuid.UUID, feats []FeatOption) []discordgo.MessageComponent {
	customID := fmt.Sprintf("%s:%s", asiFeatSelectPrefix, charID.String())
	options := make([]discordgo.SelectMenuOption, 0, len(feats))
	for i, f := range feats {
		if i >= 25 {
			break
		}
		desc := f.Description
		if len(desc) > 100 {
			desc = desc[:97] + "..."
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:       f.Name,
			Value:       f.ID,
			Description: desc,
		})
	}
	return []discordgo.MessageComponent{
		&discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    customID,
					Placeholder: "Pick a feat",
					Options:     options,
				},
			},
		},
	}
}

// ParseASIFeatSelectCustomID parses a custom ID like
// "asi_feat_select:<charID>" and returns the character ID.
func ParseASIFeatSelectCustomID(customID string) (uuid.UUID, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 2 || parts[0] != asiFeatSelectPrefix {
		return uuid.Nil, fmt.Errorf("invalid %s custom ID: %s", asiFeatSelectPrefix, customID)
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid character ID: %w", err)
	}
	return charID, nil
}

// HandleASIFeatSelect handles the player's feat selection from the select
// menu. Stores the choice as a pending request and posts to the DM queue
// for approval (mirroring the ASI ability-score path). med-36 / Phase 89.
func (h *ASIHandler) HandleASIFeatSelect(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, err := ParseASIFeatSelectCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid feat selection: %v", err))
		return
	}
	if len(data.Values) == 0 {
		respondEphemeral(h.session, interaction, "No feat selected.")
		return
	}
	featID := data.Values[0]

	charData, err := h.service.GetCharacterForASI(context.Background(), charID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not load character data.")
		return
	}

	if h.validateASIOwner(interaction, charData) != nil {
		return
	}

	// Resolve the feat name for the description; default to the ID when
	// the lister is missing or doesn't surface the chosen feat.
	featName := featID
	if h.featLister != nil {
		feats, listErr := h.featLister.ListEligibleFeats(context.Background(), charID)
		if listErr == nil {
			for _, f := range feats {
				if f.ID == featID {
					featName = f.Name
					break
				}
			}
		}
	}
	description := fmt.Sprintf("Feat: %s", featName)
	if components, ok := buildFeatSubChoiceMenu(charID, featID); ok {
		_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("\U0001f4d6 Choose options for **%s**:", featName),
				Components: components,
				Flags:      discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("⏳ Your choice (%s) has been sent to the DM for approval.", description),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	playerID := discordUserID(interaction)
	pending := PendingASIChoice{
		CharID:      charID,
		ASIType:     "feat",
		FeatID:      featID,
		PlayerID:    playerID,
		Description: description,
	}
	h.storePendingChoice(charID, pending)

	if h.dmQueueFunc == nil {
		return
	}
	channelID := h.dmQueueFunc(interaction.GuildID)
	if channelID == "" {
		return
	}

	msg := FormatDMQueueASIMessage(charData.Name, charData.ClassInfo, description)
	components := BuildDMApprovalComponents(charID)
	if _, err := h.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:    msg,
		Components: components,
	}); err != nil {
		slog.Error("failed to post feat choice to DM queue", "error", err, "character", charData.Name)
	}
}

// HandleASIFeatSubChoiceSelect handles the second step for feats that need
// internal choices before the DM approval request is posted.
func (h *ASIHandler) HandleASIFeatSubChoiceSelect(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	charID, featID, choiceKind, err := parseFeatSubChoiceCustomID(data.CustomID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Invalid feat choice: %v", err))
		return
	}
	if len(data.Values) == 0 {
		respondEphemeral(h.session, interaction, "No feat option selected.")
		return
	}

	charData, err := h.service.GetCharacterForASI(context.Background(), charID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not load character data.")
		return
	}

	if h.validateASIOwner(interaction, charData) != nil {
		return
	}

	featName := h.lookupFeatName(context.Background(), charID, featID)
	choices := map[string][]string{choiceKind: append([]string(nil), data.Values...)}
	description := fmt.Sprintf("Feat: %s (%s)", featName, strings.Join(data.Values, ", "))

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("\u23f3 Your choice (%s) has been sent to the DM for approval.", description),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	pending := PendingASIChoice{
		CharID:      charID,
		ASIType:     "feat",
		FeatID:      featID,
		FeatChoices: choices,
		PlayerID:    discordUserID(interaction),
		Description: description,
	}
	h.storePendingChoice(charID, pending)

	if h.dmQueueFunc == nil {
		return
	}
	channelID := h.dmQueueFunc(interaction.GuildID)
	if channelID == "" {
		return
	}
	msg := FormatDMQueueASIMessage(charData.Name, charData.ClassInfo, description)
	components := BuildDMApprovalComponents(charID)
	if _, err := h.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:    msg,
		Components: components,
	}); err != nil {
		slog.Error("failed to post feat sub-choice to DM queue", "error", err, "character", charData.Name)
	}
}

func (h *ASIHandler) lookupFeatName(ctx context.Context, charID uuid.UUID, featID string) string {
	if h.featLister == nil {
		return featID
	}
	feats, err := h.featLister.ListEligibleFeats(ctx, charID)
	if err != nil {
		return featID
	}
	for _, f := range feats {
		if f.ID == featID {
			return f.Name
		}
	}
	return featID
}

func buildFeatSubChoiceMenu(charID uuid.UUID, featID string) ([]discordgo.MessageComponent, bool) {
	switch featID {
	case "resilient":
		return buildFeatSelect(charID, featID, "ability", "Choose ability", abilityFeatOptions(), 1), true
	case "skilled":
		return buildFeatSelect(charID, featID, "skills", "Choose three skills", skillFeatOptions(), 3), true
	case "elemental-adept":
		return buildFeatSelect(charID, featID, "damage_type", "Choose damage type", damageTypeOptions(), 1), true
	default:
		return nil, false
	}
}

func buildFeatSelect(charID uuid.UUID, featID, kind, placeholder string, options []discordgo.SelectMenuOption, maxValues int) []discordgo.MessageComponent {
	minValues := &maxValues
	return []discordgo.MessageComponent{
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    fmt.Sprintf("%s:%s:%s:%s", asiFeatChoicePrefix, charID.String(), featID, kind),
				Placeholder: placeholder,
				MinValues:   minValues,
				MaxValues:   maxValues,
				Options:     options,
			},
		}},
	}
}

func abilityFeatOptions() []discordgo.SelectMenuOption {
	options := make([]discordgo.SelectMenuOption, 0, len(abilityOrder))
	for _, ab := range abilityOrder {
		options = append(options, discordgo.SelectMenuOption{Label: ab.Label, Value: ab.Key})
	}
	return options
}

func skillFeatOptions() []discordgo.SelectMenuOption {
	skills := make([]string, 0, len(character.SkillAbilityMap))
	for skill := range character.SkillAbilityMap {
		skills = append(skills, skill)
	}
	sort.Strings(skills)
	options := make([]discordgo.SelectMenuOption, 0, len(skills))
	for _, skill := range skills {
		options = append(options, discordgo.SelectMenuOption{
			Label: titleFeatOption(skill),
			Value: skill,
		})
	}
	return options
}

func damageTypeOptions() []discordgo.SelectMenuOption {
	types := []string{"acid", "cold", "fire", "lightning", "thunder"}
	options := make([]discordgo.SelectMenuOption, 0, len(types))
	for _, typ := range types {
		options = append(options, discordgo.SelectMenuOption{Label: titleFeatOption(typ), Value: typ})
	}
	return options
}

func titleFeatOption(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '-' || r == '_' })
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func parseFeatSubChoiceCustomID(customID string) (uuid.UUID, string, string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 4 || parts[0] != asiFeatChoicePrefix {
		return uuid.Nil, "", "", fmt.Errorf("invalid %s custom ID: %s", asiFeatChoicePrefix, customID)
	}
	charID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, "", "", fmt.Errorf("invalid character ID: %w", err)
	}
	return charID, parts[2], parts[3], nil
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
