package ddbimport

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var ddbCharacterURLPattern = regexp.MustCompile(`^(\d+)`)

// ParseCharacterURL extracts the numeric character ID from a D&D Beyond character URL.
func ParseCharacterURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.TrimPrefix(parsed.Hostname(), "www.")
	if host != "dndbeyond.com" {
		return "", fmt.Errorf("not a D&D Beyond URL: %s", host)
	}

	// Path should be /characters/{id} or /characters/{id}/slug
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "characters" {
		return "", fmt.Errorf("URL path does not contain /characters/{id}")
	}

	id := parts[1]
	if !ddbCharacterURLPattern.MatchString(id) {
		return "", fmt.Errorf("character ID is not numeric: %s", id)
	}

	return id, nil
}
