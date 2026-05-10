package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
)

// PrepareService is the combat-side surface /prepare needs. *combat.Service
// satisfies it structurally.
type PrepareService interface {
	GetPreparationInfo(ctx context.Context, charID uuid.UUID, className, subclass string) (combat.PreparationInfo, error)
	PrepareSpells(ctx context.Context, in combat.PrepareSpellsInput) (combat.PrepareSpellsResult, error)
}

// PrepareEncounterProvider provides the (optional) encounter lookup used to
// gate /prepare from running during active combat. Resolution failure is
// tolerated so a player out-of-encounter can still call /prepare.
type PrepareEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
}

// PrepareCampaignProvider resolves the guild → campaign mapping needed to
// look up the invoker's character.
type PrepareCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// PrepareCharacterLookup resolves a Discord user to their character within
// a campaign.
type PrepareCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// PrepareHandler handles the /prepare slash command. The MVP UX is text-
// based: with no `--spells` arg, the handler posts the ephemeral text
// preview produced by combat.FormatPreparationMessage. With `--spells`,
// the handler commits the comma-separated list via PrepareSpells.
//
// The paginated select-menu UX (per spec lines 1018–1026) is deferred —
// see chunk5_spells_reactions.md Phase 65.
type PrepareHandler struct {
	session         Session
	prepareService  PrepareService
	encounterProv   PrepareEncounterProvider
	campaignProv    PrepareCampaignProvider
	characterLookup PrepareCharacterLookup
}

// preparedClassEntry is the local subset of character.ClassEntry the
// handler reads from the character.Classes JSON column. We unmarshal into
// this rather than depending on the character package directly.
type preparedClassEntry struct {
	Class    string `json:"class"`
	Subclass string `json:"subclass,omitempty"`
	Level    int    `json:"level"`
}

// NewPrepareHandler constructs a /prepare handler.
func NewPrepareHandler(
	session Session,
	prepareService PrepareService,
	encounterProv PrepareEncounterProvider,
	campaignProv PrepareCampaignProvider,
	characterLookup PrepareCharacterLookup,
) *PrepareHandler {
	return &PrepareHandler{
		session:         session,
		prepareService:  prepareService,
		encounterProv:   encounterProv,
		campaignProv:    campaignProv,
		characterLookup: characterLookup,
	}
}

// Handle processes the /prepare command interaction.
func (h *PrepareHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()
	userID := discordUserID(interaction)

	classOverride := strings.TrimSpace(strings.ToLower(optionString(interaction, "class")))
	subclassOverride := strings.TrimSpace(strings.ToLower(optionString(interaction, "subclass")))
	spellsArg := strings.TrimSpace(optionString(interaction, "spells"))

	// Encounter resolution is best-effort — a player who isn't in an
	// encounter (e.g. between sessions) can still re-prep. When they ARE
	// in an active combat, gate the commit path.
	if h.encounterProv != nil {
		encID, err := h.encounterProv.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
		if err == nil {
			enc, err := h.encounterProv.GetEncounter(ctx, encID)
			if err == nil && enc.Status == "active" && spellsArg != "" {
				respondEphemeral(h.session, interaction, "❌ /prepare is only available out of combat — finish or pause the encounter first.")
				return
			}
		}
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not resolve your campaign for this guild.")
		return
	}

	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "Could not find a registered character for you in this campaign.")
		return
	}

	className, subclass, ok := h.resolveClass(char, classOverride, subclassOverride)
	if !ok {
		respondEphemeral(h.session, interaction, "❌ You are not a prepared caster (Cleric, Druid, or Paladin).")
		return
	}

	if spellsArg == "" {
		h.preview(ctx, interaction, char, className, subclass)
		return
	}

	h.commit(ctx, interaction, char, className, subclass, spellsArg)
}

// resolveClass picks the prepared-caster class entry from the character's
// classes JSON, optionally honoring an explicit override. Returns
// (className, subclass, true) on success and ("", "", false) when the
// character has no prepared-caster class.
func (h *PrepareHandler) resolveClass(char refdata.Character, classOverride, subclassOverride string) (string, string, bool) {
	var entries []preparedClassEntry
	if err := json.Unmarshal(char.Classes, &entries); err != nil {
		return "", "", false
	}

	if classOverride != "" {
		for _, c := range entries {
			if !strings.EqualFold(c.Class, classOverride) {
				continue
			}
			subclass := c.Subclass
			if subclassOverride != "" {
				subclass = subclassOverride
			}
			return strings.ToLower(c.Class), strings.ToLower(subclass), true
		}
		return "", "", false
	}

	for _, c := range entries {
		if !combat.IsPreparedCaster(c.Class) {
			continue
		}
		subclass := c.Subclass
		if subclassOverride != "" {
			subclass = subclassOverride
		}
		return strings.ToLower(c.Class), strings.ToLower(subclass), true
	}
	return "", "", false
}

// preview builds the ephemeral text preview using FormatPreparationMessage.
func (h *PrepareHandler) preview(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, className, subclass string) {
	info, err := h.prepareService.GetPreparationInfo(ctx, char.ID, className, subclass)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to load preparation info: %v", err))
		return
	}
	msg := combat.FormatPreparationMessage(char.Name, info)
	msg += "\n_To commit a list, run `/prepare spells:id1,id2,id3`._"
	respondEphemeral(h.session, interaction, msg)
}

// commit invokes PrepareSpells with the parsed comma-separated spell list
// and posts a confirmation summary on success.
func (h *PrepareHandler) commit(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, className, subclass, spellsArg string) {
	selected := splitSpellsArg(spellsArg)
	if len(selected) == 0 {
		respondEphemeral(h.session, interaction, "Please supply at least one spell ID (e.g. `spells:bless,cure-wounds`).")
		return
	}

	result, err := h.prepareService.PrepareSpells(ctx, combat.PrepareSpellsInput{
		CharacterID: char.ID,
		ClassName:   className,
		Subclass:    subclass,
		Selected:    selected,
	})
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to prepare spells: %v", err))
		return
	}

	respondEphemeral(h.session, interaction, fmt.Sprintf(
		"✅ Prepared %d/%d spells for %s.",
		result.PreparedCount, result.MaxPrepared, char.Name,
	))
}

// splitSpellsArg splits the comma-separated spell IDs and trims whitespace
// around each entry. Empty entries are dropped so a trailing comma is fine.
func splitSpellsArg(arg string) []string {
	parts := strings.Split(arg, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
