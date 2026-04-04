package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// CharacterLookup abstracts the queries needed for the /character command.
type CharacterLookup interface {
	GetPlayerCharacterByDiscordUser(ctx context.Context, arg refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error)
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
}

// CharacterHandler handles the /character slash command.
type CharacterHandler struct {
	session      Session
	campaignProv CampaignProvider
	lookup       CharacterLookup
	portalBaseURL string
}

// NewCharacterHandler creates a new CharacterHandler.
func NewCharacterHandler(session Session, campaignProv CampaignProvider, lookup CharacterLookup, portalBaseURL string) *CharacterHandler {
	return &CharacterHandler{
		session:       session,
		campaignProv:  campaignProv,
		lookup:        lookup,
		portalBaseURL: portalBaseURL,
	}
}

// Handle processes a /character interaction.
func (h *CharacterHandler) Handle(interaction *discordgo.Interaction) {
	userID := interactionUserID(interaction)

	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	pc, err := h.lookup.GetPlayerCharacterByDiscordUser(context.Background(), refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaign.ID,
		DiscordUserID: userID,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, "No character found. Use /register, /import, or /create-character first.")
		return
	}

	if pc.Status != "approved" {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Your character is currently **%s**. It must be approved by the DM before you can view the full sheet.", pc.Status))
		return
	}

	ch, err := h.lookup.GetCharacter(context.Background(), pc.CharacterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not load your character. Please try again later.")
		return
	}

	embed := h.buildCharacterEmbed(ch)
	portalLink := fmt.Sprintf("%s/portal/character/%s", h.portalBaseURL, ch.ID.String())

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("View full character sheet: %s", portalLink),
			Embeds:  []*discordgo.MessageEmbed{embed},
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *CharacterHandler) buildCharacterEmbed(ch refdata.Character) *discordgo.MessageEmbed {
	var scores character.AbilityScores
	_ = json.Unmarshal(ch.AbilityScores, &scores)

	var classes []character.ClassEntry
	_ = json.Unmarshal(ch.Classes, &classes)

	classStr := character.FormatClassSummary(classes)

	var desc strings.Builder
	fmt.Fprintf(&desc, "**Level %d %s %s**\n\n", ch.Level, ch.Race, classStr)
	fmt.Fprintf(&desc, "HP: %d/%d", ch.HpCurrent, ch.HpMax)
	if ch.TempHp > 0 {
		fmt.Fprintf(&desc, " (+%d temp)", ch.TempHp)
	}
	fmt.Fprintf(&desc, " | AC: %d | Speed: %dft\n", ch.Ac, ch.SpeedFt)
	fmt.Fprintf(&desc, "STR %d | DEX %d | CON %d | INT %d | WIS %d | CHA %d\n\n",
		scores.STR, scores.DEX, scores.CON, scores.INT, scores.WIS, scores.CHA)

	mainHand := "empty"
	if ch.EquippedMainHand.Valid && ch.EquippedMainHand.String != "" {
		mainHand = ch.EquippedMainHand.String
	}
	offHand := "empty"
	if ch.EquippedOffHand.Valid && ch.EquippedOffHand.String != "" {
		offHand = ch.EquippedOffHand.String
	}
	fmt.Fprintf(&desc, "Equipped: %s (main) | %s (off-hand)\n", mainHand, offHand)
	fmt.Fprintf(&desc, "Gold: %dgp\n", ch.Gold)

	if len(ch.Languages) > 0 {
		fmt.Fprintf(&desc, "Languages: %s\n", strings.Join(ch.Languages, ", "))
	}

	if spellSummary := buildSpellSummary(ch); spellSummary != "" {
		fmt.Fprintf(&desc, "Spells: %s", spellSummary)
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("⚔️ %s", ch.Name),
		Description: desc.String(),
		Color:       0xe94560, // DnDnD red
	}
}

// ddbSpellEntry matches the DDB import spell format in character_data.
type ddbSpellEntry struct {
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Source string `json:"source"`
}

// buildSpellSummary extracts spells from character_data and returns a count-by-level summary.
func buildSpellSummary(ch refdata.Character) string {
	if !ch.CharacterData.Valid || len(ch.CharacterData.RawMessage) == 0 {
		return ""
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(ch.CharacterData.RawMessage, &data); err != nil {
		return ""
	}

	spellsRaw, ok := data["spells"]
	if !ok {
		return ""
	}

	// Count spells by level
	counts := make(map[int]int)

	// Try DDB format: []ddbSpellEntry
	var ddbSpells []ddbSpellEntry
	if err := json.Unmarshal(spellsRaw, &ddbSpells); err == nil && len(ddbSpells) > 0 && ddbSpells[0].Name != "" {
		for _, s := range ddbSpells {
			counts[s.Level]++
		}
	} else {
		// Try portal format: []string — no level info, just count total
		var portalSpells []string
		if err := json.Unmarshal(spellsRaw, &portalSpells); err == nil && len(portalSpells) > 0 {
			return fmt.Sprintf("%d known", len(portalSpells))
		}
		return ""
	}

	if len(counts) == 0 {
		return ""
	}

	// Sort levels and format
	levels := make([]int, 0, len(counts))
	for lvl := range counts {
		levels = append(levels, lvl)
	}
	sort.Ints(levels)

	parts := make([]string, 0, len(levels))
	for _, lvl := range levels {
		if lvl == 0 {
			parts = append(parts, fmt.Sprintf("Cantrips: %d", counts[lvl]))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %d", slotOrdinal(lvl), counts[lvl]))
		}
	}
	return strings.Join(parts, " | ")
}

// slotOrdinal converts a number to ordinal string.
func slotOrdinal(level int) string {
	switch level {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", level)
	}
}
