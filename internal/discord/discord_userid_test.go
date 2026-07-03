package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// discordUserID must resolve the acting user in BOTH contexts:
//   - guild interactions carry the user under interaction.Member.User
//   - DM interactions (e.g. the ASI prompt buttons) carry it under
//     interaction.User, with interaction.Member == nil
//
// A regression here made every DM button click resolve to "" and fail the
// ownership check with "This is not your character."
func TestDiscordUserID(t *testing.T) {
	tests := []struct {
		name        string
		interaction *discordgo.Interaction
		want        string
	}{
		{
			name: "guild interaction uses Member.User",
			interaction: &discordgo.Interaction{
				Member: &discordgo.Member{User: &discordgo.User{ID: "guild-user"}},
			},
			want: "guild-user",
		},
		{
			name: "DM interaction uses User when Member is nil",
			interaction: &discordgo.Interaction{
				User: &discordgo.User{ID: "dm-user"},
			},
			want: "dm-user",
		},
		{
			name: "guild Member takes precedence over User",
			interaction: &discordgo.Interaction{
				Member: &discordgo.Member{User: &discordgo.User{ID: "guild-user"}},
				User:   &discordgo.User{ID: "other"},
			},
			want: "guild-user",
		},
		{
			name:        "no user present returns empty",
			interaction: &discordgo.Interaction{},
			want:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := discordUserID(tc.interaction); got != tc.want {
				t.Fatalf("discordUserID() = %q, want %q", got, tc.want)
			}
		})
	}
}
