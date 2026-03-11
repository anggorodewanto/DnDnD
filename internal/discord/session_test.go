package discord

import "testing"

func TestMockSession_ImplementsSession(t *testing.T) {
	var _ Session = &MockSession{}
}
