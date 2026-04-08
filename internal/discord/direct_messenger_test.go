package discord

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestSendContentReturningIDs_Short(t *testing.T) {
	var sent []string
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			sent = append(sent, content)
			return &discordgo.Message{ID: "id-" + content[:3]}, nil
		},
	}
	ids, err := SendContentReturningIDs(mock, "ch-1", "hello")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(ids) != 1 || ids[0] != "id-hel" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestSendContentReturningIDs_MultiPart(t *testing.T) {
	// Use newline-separated content > 2000 chars to force splitting.
	s1 := strings.Repeat("a", 1500) + "\n"
	s2 := strings.Repeat("b", 1400)
	content := s1 + s2

	var calls int
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, body string) (*discordgo.Message, error) {
			calls++
			return &discordgo.Message{ID: "part-" + string(rune('0'+calls))}, nil
		},
	}
	ids, err := SendContentReturningIDs(mock, "ch-1", content)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 parts, got %d (%v)", len(ids), ids)
	}
}

func TestSendContentReturningIDs_SendError(t *testing.T) {
	mock := &MockSession{
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return nil, errors.New("send failed")
		},
	}
	_, err := SendContentReturningIDs(mock, "ch-1", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSendContentReturningIDs_LargeFallsBackToFile(t *testing.T) {
	content := strings.Repeat("x", MaxSplitLen+1)
	var complexCalled bool
	mock := &MockSession{
		ChannelMessageSendComplexFunc: func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
			complexCalled = true
			return &discordgo.Message{ID: "file-1"}, nil
		},
	}
	ids, err := SendContentReturningIDs(mock, "ch-1", content)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !complexCalled {
		t.Fatal("expected file fallback")
	}
	if len(ids) != 1 || ids[0] != "file-1" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestDirectMessenger_SendDirectMessage_Success(t *testing.T) {
	var dmChannel string
	var sent string
	mock := &MockSession{
		UserChannelCreateFunc: func(userID string) (*discordgo.Channel, error) {
			if userID != "user-42" {
				t.Fatalf("userID = %s", userID)
			}
			return &discordgo.Channel{ID: "dm-ch-1"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			dmChannel = channelID
			sent = content
			return &discordgo.Message{ID: "m-1"}, nil
		},
	}

	dm := NewDirectMessenger(mock)
	ids, err := dm.SendDirectMessage("user-42", "psst, secret")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if dmChannel != "dm-ch-1" {
		t.Fatalf("dm channel = %q", dmChannel)
	}
	if sent != "psst, secret" {
		t.Fatalf("content = %q", sent)
	}
	if len(ids) != 1 || ids[0] != "m-1" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestDirectMessenger_SendDirectMessage_UserChannelCreateError(t *testing.T) {
	mock := &MockSession{
		UserChannelCreateFunc: func(userID string) (*discordgo.Channel, error) {
			return nil, errors.New("cannot DM")
		},
	}
	dm := NewDirectMessenger(mock)
	_, err := dm.SendDirectMessage("user-42", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "creating DM channel") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDirectMessenger_SendDirectMessage_SendError(t *testing.T) {
	mock := &MockSession{
		UserChannelCreateFunc: func(userID string) (*discordgo.Channel, error) {
			return &discordgo.Channel{ID: "dm-ch"}, nil
		},
		ChannelMessageSendFunc: func(channelID, content string) (*discordgo.Message, error) {
			return nil, errors.New("send failed")
		},
	}
	dm := NewDirectMessenger(mock)
	_, err := dm.SendDirectMessage("user-42", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sending direct message") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDirectMessenger_SendDirectMessage_MultiChunk(t *testing.T) {
	s1 := strings.Repeat("a", 1500) + "\n"
	s2 := strings.Repeat("b", 1400)
	content := s1 + s2

	var calls int
	mock := &MockSession{
		UserChannelCreateFunc: func(userID string) (*discordgo.Channel, error) {
			return &discordgo.Channel{ID: "dm-ch"}, nil
		},
		ChannelMessageSendFunc: func(channelID, body string) (*discordgo.Message, error) {
			calls++
			return &discordgo.Message{ID: "p" + string(rune('0'+calls))}, nil
		},
	}
	dm := NewDirectMessenger(mock)
	ids, err := dm.SendDirectMessage("user-42", content)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(ids), ids)
	}
}
