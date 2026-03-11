package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestMockSession_ImplementsSession(t *testing.T) {
	var _ Session = &MockSession{}
}

func TestMockSession_UserChannelCreate(t *testing.T) {
	mock := &MockSession{}
	mock.UserChannelCreateFunc = func(recipientID string) (*discordgo.Channel, error) {
		return &discordgo.Channel{ID: "ch-123"}, nil
	}

	ch, err := mock.UserChannelCreate("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.ID != "ch-123" {
		t.Fatalf("expected channel ID ch-123, got %s", ch.ID)
	}
}

func TestMockSession_ChannelMessageSend(t *testing.T) {
	mock := &MockSession{}
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		return &discordgo.Message{ID: "msg-1", Content: content}, nil
	}

	msg, err := mock.ChannelMessageSend("ch-1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "hello" {
		t.Fatalf("expected content 'hello', got %s", msg.Content)
	}
}

func TestMockSession_ApplicationCommandBulkOverwrite(t *testing.T) {
	mock := &MockSession{}
	mock.ApplicationCommandBulkOverwriteFunc = func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
		return cmds, nil
	}

	cmds := []*discordgo.ApplicationCommand{{Name: "test"}}
	result, err := mock.ApplicationCommandBulkOverwrite("app-1", "guild-1", cmds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 command, got %d", len(result))
	}
}

func TestMockSession_GuildCommands(t *testing.T) {
	mock := &MockSession{}
	mock.ApplicationCommandsFunc = func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
		return []*discordgo.ApplicationCommand{{Name: "old-cmd", ID: "cmd-1"}}, nil
	}

	cmds, err := mock.ApplicationCommands("app-1", "guild-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "old-cmd" {
		t.Fatalf("unexpected commands: %v", cmds)
	}
}

func TestMockSession_ApplicationCommandDelete(t *testing.T) {
	mock := &MockSession{}
	var deletedCmdID string
	mock.ApplicationCommandDeleteFunc = func(appID, guildID, cmdID string) error {
		deletedCmdID = cmdID
		return nil
	}

	err := mock.ApplicationCommandDelete("app-1", "guild-1", "cmd-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedCmdID != "cmd-1" {
		t.Fatalf("expected cmd-1, got %s", deletedCmdID)
	}
}

func TestMockSession_State(t *testing.T) {
	mock := &MockSession{}
	state := &discordgo.State{
		Ready: discordgo.Ready{
			User: &discordgo.User{ID: "bot-user"},
		},
	}
	mock.StateValue = state

	got := mock.GetState()
	if got.User.ID != "bot-user" {
		t.Fatalf("expected bot-user, got %s", got.User.ID)
	}
}
