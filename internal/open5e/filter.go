package open5e

import (
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// SourcePrefix is the prefix every Open5e-cached row's source column
// carries: "open5e:<document_slug>".
const SourcePrefix = "open5e:"

// IsOpen5eSource reports whether the given refdata source value was
// written by the Open5e cache (i.e. it starts with "open5e:").
func IsOpen5eSource(source string) bool {
	return strings.HasPrefix(source, SourcePrefix)
}

// DocumentSlug extracts the Open5e document slug from a source attribution
// string, or "" when the source is not from Open5e.
func DocumentSlug(source string) string {
	slug, ok := strings.CutPrefix(source, SourcePrefix)
	if !ok {
		return ""
	}
	return slug
}

// FilterSpellsByOpen5eSources returns the subset of spells visible under
// the given enabled Open5e document slugs. SRD and homebrew rows pass
// through untouched; open5e:* rows are kept only when their slug appears
// in enabled. Intended for layering over raw refdata.ListSpells queries
// in consumers that do not want to re-implement the filter themselves.
func FilterSpellsByOpen5eSources(spells []refdata.Spell, enabled []string) []refdata.Spell {
	allowed := make(map[string]struct{}, len(enabled))
	for _, s := range enabled {
		allowed[s] = struct{}{}
	}
	out := make([]refdata.Spell, 0, len(spells))
	for _, s := range spells {
		if !s.Source.Valid || !IsOpen5eSource(s.Source.String) {
			out = append(out, s)
			continue
		}
		if _, ok := allowed[DocumentSlug(s.Source.String)]; ok {
			out = append(out, s)
		}
	}
	return out
}
