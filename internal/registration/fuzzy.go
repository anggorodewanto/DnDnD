package registration

import "strings"

// levenshteinDistance computes the edit distance between two strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			curr[j] = min(ins, min(del, sub))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// FuzzyMatch holds a candidate name and its distance from the query.
type FuzzyMatch struct {
	Name     string
	Distance int
}

// FindFuzzyMatches returns up to maxResults names from candidates that are
// close to query (case-insensitive comparison). Results are sorted by distance.
func FindFuzzyMatches(query string, candidates []string, maxResults int) []FuzzyMatch {
	queryLower := strings.ToLower(query)
	type scored struct {
		name     string
		distance int
	}
	var matches []scored

	maxDist := max(len(queryLower)/2, 3)

	for _, name := range candidates {
		dist := levenshteinDistance(queryLower, strings.ToLower(name))
		if dist == 0 {
			continue // exact match, skip
		}
		if dist <= maxDist {
			matches = append(matches, scored{name: name, distance: dist})
		}
	}

	// Sort by distance, then alphabetically
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].distance < matches[i].distance ||
				(matches[j].distance == matches[i].distance && matches[j].name < matches[i].name) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	result := make([]FuzzyMatch, len(matches))
	for i, m := range matches {
		result[i] = FuzzyMatch{Name: m.name, Distance: m.distance}
	}
	return result
}
