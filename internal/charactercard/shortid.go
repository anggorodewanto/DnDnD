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

	// For single-word names with 2+ chars, use first two letters;
	// otherwise use the first letter of each word.
	var base string
	if len(words) == 1 && len(words[0]) >= 2 {
		base = words[0][:2]
	} else {
		var b strings.Builder
		for _, w := range words {
			b.WriteByte(w[0])
		}
		base = b.String()
	}

	candidate := strings.ToUpper(base)

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
