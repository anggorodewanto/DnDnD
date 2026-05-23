// Package discordcheck performs startup self-checks against the Discord API
// so misconfigured deploys fail loudly at boot instead of at the first user
// interaction. It verifies the bot token is accepted, the configured
// DISCORD_APPLICATION_ID matches the actual bot identity, and that every
// guild_id stored in the campaigns table belongs to a guild the bot is
// currently a member of.
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
// actual bot application. The check is skipped (recorded as OK) when the
// env var is empty so optional deploys can omit it.
func runAppIDMatch(sess Session, expectedAppID string, identity *discordgo.User) Result {
	if expectedAppID == "" {
		return Result{
			Name:   "application-id-match",
			OK:     true,
			Detail: "skipped (env not set)",
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
