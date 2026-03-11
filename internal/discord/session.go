package discord

import "github.com/bwmarrin/discordgo"

// Session abstracts the Discord API so tests can inject a mock.
type Session interface {
	UserChannelCreate(recipientID string) (*discordgo.Channel, error)
	ChannelMessageSend(channelID, content string) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error)
	ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error)
	ApplicationCommandDelete(appID, guildID, cmdID string) error
	GetState() *discordgo.State
}
