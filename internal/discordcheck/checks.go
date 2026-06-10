// Package discordcheck performs startup self-checks against the Discord API
// so misconfigured deploys fail loudly at boot instead of at the first user
// interaction. It verifies the bot token is accepted, the configured
// DISCORD_APPLICATION_ID matches the actual bot identity, that the privileged
// Server Members (GuildMembers) gateway intent is enabled, and that every
// guild_id stored in the campaigns table belongs to a guild the bot is
// currently a member of. RunChannelBindings additionally flags guilds whose
// channel bindings were never persisted by /setup (without which combat /
// turn / dm-queue posts silently no-op), and RunTokenEncryption flags a
// wrong-length TOKEN_ENCRYPTION_KEY (which silently downgrades OAuth token
// storage to plaintext). The latter two take plain config inputs rather than a
// Session so the command layer appends their Results to the same Report.
//
// The Session interface keeps the check functions decoupled from
// *discordgo.Session so unit tests can drive them with a hand-rolled fake.
// The From helper wraps a live *discordgo.Session for production callers.
package discordcheck

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Result is one row in a startup self-check Report. OK reports success;
// Detail carries either a human-readable description of the successful
// finding (e.g. the bot username) or the error message for a failure.
type Result struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// Report is the aggregate output of Run. RanAt records when the checks were
// executed so the dashboard can show "last verified at" alongside the badge.
type Report struct {
	Results []Result  `json:"results"`
	RanAt   time.Time `json:"ran_at"`
}

// AllOK returns true when every Result in the Report has OK == true. An
// empty Report is considered passing — nothing has failed.
func (r Report) AllOK() bool {
	for _, res := range r.Results {
		if !res.OK {
			return false
		}
	}
	return true
}

// Session is the narrow Discord API surface needed by the startup checks.
// It mirrors the three *discordgo.Session methods we call so tests can
// substitute a hand-rolled fake without depending on the live gateway.
type Session interface {
	User(userID string) (*discordgo.User, error)
	Application(appID string) (*discordgo.Application, error)
	Guild(guildID string) (*discordgo.Guild, error)
}

// discordgoAdapter wraps a *discordgo.Session so it satisfies the Session
// interface. The real discordgo methods accept variadic RequestOption
// arguments; we drop them since startup checks never need request-level
// overrides.
type discordgoAdapter struct {
	s *discordgo.Session
}

func (a *discordgoAdapter) User(userID string) (*discordgo.User, error) {
	return a.s.User(userID)
}

func (a *discordgoAdapter) Application(appID string) (*discordgo.Application, error) {
	return a.s.Application(appID)
}

func (a *discordgoAdapter) Guild(guildID string) (*discordgo.Guild, error) {
	return a.s.Guild(guildID)
}

// From wraps a live *discordgo.Session in a Session adapter so callers can
// pass the raw gateway session into Run without defining their own adapter.
func From(s *discordgo.Session) Session {
	return &discordgoAdapter{s: s}
}

// Run executes every Discord startup check sequentially. Each check is
// independent: a failure in one does not abort the rest, and each check
// always appends a Result so the Report shape is stable for the dashboard.
//
// The ctx argument is accepted for forward compatibility (future checks may
// need per-call deadlines via discordgo RequestOptions) but is currently
// unused — the *discordgo.Session honours its own client timeout.
func Run(ctx context.Context, sess Session, expectedAppID string, guildIDs []string) Report {
	_ = ctx
	report := Report{RanAt: time.Now().UTC()}

	identity, identityResult := runTokenIdentity(sess)
	report.Results = append(report.Results, identityResult)

	report.Results = append(report.Results, runAppIDMatch(sess, expectedAppID, identity))

	report.Results = append(report.Results, runServerMembersIntent(sess))

	for _, gid := range guildIDs {
		report.Results = append(report.Results, runGuildMembership(sess, gid))
	}

	return report
}

// runTokenIdentity calls User("@me") to confirm the bot token is accepted.
// The returned *discordgo.User (when successful) feeds into the app-id
// match check so we only need one round-trip to learn the bot ID.
func runTokenIdentity(sess Session) (*discordgo.User, Result) {
	user, err := sess.User("@me")
	if err != nil {
		return nil, Result{
			Name:   "token-identity",
			OK:     false,
			Detail: fmt.Sprintf("token rejected by Discord: %v", err),
		}
	}
	return user, Result{
		Name:   "token-identity",
		OK:     true,
		Detail: fmt.Sprintf("bot %s (id=%s)", user.Username, user.ID),
	}
}

// runAppIDMatch compares the configured DISCORD_APPLICATION_ID against the
// actual bot application. An empty env var is a failure, not a skip: Run is
// only invoked when a bot token is configured (see cmd/dndnd/main.go, behind
// `if rawDG != nil`), and without DISCORD_APPLICATION_ID per-guild slash-command
// registration and permission validation silently no-op — the bot comes online
// with a green banner but no usable commands (finding 6·c).
func runAppIDMatch(sess Session, expectedAppID string, identity *discordgo.User) Result {
	if expectedAppID == "" {
		return Result{
			Name:   "application-id-match",
			OK:     false,
			Detail: "DISCORD_APPLICATION_ID not set — slash-command registration and permission checks will silently no-op; set it to the bot's application id",
		}
	}

	app, err := sess.Application("@me")
	if err != nil {
		return Result{
			Name:   "application-id-match",
			OK:     false,
			Detail: fmt.Sprintf("application lookup failed: %v", err),
		}
	}

	actualID := app.ID
	if actualID == "" && identity != nil {
		actualID = identity.ID
	}

	if actualID != expectedAppID {
		return Result{
			Name:   "application-id-match",
			OK:     false,
			Detail: fmt.Sprintf("DISCORD_APPLICATION_ID mismatch: env=%s, actual=%s", expectedAppID, actualID),
		}
	}

	return Result{
		Name:   "application-id-match",
		OK:     true,
		Detail: fmt.Sprintf("application id matches (id=%s)", actualID),
	}
}

// Discord application flags that reflect whether the privileged Server Members
// (GuildMembers) gateway intent is enabled in the developer portal. discordgo
// v0.29.0 exposes Application.Flags as a raw int with no named constants, so the
// two relevant bits are defined here. GATEWAY_GUILD_MEMBERS is set once the app
// is verified; GATEWAY_GUILD_MEMBERS_LIMITED is set for unverified apps (under
// 100 guilds — the self-host case). Either bit means the toggle is ON.
const (
	appFlagGatewayGuildMembers        = 1 << 14
	appFlagGatewayGuildMembersLimited = 1 << 15
)

// runServerMembersIntent verifies the privileged Server Members (GuildMembers)
// gateway intent is enabled for the bot's application. docs/setup.html step 2
// tells operators to flip this toggle; without it the welcome-DM member-join
// handler never fires and the gateway eventually drops with an opaque close
// 4014. Reading Application.Flags surfaces the misconfiguration at boot with an
// actionable message instead of leaving it to manifest as that close code
// later (finding 6·d / T07).
func runServerMembersIntent(sess Session) Result {
	const name = "server-members-intent"
	app, err := sess.Application("@me")
	if err != nil {
		return Result{
			Name:   name,
			OK:     false,
			Detail: fmt.Sprintf("application lookup failed: %v", err),
		}
	}
	if app.Flags&(appFlagGatewayGuildMembers|appFlagGatewayGuildMembersLimited) == 0 {
		return Result{
			Name:   name,
			OK:     false,
			Detail: "Server Members Intent is disabled — welcome DMs and member-join handling will not fire and the gateway will eventually close 4014; enable it in the Discord Developer Portal (Bot → Privileged Gateway Intents → Server Members Intent)",
		}
	}
	return Result{
		Name:   name,
		OK:     true,
		Detail: "Server Members Intent enabled",
	}
}

// runGuildMembership verifies the bot is currently a member of guildID by
// fetching the guild metadata. A failure typically means the bot was kicked
// from that guild or the campaign's guild_id was mistyped at /setup time.
func runGuildMembership(sess Session, guildID string) Result {
	name := "guild-membership-" + guildID
	guild, err := sess.Guild(guildID)
	if err != nil {
		return Result{
			Name:   name,
			OK:     false,
			Detail: fmt.Sprintf("bot is not a member of guild %s: %v", guildID, err),
		}
	}
	return Result{
		Name:   name,
		OK:     true,
		Detail: fmt.Sprintf("guild %s", guild.Name),
	}
}

// ChannelBindingLookup reports whether a guild has channel bindings persisted
// by /setup. It returns false when no campaign settings exist for the guild or
// the persisted channel map is empty — the signal that /setup never ran (or the
// channels were cleared), in which case combat-log / combat-map / turn posts
// and dm-queue notices silently no-op.
type ChannelBindingLookup func(guildID string) bool

// RunChannelBindings returns one Result per guild, failing with a "run /setup"
// hint when a guild has no persisted channel bindings. It is exposed
// separately from Run (rather than folded into it) so the DB-backed lookup
// stays in the command layer and Run keeps its Discord-API-only dependency
// surface. A nil lookup yields no Results so callers that cannot build one skip
// the check rather than emit misleading failures.
func RunChannelBindings(guildIDs []string, lookup ChannelBindingLookup) []Result {
	if lookup == nil {
		return nil
	}
	results := make([]Result, 0, len(guildIDs))
	for _, gid := range guildIDs {
		results = append(results, runChannelBinding(gid, lookup))
	}
	return results
}

// runChannelBinding produces the channel-binding Result for a single guild.
func runChannelBinding(guildID string, lookup ChannelBindingLookup) Result {
	name := "channel-bindings-" + guildID
	if !lookup(guildID) {
		return Result{
			Name:   name,
			OK:     false,
			Detail: fmt.Sprintf("no channel bindings for guild %s — run /setup in this server", guildID),
		}
	}
	return Result{
		Name:   name,
		OK:     true,
		Detail: fmt.Sprintf("channels bound for guild %s", guildID),
	}
}

// tokenEncryptionKeyBytes is the AES-256 key length auth.NewTokenEncryptor
// requires. Mirrored here (rather than importing internal/auth) to keep this
// package free of crypto dependencies; the two must stay in sync.
const tokenEncryptionKeyBytes = 32

// RunTokenEncryption validates the TOKEN_ENCRYPTION_KEY config so a misconfig
// surfaces on the dashboard banner instead of silently downgrading OAuth tokens
// to plaintext-at-rest (finding 6·e / T08). An empty key is OK — plaintext
// storage is the deliberate local-dev default. A non-empty key of the wrong
// length is a failure: auth.NewTokenEncryptor rejects it and buildAuth then
// falls back to storing tokens unencrypted. It takes the raw key string (not a
// Session) so callers append its Result to the report alongside RunChannelBindings.
func RunTokenEncryption(key string) Result {
	const name = "token-encryption-key"
	if key == "" {
		return Result{
			Name:   name,
			OK:     true,
			Detail: "TOKEN_ENCRYPTION_KEY not set — OAuth tokens stored unencrypted at rest; set a 32-byte key to enable encryption",
		}
	}
	if len(key) != tokenEncryptionKeyBytes {
		return Result{
			Name: name,
			OK:   false,
			Detail: fmt.Sprintf(
				"TOKEN_ENCRYPTION_KEY must be %d bytes, got %d — OAuth tokens fall back to plaintext-at-rest; note `openssl rand -hex 32` yields 64 chars, use a 32-byte key such as `openssl rand -hex 16`",
				tokenEncryptionKeyBytes, len(key),
			),
		}
	}
	return Result{
		Name:   name,
		OK:     true,
		Detail: "TOKEN_ENCRYPTION_KEY valid (32 bytes)",
	}
}
