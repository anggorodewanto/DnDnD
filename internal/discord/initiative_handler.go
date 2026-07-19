package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// APP-5: /initiative lets a player stage their OWN initiative total before
// combat starts, so the DM never has to roll (or override) a player's die. The
// value is written to a per-(campaign, character) staging table; StartCombat
// folds it into the APP-1 supplied-initiative map and clears it. Players roll
// their own d20 and report the total here — the app never rolls for them.

// InitiativeStagingStore records, reads, and clears a player's staged initiative.
type InitiativeStagingStore interface {
	UpsertPendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID, roll int32) error
	GetPendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID) (roll int32, found bool, err error)
	DeletePendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID) error
}

// Bounds on a reported initiative total. A natural d20 is 1..20; even with
// large modifiers a real total stays well under 50, so anything outside 1..50
// is a typo we reject rather than stage.
const (
	minInitiative = 1
	maxInitiative = 50
)

// InitiativeHandler serves the /initiative command. It reuses the shared
// campaign + character lookups (CheckCampaignProvider / CheckCharacterLookup)
// and does NOT depend on an active encounter — the whole point is to collect
// initiative before combat exists.
type InitiativeHandler struct {
	session          Session
	campaignProvider CheckCampaignProvider
	characterLookup  CheckCharacterLookup
	store            InitiativeStagingStore
}

// NewInitiativeHandler builds an InitiativeHandler.
func NewInitiativeHandler(session Session, campaignProvider CheckCampaignProvider, characterLookup CheckCharacterLookup, store InitiativeStagingStore) *InitiativeHandler {
	return &InitiativeHandler{
		session:          session,
		campaignProvider: campaignProvider,
		characterLookup:  characterLookup,
		store:            store,
	}
}

// Handle routes /initiative to submit (roll:), clear (clear:), or — with
// neither — echo the caller's currently staged value.
func (h *InitiativeHandler) Handle(interaction *discordgo.Interaction) {
	ctx := context.Background()

	// optionInt returns 0 when the option is absent; a valid initiative total is
	// always >= 1, so roll <= 0 cleanly means "no roll supplied".
	roll := optionInt(interaction, "roll")
	clear := optionBool(interaction, "clear")

	if clear && roll > 0 {
		respondEphemeral(h.session, interaction, "❌ Use either `roll:` or `clear:`, not both.")
		return
	}
	// roll > 0 already implies roll >= minInitiative (1), so only the upper bound
	// can trip here.
	if roll > maxInitiative {
		respondEphemeral(h.session, interaction, fmt.Sprintf("❌ Initiative total must be between %d and %d.", minInitiative, maxInitiative))
		return
	}

	campaign, err := h.campaignProvider.GetCampaignByGuildID(ctx, interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "❌ No campaign found for this server.")
		return
	}
	char, err := h.characterLookup.GetCharacterByCampaignAndDiscord(ctx, campaign.ID, discordUserID(interaction))
	if err != nil {
		respondEphemeral(h.session, interaction, "❌ Could not find your character. Use /register first.")
		return
	}

	if clear {
		if err := h.store.DeletePendingInitiative(ctx, campaign.ID, char.ID); err != nil {
			respondEphemeral(h.session, interaction, "❌ Couldn't clear your staged initiative. Try again.")
			return
		}
		respondPublic(h.session, interaction, fmt.Sprintf("🎲 Cleared your staged initiative, %s.", char.Name))
		return
	}

	if roll <= 0 {
		h.respondWithCurrent(ctx, interaction, campaign.ID, char)
		return
	}

	if err := h.store.UpsertPendingInitiative(ctx, campaign.ID, char.ID, int32(roll)); err != nil {
		respondEphemeral(h.session, interaction, "❌ Couldn't record your initiative. Try again.")
		return
	}
	respondPublic(h.session, interaction, fmt.Sprintf(
		"🎲 Initiative **%d** recorded for %s. The DM will use it when combat starts — re-run `/initiative` to change it, or `/initiative clear:true` to remove it.",
		roll, char.Name))
}

// respondWithCurrent echoes the caller's staged value, or a prompt when none is
// staged. Runs when /initiative is invoked with neither roll: nor clear:.
func (h *InitiativeHandler) respondWithCurrent(ctx context.Context, interaction *discordgo.Interaction, campaignID uuid.UUID, char refdata.Character) {
	cur, found, err := h.store.GetPendingInitiative(ctx, campaignID, char.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, "❌ Couldn't read your staged initiative. Try again.")
		return
	}
	if !found {
		respondEphemeral(h.session, interaction, "🎲 You haven't staged an initiative yet. Run `/initiative roll:<total>` (your d20 roll + your initiative modifier).")
		return
	}
	respondPublic(h.session, interaction, fmt.Sprintf(
		"🎲 Your staged initiative is **%d**, %s. Run `/initiative roll:<total>` to change it, or `/initiative clear:true` to remove it.",
		cur, char.Name))
}
