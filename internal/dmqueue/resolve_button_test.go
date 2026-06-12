package dmqueue

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// componentSender implements both Sender and the optional ComponentSender
// capability so Post is expected to attach a resolve button via the latter.
type componentSender struct {
	channel    string
	content    string
	components []discordgo.MessageComponent
	msgID      string
	calls      int
	plainCalls int

	editCalls          int    // EditWithComponents (ComponentEditor) calls
	plainEditCalls     int    // Edit (content-only) calls
	editContent        string // content of the last edit (either path)
	editComponents     []discordgo.MessageComponent
	editComponentsSeen bool // EditWithComponents was given a components arg
}

func (c *componentSender) Send(channelID, content string) (string, error) {
	c.plainCalls++
	c.channel = channelID
	c.content = content
	return c.msgID, nil
}

func (c *componentSender) Edit(channelID, messageID, content string) error {
	c.plainEditCalls++
	c.editContent = content
	return nil
}

func (c *componentSender) EditWithComponents(channelID, messageID, content string, components []discordgo.MessageComponent) error {
	c.editCalls++
	c.editContent = content
	c.editComponents = components
	c.editComponentsSeen = true
	return nil
}

func (c *componentSender) SendWithComponents(channelID, content string, components []discordgo.MessageComponent) (string, error) {
	c.calls++
	c.channel = channelID
	c.content = content
	c.components = components
	return c.msgID, nil
}

func TestPost_AttachesResolveButton(t *testing.T) {
	cs := &componentSender{msgID: "disc-1"}
	n := NewNotifier(cs, staticChannelResolver("chan-1"), func(id string) string { return "/q/" + id })

	itemID, err := n.Post(context.Background(), Event{
		Kind:       KindFreeformAction,
		PlayerName: "Thorn",
		Summary:    "flips the table",
		GuildID:    "g1",
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if cs.calls != 1 {
		t.Fatalf("expected SendWithComponents called once, got %d (plain Send %d)", cs.calls, cs.plainCalls)
	}
	if len(cs.components) == 0 {
		t.Fatalf("expected components attached to the dm-queue message")
	}
	row, ok := cs.components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected ActionsRow, got %T", cs.components[0])
	}
	if len(row.Components) == 0 {
		t.Fatalf("expected a button in the row")
	}
	btn, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected Button, got %T", row.Components[0])
	}
	want := ResolveButtonCustomIDPrefix + itemID
	if btn.CustomID != want {
		t.Errorf("button CustomID = %q want %q", btn.CustomID, want)
	}
}

func TestSessionSender_SendWithComponents(t *testing.T) {
	cs := &captureSession{sentMsgID: "m-1"}
	s := NewSessionSender(cs)
	id, err := s.SendWithComponents("c1", "hi", resolveButtonComponents("item-9"))
	if err != nil {
		t.Fatalf("SendWithComponents: %v", err)
	}
	if id != "m-1" {
		t.Errorf("id = %q want m-1", id)
	}
	if cs.complexChannel != "c1" || cs.complexContent != "hi" {
		t.Errorf("captured = %+v", cs)
	}
	if len(cs.complexComponents) == 0 {
		t.Error("expected components forwarded to ChannelMessageSendComplex")
	}
}

func TestResolve_StripsResolveButton(t *testing.T) {
	cs := &componentSender{msgID: "disc-1"}
	n := NewNotifier(cs, staticChannelResolver("chan-1"), func(id string) string { return "/q/" + id })

	itemID, err := n.Post(context.Background(), Event{
		Kind:       KindFreeformAction,
		PlayerName: "Thorn",
		Summary:    "flips the table",
		GuildID:    "g1",
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	if err := n.Resolve(context.Background(), itemID, "handled"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if cs.editCalls != 1 {
		t.Fatalf("expected EditWithComponents called once, got %d (plain Edit %d)", cs.editCalls, cs.plainEditCalls)
	}
	if cs.plainEditCalls != 0 {
		t.Errorf("expected component-aware edit, not plain Edit (%d plain calls)", cs.plainEditCalls)
	}
	if !cs.editComponentsSeen || cs.editComponents == nil {
		t.Fatal("expected a non-nil components slice so the resolve button is stripped")
	}
	if len(cs.editComponents) != 0 {
		t.Errorf("expected empty (stripping) components, got %d", len(cs.editComponents))
	}
	item, _ := n.Get(itemID)
	if want := FormatResolved(item.PostedText, "handled"); cs.editContent != want {
		t.Errorf("edit content = %q want %q", cs.editContent, want)
	}
}

func TestCancel_StripsResolveButton(t *testing.T) {
	cs := &componentSender{msgID: "disc-1"}
	n := NewNotifier(cs, staticChannelResolver("chan-1"), func(id string) string { return "/q/" + id })

	itemID, err := n.Post(context.Background(), Event{
		Kind:       KindFreeformAction,
		PlayerName: "Thorn",
		Summary:    "flips the table",
		GuildID:    "g1",
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	item, _ := n.Get(itemID)

	if err := n.Cancel(context.Background(), itemID, "changed mind"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if cs.editCalls != 1 || cs.plainEditCalls != 0 {
		t.Fatalf("expected one component-aware edit, got EditWithComponents=%d Edit=%d", cs.editCalls, cs.plainEditCalls)
	}
	if !cs.editComponentsSeen || cs.editComponents == nil || len(cs.editComponents) != 0 {
		t.Errorf("expected empty non-nil components to strip the button, got %v", cs.editComponents)
	}
	if want := FormatCancelled(item.PostedText); cs.editContent != want {
		t.Errorf("edit content = %q want %q", cs.editContent, want)
	}
}
