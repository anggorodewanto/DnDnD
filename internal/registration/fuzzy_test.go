package registration

import (
	"testing"
)

func TestFindFuzzyMatches(t *testing.T) {
	candidates := []string{"Thorn", "Thorin", "Thora", "Gandalf", "Aragorn"}

	t.Run("returns up to 3 close matches", func(t *testing.T) {
		matches := FindFuzzyMatches("Thron", candidates, 3)
		if len(matches) != 3 {
			t.Fatalf("expected 3 matches, got %d: %+v", len(matches), matches)
		}
		// All three Th* names should be suggested
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		for _, want := range []string{"Thorn", "Thorin", "Thora"} {
			found := false
			for _, n := range names {
				if n == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %q in matches, got %v", want, names)
			}
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		matches := FindFuzzyMatches("THORN", candidates, 3)
		// "THORN" is exact match (case-insensitive), so it should be excluded
		for _, m := range matches {
			if m.Name == "Thorn" {
				t.Error("exact match (case-insensitive) should be excluded from fuzzy results")
			}
		}
	})

	t.Run("no close matches returns empty", func(t *testing.T) {
		matches := FindFuzzyMatches("Xyzzy", candidates, 3)
		if len(matches) != 0 {
			t.Errorf("expected 0 matches for distant name, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("sorted by distance", func(t *testing.T) {
		matches := FindFuzzyMatches("Thron", candidates, 3)
		for i := 1; i < len(matches); i++ {
			if matches[i].Distance < matches[i-1].Distance {
				t.Errorf("matches not sorted by distance: %+v", matches)
			}
		}
	})
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"Thorn", "Thron", 2},
		{"ABC", "abc", 3}, // case-sensitive
	}
	for _, tt := range tests {
		got := levenshteinDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
