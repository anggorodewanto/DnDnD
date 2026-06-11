package main

import (
	"errors"
	"strings"
	"testing"
)

func TestDiscordCloseHint(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantSub string // substring expected in the hint; "" means empty hint
	}{
		{"nil error", nil, ""},
		{"4004 auth failed", errors.New("websocket: close 4004 (Authentication failed)"), "DISCORD_BOT_TOKEN"},
		{"4014 disallowed intent", errors.New("websocket: close 4014 (Disallowed intent(s))"), "Server Members Intent"},
		{"unrelated error", errors.New("dial tcp: connection refused"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discordCloseHint(tt.err)
			if tt.wantSub == "" {
				if got != "" {
					t.Fatalf("discordCloseHint(%v) = %q, want empty", tt.err, got)
				}
				return
			}
			if !strings.Contains(got, tt.wantSub) {
				t.Fatalf("discordCloseHint(%v) = %q, want substring %q", tt.err, got, tt.wantSub)
			}
		})
	}
}
