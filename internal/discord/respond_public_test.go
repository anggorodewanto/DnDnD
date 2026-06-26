package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// respondPublic must send a non-ephemeral interaction response so the whole
// party sees combat action results inline in the invoking channel.
func TestRespondPublic_NotEphemeral(t *testing.T) {
	var got *discordgo.InteractionResponse
	sess := &MockSession{
		InteractionRespondFunc: func(_ *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
			got = resp
			return nil
		},
	}

	respondPublic(sess, &discordgo.Interaction{}, "Aria attacks the goblin for 7 damage.")

	if got == nil {
		t.Fatal("expected an interaction response, got none")
	}
	if got.Type != discordgo.InteractionResponseChannelMessageWithSource {
		t.Errorf("expected ChannelMessageWithSource, got %v", got.Type)
	}
	if got.Data.Content != "Aria attacks the goblin for 7 damage." {
		t.Errorf("unexpected content: %q", got.Data.Content)
	}
	if got.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Errorf("response must NOT be ephemeral, got flags %d", got.Data.Flags)
	}
}
