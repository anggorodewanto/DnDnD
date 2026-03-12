package charactercard

import (
	"fmt"
	"strings"
)

// ShortID derives a short identifier from a character name using initials.
// If the result collides with an existing ID, a numeric suffix is appended.
func ShortID(name string, existing []string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "X"
	}

	words := strings.Fields(name)
	var base strings.Builder
	for _, w := range words {
		if len(w) > 0 {
			base.WriteByte(w[0])
		}
	}

	// Single-character name: just use that letter
	if base.Len() == 0 {
		return "X"
	}

	// For single-word names with 2+ chars, use first two letters
	if len(words) == 1 && len(words[0]) >= 2 {
		base.Reset()
		base.WriteByte(words[0][0])
		base.WriteByte(words[0][1])
	}

	candidate := strings.ToUpper(base.String())

	existSet := make(map[string]bool, len(existing))
	for _, e := range existing {
		existSet[e] = true
	}

	if !existSet[candidate] {
		return candidate
	}

	for i := 2; ; i++ {
		suffixed := fmt.Sprintf("%s%d", candidate, i)
		if !existSet[suffixed] {
			return suffixed
		}
	}
}
