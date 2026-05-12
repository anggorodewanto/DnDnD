package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/refdata"
)

// LootEncounterProvider finds the most recent completed encounter for a campaign.
type LootEncounterProvider interface {
	GetMostRecentCompletedEncounter(ctx context.Context, campaignID uuid.UUID) (refdata.Encounter, error)
}

// LootHandler handles the /loot slash command.
type LootHandler struct {
	session         Session
	campaignProv    InventoryCampaignProvider
	characterLookup InventoryCharacterLookup
	encounterProv   LootEncounterProvider
	lootSvc         *loot.Service
	cardUpdater     CardUpdater // SR-007
}

// SetCardUpdater wires the SR-007 character-card refresh callback fired
// after a successful /loot claim.
func (h *LootHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// NewLootHandler creates a new LootHandler.
func NewLootHandler(
	session Session,
	campaignProv InventoryCampaignProvider,
	characterLookup InventoryCharacterLookup,
	encounterProv LootEncounterProvider,
	lootSvc *loot.Service,
) *LootHandler {
	return &LootHandler{
		session:         session,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
		encounterProv:   encounterProv,
		lootSvc:         lootSvc,
	}
}

// Handle processes the /loot command interaction.
func (h *LootHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := discordUserID(interaction)
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your character. Use `/register` first.")
		return
	}

	enc, err := h.encounterProv.GetMostRecentCompletedEncounter(ctx, campaign.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No completed encounter found.")
		return
	}

	result, err := h.lootSvc.GetLootPool(ctx, enc.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No loot pool available.")
		return
	}

	if result.Pool.Status != "open" {
		respondEphemeral(h.session, interaction, "The loot pool is closed.")
		return
	}

	// Build embed with unclaimed items
	var desc strings.Builder
	if result.Pool.GoldTotal > 0 {
		fmt.Fprintf(&desc, "\U0001f4b0 **Gold:** %d gp\n\n", result.Pool.GoldTotal)
	}

	var buttons []discordgo.MessageComponent
	unclaimedCount := 0

	for _, item := range result.Items {
		if item.ClaimedBy.Valid {
			fmt.Fprintf(&desc, "\u2705 ~~%s~~ (claimed)\n", item.Name)
			continue
		}
		unclaimedCount++
		label := item.Name
		if item.Quantity > 1 {
			label = fmt.Sprintf("%s \u00d7%d", item.Name, item.Quantity)
		}
		if item.Description != "" {
			fmt.Fprintf(&desc, "\u2b50 **%s** \u2014 _%s_\n", label, item.Description)
		} else {
			fmt.Fprintf(&desc, "\u2b50 **%s**\n", label)
		}
		buttons = append(buttons, discordgo.Button{
			Label:    label,
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("loot_claim:%s:%s:%s", result.Pool.ID, item.ID, char.ID),
		})
	}

	if unclaimedCount == 0 && result.Pool.GoldTotal == 0 {
		respondEphemeral(h.session, interaction, "All items have been claimed.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "\U0001f4b0 Loot Pool",
		Description: desc.String(),
		Color:       0xFFD700,
	}

	components := []discordgo.MessageComponent{}
	// Discord allows max 5 buttons per action row
	for i := 0; i < len(buttons); i += 5 {
		end := i + 5
		if end > len(buttons) {
			end = len(buttons)
		}
		components = append(components, discordgo.ActionsRow{
			Components: buttons[i:end],
		})
	}

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// ParseLootClaimData parses a "loot_claim:poolID:itemID:characterID" custom ID.
func ParseLootClaimData(customID string) (poolID, itemID, characterID uuid.UUID, err error) {
	parts := strings.SplitN(customID, ":", 4)
	if len(parts) != 4 {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("expected 4 parts, got %d", len(parts))
	}
	poolID, err = uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("invalid pool ID: %w", err)
	}
	itemID, err = uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("invalid item ID: %w", err)
	}
	characterID, err = uuid.Parse(parts[3])
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("invalid character ID: %w", err)
	}
	return poolID, itemID, characterID, nil
}

// HandleLootClaim processes a loot claim button click.
func (h *LootHandler) HandleLootClaim(interaction *discordgo.Interaction, poolID, itemID, characterID uuid.UUID) {
	ctx := context.Background()

	claimed, err := h.lootSvc.ClaimItem(ctx, poolID, itemID, characterID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Cannot claim item: %v", err))
		return
	}

	// SR-007: refresh #character-cards for the claimant.
	notifyCardUpdate(ctx, h.cardUpdater, characterID)

	respondEphemeral(h.session, interaction, fmt.Sprintf("\u2705 You claimed **%s**!", claimed.Name))
}
