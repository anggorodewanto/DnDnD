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
