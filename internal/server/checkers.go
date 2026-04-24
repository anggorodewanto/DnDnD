package server

import (
	"context"
	"database/sql"
	"time"
)

// NewDBChecker returns a HealthChecker that pings the given *sql.DB with a
// 2-second timeout. nil db is treated as "disconnected" so deploys without
// DATABASE_URL still produce a 503 on /health instead of claiming "ok".
func NewDBChecker(db *sql.DB) HealthChecker {
	return func() (string, bool) {
		if db == nil {
			return "disconnected", false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return "disconnected", false
		}
		return "connected", true
	}
}

// NewDiscordChecker returns a HealthChecker that calls probe to determine
// gateway connectivity. A typical probe is `func() bool { return dg.DataReady }`
// using the raw *discordgo.Session, but any boolean predicate works. A nil
// probe is treated as "disconnected" so deploys without DISCORD_BOT_TOKEN
// surface the degraded state rather than silently claiming "connected".
func NewDiscordChecker(probe func() bool) HealthChecker {
	return func() (string, bool) {
		if probe == nil || !probe() {
			return "disconnected", false
		}
		return "connected", true
	}
}
