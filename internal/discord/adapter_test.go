package discord

import "testing"

func TestDiscordgoSession_ImplementsSession(t *testing.T) {
	// Compile-time check that DiscordgoSession implements Session.
	var _ Session = &DiscordgoSession{}
}
