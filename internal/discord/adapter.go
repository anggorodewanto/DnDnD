package discord

import "github.com/bwmarrin/discordgo"

// DiscordgoSession wraps a real *discordgo.Session to implement the Session interface.
type DiscordgoSession struct {
	S *discordgo.Session
}

func (d *DiscordgoSession) UserChannelCreate(recipientID string) (*discordgo.Channel, error) {
	return d.S.UserChannelCreate(recipientID)
}

func (d *DiscordgoSession) ChannelMessageSend(channelID, content string) (*discordgo.Message, error) {
	return d.S.ChannelMessageSend(channelID, content)
}

func (d *DiscordgoSession) ApplicationCommandBulkOverwrite(appID, guildID string, cmds []*discordgo.ApplicationCommand) ([]*discordgo.ApplicationCommand, error) {
	return d.S.ApplicationCommandBulkOverwrite(appID, guildID, cmds)
}

func (d *DiscordgoSession) ApplicationCommands(appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	return d.S.ApplicationCommands(appID, guildID)
}

func (d *DiscordgoSession) ApplicationCommandDelete(appID, guildID, cmdID string) error {
	return d.S.ApplicationCommandDelete(appID, guildID, cmdID)
}

func (d *DiscordgoSession) GetState() *discordgo.State {
	return d.S.State
}
