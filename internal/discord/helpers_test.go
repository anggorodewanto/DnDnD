package discord

import (
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
)

// MockSession is a test double that captures calls and returns configured responses.
type MockSession struct {
	UserChannelCreateFunc               func(recipientID string) (*discordgo.Channel, error)
	ChannelMessageSendFunc              func(channelID, content string) (*discordgo.Message, error)
	ApplicationCommandBulkOverwriteFunc func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandsFunc             func(appID, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDeleteFunc        func(appID, guildID, cmdID string) error
	StateValue                          *discordgo.State
}

func (m *MockSession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	return m.UserChannelCreateFunc(recipientID)
}

func (m *MockSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	return m.ChannelMessageSendFunc(channelID, content)
}

func (m *MockSession) ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return m.ApplicationCommandBulkOverwriteFunc(appID, guildID, cmds)
}

func (m *MockSession) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return m.ApplicationCommandsFunc(appID, guildID)
}

func (m *MockSession) ApplicationCommandDelete(appID, guildID, cmdID string) error {
	return m.ApplicationCommandDeleteFunc(appID, guildID, cmdID)
}

func (m *MockSession) GetState() *discordgo.State {
	return m.StateValue
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestMock returns a MockSession pre-configured with no-op command functions
// and a default State. Tests can override individual fields as needed.
func newTestMock() *MockSession {
	return &MockSession{
		ApplicationCommandBulkOverwriteFunc: func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
			return cmds, nil
		},
		ApplicationCommandsFunc: func(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
			return nil, nil
		},
		StateValue: &discordgo.State{
			Ready: discordgo.Ready{
				User: &discordgo.User{ID: "bot-1"},
			},
		},
	}
}