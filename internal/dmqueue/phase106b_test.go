package dmqueue

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeDeliverer captures SendDirectMessage calls.
type fakeDeliverer struct {
	calls    []delivererCall
	sendErr  error
	returnID []string
}

type delivererCall struct {
	UserID string
	Body   string
}

func (f *fakeDeliverer) SendDirectMessage(userID, body string) ([]string, error) {
	f.calls = append(f.calls, delivererCall{userID, body})
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	if f.returnID == nil {
		return []string{"dm-msg-1"}, nil
	}
	return f.returnID, nil
}

func TestPostNarrativeTeleport_PostsFormattedEvent(t *testing.T) {
	f := &fakeSender{nextMsgID: "disc-nt"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })

	itemID, err := n.PostNarrativeTeleport(context.Background(), "Kael", "Teleport", "g1", "camp-1")
	if err != nil {
		t.Fatalf("PostNarrativeTeleport: %v", err)
	}
	if itemID == "" {
		t.Fatalf("expected non-empty itemID")
	}
	if len(f.sendCalls) != 1 {
		t.Fatalf("expected 1 send, got %d", len(f.sendCalls))
	}
	got := f.sendCalls[0].Content
	if !strings.Contains(got, "✨") || !strings.Contains(got, "Spell") {
		t.Errorf("content missing emoji/label: %q", got)
	}
	if !strings.Contains(got, "Kael casts Teleport (narrative resolution)") {
		t.Errorf("content missing summary: %q", got)
	}
	item, ok := n.Get(itemID)
	if !ok {
		t.Fatalf("Get returned not ok")
	}
	if item.Event.Kind != KindNarrativeTeleport {
		t.Errorf("kind = %q want %q", item.Event.Kind, KindNarrativeTeleport)
	}
	if item.Event.GuildID != "g1" || item.Event.CampaignID != "camp-1" {
		t.Errorf("guild/campaign not propagated: %+v", item.Event)
	}
}

func TestPostWhisper_StashesDiscordUserID(t *testing.T) {
	f := &fakeSender{nextMsgID: "disc-w"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })

	itemID, err := n.PostWhisper(context.Background(), "Aria", `"I want to pickpocket the merchant"`, "user-42", "g1", "camp-1")
	if err != nil {
		t.Fatalf("PostWhisper: %v", err)
	}
	if itemID == "" {
		t.Fatalf("expected non-empty itemID")
	}
	got := f.sendCalls[0].Content
	if !strings.Contains(got, "🤫") || !strings.Contains(got, "Whisper") {
		t.Errorf("content missing emoji/label: %q", got)
	}
	if !strings.Contains(got, `Aria: "I want to pickpocket the merchant"`) {
		t.Errorf("content missing summary: %q", got)
	}
	item, ok := n.Get(itemID)
	if !ok {
		t.Fatalf("Get returned not ok")
	}
	if item.Event.Kind != KindPlayerWhisper {
		t.Errorf("kind = %q want %q", item.Event.Kind, KindPlayerWhisper)
	}
	if item.Event.ExtraMetadata[WhisperTargetDiscordUserIDKey] != "user-42" {
		t.Errorf("discord user id not stashed: %+v", item.Event.ExtraMetadata)
	}
}

func TestResolveWhisper_DeliversDMAndResolvesItem(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	d := &fakeDeliverer{}
	n.SetWhisperDeliverer(d)

	itemID, err := n.PostWhisper(context.Background(), "Aria", `"pickpocket"`, "user-42", "g1", "camp-1")
	if err != nil {
		t.Fatalf("PostWhisper: %v", err)
	}

	if err := n.ResolveWhisper(context.Background(), itemID, "You succeed, rolled 18"); err != nil {
		t.Fatalf("ResolveWhisper: %v", err)
	}

	if len(d.calls) != 1 {
		t.Fatalf("expected 1 DM delivery, got %d", len(d.calls))
	}
	if d.calls[0].UserID != "user-42" {
		t.Errorf("user id = %q want user-42", d.calls[0].UserID)
	}
	if d.calls[0].Body != "You succeed, rolled 18" {
		t.Errorf("body = %q", d.calls[0].Body)
	}

	item, _ := n.Get(itemID)
	if item.Status != StatusResolved {
		t.Errorf("status = %q want resolved", item.Status)
	}
	if item.Outcome != "You succeed, rolled 18" {
		t.Errorf("outcome not persisted: %q", item.Outcome)
	}
	if len(f.editCalls) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(f.editCalls))
	}
	if !strings.HasPrefix(f.editCalls[0].Content, "✅ ") {
		t.Errorf("edit missing checkmark: %q", f.editCalls[0].Content)
	}
}

func TestResolveWhisper_WrongKindRejected(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetWhisperDeliverer(&fakeDeliverer{})

	itemID, _ := n.Post(context.Background(), Event{Kind: KindFreeformAction, PlayerName: "Thorn", Summary: `"flip"`, GuildID: "g1"})
	err := n.ResolveWhisper(context.Background(), itemID, "reply")
	if err == nil {
		t.Fatalf("expected error for wrong kind")
	}
	if !errors.Is(err, ErrNotWhisperItem) {
		t.Errorf("got %v want ErrNotWhisperItem", err)
	}
}

func TestResolveWhisper_MissingUserID(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetWhisperDeliverer(&fakeDeliverer{})

	// Post a whisper with empty user id.
	itemID, _ := n.PostWhisper(context.Background(), "Aria", `"pickpocket"`, "", "g1", "camp-1")
	err := n.ResolveWhisper(context.Background(), itemID, "reply")
	if err == nil {
		t.Fatalf("expected error for missing user id")
	}
	if !errors.Is(err, ErrWhisperTargetMissing) {
		t.Errorf("got %v want ErrWhisperTargetMissing", err)
	}
}

func TestResolveWhisper_MissingDeliverer(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })

	itemID, _ := n.PostWhisper(context.Background(), "Aria", `"x"`, "user-1", "g1", "camp-1")
	err := n.ResolveWhisper(context.Background(), itemID, "reply")
	if err == nil {
		t.Fatalf("expected error when deliverer missing")
	}
	if !errors.Is(err, ErrWhisperDelivererMissing) {
		t.Errorf("got %v want ErrWhisperDelivererMissing", err)
	}
}

func TestResolveWhisper_DelivererErrorPropagates(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetWhisperDeliverer(&fakeDeliverer{sendErr: errors.New("discord down")})

	itemID, _ := n.PostWhisper(context.Background(), "Aria", `"x"`, "user-1", "g1", "camp-1")
	err := n.ResolveWhisper(context.Background(), itemID, "reply")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "discord down") {
		t.Errorf("wrapped err missing cause: %v", err)
	}
	// Item should remain pending since DM delivery failed.
	item, _ := n.Get(itemID)
	if item.Status != StatusPending {
		t.Errorf("status = %q want pending", item.Status)
	}
}

func TestResolveWhisper_UnknownItem(t *testing.T) {
	f := &fakeSender{}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	n.SetWhisperDeliverer(&fakeDeliverer{})
	err := n.ResolveWhisper(context.Background(), "nope", "reply")
	if !errors.Is(err, ErrItemNotFound) {
		t.Errorf("got %v want ErrItemNotFound", err)
	}
}
