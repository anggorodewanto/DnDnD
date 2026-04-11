package dmqueue

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeNarrationDeliverer captures DeliverSkillCheckNarration calls.
type fakeNarrationDeliverer struct {
	calls   []narrationCall
	sendErr error
}

type narrationCall struct {
	ChannelID string
	Body      string
}

func (f *fakeNarrationDeliverer) DeliverSkillCheckNarration(channelID, body string) error {
	f.calls = append(f.calls, narrationCall{channelID, body})
	return f.sendErr
}

func TestResolveSkillCheckNarration_DeliversFollowupAndResolves(t *testing.T) {
	f := &fakeSender{nextMsgID: "m-skill"}
	n := NewNotifier(f, staticChannelResolver("dm-q"), func(id string) string { return "/q/" + id })
	d := &fakeNarrationDeliverer{}
	n.SetSkillCheckNarrationDeliverer(d)

	itemID, err := n.Post(context.Background(), Event{
		Kind:       KindSkillCheckNarration,
		PlayerName: "Aria",
		Summary:    "Perception check — total 17",
		GuildID:    "g1",
		CampaignID: "camp-1",
		ExtraMetadata: map[string]string{
			SkillCheckChannelIDKey:        "chan-77",
			SkillCheckPlayerDiscordIDKey:  "user-42",
			SkillCheckSkillLabelKey:       "Perception",
			SkillCheckTotalKey:            "17",
		},
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	if err := n.ResolveSkillCheckNarration(context.Background(), itemID, "You spot the trap before stepping on it."); err != nil {
		t.Fatalf("ResolveSkillCheckNarration: %v", err)
	}

	if len(d.calls) != 1 {
		t.Fatalf("expected 1 narration delivery, got %d", len(d.calls))
	}
	if d.calls[0].ChannelID != "chan-77" {
		t.Errorf("channel id = %q want chan-77", d.calls[0].ChannelID)
	}
	body := d.calls[0].Body
	if !strings.Contains(body, "<@user-42>") {
		t.Errorf("body should mention player: %q", body)
	}
	if !strings.Contains(body, "Perception") {
		t.Errorf("body should contain skill: %q", body)
	}
	if !strings.Contains(body, "17") {
		t.Errorf("body should contain total: %q", body)
	}
	if !strings.Contains(body, "You spot the trap before stepping on it.") {
		t.Errorf("body should contain narration: %q", body)
	}

	item, _ := n.Get(itemID)
	if item.Status != StatusResolved {
		t.Errorf("status = %q want resolved", item.Status)
	}
	if !strings.Contains(item.Outcome, "You spot the trap") {
		t.Errorf("outcome should record narration: %q", item.Outcome)
	}
}

func TestResolveSkillCheckNarration_WrongKindRejected(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetSkillCheckNarrationDeliverer(&fakeNarrationDeliverer{})

	itemID, _ := n.Post(context.Background(), Event{Kind: KindFreeformAction, PlayerName: "Aria", Summary: "x", GuildID: "g1"})
	err := n.ResolveSkillCheckNarration(context.Background(), itemID, "narration")
	if err == nil {
		t.Fatalf("expected error for wrong kind")
	}
	if !errors.Is(err, ErrNotSkillCheckNarrationItem) {
		t.Errorf("got %v want ErrNotSkillCheckNarrationItem", err)
	}
}

func TestResolveSkillCheckNarration_MissingChannel(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetSkillCheckNarrationDeliverer(&fakeNarrationDeliverer{})

	itemID, _ := n.Post(context.Background(), Event{
		Kind:       KindSkillCheckNarration,
		PlayerName: "Aria",
		Summary:    "Perception",
		GuildID:    "g1",
		ExtraMetadata: map[string]string{
			SkillCheckPlayerDiscordIDKey: "u1",
		},
	})
	err := n.ResolveSkillCheckNarration(context.Background(), itemID, "narration")
	if err == nil {
		t.Fatalf("expected error for missing channel")
	}
	if !errors.Is(err, ErrSkillCheckChannelMissing) {
		t.Errorf("got %v want ErrSkillCheckChannelMissing", err)
	}
}

func TestResolveSkillCheckNarration_MissingDeliverer(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })

	itemID, _ := n.Post(context.Background(), Event{
		Kind:       KindSkillCheckNarration,
		PlayerName: "Aria",
		Summary:    "Perception",
		GuildID:    "g1",
		ExtraMetadata: map[string]string{
			SkillCheckChannelIDKey:       "c1",
			SkillCheckPlayerDiscordIDKey: "u1",
		},
	})
	err := n.ResolveSkillCheckNarration(context.Background(), itemID, "narration")
	if err == nil {
		t.Fatalf("expected error when deliverer missing")
	}
	if !errors.Is(err, ErrSkillCheckNarrationDelivererMissing) {
		t.Errorf("got %v want ErrSkillCheckNarrationDelivererMissing", err)
	}
}

func TestResolveSkillCheckNarration_DelivererErrorPropagates(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetSkillCheckNarrationDeliverer(&fakeNarrationDeliverer{sendErr: errors.New("channel gone")})

	itemID, _ := n.Post(context.Background(), Event{
		Kind:       KindSkillCheckNarration,
		PlayerName: "Aria",
		Summary:    "Perception",
		GuildID:    "g1",
		ExtraMetadata: map[string]string{
			SkillCheckChannelIDKey:       "c1",
			SkillCheckPlayerDiscordIDKey: "u1",
			SkillCheckSkillLabelKey:      "Perception",
			SkillCheckTotalKey:           "17",
		},
	})
	err := n.ResolveSkillCheckNarration(context.Background(), itemID, "narration")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "channel gone") {
		t.Errorf("wrapped err missing cause: %v", err)
	}
	// Item should remain pending since delivery failed.
	item, _ := n.Get(itemID)
	if item.Status != StatusPending {
		t.Errorf("status = %q want pending", item.Status)
	}
}

func TestResolveSkillCheckNarration_UnknownItem(t *testing.T) {
	f := &fakeSender{}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetSkillCheckNarrationDeliverer(&fakeNarrationDeliverer{})

	err := n.ResolveSkillCheckNarration(context.Background(), "nope", "x")
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("got %v want ErrItemNotFound", err)
	}
}
