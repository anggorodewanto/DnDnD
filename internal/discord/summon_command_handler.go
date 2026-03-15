package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
)

// SummonCommandService defines the service methods needed by the /command handler.
type SummonCommandService interface {
	CommandCreature(ctx context.Context, input combat.CommandCreatureInput) (combat.CommandCreatureResult, error)
}

// SummonCommandEncounterProvider provides the active encounter for a guild.
type SummonCommandEncounterProvider interface {
	GetActiveEncounterID(ctx context.Context, guildID string) (uuid.UUID, error)
}

// SummonCommandPlayerLookup resolves a Discord user to their combatant ID.
type SummonCommandPlayerLookup interface {
	GetCombatantIDByDiscordUser(ctx context.Context, encounterID uuid.UUID, discordUserID string) (uuid.UUID, string, error)
}

// SummonCommandHandler handles the /command slash command for summoned creatures.
type SummonCommandHandler struct {
	session           Session
	svc               SummonCommandService
	encounterProvider SummonCommandEncounterProvider
	playerLookup      SummonCommandPlayerLookup
}

// NewSummonCommandHandler creates a new SummonCommandHandler.
func NewSummonCommandHandler(session Session, svc SummonCommandService) *SummonCommandHandler {
	return &SummonCommandHandler{
		session: session,
		svc:     svc,
	}
}

// SetEncounterProvider sets the encounter provider.
func (h *SummonCommandHandler) SetEncounterProvider(ep SummonCommandEncounterProvider) {
	h.encounterProvider = ep
}

// SetPlayerLookup sets the player lookup.
func (h *SummonCommandHandler) SetPlayerLookup(pl SummonCommandPlayerLookup) {
	h.playerLookup = pl
}

// Handle processes the /command interaction.
func (h *SummonCommandHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	if h.encounterProvider == nil || h.playerLookup == nil || h.svc == nil {
		respondEphemeral(h.session, interaction, "/command is not fully configured yet.")
		return
	}

	// Extract options
	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	creatureID := ""
	action := ""
	target := ""
	for _, opt := range data.Options {
		switch opt.Name {
		case "creature_id":
			creatureID = opt.StringValue()
		case "action":
			action = opt.StringValue()
		case "target":
			target = opt.StringValue()
		}
	}

	if creatureID == "" || action == "" {
		respondEphemeral(h.session, interaction, "Usage: /command <creature_id> <action> [target]")
		return
	}

	// Get active encounter
	encounterID, err := h.encounterProvider.GetActiveEncounterID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No active encounter in this server.")
		return
	}

	// Resolve calling player's combatant ID
	userID := ""
	if interaction.Member != nil && interaction.Member.User != nil {
		userID = interaction.Member.User.ID
	}

	summonerID, summonerName, err := h.playerLookup.GetCombatantIDByDiscordUser(ctx, encounterID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find your combatant in this encounter.")
		return
	}

	var args []string
	if target != "" {
		args = append(args, target)
	}

	result, err := h.svc.CommandCreature(ctx, combat.CommandCreatureInput{
		EncounterID:     encounterID,
		SummonerID:      summonerID,
		SummonerName:    summonerName,
		CreatureShortID: creatureID,
		Action:          action,
		Args:            args,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Command failed: %s", err.Error()))
		return
	}

	// Respond with the combat log
	respondEphemeral(h.session, interaction, result.CombatLog)
}
