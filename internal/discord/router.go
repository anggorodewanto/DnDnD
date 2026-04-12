package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// CommandHandler handles a slash command interaction.
type CommandHandler interface {
	Handle(interaction *discordgo.Interaction)
}

// CommandRouter dispatches slash command interactions to the appropriate handler.
type CommandRouter struct {
	bot         *Bot
	handlers    map[string]CommandHandler
	moveHandler *MoveHandler
	flyHandler  *FlyHandler
	doneHandler *DoneHandler
	restHandler *RestHandler
	lootHandler *LootHandler
	asiHandler  *ASIHandler
}

// SetMoveHandler registers the MoveHandler for button callback routing.
func (r *CommandRouter) SetMoveHandler(h *MoveHandler) {
	r.moveHandler = h
}

// SetFlyHandler registers the FlyHandler for button callback routing.
func (r *CommandRouter) SetFlyHandler(h *FlyHandler) {
	r.flyHandler = h
}

// SetDoneHandler registers the DoneHandler for button callback routing.
func (r *CommandRouter) SetDoneHandler(h *DoneHandler) {
	r.doneHandler = h
	r.handlers["done"] = h
}

// SetDistanceHandler registers the DistanceHandler for the /distance command.
func (r *CommandRouter) SetDistanceHandler(h *DistanceHandler) {
	r.handlers["distance"] = h
}

// SetSummonCommandHandler registers the SummonCommandHandler for the /command command.
func (r *CommandRouter) SetSummonCommandHandler(h *SummonCommandHandler) {
	r.handlers["command"] = h
}

// SetRecapHandler registers the RecapHandler for the /recap command.
func (r *CommandRouter) SetRecapHandler(h *RecapHandler) {
	r.handlers["recap"] = h
}

// SetCheckHandler registers the CheckHandler for the /check command.
func (r *CommandRouter) SetCheckHandler(h *CheckHandler) {
	r.handlers["check"] = h
}

// SetSaveHandler registers the SaveHandler for the /save command.
func (r *CommandRouter) SetSaveHandler(h *SaveHandler) {
	r.handlers["save"] = h
}

// SetInventoryHandler registers the InventoryHandler for the /inventory command.
func (r *CommandRouter) SetInventoryHandler(h *InventoryHandler) {
	r.handlers["inventory"] = h
}

// SetUseHandler registers the UseHandler for the /use command.
func (r *CommandRouter) SetUseHandler(h *UseHandler) {
	r.handlers["use"] = h
}

// SetGiveHandler registers the GiveHandler for the /give command.
func (r *CommandRouter) SetGiveHandler(h *GiveHandler) {
	r.handlers["give"] = h
}

// SetRestHandler registers the RestHandler for the /rest command and component callbacks.
func (r *CommandRouter) SetRestHandler(h *RestHandler) {
	r.handlers["rest"] = h
	r.restHandler = h
}

// SetLootHandler registers the LootHandler for the /loot command and component callbacks.
func (r *CommandRouter) SetLootHandler(h *LootHandler) {
	r.handlers["loot"] = h
	r.lootHandler = h
}

// SetAttuneHandler registers the AttuneHandler for the /attune command.
func (r *CommandRouter) SetAttuneHandler(h *AttuneHandler) {
	r.handlers["attune"] = h
}

// SetUnattuneHandler registers the UnattuneHandler for the /unattune command.
func (r *CommandRouter) SetUnattuneHandler(h *UnattuneHandler) {
	r.handlers["unattune"] = h
}

// SetEquipHandler registers the EquipHandler for the /equip command.
func (r *CommandRouter) SetEquipHandler(h *EquipHandler) {
	r.handlers["equip"] = h
}

// SetCharacterHandler registers the CharacterHandler for the /character command.
func (r *CommandRouter) SetCharacterHandler(h *CharacterHandler) {
	r.handlers["character"] = h
}

// SetASIHandler registers the ASIHandler for ASI/Feat component callbacks.
func (r *CommandRouter) SetASIHandler(h *ASIHandler) {
	r.asiHandler = h
}

// SetReactionHandler registers the ReactionHandler for the /reaction command,
// replacing the status-aware stub installed by NewCommandRouter.
func (r *CommandRouter) SetReactionHandler(h *ReactionHandler) {
	r.handlers["reaction"] = h
}

// SetHelpHandler registers the HelpHandler for the /help command,
// replacing the stub installed by NewCommandRouter.
func (r *CommandRouter) SetHelpHandler(h *HelpHandler) {
	r.handlers["help"] = h
}

// RegistrationDeps holds the optional dependencies for registration command handlers.
// When nil, the router uses plain stub handlers for registration commands.
type RegistrationDeps struct {
	RegService   RegistrationService
	CampaignProv CampaignProvider
	CharCreator  CharacterCreator
	DMQueueFunc  func(guildID string) string
	DMUserFunc   func(guildID string) string
	TokenFunc    func(campaignID uuid.UUID, discordUserID string) (string, error)
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
		if deps.TokenFunc == nil {
			panic("RegistrationDeps.TokenFunc is required")
		}
		r.handlers["create-character"] = NewCreateCharacterHandler(bot.session, deps.RegService, deps.CampaignProv, deps.CharCreator, deps.DMQueueFunc, deps.DMUserFunc, deps.TokenFunc)
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
	if interaction.Type == discordgo.InteractionMessageComponent {
		r.handleComponent(interaction)
		return
	}

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

// handleComponent routes message component interactions (button clicks) to the appropriate handler.
func (r *CommandRouter) handleComponent(interaction *discordgo.Interaction) {
	data := interaction.Data.(discordgo.MessageComponentInteractionData)
	customID := data.CustomID

	// Move button callbacks
	if r.moveHandler != nil {
		if strings.HasPrefix(customID, "prone_stand:") {
			turnID, combatantID, destCol, destRow, maxSpeed, err := ParseProneMoveData(customID, "prone_stand")
			if err != nil {
				respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Invalid prone move data: %v", err))
				return
			}
			r.moveHandler.HandleProneStandAndMove(interaction, turnID, combatantID, destCol, destRow, maxSpeed)
			return
		}

		if strings.HasPrefix(customID, "prone_crawl:") {
			turnID, combatantID, destCol, destRow, _, err := ParseProneMoveData(customID, "prone_crawl")
			if err != nil {
				respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Invalid prone move data: %v", err))
				return
			}
			r.moveHandler.HandleProneCrawl(interaction, turnID, combatantID, destCol, destRow)
			return
		}

		if strings.HasPrefix(customID, "move_confirm:") {
			// Differentiate basic (6 colon-separated fields) vs extended (8 fields with mode)
			if isMoveConfirmWithMode(customID) {
				turnID, combatantID, destCol, destRow, costFt, mode, standCostFt, err := ParseMoveConfirmWithModeData(customID)
				if err != nil {
					respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Invalid move data: %v", err))
					return
				}
				r.moveHandler.HandleMoveConfirmWithMode(interaction, turnID, combatantID, destCol, destRow, costFt, mode, standCostFt)
			} else {
				turnID, combatantID, destCol, destRow, costFt, err := ParseMoveConfirmData(customID)
				if err != nil {
					respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Invalid move data: %v", err))
					return
				}
				r.moveHandler.HandleMoveConfirm(interaction, turnID, combatantID, destCol, destRow, costFt)
			}
			return
		}

		if strings.HasPrefix(customID, "move_cancel:") {
			r.moveHandler.HandleMoveCancel(interaction)
			return
		}
	}

	// Done button callbacks
	if r.doneHandler != nil {
		if strings.HasPrefix(customID, "done_confirm:") {
			encounterIDStr := strings.TrimPrefix(customID, "done_confirm:")
			encounterID, err := uuid.Parse(encounterIDStr)
			if err != nil {
				respondEphemeral(r.bot.session, interaction, "Invalid encounter ID.")
				return
			}
			r.doneHandler.HandleDoneConfirm(interaction, encounterID)
			return
		}

		if customID == "done_cancel" {
			r.doneHandler.HandleDoneCancel(interaction)
			return
		}
	}

	// Rest hit dice button callbacks
	if r.restHandler != nil {
		if strings.HasPrefix(customID, hitDicePrefix+":") {
			r.restHandler.HandleHitDiceComponent(interaction)
			return
		}
	}

	// Loot claim button callbacks
	if r.lootHandler != nil {
		if strings.HasPrefix(customID, "loot_claim:") {
			poolID, itemID, characterID, err := ParseLootClaimData(customID)
			if err != nil {
				respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Invalid loot claim data: %v", err))
				return
			}
			r.lootHandler.HandleLootClaim(interaction, poolID, itemID, characterID)
			return
		}
	}

	// ASI/Feat button and select menu callbacks
	if r.asiHandler != nil {
		if strings.HasPrefix(customID, asiChoicePrefix+":") {
			r.asiHandler.HandleASIChoice(interaction)
			return
		}
		if strings.HasPrefix(customID, asiSelectPrefix+":") {
			r.asiHandler.HandleASISelect(interaction)
			return
		}
		if strings.HasPrefix(customID, asiApprovePrefix+":") {
			r.asiHandler.HandleDMApprove(interaction)
			return
		}
		if strings.HasPrefix(customID, asiDenyPrefix+":") {
			r.asiHandler.HandleDMDeny(interaction)
			return
		}
	}

	// Fly button callbacks
	if r.flyHandler != nil {
		if strings.HasPrefix(customID, "fly_confirm:") {
			turnID, combatantID, newAlt, costFt, err := ParseFlyConfirmData(customID)
			if err != nil {
				respondEphemeral(r.bot.session, interaction, fmt.Sprintf("Invalid fly data: %v", err))
				return
			}
			r.flyHandler.HandleFlyConfirm(interaction, turnID, combatantID, newAlt, costFt)
			return
		}

		if strings.HasPrefix(customID, "fly_cancel:") {
			r.flyHandler.HandleFlyCancel(interaction)
			return
		}
	}
}

// isMoveConfirmWithMode checks if a move_confirm custom ID includes a mode suffix.
// Basic format has 6 colon-separated parts, extended has 8.
func isMoveConfirmWithMode(customID string) bool {
	return strings.Count(customID, ":") >= 7
}

// discordUserID extracts the Discord user ID from an interaction, returning "" if unavailable.
func discordUserID(interaction *discordgo.Interaction) string {
	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}
	return ""
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
