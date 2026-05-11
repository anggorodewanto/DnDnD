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
	ChannelMessageSendComplexFunc       func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error)
	ApplicationCommandBulkOverwriteFunc func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandsFunc             func(appID, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDeleteFunc        func(appID, guildID, cmdID string) error
	GuildChannelsFunc                   func(guildID string) ([]*discordgo.Channel, error)
	GuildChannelCreateComplexFunc       func(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error)
	InteractionRespondFunc              func(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error
	InteractionResponseEditFunc         func(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error)
	ChannelMessageEditFunc              func(channelID, messageID, content string) (*discordgo.Message, error)
	StateValue                          *discordgo.State
}

func (m *MockSession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	return m.UserChannelCreateFunc(recipientID)
}

func (m *MockSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	if m.ChannelMessageSendFunc != nil {
		return m.ChannelMessageSendFunc(channelID, content)
	}
	return &discordgo.Message{ID: "m-send", ChannelID: channelID}, nil
}

func (m *MockSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	if m.ChannelMessageSendComplexFunc != nil {
		return m.ChannelMessageSendComplexFunc(channelID, data)
	}
	return &discordgo.Message{}, nil
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

func (m *MockSession) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	if m.GuildChannelsFunc != nil {
		return m.GuildChannelsFunc(guildID)
	}
	return nil, nil
}

func (m *MockSession) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	if m.GuildChannelCreateComplexFunc != nil {
		return m.GuildChannelCreateComplexFunc(guildID, data)
	}
	return &discordgo.Channel{ID: "new-" + data.Name, Name: data.Name}, nil
}

func (m *MockSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	if m.InteractionRespondFunc != nil {
		return m.InteractionRespondFunc(interaction, resp)
	}
	return nil
}

func (m *MockSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	if m.InteractionResponseEditFunc != nil {
		return m.InteractionResponseEditFunc(interaction, newresp)
	}
	return &discordgo.Message{}, nil
}

func (m *MockSession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	if m.ChannelMessageEditFunc != nil {
		return m.ChannelMessageEditFunc(channelID, messageID, content)
	}
	return &discordgo.Message{ID: messageID}, nil
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