package discord

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	// MaxMessageLen is Discord's maximum message length.
	MaxMessageLen = 2000
	// MaxSplitLen is the maximum total length before falling back to file attachment.
	MaxSplitLen = 6000
)

// NeedsFileAttachment returns true if content exceeds the split limit
// and must be sent as a .txt file attachment.
func NeedsFileAttachment(content string) bool {
	return len(content) > MaxSplitLen
}

// SendContent sends content to a channel, handling message splitting and file attachment.
// Short messages (<=2000) are sent directly. Mid-range (2001-6000) are split at newlines.
// Large messages (>6000) are uploaded as a .txt file with a summary line.
func SendContent(s Session, channelID, content string) error {
	if NeedsFileAttachment(content) {
		return sendAsFile(s, channelID, content)
	}

	parts := SplitMessage(content)
	for _, part := range parts {
		if _, err := s.ChannelMessageSend(channelID, part); err != nil {
			return err
		}
	}
	return nil
}

func sendAsFile(s Session, channelID, content string) error {
	summary := "See details below."
	reader := strings.NewReader(content)
	_, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: summary,
		Files: []*discordgo.File{
			{
				Name:   "details.txt",
				Reader: reader,
			},
		},
	})
	return err
}

// SplitMessage splits content into chunks that fit within Discord's message size limits.
// Content <= 2000 chars returns a single chunk.
// Content 2001-6000 chars returns up to 3 chunks split at newline boundaries.
// Content > 6000 chars returns nil (caller should use file attachment).
func SplitMessage(content string) []string {
	if len(content) <= MaxMessageLen {
		return []string{content}
	}
	if len(content) > MaxSplitLen {
		return nil
	}

	parts := splitAtNewlines(content)
	if parts != nil {
		return parts
	}

	// Newline splitting didn't fit in 3 parts; fall back to hard-cuts.
	return splitHardCut(content)
}

// splitAtNewlines tries to split content into at most 3 parts at newline boundaries.
// Returns nil if the content doesn't fit into 3 parts this way.
func splitAtNewlines(content string) []string {
	var parts []string
	remaining := content
	for len(remaining) > 0 && len(parts) < 3 {
		if len(remaining) <= MaxMessageLen {
			parts = append(parts, remaining)
			remaining = ""
			break
		}

		chunk := remaining[:MaxMessageLen]
		idx := strings.LastIndex(chunk, "\n")
		if idx == -1 {
			// No newline found; can't split at newline boundary here.
			return nil
		}

		parts = append(parts, remaining[:idx])
		remaining = remaining[idx+1:]
	}

	if len(remaining) > 0 {
		// Didn't fit in 3 parts via newline splitting.
		return nil
	}
	return parts
}

// splitHardCut splits content into up to 3 parts using hard-cuts at MaxMessageLen.
// The last part may be up to MaxMessageLen chars.
func splitHardCut(content string) []string {
	var parts []string
	for len(content) > 0 {
		if len(content) <= MaxMessageLen {
			parts = append(parts, content)
			break
		}
		parts = append(parts, content[:MaxMessageLen])
		content = content[MaxMessageLen:]
	}
	return parts
}
