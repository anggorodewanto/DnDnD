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

func (d *DiscordgoSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	return d.S.ChannelMessageSendComplex(channelID, data)
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

func (d *DiscordgoSession) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	return d.S.GuildChannels(guildID)
}

func (d *DiscordgoSession) GuildChannelCreateComplex(guildID string, data discordgo.GuildChannelCreateData) (*discordgo.Channel, error) {
	return d.S.GuildChannelCreateComplex(guildID, data)
}

func (d *DiscordgoSession) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	return d.S.InteractionRespond(interaction, resp)
}

func (d *DiscordgoSession) InteractionResponseEdit(interaction *discordgo.Interaction, newresp *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return d.S.InteractionResponseEdit(interaction, newresp)
}

func (d *DiscordgoSession) GetState() *discordgo.State {
	return d.S.State
}
