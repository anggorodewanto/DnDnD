package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
	GetSpellsByIDs(ctx context.Context, ids []string) ([]refdata.Spell, error)
}

// CharacterHandler handles the /character slash command.
type CharacterHandler struct {
	session       Session
	campaignProv  CampaignProvider
	lookup        CharacterLookup
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
	callerID := interactionUserID(interaction)
	targetID := characterTargetOption(interaction)

	// lookupID is whose character we resolve: the explicit target if one was
	// supplied, otherwise the caller's own. viewingSelf gates owner-only
	// behavior (DM feedback, portal link).
	lookupID := callerID
	viewingSelf := true
	if targetID != "" {
		lookupID = targetID
		viewingSelf = targetID == callerID
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	pc, err := h.lookup.GetPlayerCharacterByDiscordUser(context.Background(), refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaign.ID,
		DiscordUserID: lookupID,
	})
	if err != nil {
		if !viewingSelf {
			respondEphemeral(h.session, interaction, "That player doesn't have a character in this campaign.")
			return
		}
		respondEphemeral(h.session, interaction, "No character found. Use /register, /import, or /create-character first.")
		return
	}

	if pc.Status != "approved" {
		// Never leak the DM's private feedback to anyone but the owner.
		if !viewingSelf {
			respondEphemeral(h.session, interaction, "That player's character hasn't been approved yet.")
			return
		}
		respondEphemeral(h.session, interaction, buildNotApprovedMessage(pc))
		return
	}

	ch, err := h.lookup.GetCharacter(context.Background(), pc.CharacterID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not load your character. Please try again later.")
		return
	}

	embed := h.buildCharacterEmbed(ch)

	// The portal sheet route admits the owner, the campaign DM, and any player
	// in the campaign, so surface the link for both self and party views. The
	// portal independently enforces access (403 for outsiders).
	portalLink := fmt.Sprintf("%s/portal/character/%s", h.portalBaseURL, ch.ID.String())
	content := fmt.Sprintf("View full character sheet: %s", portalLink)

	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Embeds:  []*discordgo.MessageEmbed{embed},
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// HandleEdit processes an /edit-character interaction: it resolves the caller's
// own character and replies with an ephemeral link to the portal builder in
// edit mode. Editing another player's character is a DM action available from
// the dashboard Party page, so this command is self-only.
func (h *CharacterHandler) HandleEdit(interaction *discordgo.Interaction) {
	callerID := interactionUserID(interaction)

	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	pc, err := h.lookup.GetPlayerCharacterByDiscordUser(context.Background(), refdata.GetPlayerCharacterByDiscordUserParams{
		CampaignID:    campaign.ID,
		DiscordUserID: callerID,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, "No character found. Use /register, /import, or /create-character first.")
		return
	}

	editLink := fmt.Sprintf("%s/portal/character/%s/edit", h.portalBaseURL, pc.CharacterID.String())
	respondEphemeral(h.session, interaction, fmt.Sprintf("Edit your character here: %s\n\nYour changes are sent back to the DM for re-approval.", editLink))
}

// characterTargetOption returns the snowflake ID of the optional "target" user
// option, or "" when absent. ApplicationCommandOptionUser values arrive over the
// wire as a string snowflake, so the value is read as a string rather than via
// discordgo's resolved-user helper (which needs the gateway's resolved data).
func characterTargetOption(interaction *discordgo.Interaction) string {
	for _, opt := range interaction.ApplicationCommandData().Options {
		if opt.Name != "target" {
			continue
		}
		if id, ok := opt.Value.(string); ok {
			return id
		}
	}
	return ""
}

// buildNotApprovedMessage produces the ephemeral text shown when a player runs
// /character on a non-approved submission. When the DM left feedback (i.e. the
// status is changes_requested or rejected with a reason), the feedback is shown
// verbatim along with a "how to resubmit" next step so the player isn't left
// guessing — the feedback used to be DM-only and silently droppable (T22).
func buildNotApprovedMessage(pc refdata.PlayerCharacter) string {
	base := fmt.Sprintf("Your character is currently **%s**. It must be approved by the DM before you can view the full sheet.", pc.Status)
	if !pc.DmFeedback.Valid || pc.DmFeedback.String == "" {
		return base
	}
	return base + fmt.Sprintf("\n\n**DM feedback:** %s\n\nRun `/create-character` to get a fresh link and resubmit.", pc.DmFeedback.String)
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

	profile := character.ProfileFromCharacterData(ch.CharacterData.RawMessage)
	if profile.Appearance != "" {
		// Collapse internal newlines so the appearance reads as one tidy line.
		appearance := strings.Join(strings.Fields(profile.Appearance), " ")
		fmt.Fprintf(&desc, "Appearance: %s\n", truncate(appearance, 180))
	}
	if profile.Backstory != "" {
		// Backstory may keep its newlines; only the length is capped.
		fmt.Fprintf(&desc, "Backstory: %s\n", truncate(profile.Backstory, 400))
	}

	if slotSummary := buildSpellSlotSummary(ch); slotSummary != "" {
		fmt.Fprintf(&desc, "Spell Slots: %s\n", slotSummary)
	}

	if spellSummary := h.buildSpellSummary(ch); spellSummary != "" {
		fmt.Fprintf(&desc, "Spells: %s", spellSummary)
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("⚔️ %s", ch.Name),
		Description: desc.String(),
		Color:       0xe94560, // DnDnD red
	}
}

// buildSpellSummary extracts spells from character_data, enriches from the
// reference table, and returns a count-by-level summary.
func (h *CharacterHandler) buildSpellSummary(ch refdata.Character) string {
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

	// Try DDB format: []character.DDBSpellEntry
	var ddbSpells []character.DDBSpellEntry
	if err := json.Unmarshal(spellsRaw, &ddbSpells); err == nil && len(ddbSpells) > 0 && ddbSpells[0].Name != "" {
		// Collect IDs for enrichment
		ids := make([]string, len(ddbSpells))
		for i, s := range ddbSpells {
			ids[i] = character.Slugify(s.Name)
		}
		enriched := h.lookupSpellLevels(ids)
		for _, s := range ddbSpells {
			id := character.Slugify(s.Name)
			if ref, ok := enriched[id]; ok {
				counts[int(ref.Level)]++
			} else {
				counts[s.Level]++
			}
		}
	} else {
		// Try portal format: []string
		var portalSpells []string
		if err := json.Unmarshal(spellsRaw, &portalSpells); err != nil || len(portalSpells) == 0 {
			return ""
		}
		enriched := h.lookupSpellLevels(portalSpells)
		for _, id := range portalSpells {
			if ref, ok := enriched[id]; ok {
				counts[int(ref.Level)]++
			} else {
				counts[0]++ // fallback to cantrip if not found
			}
		}
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

// lookupSpellLevels fetches spells by IDs and returns a map of ID->Spell.
func (h *CharacterHandler) lookupSpellLevels(ids []string) map[string]refdata.Spell {
	if len(ids) == 0 {
		return nil
	}
	spells, err := h.lookup.GetSpellsByIDs(context.Background(), ids)
	if err != nil {
		return nil
	}
	m := make(map[string]refdata.Spell, len(spells))
	for _, s := range spells {
		m[s.ID] = s
	}
	return m
}

// buildSpellSlotSummary renders the character's spell slots for the /character
// embed: standard slots when present, plus a pact-magic line for warlocks. A
// pure warlock's slots live only in pact_magic_slots (spell_slots is empty), so
// reading both is what keeps warlocks from showing nothing (ISSUE-012). Returns
// "" when the character has no slots of either kind. Mirrors the character
// card's "1st: N/M | ... | Pact Magic: N × Lvl L" format.
func buildSpellSlotSummary(ch refdata.Character) string {
	var parts []string

	if ch.SpellSlots.Valid && len(ch.SpellSlots.RawMessage) > 0 {
		var slots map[string]character.SlotInfo
		if err := json.Unmarshal(ch.SpellSlots.RawMessage, &slots); err == nil {
			levels := make([]int, 0, len(slots))
			byLevel := make(map[int]character.SlotInfo, len(slots))
			for k, v := range slots {
				lvl, convErr := strconv.Atoi(k)
				if convErr != nil {
					continue
				}
				levels = append(levels, lvl)
				byLevel[lvl] = v
			}
			sort.Ints(levels)
			for _, lvl := range levels {
				s := byLevel[lvl]
				parts = append(parts, fmt.Sprintf("%s: %d/%d", slotOrdinal(lvl), s.Current, s.Max))
			}
		}
	}

	if ch.PactMagicSlots.Valid && len(ch.PactMagicSlots.RawMessage) > 0 {
		var pact character.PactMagicSlots
		if err := json.Unmarshal(ch.PactMagicSlots.RawMessage, &pact); err == nil && pact.Max > 0 {
			parts = append(parts, fmt.Sprintf("Pact Magic: %d × Lvl %d", pact.Current, pact.SlotLevel))
		}
	}

	return strings.Join(parts, " | ")
}

// truncate clamps s to at most n runes, appending "…" when it had to cut. It
// counts runes (not bytes) so multibyte appearance/backstory text isn't sliced
// mid-character. Strings already within the limit are returned unchanged.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
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
