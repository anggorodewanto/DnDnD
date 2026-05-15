package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// RetireCombatChecker checks whether a character is currently in active combat.
// Used to block /retire during encounters per spec.
type RetireCombatChecker interface {
	GetActiveCombatantByCharacterID(ctx context.Context, characterID uuid.NullUUID) (refdata.Combatant, error)
}

// RetirePCStore marks an existing player_character row as a retire request
// (created_via='retire') so the dashboard approval queue surfaces it through
// the Phase 16 retire branch. The row stays at status='approved' until the DM
// approves the retire from the dashboard.
type RetirePCStore interface {
	MarkPlayerCharacterRetireRequested(ctx context.Context, arg refdata.MarkPlayerCharacterRetireRequestedParams) (refdata.PlayerCharacter, error)
}

// RetireHandler handles the /retire slash command. It marks the player's
// existing player_character row as a retire request and pings the DM via the
// dm-queue notifier. The actual transition to status='retired' happens later
// when the DM approves from the dashboard (internal/dashboard/approval_handler.go).
type RetireHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	pcStore         RetirePCStore
	notifier        dmqueue.Notifier
	combatChecker   RetireCombatChecker
}

// NewRetireHandler constructs a RetireHandler. The player_character store is
// optional and wired via SetPCStore; without it the handler still notifies
// the DM but does not flag the row in player_characters.
func NewRetireHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	notifier dmqueue.Notifier,
) *RetireHandler {
	return &RetireHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		notifier:        notifier,
	}
}

// SetPCStore wires the player_characters store so /retire can flag the
// existing row with created_via='retire'.
func (h *RetireHandler) SetPCStore(store RetirePCStore) {
	h.pcStore = store
}

// SetCombatChecker wires the active-combat lookup so /retire is blocked
// during encounters.
func (h *RetireHandler) SetCombatChecker(checker RetireCombatChecker) {
	h.combatChecker = checker
}

// Handle processes a /retire interaction.
func (h *RetireHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()
	reason := optionString(interaction, "reason")
	if reason == "" {
		reason = "(no reason given)"
	}

	if h.campaignProv == nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}
	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	if h.characterLookup == nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	// F-15: Block retirement during active combat.
	if h.combatChecker != nil {
		if _, err := h.combatChecker.GetActiveCombatantByCharacterID(ctx, uuid.NullUUID{UUID: char.ID, Valid: true}); err == nil {
			respondEphemeral(h.session, interaction, "❌ You can't retire mid-combat.")
			return
		}
	}

	if err := h.markPC(ctx, campaign.ID, userID); err != nil {
		respondEphemeral(h.session, interaction, "Could not record retire request. Please try again or ask the DM.")
		return
	}

	h.notifyDM(ctx, char, reason, interaction.GuildID, campaign.ID.String())

	respondEphemeral(h.session, interaction,
		fmt.Sprintf("🪦 Retire request sent to the DM. They'll review and approve from the dashboard.\n_Reason: %s_", reason))
}

// markPC flags the player's existing player_character row as a retire request
// (created_via='retire'). nil pcStore is a no-op so the handler can still
// run routing-only paths without DB wiring.
func (h *RetireHandler) markPC(ctx context.Context, campaignID uuid.UUID, discordUserID string) error {
	if h.pcStore == nil {
		return nil
	}
	_, err := h.pcStore.MarkPlayerCharacterRetireRequested(ctx, refdata.MarkPlayerCharacterRetireRequestedParams{
		CampaignID:    campaignID,
		DiscordUserID: discordUserID,
	})
	return err
}

// notifyDM posts a dm-queue retire-request notification. nil notifier is a no-op.
// SR-002: CampaignID is required by PgStore.Insert; passing it here keeps the
// row persistable so the dashboard can resolve the request.
func (h *RetireHandler) notifyDM(ctx context.Context, char refdata.Character, reason, guildID, campaignID string) {
	if h.notifier == nil {
		return
	}
	_, _ = h.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindRetireRequest,
		PlayerName: char.Name,
		Summary:    fmt.Sprintf("requests retirement: %s", reason),
		GuildID:    guildID,
		CampaignID: campaignID,
	})
}
