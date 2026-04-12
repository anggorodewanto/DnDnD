package discord

import (
	"github.com/bwmarrin/discordgo"
)

// HelpHandler handles the /help slash command.
type HelpHandler struct {
	session Session
}

// NewHelpHandler creates a new HelpHandler.
func NewHelpHandler(session Session) *HelpHandler {
	return &HelpHandler{session: session}
}

// Handle processes the /help interaction.
func (h *HelpHandler) Handle(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)

	var topic string
	for _, opt := range data.Options {
		if opt.Name == "topic" {
			topic = opt.StringValue()
			break
		}
	}

	if topic == "" {
		respondEphemeral(h.session, interaction, generalHelp)
		return
	}

	text, ok := helpTopics[topic]
	if !ok {
		respondEphemeral(h.session, interaction, "Unknown help topic: `"+topic+"`. Use `/help` to see all commands.")
		return
	}

	respondEphemeral(h.session, interaction, text)
}
