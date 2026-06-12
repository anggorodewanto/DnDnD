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

// Handle processes a /register interaction. With a name it claims an existing
// character by that name; without a name it surfaces the onboarding chooser so
// the player can claim, build, or import.
func (h *RegisterHandler) Handle(interaction *discordgo.Interaction) {
	characterName := optionString(interaction, "name")
	if characterName == "" {
		h.PromptChoice(interaction)
		return
	}
	h.registerByName(interaction, characterName)
}

// PromptChoice shows the onboarding chooser: three buttons (claim / build /
// import) plus hint text covering the equivalent slash commands.
func (h *RegisterHandler) PromptChoice(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: registerChoicePrompt,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.Button{Label: "Claim Existing", Style: discordgo.PrimaryButton, CustomID: regChoiceClaim, Emoji: &discordgo.ComponentEmoji{Name: "📋"}},
					discordgo.Button{Label: "Build New", Style: discordgo.SuccessButton, CustomID: regChoiceBuild, Emoji: &discordgo.ComponentEmoji{Name: "🆕"}},
					discordgo.Button{Label: "Import from D&D Beyond", Style: discordgo.SecondaryButton, CustomID: regChoiceImport, Emoji: &discordgo.ComponentEmoji{Name: "📥"}},
				}},
			},
		},
	})
}

// ShowClaimModal opens a modal asking for the character name to claim.
func (h *RegisterHandler) ShowClaimModal(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, textInputModal(
		regModalClaim, "Claim a Character",
		regModalNameInput, "Character name", "The name your DM gave the character",
	))
}

// HandleClaimSubmit processes the claim modal submission, registering by the
// name the player typed.
func (h *RegisterHandler) HandleClaimSubmit(interaction *discordgo.Interaction) {
	name := modalTextValue(interaction, regModalNameInput)
	if name == "" {
		respondEphemeral(h.session, interaction, "Please enter a character name.")
		return
	}
	h.registerByName(interaction, name)
}

// registerByName claims an existing character for the invoking player by name.
func (h *RegisterHandler) registerByName(interaction *discordgo.Interaction, characterName string) {
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
		postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, characterName, userID, "register", nil)

	case registration.ResultFuzzyMatch:
		bolded := make([]string, len(result.Suggestions))
		for i, s := range result.Suggestions {
			bolded[i] = "**" + s + "**"
		}
		respondEphemeral(h.session, interaction,
			fmt.Sprintf("❌ No character named \"%s\" found. Did you mean: %s? Use /register %s to confirm.", characterName, strings.Join(bolded, ", "), result.Suggestions[0]))

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
	h.processImport(interaction, ddbURL)
}

// ShowImportModal opens a modal asking for the D&D Beyond character URL.
func (h *ImportHandler) ShowImportModal(interaction *discordgo.Interaction) {
	_ = h.session.InteractionRespond(interaction, textInputModal(
		regModalImport, "Import from D&D Beyond",
		regModalURLInput, "D&D Beyond character URL", "https://www.dndbeyond.com/characters/12345678",
	))
}

// HandleImportSubmit processes the import modal submission.
func (h *ImportHandler) HandleImportSubmit(interaction *discordgo.Interaction) {
	ddbURL := modalTextValue(interaction, regModalURLInput)
	if ddbURL == "" {
		respondEphemeral(h.session, interaction, "Please enter a D&D Beyond URL.")
		return
	}
	h.processImport(interaction, ddbURL)
}

// processImport runs the import flow for a resolved URL, branching to the real
// DDB importer when wired or the placeholder path otherwise.
func (h *ImportHandler) processImport(interaction *discordgo.Interaction, ddbURL string) {
	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := interactionUserID(interaction)

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

	// Build ephemeral preview response. For re-syncs the diff is shown to the
	// DM but the character row is NOT mutated until the DM explicitly approves
	// (Phase 90 spec: "system diffs and shows DM what changed before
	// applying"). The pending update lives in ddbimport.Service's pending map
	// keyed by importResult.PendingImportID.
	var msg string
	if importResult.IsResync {
		if len(importResult.Changes) == 0 {
			msg = fmt.Sprintf("Re-import of **%s** — no changes detected.\n\n%s", importResult.Character.Name, importResult.Preview)
		} else {
			msg = fmt.Sprintf("Re-import of **%s** — changes detected and pending DM review (no changes applied yet).\n\n%s", importResult.Character.Name, importResult.Preview)
		}
	} else {
		msg = fmt.Sprintf("Import of **%s** submitted for DM approval.\n\n%s", importResult.Character.Name, importResult.Preview)
	}

	respondEphemeral(h.session, interaction, msg)

	// Finding 21: When a re-sync produces a pending import, include the
	// pending import ID in the DM queue notification so the approval flow
	// can reference it.
	if importResult.PendingImportID != uuid.Nil {
		postDMQueueResyncNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, importResult.Character.Name, userID, importResult.PendingImportID, importResult.Changes)
	} else {
		postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, importResult.Character.Name, userID, "import", importResult.Warnings)
	}
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
	postDMQueueNotification(h.session, h.dmQueueFunc, h.dmUserFunc, interaction.GuildID, charName, userID, "import", nil)
}

// defaultPortalBaseURL is the co-located dashboard origin used when no BASE_URL
// is configured. The portal is served from the same host as the dashboard, so
// this default keeps zero-config local dev working without explicit wiring.
const defaultPortalBaseURL = "http://localhost:8080"

// Registration chooser component identifiers. regChoice* are the buttons shown
// by /register with no name; regModal* are the modals the claim/import buttons
// open; regModal*Input are the text-input fields inside those modals.
const (
	regChoiceClaim  = "regchoice:claim"
	regChoiceBuild  = "regchoice:build"
	regChoiceImport = "regchoice:import"

	regModalClaim  = "regmodal:claim"
	regModalImport = "regmodal:import"

	regModalNameInput = "name"
	regModalURLInput  = "ddb-url"
)

// registerChoicePrompt is the hint text shown above the onboarding buttons.
const registerChoicePrompt = "**How do you want your character?**\n" +
	"📋 **Claim Existing** — link to a character your DM already created (you'll type the name).\n" +
	"🆕 **Build New** — open the web character builder.\n" +
	"📥 **Import** — pull a character from D&D Beyond (you'll paste the URL).\n\n" +
	"_Tip: you can also run `/register name:<name>`, `/import ddb-url:<url>`, or `/create-character` directly._"

// CreateCharacterHandler handles the /create-character slash command.
type CreateCharacterHandler struct {
	registrationBase
	tokenFunc     func(campaignID uuid.UUID, discordUserID string) (string, error)
	portalBaseURL string
}

// CreateCharacterOption configures a CreateCharacterHandler at construction time.
type CreateCharacterOption func(*CreateCharacterHandler)

// WithCreateCharacterPortalBaseURL sets the base URL used to build the
// /create-character portal link. Empty falls back to defaultPortalBaseURL.
func WithCreateCharacterPortalBaseURL(baseURL string) CreateCharacterOption {
	return func(h *CreateCharacterHandler) {
		h.portalBaseURL = strings.TrimRight(baseURL, "/")
	}
}

// NewCreateCharacterHandler creates a new CreateCharacterHandler.
func NewCreateCharacterHandler(session Session, regService RegistrationService, campaignProv CampaignProvider, dmQueueFunc func(string) string, dmUserFunc func(string) string, tokenFunc func(uuid.UUID, string) (string, error), opts ...CreateCharacterOption) *CreateCharacterHandler {
	h := &CreateCharacterHandler{
		registrationBase: registrationBase{
			session:      session,
			regService:   regService,
			campaignProv: campaignProv,
			dmQueueFunc:  dmQueueFunc,
			dmUserFunc:   dmUserFunc,
		},
		tokenFunc: tokenFunc,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Handle processes a /create-character interaction.
//
// SR-013: this no longer creates a placeholder character or a player_characters
// row up front. The row is written once, when the player submits the web
// builder — so re-running /create-character (e.g. after the link expires) just
// hands out a fresh link instead of colliding on the unique index. The only
// thing we refuse here is starting a build when an *approved* character already
// exists; that player must /retire first.
func (h *CreateCharacterHandler) Handle(interaction *discordgo.Interaction) {
	campaign, err := h.campaignProv.GetCampaignByGuildID(context.Background(), interaction.GuildID)
	if err != nil {
		respondEphemeral(h.session, interaction, "No campaign found for this server.")
		return
	}

	userID := interactionUserID(interaction)

	// GetStatus errors (including "no active character") are non-fatal: only an
	// existing *approved* character blocks a new build.
	if existing, statusErr := h.regService.GetStatus(context.Background(), campaign.ID, userID); statusErr == nil && existing != nil && existing.Status == "approved" {
		respondEphemeral(h.session, interaction,
			"You already have an active character in this campaign. Use /retire first if you want to build a new one.")
		return
	}

	token, err := h.tokenFunc(campaign.ID, userID)
	if err != nil {
		respondEphemeral(h.session, interaction, fmt.Sprintf("Error generating portal link: %s", err))
		return
	}
	base := h.portalBaseURL
	if base == "" {
		base = defaultPortalBaseURL
	}
	portalURL := fmt.Sprintf("%s/portal/create?token=%s", base, token)

	respondEphemeral(h.session, interaction,
		fmt.Sprintf("🔗 **Character Builder:** %s\n\nBuild your character there — it's submitted for DM approval when you finish.\n_(Link expires in 24 hours)_", portalURL))
}

// postDMQueueNotification sends a registration notification to the DM queue channel.
func postDMQueueNotification(session Session, dmQueueFunc, dmUserFunc func(string) string, guildID, characterName, playerUserID, via string, warnings []ddbimport.Warning) {
	channelID := dmQueueFunc(guildID)
	if channelID == "" {
		return
	}
	dmUserID := dmUserFunc(guildID)
	msg := fmt.Sprintf("🆕 <@%s> — **%s** registration by <@%s> via /%s. Pending approval.", dmUserID, characterName, playerUserID, via)
	if len(warnings) > 0 {
		var b strings.Builder
		b.WriteString(msg)
		b.WriteString("\n\n**Warnings:**")
		for _, warning := range warnings {
			b.WriteString("\n⚠️ ")
			b.WriteString(warning.Message)
		}
		msg = b.String()
	}
	_, _ = session.ChannelMessageSend(channelID, msg)
}

// postDMQueueResyncNotification sends a re-sync notification to the DM queue
// channel, including the pending import ID so the DM can approve/reject.
// Finding 21: makes the pending import reachable from the DM approval flow.
func postDMQueueResyncNotification(session Session, dmQueueFunc, dmUserFunc func(string) string, guildID, characterName, playerUserID string, pendingImportID uuid.UUID, changes []string) {
	channelID := dmQueueFunc(guildID)
	if channelID == "" {
		return
	}
	dmUserID := dmUserFunc(guildID)
	var b strings.Builder
	fmt.Fprintf(&b, "🔄 <@%s> — **%s** re-sync by <@%s> requires approval.\n", dmUserID, characterName, playerUserID)
	fmt.Fprintf(&b, "**Pending Import ID:** `%s`\n", pendingImportID)
	if len(changes) > 0 {
		b.WriteString("\n**Changes:**")
		for _, c := range changes {
			b.WriteString("\n• ")
			b.WriteString(c)
		}
	}
	b.WriteString("\n\nUse the dashboard or `/approve-import` to apply.")
	_, _ = session.ChannelMessageSend(channelID, b.String())
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

// textInputModalFrom wraps a caller-built TextInput in a single-row modal
// response. customID identifies the modal on submit. Callers set the input's
// style/length so each modal keeps its own field intent.
func textInputModalFrom(customID, title string, input discordgo.TextInput) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{input}},
			},
		},
	}
}

// textInputModal builds a modal response with a single required short text
// input. customID identifies the modal on submit; inputID identifies the field.
func textInputModal(customID, title, inputID, label, placeholder string) *discordgo.InteractionResponse {
	return textInputModalFrom(customID, title, discordgo.TextInput{
		CustomID:    inputID,
		Label:       label,
		Style:       discordgo.TextInputShort,
		Placeholder: placeholder,
		Required:    true,
		MaxLength:   200,
	})
}

// modalTextValue extracts the value of a named text input from a modal-submit
// interaction. discordgo unmarshals received components as pointers, so rows
// arrive as *ActionsRow and inputs as *TextInput.
func modalTextValue(interaction *discordgo.Interaction, inputID string) string {
	if interaction == nil {
		return ""
	}
	data, ok := interaction.Data.(discordgo.ModalSubmitInteractionData)
	if !ok {
		return ""
	}
	for _, comp := range data.Components {
		row, ok := comp.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, c := range row.Components {
			if ti, ok := c.(*discordgo.TextInput); ok && ti.CustomID == inputID {
				return ti.Value
			}
		}
	}
	return ""
}

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
