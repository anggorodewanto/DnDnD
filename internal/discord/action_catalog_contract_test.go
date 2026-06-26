package discord

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
)

// TestActionCatalog_MatchesDiscordDispatch is the SSOT drift guard between the
// canonical action catalog (refdata.ActionCatalog — what the portal character
// sheet advertises) and the /action + /bonus dispatch key sets (what the bot
// actually accepts). It fails CI in two directions:
//
//   - catalog → dispatch: every catalog entry that tells the player to run
//     "/action X" or "/bonus X" must resolve to a real dispatch key, so the
//     sheet never advertises a command the bot rejects.
//   - dispatch → catalog: every player-facing dispatch subcommand must have a
//     catalog entry, so the sheet never silently omits an ability the bot
//     supports. A short, documented allow-list excludes lifecycle/secondary
//     subcommands that are not standalone "abilities".
func TestActionCatalog_MatchesDiscordDispatch(t *testing.T) {
	byKey := refdata.ActionCatalogByKey()

	t.Run("catalog entries map to a real command", func(t *testing.T) {
		bonusKeys := map[string]bool{}
		for _, k := range bonusSubcommandKeys {
			bonusKeys[k] = true
		}
		for _, e := range refdata.ActionCatalog() {
			switch {
			case strings.HasPrefix(e.Command, "/action "):
				// "ready" is accepted by /action via the ready path rather than
				// the dispatch-subcommand router, so allow it explicitly.
				if canonicalActionSubcommand(e.Key) == "" && e.Key != "ready" {
					t.Errorf("catalog %q advertises %q but /action does not accept key %q", e.Key, e.Command, e.Key)
				}
			case strings.HasPrefix(e.Command, "/bonus "):
				if !bonusKeys[e.Key] {
					t.Errorf("catalog %q advertises %q but /bonus does not accept key %q", e.Key, e.Command, e.Key)
				}
			}
		}
	})

	t.Run("every action dispatch key is catalogued", func(t *testing.T) {
		for _, k := range actionSubcommandKeys {
			if _, ok := byKey[k]; !ok {
				t.Errorf("/action subcommand %q has no catalog entry — add it to refdata.ActionCatalog or the portal sheet will omit it", k)
			}
		}
	})

	t.Run("canonical action keys and aliases resolve", func(t *testing.T) {
		cases := map[string]string{
			"dash":            "dash",
			"drop-prone":      "drop-prone",
			"dropprone":       "drop-prone",       // alias
			"action-surge":    "surge",            // alias
			"channeldivinity": "channel-divinity", // alias
			"layonhands":      "lay-on-hands",     // alias
			"flip-the-table":  "",                 // freeform, not a dispatch subcommand
			"":                "",
		}
		for in, want := range cases {
			if got := canonicalActionSubcommand(in); got != want {
				t.Errorf("canonicalActionSubcommand(%q) = %q, want %q", in, got, want)
			}
		}
	})

	t.Run("every player-facing bonus dispatch key is catalogued", func(t *testing.T) {
		// Lifecycle / secondary subcommands that are not standalone abilities
		// shown on the sheet (toggling an effect off, the drag mechanic).
		internalOnly := map[string]bool{
			"end-rage":          true, // ends Rage; Rage itself is catalogued
			"revert-wild-shape": true, // reverts Wild Shape; Wild Shape is catalogued
			"drag":              true, // grapple-drag status helper, not a turn action
			"release-drag":      true, // releases dragged creatures
		}
		for _, k := range bonusSubcommandKeys {
			if internalOnly[k] {
				continue
			}
			if _, ok := byKey[k]; !ok {
				t.Errorf("/bonus subcommand %q has no catalog entry — add it to refdata.ActionCatalog (or to the internalOnly allow-list if it is not a player-facing ability)", k)
			}
		}
	})
}
