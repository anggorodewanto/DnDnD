package discord

import (
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Bot manages the Discord bot lifecycle: guild tracking, command registration, and event handling.
type Bot struct {
	session       Session
	appID         string
	logger        *slog.Logger
	mu            sync.RWMutex
	guilds        map[string]bool
	campaignNames map[string]string // guildID -> campaign name
}

// NewBot creates a new Bot with the given session and application ID.
func NewBot(s Session, appID string, logger *slog.Logger) *Bot {
	return &Bot{
		session:       s,
		appID:         appID,
		logger:        logger,
		guilds:        make(map[string]bool),
		campaignNames: make(map[string]string),
	}
}

// AppID returns the bot's application ID.
func (b *Bot) AppID() string {
	return b.appID
}

// HasGuild reports whether the given guild is tracked.
func (b *Bot) HasGuild(guildID string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.guilds[guildID]
}

// Guilds returns a slice of all tracked guild IDs.
func (b *Bot) Guilds() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]string, 0, len(b.guilds))
	for id := range b.guilds {
		result = append(result, id)
	}
	return result
}

// SetCampaignName sets the campaign name for a guild, used in welcome DMs.
func (b *Bot) SetCampaignName(guildID, name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.campaignNames[guildID] = name
}

func (b *Bot) campaignName(guildID string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if name := b.campaignNames[guildID]; name != "" {
		return name
	}
	return "this campaign"
}

// trackAndRegister tracks a guild and registers slash commands for it.
// Returns an error if command registration fails.
func (b *Bot) trackAndRegister(guildID string) error {
	b.mu.Lock()
	b.guilds[guildID] = true
	b.mu.Unlock()

	if err := RegisterCommands(b.session, b.appID, guildID); err != nil {
		b.logger.Error("failed to register commands", "guild_id", guildID, "error", err)
		return err
	}
	return nil
}

// HandleGuildCreate is called when the bot joins a guild or during startup enumeration.
// It registers commands for the guild and tracks it.
func (b *Bot) HandleGuildCreate(_ *discordgo.Session, event *discordgo.GuildCreate) {
	b.logger.Info("guild create event", "guild_id", event.Guild.ID)
	b.ValidateGuildPermissions(event.Guild.ID, event.Guild.Permissions)
	b.trackAndRegister(event.Guild.ID)
}

// RegisterAllGuilds registers commands for all provided guild IDs.
// It tracks each guild and collects any registration errors.
func (b *Bot) RegisterAllGuilds(guildIDs []string) []error {
	var errs []error
	for _, guildID := range guildIDs {
		if err := b.trackAndRegister(guildID); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ValidateGuildPermissions checks whether the bot has all required permissions
// in the given guild and logs a warning listing any that are missing.
func (b *Bot) ValidateGuildPermissions(guildID string, granted int64) {
	missing := ValidatePermissions(granted)
	if len(missing) == 0 {
		return
	}
	b.logger.Warn("bot missing required permissions",
		"guild_id", guildID,
		"missing", missing,
	)
}

// HandleGuildMemberAdd is called when a new member joins a guild.
// It sends a welcome DM to non-bot users.
func (b *Bot) HandleGuildMemberAdd(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
	if event.Member.User.Bot {
		return
	}

	name := b.campaignName(event.Member.GuildID)
	if err := SendWelcomeDM(b.session, event.Member.User.ID, name); err != nil {
		b.logger.Error("failed to send welcome DM",
			"user_id", event.Member.User.ID,
			"guild_id", event.Member.GuildID,
			"error", err)
	}
}
