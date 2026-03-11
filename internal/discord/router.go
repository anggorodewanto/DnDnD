package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
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

// RegistrationDeps holds the optional dependencies for registration command handlers.
// When nil, the router uses plain stub handlers for registration commands.
type RegistrationDeps struct {
	RegService   RegistrationService
	CampaignProv CampaignProvider
	CharCreator  CharacterCreator
	DMQueueFunc  func(guildID string) string
	DMUserFunc   func(guildID string) string
	TokenFunc    func(campaignID uuid.UUID, discordUserID string) string
	NameResolver CharacterNameResolver
}

// NewCommandRouter creates a CommandRouter with stub handlers for all player commands
// and routes /setup to the provided SetupHandler.
// If regDeps is non-nil, registration commands (/register, /import, /create-character)
// use real handlers and game commands become status-aware.
func NewCommandRouter(bot *Bot, setupHandler *SetupHandler, regDeps ...*RegistrationDeps) *CommandRouter {
	r := &CommandRouter{
		bot:      bot,
		handlers: make(map[string]CommandHandler),
	}

	gameCommands := []string{
		"move", "fly", "attack", "cast", "bonus", "action", "shove",
		"interact", "done", "deathsave", "command", "reaction", "check",
		"save", "rest", "whisper", "status", "equip", "undo", "inventory",
		"use", "give", "loot", "attune", "unattune", "prepare", "retire",
		"character", "recap", "distance", "help",
	}

	regCommands := []string{"register", "import", "create-character"}

	var deps *RegistrationDeps
	if len(regDeps) > 0 {
		deps = regDeps[0]
	}

	if deps != nil {
		// Wire game commands with status awareness.
		for _, name := range gameCommands {
			r.handlers[name] = NewStatusAwareStubHandler(bot.session, name, deps.RegService, deps.CampaignProv, deps.NameResolver)
		}

		// Wire registration commands to real handlers.
		r.handlers["register"] = NewRegisterHandler(bot.session, deps.RegService, deps.CampaignProv, deps.DMQueueFunc, deps.DMUserFunc)
		r.handlers["import"] = NewImportHandler(bot.session, deps.RegService, deps.CampaignProv, deps.CharCreator, deps.DMQueueFunc, deps.DMUserFunc)
		tokenFunc := deps.TokenFunc
		if tokenFunc == nil {
			tokenFunc = GeneratePortalToken
		}
		r.handlers["create-character"] = NewCreateCharacterHandler(bot.session, deps.RegService, deps.CampaignProv, deps.CharCreator, deps.DMQueueFunc, deps.DMUserFunc, tokenFunc)
	} else {
		// Fallback: all stubs.
		for _, name := range gameCommands {
			r.handlers[name] = &stubHandler{session: bot.session, name: name}
		}
		for _, name := range regCommands {
			r.handlers[name] = &stubHandler{session: bot.session, name: name}
		}
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
