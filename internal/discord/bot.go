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
	name := b.campaignNames[guildID]
	if name == "" {
		return "this campaign"
	}
	return name
}

// HandleGuildCreate is called when the bot joins a guild or during startup enumeration.
// It registers commands for the guild and tracks it.
func (b *Bot) HandleGuildCreate(_ *discordgo.Session, event *discordgo.GuildCreate) {
	guildID := event.Guild.ID
	b.logger.Info("guild create event", "guild_id", guildID)

	b.mu.Lock()
	b.guilds[guildID] = true
	b.mu.Unlock()

	if err := RegisterCommands(b.session, b.appID, guildID); err != nil {
		b.logger.Error("failed to register commands", "guild_id", guildID, "error", err)
	}
}

// RegisterAllGuilds registers commands for all provided guild IDs.
// It tracks each guild and collects any registration errors.
func (b *Bot) RegisterAllGuilds(guildIDs []string) []error {
	var errs []error
	for _, guildID := range guildIDs {
		b.mu.Lock()
		b.guilds[guildID] = true
		b.mu.Unlock()

		if err := RegisterCommands(b.session, b.appID, guildID); err != nil {
			b.logger.Error("failed to register commands", "guild_id", guildID, "error", err)
			errs = append(errs, err)
		}
	}
	return errs
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
