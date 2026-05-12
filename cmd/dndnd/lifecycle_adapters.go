package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/discord"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/levelup"
	"github.com/ab/dndnd/internal/loot"
	"github.com/ab/dndnd/internal/narration"
	"github.com/ab/dndnd/internal/refdata"
)

// combatLogNotifierAdapter satisfies combat.CombatLogNotifier by mirroring
// lifecycle announcements ("Combat ended: …", recovered-ammo summaries) to
// the encounter's #combat-log channel (B-26b-combat-log-announcement). A
// nil Discord session OR a nil CampaignSettingsProvider degrades to a
// silent no-op so headless / test deploys keep working.
type combatLogNotifierAdapter struct {
	session discord.Session
	csp     discord.CampaignSettingsProvider
}

func newCombatLogNotifierAdapter(session discord.Session, csp discord.CampaignSettingsProvider) *combatLogNotifierAdapter {
	if session == nil || csp == nil {
		return nil
	}
	return &combatLogNotifierAdapter{session: session, csp: csp}
}

func (a *combatLogNotifierAdapter) PostCombatLog(ctx context.Context, encounterID uuid.UUID, content string) {
	if a == nil || a.session == nil || a.csp == nil || content == "" {
		return
	}
	channelIDs, err := a.csp.GetChannelIDs(ctx, encounterID)
	if err != nil {
		return
	}
	combatLog, ok := channelIDs["combat-log"]
	if !ok || combatLog == "" {
		return
	}
	_, _ = a.session.ChannelMessageSend(combatLog, content)
}

// lootPoolCreatorAdapter satisfies combat.LootPoolCreator by delegating to
// loot.Service.CreateLootPool. The "already exists" sentinel is swallowed
// here so combat.Service can treat the auto-create path as idempotent
// against the manual DM-side route (B-26b-loot-auto-create).
type lootPoolCreatorAdapter struct {
	svc *loot.Service
}

func newLootPoolCreatorAdapter(svc *loot.Service) *lootPoolCreatorAdapter {
	if svc == nil {
		return nil
	}
	return &lootPoolCreatorAdapter{svc: svc}
}

func (a *lootPoolCreatorAdapter) CreateLootPool(ctx context.Context, encounterID uuid.UUID) error {
	if a == nil || a.svc == nil {
		return nil
	}
	_, err := a.svc.CreateLootPool(ctx, encounterID)
	if err == nil {
		return nil
	}
	if errors.Is(err, loot.ErrPoolAlreadyExists) {
		// Idempotent — the DM may have created the pool manually before
		// EndCombat ran. Treat as success so the lifecycle hook stays
		// safe to re-run.
		return nil
	}
	return err
}

// lootSvcForCombat builds the loot.Service used as the EndCombat
// auto-create target. Returns nil when *refdata.Queries is unavailable so
// the adapter falls back to a silent no-op in headless deploys.
func lootSvcForCombat(queries *refdata.Queries) *loot.Service {
	if queries == nil {
		return nil
	}
	return loot.NewService(queries)
}

// hostilesDefeatedNotifierAdapter satisfies
// combat.HostilesDefeatedNotifier by posting a DM-facing prompt to
// #dm-queue suggesting `/end-combat` (B-26b-all-hostiles-defeated-prompt).
// The notifier is best-effort: a nil dmqueue.Notifier degrades to a
// silent no-op so headless deploys keep working.
type hostilesDefeatedNotifierAdapter struct {
	notifier dmqueue.Notifier
}

func newHostilesDefeatedNotifierAdapter(n dmqueue.Notifier) *hostilesDefeatedNotifierAdapter {
	if n == nil {
		return nil
	}
	return &hostilesDefeatedNotifierAdapter{notifier: n}
}

func (a *hostilesDefeatedNotifierAdapter) NotifyHostilesDefeated(ctx context.Context, encounterID uuid.UUID) {
	if a == nil || a.notifier == nil {
		return
	}
	_, _ = a.notifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindFreeformAction,
		PlayerName: "Combat Lifecycle",
		Summary:    "All hostiles are down — consider `/end-combat` to close out the encounter.",
		ExtraMetadata: map[string]string{
			"encounter_id": encounterID.String(),
		},
	})
}

// levelUpStoryQueries is the subset of refdata.Queries the level-up
// StoryPoster adapter needs: character lookup (to find the campaign) and
// campaign lookup (to find the Discord guild). Declared as an interface
// so unit tests can inject a fake without touching the DB.
type levelUpStoryQueries interface {
	GetCharacter(ctx context.Context, id uuid.UUID) (refdata.Character, error)
	GetCampaignByID(ctx context.Context, id uuid.UUID) (refdata.Campaign, error)
}

// narrationPoster is the subset of narration.Poster the level-up
// StoryPoster adapter needs. *discord.NarrationPoster satisfies it.
type narrationPoster interface {
	PostToStory(guildID, body string, embeds []narration.DiscordEmbed, attachmentURLs []string) ([]string, error)
}

// levelUpStoryPosterAdapter satisfies levelup.StoryPoster by resolving the
// character → campaign → guild chain via refdata.Queries and posting the
// formatted announcement through narration.Poster (the same surface the
// /narrate handler uses). A nil poster OR a nil queries adapter degrades
// to a silent no-op so headless deploys keep working.
type levelUpStoryPosterAdapter struct {
	queries levelUpStoryQueries
	poster  narrationPoster
}

func newLevelUpStoryPosterAdapter(q levelUpStoryQueries, p narrationPoster) *levelUpStoryPosterAdapter {
	if q == nil || p == nil {
		return nil
	}
	return &levelUpStoryPosterAdapter{queries: q, poster: p}
}

func (a *levelUpStoryPosterAdapter) PostPublicLevelUp(ctx context.Context, characterID uuid.UUID, characterName string, newLevel int) error {
	if a == nil || a.queries == nil || a.poster == nil {
		return nil
	}
	char, err := a.queries.GetCharacter(ctx, characterID)
	if err != nil {
		return fmt.Errorf("resolving character for public level-up: %w", err)
	}
	camp, err := a.queries.GetCampaignByID(ctx, char.CampaignID)
	if err != nil {
		return fmt.Errorf("resolving campaign for public level-up: %w", err)
	}
	if camp.GuildID == "" {
		return nil
	}
	body := levelup.FormatPublicLevelUpMessage(characterName, newLevel)
	if _, err := a.poster.PostToStory(camp.GuildID, body, nil, nil); err != nil {
		return fmt.Errorf("posting public level-up to #the-story: %w", err)
	}
	return nil
}

// Compile-time satisfies-assertions so a future refactor of the combat
// service interfaces breaks the build instead of going stale at runtime.
var (
	_ combat.CombatLogNotifier        = (*combatLogNotifierAdapter)(nil)
	_ combat.LootPoolCreator          = (*lootPoolCreatorAdapter)(nil)
	_ combat.HostilesDefeatedNotifier = (*hostilesDefeatedNotifierAdapter)(nil)
	_ levelup.StoryPoster             = (*levelUpStoryPosterAdapter)(nil)
)
