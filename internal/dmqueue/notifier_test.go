package dmqueue

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeSender captures posted and edited messages.
type fakeSender struct {
	sendCalls []sendCall
	editCalls []editCall
	sendErr   error
	editErr   error
	nextMsgID string
}

type sendCall struct {
	ChannelID string
	Content   string
}

type editCall struct {
	ChannelID string
	MessageID string
	Content   string
}

func (f *fakeSender) Send(channelID, content string) (string, error) {
	f.sendCalls = append(f.sendCalls, sendCall{channelID, content})
	if f.sendErr != nil {
		return "", f.sendErr
	}
	id := f.nextMsgID
	if id == "" {
		id = "msg-1"
	}
	return id, nil
}

func (f *fakeSender) Edit(channelID, messageID, content string) error {
	f.editCalls = append(f.editCalls, editCall{channelID, messageID, content})
	return f.editErr
}

func staticChannelResolver(channelID string) func(string) string {
	return func(string) string { return channelID }
}

func TestNotifier_Post_SendsFormattedMessage(t *testing.T) {
	f := &fakeSender{nextMsgID: "disc-42"}
	n := NewNotifier(f, staticChannelResolver("chan-1"), func(id string) string { return "/dashboard/queue/" + id })

	itemID, err := n.Post(context.Background(), Event{
		Kind:       KindFreeformAction,
		PlayerName: "Thorn",
		Summary:    `"flip the table"`,
		GuildID:    "g1",
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if itemID == "" {
		t.Fatalf("expected non-empty itemID")
	}
	if len(f.sendCalls) != 1 {
		t.Fatalf("expected 1 send, got %d", len(f.sendCalls))
	}
	got := f.sendCalls[0]
	if got.ChannelID != "chan-1" {
		t.Errorf("channel = %q want chan-1", got.ChannelID)
	}
	if !strings.Contains(got.Content, "Thorn") || !strings.Contains(got.Content, "flip the table") {
		t.Errorf("content missing expected text: %q", got.Content)
	}
	if !strings.Contains(got.Content, "/dashboard/queue/"+itemID) {
		t.Errorf("content missing resolve path: %q", got.Content)
	}

	// Item persisted and retrievable.
	item, ok := n.Get(itemID)
	if !ok {
		t.Fatalf("Get returned not ok")
	}
	if item.Status != StatusPending {
		t.Errorf("status = %q want pending", item.Status)
	}
	if item.MessageID != "disc-42" {
		t.Errorf("message id = %q want disc-42", item.MessageID)
	}
	if item.ChannelID != "chan-1" {
		t.Errorf("channel id = %q want chan-1", item.ChannelID)
	}
	if item.Event.PlayerName != "Thorn" {
		t.Errorf("player name not preserved")
	}
}

func TestNotifier_Post_NoChannelIsNoOp(t *testing.T) {
	f := &fakeSender{}
	n := NewNotifier(f, staticChannelResolver(""), func(id string) string { return "/x/" + id })
	itemID, err := n.Post(context.Background(), Event{Kind: KindRestRequest, PlayerName: "K", Summary: "rests"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if itemID != "" {
		t.Errorf("expected empty itemID, got %q", itemID)
	}
	if len(f.sendCalls) != 0 {
		t.Errorf("expected 0 sends, got %d", len(f.sendCalls))
	}
}

func TestNotifier_Post_SendErrorReturned(t *testing.T) {
	f := &fakeSender{sendErr: errors.New("boom")}
	store := NewMemoryStore()
	n := NewNotifierWithStore(f, staticChannelResolver("c"), func(id string) string { return "/x/" + id }, store)
	_, err := n.Post(context.Background(), Event{Kind: KindConsumable, PlayerName: "A", Summary: "uses X"})
	if err == nil {
		t.Fatalf("expected error")
	}
	// SR-002: with insert-then-send ordering, a send failure leaves the row
	// pending so the DM can still see + resolve it from the dashboard. The
	// row's MessageID retains the itemID placeholder set at Insert time
	// (the real Discord message id is only written on Send success).
	pending := n.ListPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending row after send failure, got %d", len(pending))
	}
	if pending[0].MessageID != pending[0].ID {
		t.Errorf("expected placeholder message id == item id, got message_id=%q id=%q",
			pending[0].MessageID, pending[0].ID)
	}
}

// failingStore returns insertErr from Insert; used to verify Notifier.Post
// short-circuits before calling Sender.Send when persistence fails (SR-002).
type failingStore struct {
	Store
	insertErr error
	inserted  int
}

func (f *failingStore) Insert(ctx context.Context, id string, e Event, channelID, messageID, postedText string) (Item, error) {
	f.inserted++
	if f.insertErr != nil {
		return Item{}, f.insertErr
	}
	return f.Store.Insert(ctx, id, e, channelID, messageID, postedText)
}

// TestNotifier_Post_InsertFailureSkipsSend pins the insert-then-send
// ordering: when Store.Insert returns an error, no Discord message must be
// posted. Reverses the historical send-then-insert path that orphaned
// #dm-queue messages when CampaignID was missing on the Event. (SR-002)
func TestNotifier_Post_InsertFailureSkipsSend(t *testing.T) {
	f := &fakeSender{nextMsgID: "should-never-arrive"}
	store := &failingStore{Store: NewMemoryStore(), insertErr: errors.New("parse campaign id")}
	n := NewNotifierWithStore(f, staticChannelResolver("c"), func(id string) string { return "/x/" + id }, store)

	itemID, err := n.Post(context.Background(), Event{Kind: KindConsumable, PlayerName: "A", Summary: "uses X"})
	if err == nil {
		t.Fatalf("expected error from failing Insert")
	}
	if itemID != "" {
		t.Errorf("expected empty itemID on insert failure, got %q", itemID)
	}
	if store.inserted != 1 {
		t.Errorf("expected Insert attempted once, got %d", store.inserted)
	}
	if len(f.sendCalls) != 0 {
		t.Fatalf("expected 0 Sender.Send calls when Insert fails, got %d", len(f.sendCalls))
	}
}

// TestNotifier_Post_InsertBeforeSend pins call ordering: Insert must execute
// before Send so the dashboard row is created up front and a later send
// failure cannot orphan a Discord message. (SR-002)
func TestNotifier_Post_InsertBeforeSend(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	store := &orderingStore{Store: NewMemoryStore()}
	n := NewNotifierWithStore(f, staticChannelResolver("c"), func(id string) string { return "/x/" + id }, store)

	// Snapshot send-call count when Insert is invoked.
	store.beforeSendCount = func() int { return len(f.sendCalls) }

	_, err := n.Post(context.Background(), Event{
		Kind: KindRestRequest, PlayerName: "Aria", Summary: "long rest", GuildID: "g1",
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if store.sendCountAtInsert != 0 {
		t.Errorf("Insert ran AFTER Send (sendCount at Insert=%d); want Insert-first", store.sendCountAtInsert)
	}
	if len(f.sendCalls) != 1 {
		t.Fatalf("expected 1 send after successful insert, got %d", len(f.sendCalls))
	}
}

// orderingStore records the Sender.Send call-count observed at the moment
// Insert runs. With insert-then-send ordering the snapshot is 0.
type orderingStore struct {
	Store
	beforeSendCount   func() int
	sendCountAtInsert int
}

func (o *orderingStore) Insert(ctx context.Context, id string, e Event, channelID, messageID, postedText string) (Item, error) {
	if o.beforeSendCount != nil {
		o.sendCountAtInsert = o.beforeSendCount()
	}
	return o.Store.Insert(ctx, id, e, channelID, messageID, postedText)
}

func TestNotifier_Cancel_EditsMessage(t *testing.T) {
	f := &fakeSender{nextMsgID: "m1"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	itemID, _ := n.Post(context.Background(), Event{Kind: KindFreeformAction, PlayerName: "Thorn", Summary: `"flip"`})

	if err := n.Cancel(context.Background(), itemID, "Cancelled by player"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if len(f.editCalls) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(f.editCalls))
	}
	e := f.editCalls[0]
	if e.MessageID != "m1" || e.ChannelID != "c1" {
		t.Errorf("unexpected edit target: %+v", e)
	}
	if !strings.Contains(e.Content, "~~") || !strings.Contains(e.Content, "Cancelled by player") {
		t.Errorf("cancel content wrong: %q", e.Content)
	}
	item, _ := n.Get(itemID)
	if item.Status != StatusCancelled {
		t.Errorf("status = %q want cancelled", item.Status)
	}
}

func TestNotifier_Resolve_EditsWithCheckmark(t *testing.T) {
	f := &fakeSender{nextMsgID: "m2"}
	n := NewNotifier(f, staticChannelResolver("c1"), func(id string) string { return "/q/" + id })
	itemID, _ := n.Post(context.Background(), Event{Kind: KindSkillCheckNarration, PlayerName: "Thorn", Summary: "Athletics 18"})

	if err := n.Resolve(context.Background(), itemID, "climbs the cliff"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(f.editCalls) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(f.editCalls))
	}
	e := f.editCalls[0]
	if !strings.HasPrefix(e.Content, "✅ ") {
		t.Errorf("expected checkmark prefix: %q", e.Content)
	}
	if !strings.Contains(e.Content, "climbs the cliff") {
		t.Errorf("outcome missing: %q", e.Content)
	}
	item, _ := n.Get(itemID)
	if item.Status != StatusResolved {
		t.Errorf("status = %q want resolved", item.Status)
	}
	if item.Outcome != "climbs the cliff" {
		t.Errorf("outcome not persisted")
	}
}

func TestNotifier_UnknownItem(t *testing.T) {
	f := &fakeSender{}
	n := NewNotifier(f, staticChannelResolver("c"), func(id string) string { return "/q/" + id })
	if err := n.Cancel(context.Background(), "nope", ""); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("Cancel unknown: got %v want ErrItemNotFound", err)
	}
	if err := n.Resolve(context.Background(), "nope", "x"); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("Resolve unknown: got %v want ErrItemNotFound", err)
	}
}

func TestNotifier_ListPending(t *testing.T) {
	f := &fakeSender{nextMsgID: "m"}
	n := NewNotifier(f, staticChannelResolver("c"), func(id string) string { return "/q/" + id })
	id1, _ := n.Post(context.Background(), Event{Kind: KindRestRequest, PlayerName: "A", Summary: "rests"})
	id2, _ := n.Post(context.Background(), Event{Kind: KindConsumable, PlayerName: "B", Summary: "uses"})
	_ = n.Resolve(context.Background(), id1, "ok")

	pending := n.ListPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ID != id2 {
		t.Errorf("wrong pending item")
	}
}
