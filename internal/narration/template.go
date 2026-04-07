package narration

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

// placeholderRE matches `{name}` tokens where `name` starts with a letter or
// underscore and continues with letters, digits, or underscores. Whitespace
// inside the braces is not allowed.
var placeholderRE = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// Template is a reusable narration body with optional placeholder tokens.
type Template struct {
	ID         uuid.UUID `json:"id"`
	CampaignID uuid.UUID `json:"campaign_id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ExtractPlaceholders returns the unique placeholder names appearing in body
// in the order of first occurrence.
func ExtractPlaceholders(body string) []string {
	matches := placeholderRE.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// SubstitutePlaceholders replaces every `{name}` token whose name appears in
// values with the corresponding string. Tokens missing from the map are left
// untouched so the caller can detect omissions.
func SubstitutePlaceholders(body string, values map[string]string) string {
	if len(values) == 0 {
		return body
	}
	return placeholderRE.ReplaceAllStringFunc(body, func(match string) string {
		name := match[1 : len(match)-1]
		v, ok := values[name]
		if !ok {
			return match
		}
		return v
	})
}

