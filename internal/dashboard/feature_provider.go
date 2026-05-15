package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

// RefDataFeatureProvider implements FeatureProvider by reading class/race data
// from refdata.Queries. It loads features at construction time so the maps are
// available synchronously via the interface methods.
type RefDataFeatureProvider struct {
	classFeatures    map[string]map[string][]character.Feature
	subclassFeatures map[string]map[string]map[string][]character.Feature
	races            map[string][]character.Feature
}

// NewRefDataFeatureProvider builds a FeatureProvider from the database.
// Errors are logged and degraded to empty maps so character creation still works.
func NewRefDataFeatureProvider(ctx context.Context, queries *refdata.Queries, logger *slog.Logger) *RefDataFeatureProvider {
	fp := &RefDataFeatureProvider{
		classFeatures:    make(map[string]map[string][]character.Feature),
		subclassFeatures: make(map[string]map[string]map[string][]character.Feature),
		races:            make(map[string][]character.Feature),
	}

	classes, err := queries.ListClasses(ctx)
	if err != nil {
		if logger != nil {
			logger.Warn("feature provider: failed to list classes", "error", err)
		}
		return fp
	}
	for _, cls := range classes {
		fp.classFeatures[cls.ID] = parseFeaturesByLevel(cls.FeaturesByLevel)
	}

	races, err := queries.ListRaces(ctx)
	if err != nil {
		if logger != nil {
			logger.Warn("feature provider: failed to list races", "error", err)
		}
		return fp
	}
	for _, r := range races {
		fp.races[r.ID] = parseTraits(r.Traits)
	}

	return fp
}

func (fp *RefDataFeatureProvider) ClassFeatures() map[string]map[string][]character.Feature {
	return fp.classFeatures
}

func (fp *RefDataFeatureProvider) SubclassFeatures() map[string]map[string]map[string][]character.Feature {
	return fp.subclassFeatures
}

func (fp *RefDataFeatureProvider) RacialTraits(race string) []character.Feature {
	return fp.races[strings.ToLower(race)]
}

// parseFeaturesByLevel decodes the JSONB features_by_level column into the
// map[level_string][]Feature shape the FeatureProvider interface expects.
func parseFeaturesByLevel(raw json.RawMessage) map[string][]character.Feature {
	if len(raw) == 0 {
		return nil
	}
	var byLevel map[string][]character.Feature
	if err := json.Unmarshal(raw, &byLevel); err != nil {
		return nil
	}
	return byLevel
}

// parseTraits decodes the JSONB traits column into a flat []Feature slice.
func parseTraits(raw json.RawMessage) []character.Feature {
	if len(raw) == 0 {
		return nil
	}
	var traits []character.Feature
	if err := json.Unmarshal(raw, &traits); err != nil {
		return nil
	}
	return traits
}
