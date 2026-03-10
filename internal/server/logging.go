package server

import (
	"io"
	"log/slog"
)

// NewLogger creates a structured JSON logger writing to w.
// When debug is true, the minimum log level is set to DEBUG;
// otherwise it defaults to INFO.
func NewLogger(w io.Writer, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
}
