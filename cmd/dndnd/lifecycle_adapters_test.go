package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// captureSender is a minimal dmqueue.Sender that records posted messages so
// tests can drive the real DefaultNotifier (insert + send) without Discord.
type captureSender struct {
	sentChannels []string
	sentContents []string
}

func (c *captureSender) Send(channelID, content string) (string, error) {
	c.sentChannels = append(c.sentChannels, channelID)
	c.sentContents = append(c.sentContents, content)
	return "msg-1", nil
}

func (c *captureSender) Edit(string, string, string) error { return nil }

func newTestNotifier(sender *captureSender) *dmqueue.DefaultNotifier {
	return dmqueue.NewNotifier(
		sender,
		func(guildID string) string { return "dmq-" + guildID },
		func(itemID string) string { return "/dashboard/queue/" + itemID },
	)
}

// T09 / Finding 6f: a combat-map render failure posts a scoped #dm-queue alert.
func TestCombatMapRenderFailureNotifierAdapter_PostsScopedAlert(t *testing.T) {
	campID := uuid.New()
	encID := uuid.New()
	sender := &captureSender{}
	notifier := newTestNotifier(sender)

	var gotEncounterID uuid.UUID
	adapter := newCombatMapRenderFailureNotifierAdapter(notifier,
		func(_ context.Context, encounterID uuid.UUID) (refdata.Campaign, error) {
			gotEncounterID = encounterID
			return refdata.Campaign{ID: campID, GuildID: "g-123"}, nil
		})
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}

	adapter.NotifyCombatMapRenderFailed(context.Background(), encID)

	if gotEncounterID != encID {
		t.Errorf("campaign resolved for wrong encounter: got %s want %s", gotEncounterID, encID)
	}

	pending := notifier.ListPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending dm-queue item, got %d", len(pending))
	}
	ev := pending[0].Event
	if ev.Kind != dmqueue.KindMapRenderFailure {
		t.Errorf("kind = %q, want %q", ev.Kind, dmqueue.KindMapRenderFailure)
	}
	if ev.GuildID != "g-123" {
		t.Errorf("guild = %q, want g-123", ev.GuildID)
	}
	if ev.CampaignID != campID.String() {
		t.Errorf("campaign = %q, want %q", ev.CampaignID, campID.String())
	}
	if ev.ExtraMetadata["encounter_id"] != encID.String() {
		t.Errorf("encounter_id metadata = %q, want %q", ev.ExtraMetadata["encounter_id"], encID.String())
	}
	// Posted to the resolved guild's #dm-queue channel.
	if len(sender.sentChannels) != 1 || sender.sentChannels[0] != "dmq-g-123" {
		t.Errorf("sent channels = %v, want [dmq-g-123]", sender.sentChannels)
	}
}

// When the campaign cannot be resolved the item has nowhere to be scoped, so
// the adapter skips the post (the render error is already logged at ERROR in
// discord.PostCombatMap). It must not panic.
func TestCombatMapRenderFailureNotifierAdapter_CampaignLookupErrorSkips(t *testing.T) {
	sender := &captureSender{}
	notifier := newTestNotifier(sender)
	adapter := newCombatMapRenderFailureNotifierAdapter(notifier,
		func(context.Context, uuid.UUID) (refdata.Campaign, error) {
			return refdata.Campaign{}, errors.New("not found")
		})

	adapter.NotifyCombatMapRenderFailed(context.Background(), uuid.New())

	if got := len(notifier.ListPending()); got != 0 {
		t.Errorf("expected no dm-queue item on lookup error, got %d", got)
	}
}

// A nil notifier (headless deploy) yields a nil adapter and is a safe no-op.
func TestCombatMapRenderFailureNotifierAdapter_NilDependenciesNoOp(t *testing.T) {
	if a := newCombatMapRenderFailureNotifierAdapter(nil, func(context.Context, uuid.UUID) (refdata.Campaign, error) {
		return refdata.Campaign{}, nil
	}); a != nil {
		t.Error("expected nil adapter when notifier is nil")
	}

	// A nil-typed adapter must not panic when invoked.
	var a *combatMapRenderFailureNotifierAdapter
	a.NotifyCombatMapRenderFailed(context.Background(), uuid.New())
}
