// Package narration implements the DM Dashboard Narrate Panel (Phase 100a).
//
// It exposes a rich narration composer for posting to the Discord `#the-story`
// channel: Discord-flavored markdown rendering (bold, italic, blockquotes, and
// a custom "read-aloud" fenced block), image attachments stored in the Asset
// Library, a preview endpoint, and a post history log.
package narration

import "strings"

// ReadAloudColor is the left-border accent color used for read-aloud embeds
// (parchment gold) so that "boxed text" is visually distinct in Discord.
const ReadAloudColor = 0xD4AF37

// readAloudFenceOpen and readAloudFenceClose delimit a read-aloud block.
// Syntax:
//
//	:::read-aloud
//	(boxed text)
//	:::
const (
	readAloudFenceOpen  = ":::read-aloud"
	readAloudFenceClose = ":::"
)

// DiscordEmbed is a minimal representation of a Discord embed used by the
// preview + poster. Description holds the read-aloud boxed text; Color sets
// the left-border accent.
type DiscordEmbed struct {
	Description string `json:"description"`
	Color       int    `json:"color"`
}

// DiscordMessage is the rendered output ready to send to Discord: the main
// body text (which may contain Discord markdown) plus any extracted embeds.
type DiscordMessage struct {
	Body   string         `json:"body"`
	Embeds []DiscordEmbed `json:"embeds"`
}

// RenderDiscord converts editor source into the exact payload that will be
// sent to Discord. Discord markdown (bold/italic/blockquote) is passed through
// untouched; read-aloud fenced blocks are extracted into embeds.
//
// The body preserves the relative ordering of non-block text with the read-
// aloud sections removed. Unclosed fences are treated as literal text so that
// a user typing a block mid-edit still renders legibly.
func RenderDiscord(src string) DiscordMessage {
	lines := strings.Split(src, "\n")
	var bodyLines []string
	embeds := []DiscordEmbed{}

	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) != readAloudFenceOpen {
			bodyLines = append(bodyLines, lines[i])
			i++
			continue
		}

		block, next, ok := collectReadAloud(lines, i+1)
		if !ok {
			// Unclosed fence: emit the opener literally and continue.
			bodyLines = append(bodyLines, lines[i])
			i++
			continue
		}
		embeds = append(embeds, DiscordEmbed{
			Description: block,
			Color:       ReadAloudColor,
		})
		i = next
	}

	return DiscordMessage{
		Body:   collapseBlankLines(strings.Join(bodyLines, "\n")),
		Embeds: embeds,
	}
}

// collectReadAloud scans lines starting at start for the closing `:::` fence.
// Returns the joined block content, the index of the line after the closer,
// and ok=true on a successful match.
func collectReadAloud(lines []string, start int) (string, int, bool) {
	var block []string
	for j := start; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == readAloudFenceClose {
			return strings.Join(block, "\n"), j + 1, true
		}
		block = append(block, lines[j])
	}
	return "", start, false
}

// collapseBlankLines replaces runs of 2+ blank lines with a single blank line
// and trims leading/trailing whitespace. This keeps the body tidy when read-
// aloud blocks are removed from the middle of a narration.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
			continue
		}
		blank = 0
		out = append(out, line)
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}
