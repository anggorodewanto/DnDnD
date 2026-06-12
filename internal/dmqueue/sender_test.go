package dmqueue

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type captureSession struct {
	sentChannel string
	sentContent string
	sentMsgID   string
	sendErr     error

	editedChannel string
	editedMsgID   string
	editedContent string
	editErr       error

	complexChannel    string
	complexContent    string
	complexComponents []discordgo.MessageComponent

	complexEditChannel    string
	complexEditMsgID      string
	complexEditContent    string
	complexEditComponents *[]discordgo.MessageComponent
}

func (c *captureSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	c.complexChannel = channelID
	c.complexContent = data.Content
	c.complexComponents = data.Components
	if c.sendErr != nil {
		return nil, c.sendErr
	}
	return &discordgo.Message{ID: c.sentMsgID}, nil
}

func (c *captureSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	c.sentChannel = channelID
	c.sentContent = content
	if c.sendErr != nil {
		return nil, c.sendErr
	}
	return &discordgo.Message{ID: c.sentMsgID}, nil
}

func (c *captureSession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	c.editedChannel = channelID
	c.editedMsgID = messageID
	c.editedContent = content
	if c.editErr != nil {
		return nil, c.editErr
	}
	return &discordgo.Message{ID: messageID}, nil
}

func (c *captureSession) ChannelMessageEditComplex(m *discordgo.MessageEdit) (*discordgo.Message, error) {
	c.complexEditChannel = m.Channel
	c.complexEditMsgID = m.ID
	if m.Content != nil {
		c.complexEditContent = *m.Content
	}
	c.complexEditComponents = m.Components
	if c.editErr != nil {
		return nil, c.editErr
	}
	return &discordgo.Message{ID: m.ID}, nil
}

func TestSessionSender_Send(t *testing.T) {
	cs := &captureSession{sentMsgID: "m-xyz"}
	s := NewSessionSender(cs)
	id, err := s.Send("c1", "hello")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if id != "m-xyz" {
		t.Errorf("id = %q want m-xyz", id)
	}
	if cs.sentChannel != "c1" || cs.sentContent != "hello" {
		t.Errorf("captured = %+v", cs)
	}
}

func TestSessionSender_Send_Error(t *testing.T) {
	cs := &captureSession{sendErr: errors.New("boom")}
	s := NewSessionSender(cs)
	if _, err := s.Send("c1", "hi"); err == nil {
		t.Errorf("expected error")
	}
}

func TestSessionSender_Edit(t *testing.T) {
	cs := &captureSession{}
	s := NewSessionSender(cs)
	if err := s.Edit("c1", "m1", "updated"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if cs.editedChannel != "c1" || cs.editedMsgID != "m1" || cs.editedContent != "updated" {
		t.Errorf("captured = %+v", cs)
	}
}

func TestSessionSender_Edit_Error(t *testing.T) {
	cs := &captureSession{editErr: errors.New("boom")}
	s := NewSessionSender(cs)
	if err := s.Edit("c", "m", "x"); err == nil {
		t.Errorf("expected error")
	}
}

func TestSessionSender_EditWithComponents_StripsButtons(t *testing.T) {
	cs := &captureSession{}
	s := NewSessionSender(cs)
	if err := s.EditWithComponents("c1", "m1", "done", []discordgo.MessageComponent{}); err != nil {
		t.Fatalf("EditWithComponents: %v", err)
	}
	if cs.complexEditChannel != "c1" || cs.complexEditMsgID != "m1" || cs.complexEditContent != "done" {
		t.Errorf("captured = %+v", cs)
	}
	// discordgo strips components only on a non-nil pointer to an empty slice.
	if cs.complexEditComponents == nil {
		t.Fatal("expected non-nil Components pointer so buttons are removed")
	}
	if got := len(*cs.complexEditComponents); got != 0 {
		t.Errorf("expected empty components slice, got %d", got)
	}
}

func TestSessionSender_EditWithComponents_NilSliceStillStrips(t *testing.T) {
	cs := &captureSession{}
	s := NewSessionSender(cs)
	if err := s.EditWithComponents("c1", "m1", "done", nil); err != nil {
		t.Fatalf("EditWithComponents: %v", err)
	}
	if cs.complexEditComponents == nil || len(*cs.complexEditComponents) != 0 {
		t.Errorf("nil slice should normalize to non-nil empty pointer, got %v", cs.complexEditComponents)
	}
}

func TestSessionSender_EditWithComponents_Error(t *testing.T) {
	cs := &captureSession{editErr: errors.New("boom")}
	s := NewSessionSender(cs)
	if err := s.EditWithComponents("c", "m", "x", nil); err == nil {
		t.Errorf("expected error")
	}
}
