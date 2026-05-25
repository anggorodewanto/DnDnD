package main

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
)

// noopNotifier is the fallback combat.Notifier used when DISCORD_BOT_TOKEN is
// unset. Turn-timer messages are silently dropped so the timer can still
// update DB state (nudge_sent_at, warning_sent_at, etc.) without needing a
// live Discord session.
type noopNotifier struct{}

func (noopNotifier) SendMessage(_ string, _ string) error { return nil }

// portalDMQueueNotifier posts a #dm-queue notice when a player submits a
// character through the web builder. It satisfies portal.DMQueueNotifier.
//
// SR-013 follow-up: /create-character no longer notifies the DM up front (it
// only hands out the builder link), so the notice now fires here, at submit
// time, when there is actually a character pending approval. Best-effort: a nil
// sender (bot offline), an unparseable/unknown campaign, or an unresolved
// channel all degrade to a silent no-op — matching the register/import path.
type portalDMQueueNotifier struct {
	sender          dmqueue.Sender
	queries         *refdata.Queries
	channelForGuild func(guildID string) string
	dmUserForGuild  func(guildID string) string
}

func (n *portalDMQueueNotifier) NotifyDMQueue(ctx context.Context, campaignID, characterName, playerDiscordID, via string) error {
	if n == nil || n.sender == nil || n.queries == nil {
		return nil
	}
	campID, err := uuid.Parse(campaignID)
	if err != nil {
		return nil
	}
	camp, err := n.queries.GetCampaignByID(ctx, campID)
	if err != nil {
		return nil
	}
	channelID := n.channelForGuild(camp.GuildID)
	if channelID == "" {
		return nil
	}
	dmUserID := n.dmUserForGuild(camp.GuildID)
	content := fmt.Sprintf("🆕 <@%s> — **%s** registration by <@%s> via /%s. Pending approval.", dmUserID, characterName, playerDiscordID, via)
	_, err = n.sender.Send(channelID, content)
	return err
}
