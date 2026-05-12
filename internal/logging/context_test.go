package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/ab/dndnd/internal/logging"
)

func newJSONLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("decode JSON log: %v\nraw=%q", err, buf.String())
	}
	return m
}

func TestSettersStashValuesOnContext(t *testing.T) {
	ctx := context.Background()
	ctx = logging.WithUserID(ctx, "user-42")
	ctx = logging.WithGuildID(ctx, "guild-7")
	ctx = logging.WithEncounterID(ctx, "enc-99")
	ctx = logging.WithCommand(ctx, "move")

	buf := &bytes.Buffer{}
	logger := logging.WithContext(ctx, newJSONLogger(buf))
	logger.Info("hello")

	m := decode(t, buf)
	if m["user_id"] != "user-42" {
		t.Errorf("user_id = %v, want user-42", m["user_id"])
	}
	if m["guild_id"] != "guild-7" {
		t.Errorf("guild_id = %v, want guild-7", m["guild_id"])
	}
	if m["encounter_id"] != "enc-99" {
		t.Errorf("encounter_id = %v, want enc-99", m["encounter_id"])
	}
	if m["command"] != "move" {
		t.Errorf("command = %v, want move", m["command"])
	}
}

func TestWithContextOmitsMissingFields(t *testing.T) {
	ctx := logging.WithCommand(context.Background(), "attack")

	buf := &bytes.Buffer{}
	logger := logging.WithContext(ctx, newJSONLogger(buf))
	logger.Info("partial")

	m := decode(t, buf)
	if m["command"] != "attack" {
		t.Errorf("command = %v, want attack", m["command"])
	}
	for _, k := range []string{"user_id", "guild_id", "encounter_id"} {
		if _, ok := m[k]; ok {
			t.Errorf("expected %q to be omitted, got %v", k, m[k])
		}
	}
}

func TestWithContextEmptyContextOmitsAllFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.WithContext(context.Background(), newJSONLogger(buf))
	logger.Info("bare")

	m := decode(t, buf)
	for _, k := range []string{"user_id", "guild_id", "encounter_id", "command"} {
		if _, ok := m[k]; ok {
			t.Errorf("expected %q to be omitted on empty ctx, got %v", k, m[k])
		}
	}
}

func TestNilLoggersFallBackToDefault(t *testing.T) {
	// Both helpers must accept a nil base without panicking. The output
	// goes to slog.Default(), which we don't capture here — we only assert
	// no panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	logging.WithContext(logging.WithCommand(context.Background(), "x"), nil).Info("ok")
	logging.WithDuration(nil, time.Now()).Info("ok")
}

func TestWithDurationWritesPositiveInteger(t *testing.T) {
	buf := &bytes.Buffer{}
	base := newJSONLogger(buf)
	start := time.Now().Add(-25 * time.Millisecond)

	logger := logging.WithDuration(base, start)
	logger.Info("done")

	m := decode(t, buf)
	raw, ok := m["duration_ms"]
	if !ok {
		t.Fatalf("duration_ms missing, raw=%q", buf.String())
	}
	// JSON numbers decode to float64.
	f, ok := raw.(float64)
	if !ok {
		t.Fatalf("duration_ms = %v (%T), want number", raw, raw)
	}
	if f < 1 {
		t.Errorf("duration_ms = %v, want >= 1", f)
	}
}
