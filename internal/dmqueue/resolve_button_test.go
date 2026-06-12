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
}

func (c *componentSender) Send(channelID, content string) (string, error) {
	c.plainCalls++
	c.channel = channelID
	c.content = content
	return c.msgID, nil
}

func (c *componentSender) Edit(channelID, messageID, content string) error { return nil }

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
