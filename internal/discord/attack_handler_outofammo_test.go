package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/combat"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

// The out-of-ammo tests reuse stubUndoNotifier (undo_handler_test.go) as a
// full-surface recording Notifier.

type fakeAmmoCampaignProvider struct {
	id  uuid.UUID
	err error
}

func (f fakeAmmoCampaignProvider) GetCampaignByGuildID(_ context.Context, _ string) (refdata.Campaign, error) {
	if f.err != nil {
		return refdata.Campaign{}, f.err
	}
	return refdata.Campaign{ID: f.id}, nil
}

// TestAttackHandler_OutOfAmmo_RoutesToDMQueue verifies that a shot the service
// rejects with NoAmmunitionError is forwarded to #dm-queue (so the DM can
// waive ammo) and the player is told the DM was flagged — instead of a
// dead-end "no bolts" rejection.
func TestAttackHandler_OutOfAmmo_RoutesToDMQueue(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.attackErr = combat.NoAmmunitionError{AmmoName: "Bolts"}

	notifier := &stubUndoNotifier{}
	campID := uuid.New()
	h.SetNotifier(notifier)
	h.SetCampaignProvider(fakeAmmoCampaignProvider{id: campID})

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	if len(notifier.posted) != 1 {
		t.Fatalf("expected 1 dm-queue post, got %d", len(notifier.posted))
	}
	ev := notifier.posted[0]
	if ev.Kind != dmqueue.KindFreeformAction {
		t.Errorf("expected KindFreeformAction, got %q", ev.Kind)
	}
	if ev.PlayerName != "Aria" {
		t.Errorf("expected player Aria, got %q", ev.PlayerName)
	}
	if !strings.Contains(ev.Summary, "bolts") || !strings.Contains(ev.Summary, "Orc") {
		t.Errorf("expected summary to mention bolts and the target, got %q", ev.Summary)
	}
	if ev.CampaignID != campID.String() {
		t.Errorf("expected campaign %s, got %q", campID, ev.CampaignID)
	}

	content := sess.lastResponse.Data.Content
	if !strings.Contains(strings.ToLower(content), "out of bolts") || !strings.Contains(strings.ToLower(content), "dm") {
		t.Errorf("expected player to be told the DM was flagged, got %q", content)
	}
}

// TestAttackHandler_OutOfAmmo_NoNotifier_PlainMessage verifies the degraded
// path: with no Notifier wired, the player still gets a clear out-of-ammo
// message and nothing panics.
func TestAttackHandler_OutOfAmmo_NoNotifier_PlainMessage(t *testing.T) {
	h, sess, svc, _ := setupAttackHandler()
	svc.attackErr = combat.NoAmmunitionError{AmmoName: "Arrows"}

	h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

	content := strings.ToLower(sess.lastResponse.Data.Content)
	if !strings.Contains(content, "out of arrows") {
		t.Errorf("expected plain out-of-arrows message, got %q", sess.lastResponse.Data.Content)
	}
}

// TestAttackHandler_OutOfAmmo_CampaignIDFallbacks confirms the dm-queue post
// still fires (with an empty CampaignID) when the campaign lookup is absent or
// errors — the Notifier tolerates it and the player is never left hanging.
func TestAttackHandler_OutOfAmmo_CampaignIDFallbacks(t *testing.T) {
	cases := map[string]CheckCampaignProvider{
		"nil provider":    nil,
		"provider errors": fakeAmmoCampaignProvider{err: context.DeadlineExceeded},
	}
	for name, prov := range cases {
		t.Run(name, func(t *testing.T) {
			h, _, svc, _ := setupAttackHandler()
			svc.attackErr = combat.NoAmmunitionError{AmmoName: "Bolts"}
			notifier := &stubUndoNotifier{}
			h.SetNotifier(notifier)
			if prov != nil {
				h.SetCampaignProvider(prov)
			}

			h.Handle(makeAttackInteraction(map[string]any{"target": "OS"}))

			if len(notifier.posted) != 1 {
				t.Fatalf("expected 1 dm-queue post, got %d", len(notifier.posted))
			}
			if notifier.posted[0].CampaignID != "" {
				t.Errorf("expected empty CampaignID fallback, got %q", notifier.posted[0].CampaignID)
			}
		})
	}
}
