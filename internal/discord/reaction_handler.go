package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// ReactionService is the combat-side surface the /reaction handler needs.
// combat.Service satisfies this structurally.
type ReactionService interface {
	CanDeclareReaction(ctx context.Context, encounterID, combatantID uuid.UUID) (bool, error)
	DeclareReaction(ctx context.Context, encounterID, combatantID uuid.UUID, description string) (refdata.ReactionDeclaration, error)
	CancelReactionByDescription(ctx context.Context, combatantID, encounterID uuid.UUID, descSubstring string) (refdata.ReactionDeclaration, error)
	CancelAllReactions(ctx context.Context, combatantID, encounterID uuid.UUID) error
	ListReactionsByCombatant(ctx context.Context, combatantID, encounterID uuid.UUID) ([]refdata.ReactionDeclaration, error)
}

// ReactionEncounterResolver is the per-user encounter lookup.
type ReactionEncounterResolver interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// ReactionCombatantLookup resolves a Discord user to their combatant in a
// specific encounter.
type ReactionCombatantLookup interface {
	GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error)
}

// ReactionHandler handles the /reaction slash command (declare/cancel/
// cancel-all). It wires reaction declarations into the unified dm-queue
// notification framework so the DM sees each declaration in #dm-queue and
// can jump to the resolver page with a single click.
//
// Cancel linkage: after a successful DeclareReaction + Notifier.Post, the
// returned dmqueue item ID is stashed in an in-memory map keyed by
// declaration UUID. A subsequent /reaction cancel (or cancel-all) looks up
// this map to call Notifier.Cancel on the matching item. The mapping is
// intentionally in-memory: reaction cancels happen shortly after declaration
// within a single process lifetime; a restart losing the map is tolerable
// because the reaction_declarations row still exists and the DM can resolve
// or clear the pending dm-queue item manually. This keeps phase 106c focused
// on handler wiring without requiring a schema migration.
type ReactionHandler struct {
	session  Session
	service  ReactionService
	resolver ReactionEncounterResolver
	lookup   ReactionCombatantLookup
	notifier dmqueue.Notifier

	mu      sync.Mutex
	itemIDs map[uuid.UUID]string // declarationID → dm-queue itemID
}

// NewReactionHandler constructs a ReactionHandler. notifier is wired via
// SetNotifier and may be left nil for headless tests / headless deploys.
func NewReactionHandler(session Session, service ReactionService, resolver ReactionEncounterResolver, lookup ReactionCombatantLookup) *ReactionHandler {
	return &ReactionHandler{
		session:  session,
		service:  service,
		resolver: resolver,
		lookup:   lookup,
		itemIDs:  make(map[uuid.UUID]string),
	}
}

// SetNotifier wires the dm-queue Notifier. When nil, declare/cancel flows
// still persist reaction_declarations rows but skip the #dm-queue post/edit.
func (h *ReactionHandler) SetNotifier(n dmqueue.Notifier) { h.notifier = n }

// ItemIDForDeclaration returns the stashed dm-queue item ID for a
// declaration, or "" if none. Exposed for tests and potential future
// diagnostics.
func (h *ReactionHandler) ItemIDForDeclaration(declID uuid.UUID) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.itemIDs[declID]
}

// Handle routes the /reaction interaction to the matching subcommand.
func (h *ReactionHandler) Handle(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	sub := firstSubcommand(data.Options)
	if sub == nil {
		respondEphemeral(h.session, interaction, "Unknown /reaction subcommand.")
		return
	}

	switch sub.Name {
	case "declare":
		h.handleDeclare(interaction, sub)
	case "cancel":
		h.handleCancel(interaction, sub)
	case "cancel-all":
		h.handleCancelAll(interaction)
	default:
		respondEphemeral(h.session, interaction, fmt.Sprintf("Unknown /reaction subcommand: %s", sub.Name))
	}
}

func (h *ReactionHandler) handleDeclare(interaction *discordgo.Interaction, sub *discordgo.ApplicationCommandInteractionDataOption) {
	description := strings.TrimSpace(reactionStringOption(sub.Options, "description"))
	if description == "" {
		respondEphemeral(h.session, interaction, "Please provide a reaction description.")
		return
	}

	ctx := context.Background()
	encounterID, combatantID, displayName, ok := h.resolveUserCombat(ctx, interaction)
	if !ok {
		return
	}

	canDeclare, err := h.service.CanDeclareReaction(ctx, encounterID, combatantID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to check reaction availability: %v", err))
		return
	}
	if !canDeclare {
		respondEphemeral(h.session, interaction, "❌ You have already used your reaction this round.")
		return
	}

	decl, err := h.service.DeclareReaction(ctx, encounterID, combatantID, description)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to declare reaction: %v", err))
		return
	}

	h.postReactionDeclaration(ctx, decl, displayName, interaction.GuildID)

	respondEphemeral(h.session, interaction, fmt.Sprintf("⚡ Reaction declared: %s", decl.Description))
}

// postReactionDeclaration posts the dm-queue event for a freshly-persisted
// declaration and stashes the returned item ID for later Cancel lookup.
// No-op when no Notifier is wired.
func (h *ReactionHandler) postReactionDeclaration(ctx context.Context, decl refdata.ReactionDeclaration, playerName, guildID string) {
	if h.notifier == nil {
		return
	}
	itemID, err := h.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindReactionDeclaration,
		PlayerName: playerName,
		Summary:    decl.Description,
		GuildID:    guildID,
		ExtraMetadata: map[string]string{
			"reaction_declaration_id": decl.ID.String(),
		},
	})
	if err != nil || itemID == "" {
		return
	}
	h.mu.Lock()
	h.itemIDs[decl.ID] = itemID
	h.mu.Unlock()
}

func (h *ReactionHandler) handleCancel(interaction *discordgo.Interaction, sub *discordgo.ApplicationCommandInteractionDataOption) {
	substring := strings.TrimSpace(reactionStringOption(sub.Options, "description"))
	if substring == "" {
		respondEphemeral(h.session, interaction, "Please provide a description substring to cancel.")
		return
	}

	ctx := context.Background()
	encounterID, combatantID, _, ok := h.resolveUserCombat(ctx, interaction)
	if !ok {
		return
	}

	decl, err := h.service.CancelReactionByDescription(ctx, combatantID, encounterID, substring)
	if err != nil {
		if strings.Contains(err.Error(), "no active reaction") {
			respondEphemeral(h.session, interaction, fmt.Sprintf("No active reaction matching %q.", substring))
			return
		}
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to cancel reaction: %v", err))
		return
	}

	h.cancelDMQueueItem(ctx, decl.ID, "Cancelled by player")
	respondEphemeral(h.session, interaction, fmt.Sprintf("⚡ Cancelled reaction: %s", decl.Description))
}

func (h *ReactionHandler) handleCancelAll(interaction *discordgo.Interaction) {
	ctx := context.Background()
	encounterID, combatantID, _, ok := h.resolveUserCombat(ctx, interaction)
	if !ok {
		return
	}

	if err := h.service.CancelAllReactions(ctx, combatantID, encounterID); err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to cancel reactions: %v", err))
		return
	}

	// Cancel every dm-queue item we stashed for this handler-lifetime.
	h.mu.Lock()
	pending := make(map[uuid.UUID]string, len(h.itemIDs))
	for k, v := range h.itemIDs {
		pending[k] = v
	}
	h.itemIDs = make(map[uuid.UUID]string)
	h.mu.Unlock()

	if h.notifier != nil {
		for _, itemID := range pending {
			_ = h.notifier.Cancel(ctx, itemID, "Cancelled by player")
		}
	}

	respondEphemeral(h.session, interaction, "⚡ Cancelled all declared reactions for this round.")
}

// cancelDMQueueItem looks up the stashed item ID for a declaration and
// issues Notifier.Cancel. The mapping entry is removed on success so a
// later cancel-all does not double-cancel.
func (h *ReactionHandler) cancelDMQueueItem(ctx context.Context, declID uuid.UUID, reason string) {
	h.mu.Lock()
	itemID, found := h.itemIDs[declID]
	if found {
		delete(h.itemIDs, declID)
	}
	h.mu.Unlock()
	if !found || itemID == "" || h.notifier == nil {
		return
	}
	_ = h.notifier.Cancel(ctx, itemID, reason)
}

// resolveUserCombat resolves the invoking Discord user to (encounterID,
// combatantID, displayName) and replies with an ephemeral error if either
// step fails. Returns ok=false on failure so the caller can abort without
// another response.
func (h *ReactionHandler) resolveUserCombat(ctx context.Context, interaction *discordgo.Interaction) (uuid.UUID, uuid.UUID, string, bool) {
	userID := discordUserID(interaction)

	encounterID, err := h.resolver.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "You are not in an active encounter.")
		return uuid.Nil, uuid.Nil, "", false
	}

	combatantID, displayName, err := h.lookup.GetCombatantIDByDiscordUser(ctx, encounterID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not resolve your combatant in this encounter.")
		return uuid.Nil, uuid.Nil, "", false
	}

	return encounterID, combatantID, displayName, true
}

// firstSubcommand returns the first subcommand-typed option, or nil.
func firstSubcommand(opts []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	for _, opt := range opts {
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			return opt
		}
	}
	return nil
}

// reactionStringOption finds an option by name and returns its string value, or "".
func reactionStringOption(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, opt := range opts {
		if opt.Name == name {
			return opt.StringValue()
		}
	}
	return ""
}
