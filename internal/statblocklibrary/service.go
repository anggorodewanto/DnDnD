// Package statblocklibrary implements the DM Dashboard Stat Block Library panel.
// It exposes a searchable, filterable, paginated view over the creatures table
// (both SRD and homebrew), enforcing campaign scoping so that homebrew entries
// are only visible to their owning campaign.
package statblocklibrary

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// ErrNotFound is returned when a stat block does not exist or is not visible
// to the requesting campaign.
var ErrNotFound = errors.New("stat block not found")

// Source is the source filter for ListStatBlocks.
type Source string

const (
	// SourceAny returns both SRD and (campaign-scoped) homebrew entries. Default.
	SourceAny Source = ""
	// SourceSRD returns only SRD entries (campaign_id IS NULL).
	SourceSRD Source = "srd"
	// SourceHomebrew returns only homebrew entries belonging to the filter's campaign.
	SourceHomebrew Source = "homebrew"
)

// StatBlockFilter is the set of optional filters the service accepts.
// All fields are optional; zero values mean "no filter".
type StatBlockFilter struct {
	Search     string    // case-insensitive substring match on name
	Types      []string  // creature types (beast, undead, ...)
	Sizes      []string  // sizes (Tiny, Small, ...)
	CRMin      *float64  // inclusive minimum CR
	CRMax      *float64  // inclusive maximum CR
	Source     Source    // SRD, homebrew, or any
	CampaignID uuid.UUID // campaign scoping for homebrew; zero = no campaign (hides all homebrew)
	Limit      int       // 0 = no limit
	Offset     int
}

// Store is the minimal subset of refdata the service needs; defined as an
// interface so tests can swap in an in-memory fake.
type Store interface {
	ListCreatures(ctx context.Context) ([]refdata.Creature, error)
	GetCreature(ctx context.Context, id string) (refdata.Creature, error)
}

// Service is the stat block library service.
type Service struct {
	store Store
}

// NewService constructs a Service with the given store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// ListStatBlocks returns creatures matching the filter, sorted by name,
// with campaign scoping applied for homebrew entries, and pagination applied last.
func (s *Service) ListStatBlocks(ctx context.Context, filter StatBlockFilter) ([]refdata.Creature, error) {
	all, err := s.store.ListCreatures(ctx)
	if err != nil {
		return nil, err
	}

	typeSet := toLowerSet(filter.Types)
	sizeSet := toSet(filter.Sizes)
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))

	filtered := make([]refdata.Creature, 0, len(all))
	for _, c := range all {
		if !matches(c, filter, typeSet, sizeSet, searchLower) {
			continue
		}
		filtered = append(filtered, c)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	return paginate(filtered, filter.Limit, filter.Offset), nil
}

// GetStatBlock fetches a single creature, enforcing campaign scoping for homebrew.
// SRD entries (campaign_id NULL) are always visible. Homebrew entries are only
// visible to their owning campaign.
func (s *Service) GetStatBlock(ctx context.Context, id string, campaignID uuid.UUID) (refdata.Creature, error) {
	c, err := s.store.GetCreature(ctx, id)
	if err != nil {
		return refdata.Creature{}, err
	}
	if !isVisible(c, campaignID) {
		return refdata.Creature{}, ErrNotFound
	}
	return c, nil
}

// --- internal helpers ---

// matches returns true if the creature satisfies all active filters.
func matches(c refdata.Creature, f StatBlockFilter, typeSet map[string]struct{}, sizeSet map[string]struct{}, searchLower string) bool {
	if !matchesVisibility(c, f) {
		return false
	}
	if searchLower != "" && !strings.Contains(strings.ToLower(c.Name), searchLower) {
		return false
	}
	if typeSet != nil {
		if _, ok := typeSet[strings.ToLower(c.Type)]; !ok {
			return false
		}
	}
	if sizeSet != nil {
		if _, ok := sizeSet[c.Size]; !ok {
			return false
		}
	}
	if !matchesCR(c.Cr, f.CRMin, f.CRMax) {
		return false
	}
	return true
}

// matchesVisibility checks both the source filter and campaign scoping for homebrew.
func matchesVisibility(c refdata.Creature, f StatBlockFilter) bool {
	isHomebrew := c.Homebrew.Valid && c.Homebrew.Bool
	if isHomebrew {
		return matchesHomebrew(c, f)
	}
	// SRD entry (campaign_id NULL / homebrew false): excluded only if source=homebrew.
	if f.Source == SourceHomebrew {
		return false
	}
	return true
}

// matchesHomebrew returns true when a homebrew creature is visible per the filter.
func matchesHomebrew(c refdata.Creature, f StatBlockFilter) bool {
	if f.Source == SourceSRD {
		return false
	}
	if f.CampaignID == uuid.Nil {
		return false
	}
	if !c.CampaignID.Valid {
		return false
	}
	return c.CampaignID.UUID == f.CampaignID
}

// matchesCR returns true if cr is inside [min, max] (either bound optional).
func matchesCR(crStr string, min, max *float64) bool {
	if min == nil && max == nil {
		return true
	}
	cr := parseCR(crStr)
	if min != nil && cr < *min {
		return false
	}
	if max != nil && cr > *max {
		return false
	}
	return true
}

// isVisible is the single-entry equivalent of matchesVisibility, used by GetStatBlock.
func isVisible(c refdata.Creature, campaignID uuid.UUID) bool {
	isHomebrew := c.Homebrew.Valid && c.Homebrew.Bool
	if !isHomebrew {
		return true
	}
	if campaignID == uuid.Nil {
		return false
	}
	if !c.CampaignID.Valid {
		return false
	}
	return c.CampaignID.UUID == campaignID
}

// parseCR converts a CR string like "1/4", "1", "17" to a float64.
// Returns 0 for empty or unparseable input.
func parseCR(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if idx := strings.Index(s, "/"); idx >= 0 {
		num, err1 := strconv.ParseFloat(s[:idx], 64)
		den, err2 := strconv.ParseFloat(s[idx+1:], 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0
		}
		return num / den
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// toLowerSet builds a lowercase set from the given values.
func toLowerSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		set[strings.ToLower(t)] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// toSet builds a set from the given values preserving case.
func toSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		set[t] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// paginate applies limit/offset to the given slice.
func paginate(items []refdata.Creature, limit, offset int) []refdata.Creature {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []refdata.Creature{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}
