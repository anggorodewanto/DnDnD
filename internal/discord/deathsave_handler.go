package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// DeathSaveStore persists death-save and HP updates for the dying combatant.
// Implementations are typically `*refdata.Queries`.
type DeathSaveStore interface {
	UpdateCombatantDeathSaves(ctx context.Context, arg refdata.UpdateCombatantDeathSavesParams) (refdata.Combatant, error)
	UpdateCombatantHP(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error)
}

// DeathSaveHandler handles the /deathsave slash command. The command is
// off-turn (a dying PC rolls when the DM ticks the death-save timer or
// invokes the slash) so the TurnGate is intentionally NOT consulted —
// task crit-01a treats /deathsave as exempt even though
// `combat.IsExemptCommand("deathsave")` returns false today.
type DeathSaveHandler struct {
	session           Session
	roller            *dice.Roller
	resolver          ActionEncounterResolver
	combatantLookup   CheckCombatantLookup
	store             DeathSaveStore
	campaignProvider  CheckCampaignProvider
	characterLookup   CheckCharacterLookup
	channelIDProvider CampaignSettingsProvider
}

// NewDeathSaveHandler constructs a /deathsave handler.
func NewDeathSaveHandler(
	session Session,
	roller *dice.Roller,
	resolver ActionEncounterResolver,
	combatantLookup CheckCombatantLookup,
	store DeathSaveStore,
	campaignProvider CheckCampaignProvider,
	characterLookup CheckCharacterLookup,
) *DeathSaveHandler {
	return &DeathSaveHandler{
		session:          session,
		roller:           roller,
		resolver:         resolver,
		combatantLookup:  combatantLookup,
		store:            store,
		campaignProvider: campaignProvider,
		characterLookup:  characterLookup,
	}
}

// SetChannelIDProvider wires a CampaignSettingsProvider so death-save
// outcomes are mirrored to the encounter's #combat-log channel. When
// nil, the outcome is only sent ephemerally to the invoker.
func (h *DeathSaveHandler) SetChannelIDProvider(p CampaignSettingsProvider) {
	h.channelIDProvider = p
}

// Handle processes the /deathsave command interaction.
func (h *DeathSaveHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()
	userID := discordUserID(interaction)

	encounterID, err := h.resolver.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, "You are not in an active encounter.")
		return
	}

	combatant, ok := h.resolveOwnCombatant(ctx, interaction.GuildID, userID, encounterID)
	if !ok {
		respondEphemeral(h.session, interaction, "Could not find your character in this encounter.")
		return
	}

	ds, err := combat.ParseDeathSaves(combatant.DeathSaves.RawMessage)
	if err != nil {
		respondEphemeral(h.session, interaction, "Failed to read death save state.")
		return
	}

	if !combat.IsDying(combatant.IsAlive, int(combatant.HpCurrent), ds) {
		respondEphemeral(h.session, interaction, "You are not dying — death saves only roll while at 0 HP.")
		return
	}

	rollResult, err := h.roller.Roll("1d20")
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to roll death save: %v", err))
		return
	}

	outcome := combat.RollDeathSave(combatant.DisplayName, ds, rollResult.Total)

	if persistErr := h.persistOutcome(ctx, combatant, outcome); persistErr != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Failed to save death save: %v", persistErr))
		return
	}

	msg := joinMessages(outcome.Messages)
	h.postCombatLog(ctx, encounterID, msg)
	respondEphemeral(h.session, interaction, msg)
}

// persistOutcome applies the death-save outcome to the combatant row.
// On nat-20 healing the death-save tallies are reset to zero before the HP
// update so the next time the PC drops to 0 they start fresh.
func (h *DeathSaveHandler) persistOutcome(ctx context.Context, combatant refdata.Combatant, outcome combat.DeathSaveOutcome) error {
	if outcome.HPCurrent > 0 {
		if _, err := h.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
			ID:         combatant.ID,
			DeathSaves: combat.MarshalDeathSaves(combat.DeathSaves{}),
		}); err != nil {
			return err
		}
		_, err := h.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
			ID:        combatant.ID,
			HpCurrent: int32(outcome.HPCurrent),
			TempHp:    combatant.TempHp,
			IsAlive:   true,
		})
		return err
	}

	if _, err := h.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         combatant.ID,
		DeathSaves: combat.MarshalDeathSaves(outcome.DeathSaves),
	}); err != nil {
		return err
	}

	if outcome.IsAlive {
		return nil
	}

	_, err := h.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
		ID:        combatant.ID,
		HpCurrent: 0,
		TempHp:    0,
		IsAlive:   false,
	})
	return err
}

// resolveOwnCombatant maps the invoker's Discord user to their combatant
// in the given encounter. Returns false (without an error path to the
// user message) when the lookup fails.
func (h *DeathSaveHandler) resolveOwnCombatant(ctx context.Context, guildID, userID string, encounterID uuid.UUID) (refdata.Combatant, bool) {
	if h.campaignProvider == nil || h.characterLookup == nil {
		return refdata.Combatant{}, false
	}
	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return refdata.Combatant{}, false
	}
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, userID)
	if err != nil {
		return refdata.Combatant{}, false
	}
	combatants, err := h.combatantLookup.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return refdata.Combatant{}, false
	}
	for _, c := range combatants {
		if c.CharacterID.Valid && c.CharacterID.UUID == char.ID {
			return c, true
		}
	}
	return refdata.Combatant{}, false
}

// postCombatLog mirrors a combat log line to the encounter's #combat-log
// channel. Best-effort: failures (no provider, no channel, or send error)
// are silently ignored so the player still receives the ephemeral.
func (h *DeathSaveHandler) postCombatLog(ctx context.Context, encounterID uuid.UUID, msg string) {
	postCombatLogChannel(ctx, h.session, h.channelIDProvider, encounterID, msg)
}

// joinMessages joins outcome messages with newlines. Most outcomes carry
// exactly one entry; extra lines (e.g. healed-and-stabilized) are stacked.
func joinMessages(msgs []string) string {
	if len(msgs) == 0 {
		return ""
	}
	if len(msgs) == 1 {
		return msgs[0]
	}
	out := msgs[0]
	for _, m := range msgs[1:] {
		out += "\n" + m
	}
	return out
}
