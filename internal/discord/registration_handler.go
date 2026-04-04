package discord

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/ddbimport"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/ab/dndnd/internal/registration"
)

// RegistrationService abstracts the registration package for handler use.
type RegistrationService interface {
	Register(ctx context.Context, campaignID uuid.UUID, discordUserID, characterName string) (*registration.RegisterResult, error)
	Import(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error)
	Create(ctx context.Context, campaignID uuid.UUID, discordUserID string, characterID uuid.UUID) (*refdata.PlayerCharacter, error)
	GetStatus(ctx context.Context, campaignID uuid.UUID, discordUserID string) (*refdata.PlayerCharacter, error)
}

// CampaignProvider looks up campaign data for command handlers.
type CampaignProvider interface {
	GetCampaignByGuildID(ctx context.Context, guildID string) (refdata.Campaign, error)
}

// CharacterCreator creates placeholder characters for import/create flows.
type CharacterCreator interface {
	CreatePlaceholder(ctx context.Context, campaignID uuid.UUID, name string, ddbURL string) (refdata.Character, error)
}

// DDBImporter handles D&D Beyond character import.
type DDBImporter interface {
	Import(ctx context.Context, campaignID uuid.UUID, ddbURL string) (*ddbimport.ImportResult, error)
}

// CharacterNameResolver resolves a character ID to its name.
type CharacterNameResolver func(ctx context.Context, characterID uuid.UUID) (string, error)

// registrationBase holds shared dependencies for registration command handlers.
type registrationBase struct {
	session      Session
	regService   RegistrationService
	campaignProv CampaignProvider
	dmQueueFunc  func(guildID string) string
	dmUserFunc   func(guildID string) string
}

// RegisterHandler handles the /register slash command.
type RegisterHandler struct {
	registrationBase
}

// NewRegisterHandler creates a new RegisterHandler.
func NewRegisterHandler(session Session, regService RegistrationService, campaignProv CampaignProvider, dmQueueFunc func(string) string, dmUserFunc func(string) string) *RegisterHandler {
	return &RegisterHandler{
		registrationBase: registrationBase{
			session:      session,
			regService:   regService,
			campaignProv: campaignProv,
			dmQueueFunc:  dmQueueFunc,
			dmUserFunc:   dmUserFunc,
		},
	}
}

// Handle processes a /register interaction.
func (h *RegisterHandler) Handle(interaction *discordgo.Interaction) {
	characterName := optionString(interaction, "name")
	if characterName == "" {
		respondEphemeral(h.session, interaction, "Please provide a character name.")
		return
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := interactionUserID(interaction)
	result, err := h.regService.Register(context.Background(), campaign.ID, userID, characterName)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Registration error: %s", err))
		return
	}

	switch result.Status {
	case registration.ResultExactMatch:
		respondEphemeral(h.session, interaction,
			fmt.Sprintf("✅ Registration submitted — %s is pending DM approval. You'll be pinged when approved.", characterName))
		postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, characterName, userID, "register")

	case registration.ResultFuzzyMatch:
		suggestions := strings.Join(result.Suggestions, ", ")
		respondEphemeral(h.session, interaction,
			fmt.Sprintf("❌ No character named \"%s\" found. Did you mean: **%s**? Use /register <name> to confirm.", characterName, suggestions))

	case registration.ResultNoMatch:
		respondEphemeral(h.session, interaction,
			fmt.Sprintf("❌ No character named \"%s\" found. No close matches available.", characterName))
	}
}

// ImportHandler handles the /import slash command.
type ImportHandler struct {
	registrationBase
	charCreator CharacterCreator
	ddbImporter DDBImporter
}

// NewImportHandler creates a new ImportHandler.
func NewImportHandler(session Session, regService RegistrationService, campaignProv CampaignProvider, charCreator CharacterCreator, dmQueueFunc func(string) string, dmUserFunc func(string) string, opts ...ImportHandlerOption) *ImportHandler {
	h := &ImportHandler{
		registrationBase: registrationBase{
			session:      session,
			regService:   regService,
			campaignProv: campaignProv,
			dmQueueFunc:  dmQueueFunc,
			dmUserFunc:   dmUserFunc,
		},
		charCreator: charCreator,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// ImportHandlerOption configures the ImportHandler.
type ImportHandlerOption func(*ImportHandler)

// WithDDBImporter sets the DDB import service on the handler.
func WithDDBImporter(importer DDBImporter) ImportHandlerOption {
	return func(h *ImportHandler) {
		h.ddbImporter = importer
	}
}

// Handle processes an /import interaction.
func (h *ImportHandler) Handle(interaction *discordgo.Interaction) {
	ddbURL := optionString(interaction, "ddb-url")
	if ddbURL == "" {
		respondEphemeral(h.session, interaction, "Please provide a D&D Beyond URL.")
		return
	}

	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := interactionUserID(interaction)

	// Use DDB importer if available (real import), otherwise fall back to placeholder
	if h.ddbImporter != nil {
		h.handleDDBImport(interaction, campaign, userID, ddbURL)
		return
	}

	h.handlePlaceholderImport(interaction, campaign, userID, ddbURL)
}

func (h *ImportHandler) handleDDBImport(interaction *discordgo.Interaction, campaign refdata.Campaign, userID, ddbURL string) {
	importResult, err := h.ddbImporter.Import(context.Background(), campaign.ID, ddbURL)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Import error: %s", err))
		return
	}

	_, err = h.regService.Import(context.Background(), campaign.ID, userID, importResult.Character.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Import error: %s", err))
		return
	}

	// Build ephemeral preview response
	var msg string
	if importResult.IsResync {
		if len(importResult.Changes) == 0 {
			msg = fmt.Sprintf("Re-import of **%s** — no changes detected.\n\n%s", importResult.Character.Name, importResult.Preview)
		} else {
			msg = fmt.Sprintf("Re-import of **%s** — changes detected and pending DM approval.\n\n%s", importResult.Character.Name, importResult.Preview)
		}
	} else {
		msg = fmt.Sprintf("Import of **%s** submitted for DM approval.\n\n%s", importResult.Character.Name, importResult.Preview)
	}

	respondEphemeral(h.session, interaction, msg)
	postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, importResult.Character.Name, userID, "import")
}

func (h *ImportHandler) handlePlaceholderImport(interaction *discordgo.Interaction, campaign refdata.Campaign, userID, ddbURL string) {
	charName := fmt.Sprintf("Imported (%s)", truncateURL(ddbURL, 40))

	char, err := h.charCreator.CreatePlaceholder(context.Background(), campaign.ID, charName, ddbURL)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Import error: %s", err))
		return
	}

	_, err = h.regService.Import(context.Background(), campaign.ID, userID, char.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Import error: %s", err))
		return
	}

	respondEphemeral(h.session, interaction,
		fmt.Sprintf("✅ Registration submitted — %s is pending DM approval. You'll be pinged when approved.", charName))
	postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, charName, userID, "import")
}

// CreateCharacterHandler handles the /create-character slash command.
type CreateCharacterHandler struct {
	registrationBase
	charCreator CharacterCreator
	tokenFunc   func(campaignID uuid.UUID, discordUserID string) string
}

// NewCreateCharacterHandler creates a new CreateCharacterHandler.
func NewCreateCharacterHandler(session Session, regService RegistrationService, campaignProv CampaignProvider, charCreator CharacterCreator, dmQueueFunc func(string) string, dmUserFunc func(string) string, tokenFunc func(uuid.UUID, string) string) *CreateCharacterHandler {
	return &CreateCharacterHandler{
		registrationBase: registrationBase{
			session:      session,
			regService:   regService,
			campaignProv: campaignProv,
			dmQueueFunc:  dmQueueFunc,
			dmUserFunc:   dmUserFunc,
		},
		charCreator: charCreator,
		tokenFunc:   tokenFunc,
	}
}

// Handle processes a /create-character interaction.
func (h *CreateCharacterHandler) Handle(interaction *discordgo.Interaction) {
	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := interactionUserID(interaction)
	charName := fmt.Sprintf("New Character (by <@%s>)", userID)

	char, err := h.charCreator.CreatePlaceholder(context.Background(), campaign.ID, charName, "")
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Error creating character: %s", err))
		return
	}

	_, err = h.regService.Create(context.Background(), campaign.ID, userID, char.ID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Error: %s", err))
		return
	}

	token := h.tokenFunc(campaign.ID, userID)
	portalURL := fmt.Sprintf("https://portal.dndnd.app/create?token=%s", token)

	respondEphemeral(h.session, interaction,
		fmt.Sprintf("✅ Registration submitted — your character is pending DM approval. You'll be pinged when approved.\n\n🔗 **Character Builder:** %s\n_(Link expires in 24 hours)_", portalURL))
	postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, charName, userID, "create-character")
}


// postDMQueueNotification sends a registration notification to the DM queue channel.
func postDMQueueNotification(session Session, dmQueueFunc, dmUserFunc func(string) string, guildID, characterName, playerUserID, via string) {
	channelID := dmQueueFunc(guildID)
	if channelID == "" {
		return
	}
	dmUserID := dmUserFunc(guildID)
	msg := fmt.Sprintf("🆕 <@%s> — **%s** registration by <@%s> via /%s. Pending approval.", dmUserID, characterName, playerUserID, via)
	_, _ = session.ChannelMessageSend(channelID, msg)
}

// StatusCheckResponse returns a status message for a player's current registration state.
// Returns empty string if the player has no registration (unregistered).
func StatusCheckResponse(pc *refdata.PlayerCharacter, characterName string) string {
	switch pc.Status {
	case "pending":
		elapsed := time.Since(pc.CreatedAt)
		return fmt.Sprintf("⏳ %s — pending DM approval since %s. You'll be pinged when approved.", characterName, formatRelativeTime(elapsed))
	case "changes_requested":
		return fmt.Sprintf("🔄 %s — DM requested changes: %s. Use `/create-character` or `/import` to resubmit.", characterName, pc.DmFeedback.String)
	case "approved":
		return "" // no status message needed
	case "rejected":
		return fmt.Sprintf("❌ %s — registration was rejected. Use `/create-character`, `/import`, or `/register` to try again.", characterName)
	default:
		return ""
	}
}

// NoRegistrationMessage is returned when a player runs a game command without registering.
const NoRegistrationMessage = "❌ No character found. Use `/create-character`, `/import`, or `/register` to get started."

// optionString extracts a named string option from an interaction's command data.
func optionString(interaction *discordgo.Interaction, name string) string {
	data := interaction.Data.(discordgo.ApplicationCommandInteractionData)
	for _, opt := range data.Options {
		if opt.Name == name {
			return opt.StringValue()
		}
	}
	return ""
}

// interactionUserID extracts the user ID from an interaction, handling both guild and DM contexts.
func interactionUserID(interaction *discordgo.Interaction) string {
	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}
	if interaction.User != nil {
		return interaction.User.ID
	}
	return ""
}

// GeneratePortalToken generates a stub token for the character builder portal.
func GeneratePortalToken(campaignID uuid.UUID, discordUserID string) string {
	return fmt.Sprintf("%s-%s-%d", campaignID.String()[:8], discordUserID, time.Now().Unix())
}

// formatRelativeTime formats a duration as a human-readable relative time.
func formatRelativeTime(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// truncateURL shortens a URL to maxLen characters.
func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// GameCommandStatusCheck checks a player's registration status before a game command.
// Returns a status message if the player should not proceed, or empty string if OK.
func GameCommandStatusCheck(ctx context.Context, regService RegistrationService, campaignProv CampaignProvider, nameResolver CharacterNameResolver, guildID, discordUserID string) string {
	campaign, err := campaignProv.GetCampaignByGuildID(ctx, guildID)
	if err != nil {
		return ""
	}

	pc, err := regService.GetStatus(ctx, campaign.ID, discordUserID)
	if err != nil {
		return NoRegistrationMessage
	}

	if pc.Status == "approved" {
		return ""
	}

	characterName := "Your character"
	if nameResolver != nil {
		if name, err := nameResolver(ctx, pc.CharacterID); err == nil {
			characterName = name
		}
	}

	return StatusCheckResponse(pc, characterName)
}

// StatusAwareStubHandler wraps a stub handler with registration status awareness.
type StatusAwareStubHandler struct {
	session      Session
	name         string
	regService   RegistrationService
	campaignProv CampaignProvider
	nameResolver CharacterNameResolver
}

// NewStatusAwareStubHandler creates a handler that checks registration status before responding.
func NewStatusAwareStubHandler(session Session, name string, regService RegistrationService, campaignProv CampaignProvider, nameResolver CharacterNameResolver) *StatusAwareStubHandler {
	return &StatusAwareStubHandler{
		session:      session,
		name:         name,
		regService:   regService,
		campaignProv: campaignProv,
		nameResolver: nameResolver,
	}
}

// Handle checks the player's registration status. If not approved, shows status.
// Otherwise, falls through to the stub response.
func (h *StatusAwareStubHandler) Handle(interaction *discordgo.Interaction) {
	userID := interactionUserID(interaction)
	statusMsg := GameCommandStatusCheck(context.Background(), h.regService, h.campaignProv, h.nameResolver, interaction.GuildID, userID)
	if statusMsg != "" {
		respondEphemeral(h.session, interaction, statusMsg)
		return
	}
	respondEphemeral(h.session, interaction, fmt.Sprintf("/%s is not yet implemented.", h.name))
}

