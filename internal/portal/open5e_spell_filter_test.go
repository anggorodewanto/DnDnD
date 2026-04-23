package portal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/dndnd/internal/auth"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Phase 111 (iteration 2): the portal /api/spells endpoint must apply the
// campaign's open5e_sources gate so third-party content does not leak
// across campaigns. The tests below lock in that behaviour at the HTTP
// layer using a fake campaign lookup; the adapter-level unit tests live
// alongside the refdata adapter.

type fakeOpen5eCampaignLookup struct {
	slugs map[uuid.UUID][]string
}

func (f *fakeOpen5eCampaignLookup) EnabledOpen5eSources(campaignID uuid.UUID) []string {
	if f == nil {
		return nil
	}
	return f.slugs[campaignID]
}

type fakeSpellQuerier struct {
	spells []refdata.Spell
}

func (f *fakeSpellQuerier) ListRaces(_ context.Context) ([]refdata.Race, error) {
	return nil, nil
}
func (f *fakeSpellQuerier) ListClasses(_ context.Context) ([]refdata.Class, error) {
	return nil, nil
}
func (f *fakeSpellQuerier) ListSpellsByClass(_ context.Context, class string) ([]refdata.Spell, error) {
	var out []refdata.Spell
	for _, s := range f.spells {
		for _, c := range s.Classes {
			if c == class {
				out = append(out, s)
				break
			}
		}
	}
	return out, nil
}
func (f *fakeSpellQuerier) ListWeapons(_ context.Context) ([]refdata.Weapon, error) {
	return nil, nil
}
func (f *fakeSpellQuerier) ListArmor(_ context.Context) ([]refdata.Armor, error) {
	return nil, nil
}

func newSpellForTest(id, name, source string, classes []string) refdata.Spell {
	src := sql.NullString{}
	if source != "" {
		src = sql.NullString{String: source, Valid: true}
	}
	return refdata.Spell{
		ID:          id,
		Name:        name,
		Level:       1,
		School:      "evocation",
		CastingTime: "1 action",
		Duration:    "Instantaneous",
		Classes:     classes,
		Source:      src,
	}
}

// Helper: run the /portal/api/spells handler and return decoded spells.
func runListSpells(t *testing.T, h *portal.APIHandler, rawQuery string) []portal.SpellInfo {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/portal/api/spells?"+rawQuery, nil)
	ctx := auth.ContextWithDiscordUserID(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListSpells(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var out []portal.SpellInfo
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
	return out
}

func newCampaignAwareHandler(t *testing.T, q *fakeSpellQuerier, lookup *fakeOpen5eCampaignLookup) *portal.APIHandler {
	t.Helper()
	adapter := portal.NewRefDataAdapterWithOpen5eLookup(q, lookup)
	return portal.NewAPIHandler(slog.Default(), adapter, nil)
}

func TestListSpells_HidesOpen5eWhenCampaignHasNoEnabledSources(t *testing.T) {
	campID := uuid.New()
	q := &fakeSpellQuerier{
		spells: []refdata.Spell{
			newSpellForTest("fire-bolt", "Fire Bolt", "", []string{"wizard"}),
			newSpellForTest("open5e_shadow", "Shadow", "open5e:tome-of-beasts", []string{"wizard"}),
		},
	}
	lookup := &fakeOpen5eCampaignLookup{slugs: map[uuid.UUID][]string{}}
	h := newCampaignAwareHandler(t, q, lookup)

	spells := runListSpells(t, h, "class=wizard&campaign_id="+campID.String())

	names := spellNames(spells)
	assert.Contains(t, names, "Fire Bolt")
	assert.NotContains(t, names, "Shadow", "open5e spell must be hidden when campaign has no enabled sources")
}

func TestListSpells_ShowsOpen5eSpellWhenItsSlugEnabled(t *testing.T) {
	campID := uuid.New()
	q := &fakeSpellQuerier{
		spells: []refdata.Spell{
			newSpellForTest("fire-bolt", "Fire Bolt", "", []string{"wizard"}),
			newSpellForTest("open5e_shadow", "Shadow", "open5e:tome-of-beasts", []string{"wizard"}),
		},
	}
	lookup := &fakeOpen5eCampaignLookup{
		slugs: map[uuid.UUID][]string{campID: {"tome-of-beasts"}},
	}
	h := newCampaignAwareHandler(t, q, lookup)

	spells := runListSpells(t, h, "class=wizard&campaign_id="+campID.String())

	names := spellNames(spells)
	assert.Contains(t, names, "Fire Bolt")
	assert.Contains(t, names, "Shadow")
}

func TestListSpells_HidesOpen5eSpellWhenDifferentSlugEnabled(t *testing.T) {
	campID := uuid.New()
	q := &fakeSpellQuerier{
		spells: []refdata.Spell{
			newSpellForTest("fire-bolt", "Fire Bolt", "", []string{"wizard"}),
			newSpellForTest("open5e_shadow", "Shadow", "open5e:tome-of-beasts", []string{"wizard"}),
		},
	}
	lookup := &fakeOpen5eCampaignLookup{
		slugs: map[uuid.UUID][]string{campID: {"deep-magic"}},
	}
	h := newCampaignAwareHandler(t, q, lookup)

	spells := runListSpells(t, h, "class=wizard&campaign_id="+campID.String())

	names := spellNames(spells)
	assert.Contains(t, names, "Fire Bolt", "SRD spell must remain visible")
	assert.NotContains(t, names, "Shadow", "open5e spell must be hidden when only a different slug is enabled")
}

func TestListSpells_HomebrewAndSRDUnchangedByFilter(t *testing.T) {
	// Homebrew spells carry a non-open5e source string (e.g. "campaign:xyz")
	// or nil. They must always pass through the filter untouched.
	campID := uuid.New()
	q := &fakeSpellQuerier{
		spells: []refdata.Spell{
			newSpellForTest("fire-bolt", "Fire Bolt", "", []string{"wizard"}),
			newSpellForTest("custom-blast", "Custom Blast", "campaign:xyz", []string{"wizard"}),
		},
	}
	lookup := &fakeOpen5eCampaignLookup{slugs: map[uuid.UUID][]string{campID: {}}}
	h := newCampaignAwareHandler(t, q, lookup)

	spells := runListSpells(t, h, "class=wizard&campaign_id="+campID.String())

	names := spellNames(spells)
	assert.Contains(t, names, "Fire Bolt")
	assert.Contains(t, names, "Custom Blast")
}

func TestListSpells_MissingCampaignID_HidesOpen5eAsSafeDefault(t *testing.T) {
	q := &fakeSpellQuerier{
		spells: []refdata.Spell{
			newSpellForTest("fire-bolt", "Fire Bolt", "", []string{"wizard"}),
			newSpellForTest("open5e_shadow", "Shadow", "open5e:tome-of-beasts", []string{"wizard"}),
		},
	}
	h := newCampaignAwareHandler(t, q, &fakeOpen5eCampaignLookup{})

	spells := runListSpells(t, h, "class=wizard")

	names := spellNames(spells)
	assert.Contains(t, names, "Fire Bolt")
	assert.NotContains(t, names, "Shadow", "without a campaign_id the adapter must default to no Open5e sources")
}

func TestListSpells_UnparseableCampaignID_HidesOpen5eAsSafeDefault(t *testing.T) {
	q := &fakeSpellQuerier{
		spells: []refdata.Spell{
			newSpellForTest("fire-bolt", "Fire Bolt", "", []string{"wizard"}),
			newSpellForTest("open5e_shadow", "Shadow", "open5e:tome-of-beasts", []string{"wizard"}),
		},
	}
	h := newCampaignAwareHandler(t, q, &fakeOpen5eCampaignLookup{})

	spells := runListSpells(t, h, "class=wizard&campaign_id=not-a-uuid")

	names := spellNames(spells)
	assert.Contains(t, names, "Fire Bolt")
	assert.NotContains(t, names, "Shadow", "unparseable campaign_id must fall back to safe default (no Open5e sources)")
}

func spellNames(spells []portal.SpellInfo) []string {
	out := make([]string, len(spells))
	for i, s := range spells {
		out[i] = s.Name
	}
	return out
}
