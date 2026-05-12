// Package logging centralises the structured-log contextual fields the
// spec calls out (user_id, guild_id, encounter_id, command, duration_ms)
// so handlers no longer attach them ad hoc. Callers stash field values on
// the context via the Setters, then derive an enriched *slog.Logger with
// WithContext.
package logging

import (
	"context"
	"log/slog"
	"time"
)

// ctxKey is unexported so foreign packages can't read or shadow these keys.
type ctxKey int

const (
	userIDKey ctxKey = iota
	guildIDKey
	encounterIDKey
	commandKey
)

// WithUserID stashes the Discord user ID on ctx.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// WithGuildID stashes the Discord guild ID on ctx.
func WithGuildID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, guildIDKey, id)
}

// WithEncounterID stashes the encounter UUID (string form) on ctx.
func WithEncounterID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, encounterIDKey, id)
}

// WithCommand stashes the slash-command name (e.g. "move") on ctx.
func WithCommand(ctx context.Context, cmd string) context.Context {
	return context.WithValue(ctx, commandKey, cmd)
}

// WithContext returns base enriched with whichever of the standard fields
// are present on ctx. Missing fields are simply omitted so log lines stay
// terse on the early-return paths that haven't populated everything yet.
func WithContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	attrs := make([]any, 0, 8)
	if v, ok := ctx.Value(userIDKey).(string); ok && v != "" {
		attrs = append(attrs, "user_id", v)
	}
	if v, ok := ctx.Value(guildIDKey).(string); ok && v != "" {
		attrs = append(attrs, "guild_id", v)
	}
	if v, ok := ctx.Value(encounterIDKey).(string); ok && v != "" {
		attrs = append(attrs, "encounter_id", v)
	}
	if v, ok := ctx.Value(commandKey).(string); ok && v != "" {
		attrs = append(attrs, "command", v)
	}
	if len(attrs) == 0 {
		return base
	}
	return base.With(attrs...)
}

// WithDuration returns logger with a duration_ms attribute computed from
// time.Since(start).Milliseconds(). Pair with a deferred call at the top
// of a handler to record per-command latency without bespoke math.
func WithDuration(logger *slog.Logger, start time.Time) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return logger.With("duration_ms", time.Since(start).Milliseconds())
}
