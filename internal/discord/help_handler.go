package discord

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// HelpEncounterProvider resolves the user's active encounter for context tips.
type HelpEncounterProvider interface {
	ActiveEncounterForUser(ctx context.Context, guildID, discordUserID string) (uuid.UUID, error)
	GetEncounter(ctx context.Context, id uuid.UUID) (refdata.Encounter, error)
}

// HelpHandler handles the /help slash command.
type HelpHandler struct {
	session           Session
	encounterProvider HelpEncounterProvider
}

// NewHelpHandler creates a new HelpHandler.
func NewHelpHandler(session Session) *HelpHandler {
	return &HelpHandler{session: session}
}

// SetEncounterProvider wires the optional encounter provider for context tips.
func (h *HelpHandler) SetEncounterProvider(p HelpEncounterProvider) {
	h.encounterProvider = p
}

// Handle processes the /help interaction.
func (h *HelpHandler) Handle(interaction *discordgo.Interaction) {
	topic := optionString(interaction, "topic")

	if topic == "" {
		text := generalHelp + h.contextTips(interaction)
		respondEphemeral(h.session, interaction, text)
		return
	}

	text, ok := helpTopics[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		respondEphemeral(h.session, interaction, "Unknown help topic: `"+topic+"`. Use `/help` to see all commands.")
		return
	}

	respondEphemeral(h.session, interaction, text)
}

// contextTips returns context-specific tips based on the user's current encounter mode.
func (h *HelpHandler) contextTips(interaction *discordgo.Interaction) string {
	if h.encounterProvider == nil {
		return ""
	}

	ctx := context.Background()
	userID := discordUserID(interaction)
	encID, err := h.encounterProvider.ActiveEncounterForUser(ctx, interaction.GuildID, userID)
	if err != nil {
		return ""
	}

	enc, err := h.encounterProvider.GetEncounter(ctx, encID)
	if err != nil {
		return ""
	}

	switch enc.Mode {
	case "combat":
		return combatContextTips
	case "exploration":
		return explorationContextTips
	default:
		return ""
	}
}

const combatContextTips = `

**⚔️ Context Tips (Combat)**
• Use /move to relocate (costs movement speed)
• Use /attack to strike enemies in range
• Use /done when your turn is complete
• Use /action for special actions (Dash, Dodge, Disengage, Help, Hide)
• While prone: stand up costs half your movement`

const explorationContextTips = `

**🧭 Context Tips (Exploration)**
• Use /move to explore freely (no speed limit)
• Use /action to describe freeform actions for the DM
• Use /check for skill checks (Investigation, Perception, etc.)
• Use /whisper to send a private message to the DM`
