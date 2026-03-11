package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// CommandHandler handles a slash command interaction.
type CommandHandler interface {
	Handle(interaction *discordgo.Interaction)
}

// CommandRouter dispatches slash command interactions to the appropriate handler.
type CommandRouter struct {
	bot      *Bot
	handlers map[string]CommandHandler
}

// NewCommandRouter creates a CommandRouter with stub handlers for all player commands
// and routes /setup to the provided SetupHandler.
func NewCommandRouter(bot *Bot, setupHandler *SetupHandler) *CommandRouter {
	r := &CommandRouter{
		bot:      bot,
		handlers: make(map[string]CommandHandler),
	}

	// Register stub handlers for all player-facing commands.
	stubCommands := []string{
		"move", "fly", "attack", "cast", "bonus", "action", "shove",
		"interact", "done", "deathsave", "command", "reaction", "check",
		"save", "rest", "whisper", "status", "equip", "undo", "inventory",
		"use", "give", "loot", "attune", "unattune", "prepare", "retire",
		"register", "import", "create-character", "character", "recap",
		"distance", "help",
	}
	for _, name := range stubCommands {
		r.handlers[name] = &stubHandler{session: bot.session, name: name}
	}

	// Route /setup to its dedicated handler if provided.
	if setupHandler != nil {
		r.handlers["setup"] = setupHandler
	}

	return r
}

// Handle dispatches an interaction to the correct command handler.
func (r *CommandRouter) Handle(interaction *discordgo.Interaction) {
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	handler, ok := r.handlers[data.Name]
	if !ok {
		respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Unknown command: /%s", data.Name))
		return
	}

	handler.Handle(interaction)
}

// respondEphemeral sends an ephemeral message as an interaction response.
func respondEphemeral(s Session, interaction *discordgo.Interaction, msg string) {
	_ = s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// stubHandler responds with a "not yet implemented" message for a command.
type stubHandler struct {
	session Session
	name    string
}

func (h *stubHandler) Handle(interaction *discordgo.Interaction) {
	respondEphemeral(h.session, interaction, fmt.Sprintf("/%s is not yet implemented.", h.name))
}
