package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// APP-5: /initiative lets a player stage their OWN initiative total before
// combat starts, so the DM never has to roll (or override) a player's die. The
// value is written to a per-(campaign, character) staging table; StartCombat
// folds it into the APP-1 supplied-initiative map and clears it. Players roll
// their own d20 and report the total here — the app never rolls for them.

// InitiativeStagingStore records, reads, and clears a player's staged
// initiative. dmQueueItemID carries the id of the #dm-queue / DM-Console item
// the stage posted (empty when no notifier is wired) so a re-roll, a clear, or
// StartCombat can cancel the prior item.
type InitiativeStagingStore interface {
	UpsertPendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID, roll int32, dmQueueItemID string) error
	GetPendingInitiative(ctx context.Context, campaignID, characterID uuid.UUID) (roll int32, dmQueueItemID string, found bool, err error)
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
	notifier         dmqueue.Notifier
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

// SetNotifier wires the dm-queue notifier so a staged /initiative surfaces as
// an item in #dm-queue and the DM Console. Nil-safe: with no notifier wired
// (unit tests, headless callers) the handler stages silently and skips all
// queue ops. Mirrors CheckHandler.SetNotifier.
func (h *InitiativeHandler) SetNotifier(n dmqueue.Notifier) {
	h.notifier = n
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
		// Cancel the staged item's #dm-queue entry (if any) before deleting the
		// row, so a cleared stage doesn't leave a dangling pending item.
		h.cancelStagedInitiative(ctx, campaign.ID, char.ID, "cleared by player")
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

	// APP-5: surface the staged roll in #dm-queue + DM Console. A re-roll first
	// cancels the prior item (so the DM never sees a stale duplicate), then posts
	// a fresh one whose id is stored on the row for later cancel (clear /
	// StartCombat). Nil-safe: with no notifier wired the player still stages.
	itemID := h.postStagedInitiative(ctx, interaction, campaign.ID, char, roll)
	if err := h.store.UpsertPendingInitiative(ctx, campaign.ID, char.ID, int32(roll), itemID); err != nil {
		// Roll back the just-posted item so a failed stage never leaves an
		// orphaned #dm-queue entry the DM can't tie back to a staged roll.
		if h.notifier != nil && itemID != "" {
			_ = h.notifier.Cancel(ctx, itemID, "staging failed")
		}
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
	cur, _, found, err := h.store.GetPendingInitiative(ctx, campaignID, char.ID)
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

// postStagedInitiative cancels any prior staged item for this (campaign,
// character) — so a re-roll replaces rather than duplicates the DM's view —
// then posts a fresh initiative_staged item, returning its id (stored on the
// staging row for later cancel). Nil-safe: returns "" when no notifier is
// wired or the post fails, so the caller records no dangling id. CampaignID is
// threaded onto the Event so PgStore.Insert can persist the row (SR-002).
func (h *InitiativeHandler) postStagedInitiative(ctx context.Context, interaction *discordgo.Interaction, campaignID uuid.UUID, char refdata.Character, roll int) string {
	if h.notifier == nil {
		return ""
	}
	if _, priorID, found, err := h.store.GetPendingInitiative(ctx, campaignID, char.ID); err == nil && found && priorID != "" {
		_ = h.notifier.Cancel(ctx, priorID, "re-rolled")
	}
	itemID, err := h.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindInitiativeStaged,
		PlayerName: char.Name,
		Summary:    fmt.Sprintf("rolled initiative %d (pre-combat)", roll),
		GuildID:    interaction.GuildID,
		CampaignID: campaignID.String(),
	})
	if err != nil {
		return ""
	}
	return itemID
}

// cancelStagedInitiative cancels the #dm-queue item recorded on the current
// staging row (if any). Nil-safe on the notifier and best-effort: a missing
// row, empty id, or Cancel error is swallowed so it never blocks a clear.
func (h *InitiativeHandler) cancelStagedInitiative(ctx context.Context, campaignID, characterID uuid.UUID, reason string) {
	if h.notifier == nil {
		return
	}
	_, itemID, found, err := h.store.GetPendingInitiative(ctx, campaignID, characterID)
	if err != nil || !found || itemID == "" {
		return
	}
	_ = h.notifier.Cancel(ctx, itemID, reason)
}
