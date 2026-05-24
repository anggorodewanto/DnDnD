package discord

import (
	"context"
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

// PrepareHandler handles the /prepare slash command. With no `spells` arg the
// handler posts an ephemeral link to the web spell-prep page (browsing the long
// spell list in Discord is unwieldy). With `spells:id1,id2,…` it commits the
// comma-separated list directly via PrepareSpells. Both paths share the same
// server-side validation (count + slot-level limits).
type PrepareHandler struct {
	session         Session
	prepareService  PrepareService
	encounterProv   PrepareEncounterProvider
	campaignProv    PrepareCampaignProvider
	characterLookup PrepareCharacterLookup
	cardUpdater     CardUpdater // SR-007
	portalBaseURL   string
}

// SetCardUpdater wires the SR-007 character-card refresh callback fired
// after a successful /prepare commit.
func (h *PrepareHandler) SetCardUpdater(u CardUpdater) {
	h.cardUpdater = u
}

// SetPortalBaseURL sets the base URL used to build the /prepare web-page link.
// Empty falls back to defaultPortalBaseURL.
func (h *PrepareHandler) SetPortalBaseURL(baseURL string) {
	h.portalBaseURL = strings.TrimRight(baseURL, "/")
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
// classes JSON, optionally honoring an explicit override. It delegates to the
// pure combat.ResolvePreparedClass so the web endpoint can share the logic.
func (h *PrepareHandler) resolveClass(char refdata.Character, classOverride, subclassOverride string) (string, string, bool) {
	return combat.ResolvePreparedClass(char.Classes, classOverride, subclassOverride)
}

// preview posts the link to the web spell-preparation page (browsing the long
// spell list in Discord is unwieldy). The prepared-spell cap is surfaced inline.
// Committing a list directly via `/prepare spells:...` remains available.
func (h *PrepareHandler) preview(ctx context.Context, interaction *discordgo.Interaction, char refdata.Character, className, subclass string) {
	info, err := h.prepareService.GetPreparationInfo(ctx, char.ID, className, subclass)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to load preparation info: %v", err))
		return
	}
	base := h.portalBaseURL
	if base == "" {
		base = defaultPortalBaseURL
	}
	link := fmt.Sprintf("%s/portal/character/%s/prepare", base, char.ID.String())
	msg := fmt.Sprintf(
		"**%s** — prepare up to **%d** spells in the browser:\n%s\n\n_Or commit a list directly: `/prepare spells:id1,id2,id3`._",
		char.Name, info.MaxPrepared, link,
	)
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

	// SR-007: refresh #character-cards after a successful /prepare commit.
	notifyCardUpdate(ctx, h.cardUpdater, char.ID)

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
