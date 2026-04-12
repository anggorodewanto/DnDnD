package discord

import (
	"context"
	"encoding/json"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/status"
)

// StatusCampaignProvider resolves a guild to its campaign.
type StatusCampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// StatusCharacterLookup resolves a Discord user to their character.
type StatusCharacterLookup interface {
	GetCharacterByCampaignAndDiscord(ctx context.Context, campaignID uuid.UUID, discordUserID string) (refdata.Character, error)
}

// StatusEncounterProvider finds the active encounter for a user.
type StatusEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
}

// StatusCombatantLookup lists combatants in an encounter.
type StatusCombatantLookup interface {
	ListCombatantsByEncounterID(ctx context.Context, encounterID uuid.UUID) ([]refdata.Combatant, error)
}

// StatusConcentrationLookup lists concentration zones for a combatant.
type StatusConcentrationLookup interface {
	ListConcentrationZonesByCombatant(ctx context.Context, sourceCombatantID uuid.UUID) ([]refdata.EncounterZone, error)
}

// StatusReactionLookup lists active reaction declarations for a combatant.
type StatusReactionLookup interface {
	ListActiveReactionDeclarationsByCombatant(ctx context.Context, arg refdata.ListActiveReactionDeclarationsByCombatantParams) ([]refdata.ReactionDeclaration, error)
}

// StatusHandler handles the /status slash command.
type StatusHandler struct {
	session             Session
	campaignProvider    StatusCampaignProvider
	characterLookup     StatusCharacterLookup
	encounterProvider   StatusEncounterProvider
	combatantLookup     StatusCombatantLookup
	concentrationLookup StatusConcentrationLookup
	reactionLookup      StatusReactionLookup
}

// NewStatusHandler creates a new StatusHandler.
func NewStatusHandler(
	session Session,
	campaignProvider StatusCampaignProvider,
	characterLookup StatusCharacterLookup,
	encounterProvider StatusEncounterProvider,
	combatantLookup StatusCombatantLookup,
	concentrationLookup StatusConcentrationLookup,
	reactionLookup StatusReactionLookup,
) *StatusHandler {
	return &StatusHandler{
		session:             session,
		campaignProvider:    campaignProvider,
		characterLookup:     characterLookup,
		encounterProvider:   encounterProvider,
		combatantLookup:     combatantLookup,
		concentrationLookup: concentrationLookup,
		reactionLookup:      reactionLookup,
	}
}

// Handle processes the /status command interaction.
func (h *StatusHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
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

	info := h.buildStatusInfo(ctx, interaction.GuildID, userID, char)
	respondEphemeral(h.session, interaction, status.FormatStatus(info))
}

// buildStatusInfo gathers all status data for the character, enriching with
// combat data if the character is in an active encounter.
func (h *StatusHandler) buildStatusInfo(ctx context.Context, guildID, userID string, char refdata.Character) status.Info {
	info := status.Info{
		CharacterName: char.Name,
	}

	// Always populate class-based feature uses (ki, sorcery points) from character data.
	h.populateFeatureUses(&info, char)

	// Try to find an active encounter.
	comb, encounterID, found := h.findCombatant(ctx, guildID, userID, char.ID)
	if !found {
		return info
	}

	info.ShortID = comb.ShortID

	// Combat-specific data from combatant.
	h.populateCombatantState(&info, comb)

	// Concentration zones.
	h.populateConcentration(ctx, &info, comb.ID)

	// Reaction declarations and readied actions.
	h.populateReactions(ctx, &info, comb.ID, encounterID)

	return info
}

// findCombatant attempts to locate the character's combatant in an active encounter.
func (h *StatusHandler) findCombatant(ctx context.Context, guildID, userID string, charID uuid.UUID) (refdata.Combatant, uuid.UUID, bool) {
	if h.encounterProvider == nil || h.combatantLookup == nil {
		return refdata.Combatant{}, uuid.Nil, false
	}

	encounterID, err := h.encounterProvider.ActiveEncounterForUser(ctx, guildID, userID)
	if err != nil {
		return refdata.Combatant{}, uuid.Nil, false
	}

	combatants, err := h.combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return refdata.Combatant{}, uuid.Nil, false
	}

	for _, c := range combatants {
		if c.CharacterID.Valid && c.CharacterID.UUID == charID {
			return c, encounterID, true
		}
	}

	return refdata.Combatant{}, uuid.Nil, false
}

// populateCombatantState fills in conditions, temp HP, exhaustion, rage, wild shape,
// and bardic inspiration from the combatant data.
func (h *StatusHandler) populateCombatantState(info *status.Info, comb refdata.Combatant) {
	// Conditions
	conds, err := combat.ListConditions(comb.Conditions)
	if err == nil {
		for _, c := range conds {
			entry := status.ConditionEntry{Name: titleCase(c.Condition)}
			if c.DurationRounds > 0 {
				entry.RemainingRounds = c.DurationRounds
			}
			info.Conditions = append(info.Conditions, entry)
		}
	}

	// Temp HP
	info.TempHP = int(comb.TempHp)

	// Exhaustion
	info.ExhaustionLevel = int(comb.ExhaustionLevel)

	// Rage
	if comb.IsRaging {
		info.IsRaging = true
		if comb.RageRoundsRemaining.Valid {
			info.RageRoundsRemaining = int(comb.RageRoundsRemaining.Int32)
		}
	}

	// Wild Shape
	if comb.IsWildShaped {
		info.IsWildShaped = true
		if comb.WildShapeCreatureRef.Valid {
			info.WildShapeCreature = comb.WildShapeCreatureRef.String
		}
	}

	// Bardic Inspiration
	if comb.BardicInspirationDie.Valid && comb.BardicInspirationDie.String != "" {
		info.BardicInspirationDie = comb.BardicInspirationDie.String
		if comb.BardicInspirationSource.Valid {
			info.BardicInspirationSrc = comb.BardicInspirationSource.String
		}
	}
}

// populateConcentration queries concentration zones for the combatant.
func (h *StatusHandler) populateConcentration(ctx context.Context, info *status.Info, combatantID uuid.UUID) {
	if h.concentrationLookup == nil {
		return
	}
	zones, err := h.concentrationLookup.ListConcentrationZonesByCombatant(ctx, combatantID)
	if err != nil || len(zones) == 0 {
		return
	}
	// A combatant can only concentrate on one spell at a time.
	info.Concentration = zones[0].SourceSpell
}

// populateReactions queries active reaction declarations and splits them into
// regular reactions and readied actions.
func (h *StatusHandler) populateReactions(ctx context.Context, info *status.Info, combatantID, encounterID uuid.UUID) {
	if h.reactionLookup == nil {
		return
	}
	reactions, err := h.reactionLookup.ListActiveReactionDeclarationsByCombatant(ctx, refdata.ListActiveReactionDeclarationsByCombatantParams{
		CombatantID: combatantID,
		EncounterID: encounterID,
	})
	if err != nil {
		return
	}
	for _, r := range reactions {
		if r.IsReadiedAction {
			info.ReadiedActions = append(info.ReadiedActions, r.Description)
		} else {
			info.Reactions = append(info.Reactions, r.Description)
		}
	}
}

// populateFeatureUses populates ki points and sorcery points from character data.
func (h *StatusHandler) populateFeatureUses(info *status.Info, char refdata.Character) {
	// Parse classes to determine max values
	var classes []classEntry
	if len(char.Classes) > 0 {
		_ = json.Unmarshal(char.Classes, &classes)
	}

	monkLevel := classLevelFrom(classes, "Monk")
	if monkLevel >= 2 {
		_, remaining, err := combat.ParseFeatureUses(char, combat.FeatureKeyKi)
		if err == nil {
			info.HasKi = true
			info.KiCurrent = remaining
			info.KiMax = monkLevel
		}
	}

	sorcLevel := classLevelFrom(classes, "Sorcerer")
	if sorcLevel >= 1 {
		_, remaining, err := combat.ParseFeatureUses(char, combat.FeatureKeySorceryPoints)
		if err == nil {
			info.HasSorcery = true
			info.SorceryCurrent = remaining
			info.SorceryMax = sorcLevel
		}
	}
}

// classEntry is a minimal struct for parsing the classes JSON array.
type classEntry struct {
	Class string `json:"class"`
	Level int    `json:"level"`
}

// classLevelFrom returns the level for the named class, case-insensitive.
func classLevelFrom(classes []classEntry, name string) int {
	for _, c := range classes {
		if equalFoldASCII(c.Class, name) {
			return c.Level
		}
	}
	return 0
}

// equalFoldASCII is a simple ASCII case-insensitive comparison.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// titleCase capitalises the first letter of a string.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
