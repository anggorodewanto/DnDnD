package discord

import "github.com/bwmarrin/discordgo"

// Session abstracts the Discord API so tests can inject a mock.
type Session interface {
	UserChannelCreate(recipientID string) (*discordgo.Channel, error)
	ChannelMessageSend(channelID, content string) (*discordgo.Message, error)
	ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDelete(appID, guildID, cmdID string) error
	GetState() *discordgo.State
}

// MockSession is a test double that captures calls and returns configured responses.
type MockSession struct {
	UserChannelCreateFunc              func(recipientID string) (*discordgo.Channel, error)
	ChannelMessageSendFunc             func(channelID, content string) (*discordgo.Message, error)
	ApplicationCommandBulkOverwriteFunc func(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandsFunc            func(appID, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDeleteFunc       func(appID, guildID, cmdID string) error
	StateValue                         *discordgo.State
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
