package statblocklibrary

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- fake store ---

type fakeStore struct {
	creatures []refdata.Creature
	listErr   error
	getErr    error
}

func (f *fakeStore) ListCreatures(ctx context.Context) ([]refdata.Creature, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]refdata.Creature, len(f.creatures))
	copy(out, f.creatures)
	return out, nil
}

func (f *fakeStore) GetCreature(ctx context.Context, id string) (refdata.Creature, error) {
	if f.getErr != nil {
		return refdata.Creature{}, f.getErr
	}
	for _, c := range f.creatures {
		if c.ID == id {
			return c, nil
		}
	}
	return refdata.Creature{}, sql.ErrNoRows
}

// --- helpers ---

func mkCreature(id, name, typ, size, cr string) refdata.Creature {
	return refdata.Creature{
		ID:            id,
		Name:          name,
		Type:          typ,
		Size:          size,
		Cr:            cr,
		Ac:            10,
		HpFormula:     "1d8",
		HpAverage:     4,
		Speed:         json.RawMessage(`{"walk":30}`),
		AbilityScores: json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`),
		Attacks:       json.RawMessage(`[]`),
		Homebrew:      sql.NullBool{Bool: false, Valid: true},
		Source:        sql.NullString{String: "SRD", Valid: true},
	}
}

func mkHomebrew(id, name, typ, size, cr string, campaignID uuid.UUID) refdata.Creature {
	c := mkCreature(id, name, typ, size, cr)
	c.Homebrew = sql.NullBool{Bool: true, Valid: true}
	c.CampaignID = uuid.NullUUID{UUID: campaignID, Valid: true}
	c.Source = sql.NullString{String: "homebrew", Valid: true}
	return c
}

// --- TDD cycle 1: ListStatBlocks returns SRD creatures sorted by name ---

func TestService_ListStatBlocks_DefaultReturnsSRDSortedByName(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkCreature("ancient-red-dragon", "Ancient Red Dragon", "dragon", "Gargantuan", "24"),
		mkCreature("bat", "Bat", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)

	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "Ancient Red Dragon", entries[0].Name)
	assert.Equal(t, "Bat", entries[1].Name)
	assert.Equal(t, "Goblin", entries[2].Name)
}

func TestService_ListStatBlocks_StoreErrorPropagates(t *testing.T) {
	store := &fakeStore{listErr: errors.New("boom")}
	svc := NewService(store)
	_, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{})
	require.Error(t, err)
}

// --- TDD cycle 2: free-text search (case-insensitive substring on name) ---

func TestService_ListStatBlocks_SearchCaseInsensitive(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkCreature("hobgoblin", "Hobgoblin", "humanoid", "Medium", "1/2"),
		mkCreature("bat", "Bat", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)

	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Search: "GOB"})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "Goblin", entries[0].Name)
	assert.Equal(t, "Hobgoblin", entries[1].Name)
}

func TestService_ListStatBlocks_SearchNoMatches(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Search: "zzz"})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestService_ListStatBlocks_SearchSpecialChars(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("will-o-wisp", "Will-o'-Wisp", "undead", "Tiny", "2"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Search: "o'-w"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// --- TDD cycle 3: filter by type (multi-select) ---

func TestService_ListStatBlocks_TypeFilter(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkCreature("bat", "Bat", "beast", "Tiny", "0"),
		mkCreature("ghoul", "Ghoul", "undead", "Medium", "1"),
	}}
	svc := NewService(store)

	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Types: []string{"beast", "undead"}})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "Bat", entries[0].Name)
	assert.Equal(t, "Ghoul", entries[1].Name)
}

// --- TDD cycle 4: CR range filter (supports "1/4", "1", "17") ---

func TestService_ListStatBlocks_CRRange(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "beast", "Tiny", "1/8"),
		mkCreature("c", "C", "beast", "Tiny", "1/4"),
		mkCreature("d", "D", "beast", "Tiny", "1/2"),
		mkCreature("e", "E", "beast", "Tiny", "1"),
		mkCreature("f", "F", "beast", "Tiny", "5"),
		mkCreature("g", "G", "beast", "Tiny", "17"),
	}}
	svc := NewService(store)

	min := 0.25
	max := 5.0
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CRMin: &min, CRMax: &max})
	require.NoError(t, err)
	require.Len(t, entries, 4) // 1/4, 1/2, 1, 5
	names := []string{entries[0].Name, entries[1].Name, entries[2].Name, entries[3].Name}
	assert.ElementsMatch(t, []string{"C", "D", "E", "F"}, names)
}

func TestService_ListStatBlocks_CRRangeOnlyMin(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "beast", "Tiny", "10"),
	}}
	svc := NewService(store)
	min := 1.0
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CRMin: &min})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "B", entries[0].Name)
}

func TestService_ListStatBlocks_CRRangeOnlyMax(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "beast", "Tiny", "10"),
	}}
	svc := NewService(store)
	max := 5.0
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CRMax: &max})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "A", entries[0].Name)
}

func TestService_ListStatBlocks_CRNoMatches(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)
	min := 5.0
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CRMin: &min})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestService_ListStatBlocks_InvalidCRIgnored(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "??"),
		mkCreature("b", "B", "beast", "Tiny", "1"),
	}}
	svc := NewService(store)
	min := 0.5
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CRMin: &min})
	require.NoError(t, err)
	// "??" should parse to 0 and be filtered out; "1" remains
	require.Len(t, entries, 1)
	assert.Equal(t, "B", entries[0].Name)
}

// --- TDD cycle 5: size filter ---

func TestService_ListStatBlocks_SizeFilter(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "beast", "Medium", "1"),
		mkCreature("c", "C", "beast", "Large", "5"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Sizes: []string{"Tiny", "Large"}})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.ElementsMatch(t, []string{"A", "C"}, []string{entries[0].Name, entries[1].Name})
}

// --- TDD cycle 6: source filter (srd / homebrew / both) + campaign scoping ---

func TestService_ListStatBlocks_SourceSRDOnly(t *testing.T) {
	campaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("srd", "SRD Goblin", "humanoid", "Small", "1/4"),
		mkHomebrew("hb", "Homebrew Goblin", "humanoid", "Small", "1/4", campaign),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Source: SourceSRD, CampaignID: campaign})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "SRD Goblin", entries[0].Name)
}

func TestService_ListStatBlocks_SourceHomebrewOnly(t *testing.T) {
	campaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("srd", "SRD Goblin", "humanoid", "Small", "1/4"),
		mkHomebrew("hb", "Homebrew Goblin", "humanoid", "Small", "1/4", campaign),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Source: SourceHomebrew, CampaignID: campaign})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Homebrew Goblin", entries[0].Name)
}

func TestService_ListStatBlocks_HomebrewFromAnotherCampaignHidden(t *testing.T) {
	otherCampaign := uuid.New()
	myCampaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("srd", "SRD Goblin", "humanoid", "Small", "1/4"),
		mkHomebrew("hb-other", "Other Homebrew", "humanoid", "Small", "1/4", otherCampaign),
		mkHomebrew("hb-mine", "My Homebrew", "humanoid", "Small", "1/4", myCampaign),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CampaignID: myCampaign})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	names := []string{entries[0].Name, entries[1].Name}
	assert.ElementsMatch(t, []string{"My Homebrew", "SRD Goblin"}, names)
}

func TestService_ListStatBlocks_NoCampaignHidesAllHomebrew(t *testing.T) {
	otherCampaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("srd", "SRD Goblin", "humanoid", "Small", "1/4"),
		mkHomebrew("hb-other", "Other Homebrew", "humanoid", "Small", "1/4", otherCampaign),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "SRD Goblin", entries[0].Name)
}

// --- TDD cycle 7: pagination ---

func TestService_ListStatBlocks_Pagination(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "beast", "Tiny", "0"),
		mkCreature("c", "C", "beast", "Tiny", "0"),
		mkCreature("d", "D", "beast", "Tiny", "0"),
		mkCreature("e", "E", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)

	page1, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Limit: 2, Offset: 0})
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.Equal(t, "A", page1[0].Name)
	assert.Equal(t, "B", page1[1].Name)

	page2, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Len(t, page2, 2)
	assert.Equal(t, "C", page2[0].Name)
	assert.Equal(t, "D", page2[1].Name)

	page3, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Limit: 2, Offset: 4})
	require.NoError(t, err)
	require.Len(t, page3, 1)
	assert.Equal(t, "E", page3[0].Name)
}

func TestService_ListStatBlocks_PaginationOffsetBeyondLength(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Limit: 2, Offset: 10})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// --- TDD cycle 8: GetStatBlock returns full creature ---

func TestService_GetStatBlock_SRD(t *testing.T) {
	c := mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4")
	c.Abilities = pqtype.NullRawMessage{RawMessage: json.RawMessage(`[{"name":"Nimble Escape"}]`), Valid: true}
	store := &fakeStore{creatures: []refdata.Creature{c}}
	svc := NewService(store)

	got, err := svc.GetStatBlock(context.Background(), "goblin", uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, "Goblin", got.Name)
}

func TestService_GetStatBlock_NotFound(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{}}
	svc := NewService(store)
	_, err := svc.GetStatBlock(context.Background(), "nope", uuid.Nil)
	require.Error(t, err)
}

func TestService_GetStatBlock_HomebrewFromAnotherCampaignForbidden(t *testing.T) {
	other := uuid.New()
	mine := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkHomebrew("hb", "Other Homebrew", "humanoid", "Small", "1/4", other),
	}}
	svc := NewService(store)
	_, err := svc.GetStatBlock(context.Background(), "hb", mine)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_GetStatBlock_HomebrewForMyCampaign(t *testing.T) {
	mine := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkHomebrew("hb", "My Homebrew", "humanoid", "Small", "1/4", mine),
	}}
	svc := NewService(store)
	got, err := svc.GetStatBlock(context.Background(), "hb", mine)
	require.NoError(t, err)
	assert.Equal(t, "My Homebrew", got.Name)
}

func TestService_GetStatBlock_StoreError(t *testing.T) {
	store := &fakeStore{getErr: errors.New("db down")}
	svc := NewService(store)
	_, err := svc.GetStatBlock(context.Background(), "x", uuid.Nil)
	require.Error(t, err)
}

// --- TDD cycle 9: edge cases in helpers ---

func TestService_ListStatBlocks_SourceSRDExcludesHomebrew(t *testing.T) {
	campaign := uuid.New()
	store := &fakeStore{creatures: []refdata.Creature{
		mkHomebrew("hb", "Homebrew", "humanoid", "Small", "1/4", campaign),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Source: SourceSRD, CampaignID: campaign})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestService_ListStatBlocks_HomebrewWithoutValidCampaignIDSkipped(t *testing.T) {
	campaign := uuid.New()
	// homebrew=true but campaign_id is NULL (corrupt row) — should not match.
	c := mkCreature("corrupt", "Corrupt", "humanoid", "Small", "1/4")
	c.Homebrew = sql.NullBool{Bool: true, Valid: true}
	store := &fakeStore{creatures: []refdata.Creature{c}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{CampaignID: campaign})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestService_ListStatBlocks_EmptySearchReturnsAll(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Search: "   "})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestService_ListStatBlocks_TypesWithEmptyStringFilteredOut(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)
	// Only empty strings in Types → treated as no filter.
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Types: []string{"", "  "}})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestService_ListStatBlocks_SizesWithEmptyStringFilteredOut(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Sizes: []string{"", "  "}})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestService_ListStatBlocks_NegativeOffsetTreatedAsZero(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("a", "A", "beast", "Tiny", "0"),
		mkCreature("b", "B", "beast", "Tiny", "0"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{Offset: -5, Limit: 1})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "A", entries[0].Name)
}

func TestService_GetStatBlock_SRDVisibleWithoutCampaignID(t *testing.T) {
	c := mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4")
	// Homebrew invalid (not set) — treated as SRD.
	c.Homebrew = sql.NullBool{}
	store := &fakeStore{creatures: []refdata.Creature{c}}
	svc := NewService(store)
	got, err := svc.GetStatBlock(context.Background(), "goblin", uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, "Goblin", got.Name)
}

func TestService_GetStatBlock_HomebrewWithNullCampaignBlocked(t *testing.T) {
	c := mkCreature("corrupt", "Corrupt", "humanoid", "Small", "1/4")
	c.Homebrew = sql.NullBool{Bool: true, Valid: true}
	// campaign_id NULL
	store := &fakeStore{creatures: []refdata.Creature{c}}
	svc := NewService(store)
	_, err := svc.GetStatBlock(context.Background(), "corrupt", uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

// --- Open5e source gating ---

func mkOpen5eCreature(id, name, typ, size, cr, docSlug string) refdata.Creature {
	c := mkCreature(id, name, typ, size, cr)
	c.Homebrew = sql.NullBool{Bool: false, Valid: true}
	c.Source = sql.NullString{String: "open5e:" + docSlug, Valid: true}
	return c
}

func TestService_ListStatBlocks_Open5eHiddenWhenSourceNotEnabled(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkOpen5eCreature("open5e_bearfolk", "Bearfolk", "humanoid", "Medium", "1", "tome-of-beasts"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Goblin", entries[0].Name)
}

func TestService_ListStatBlocks_Open5eVisibleWhenSourceEnabled(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkOpen5eCreature("open5e_bearfolk", "Bearfolk", "humanoid", "Medium", "1", "tome-of-beasts"),
		mkOpen5eCreature("open5e_dragonkin", "Dragonkin", "humanoid", "Medium", "3", "creature-codex"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{
		EnabledOpen5eSources: []string{"tome-of-beasts"},
	})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	names := []string{entries[0].Name, entries[1].Name}
	assert.ElementsMatch(t, []string{"Bearfolk", "Goblin"}, names)
}

func TestService_ListStatBlocks_Open5eMultipleEnabled(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkOpen5eCreature("a", "AAA", "humanoid", "Medium", "1", "tome-of-beasts"),
		mkOpen5eCreature("b", "BBB", "humanoid", "Medium", "3", "creature-codex"),
		mkOpen5eCreature("c", "CCC", "humanoid", "Medium", "5", "deep-magic"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{
		EnabledOpen5eSources: []string{"tome-of-beasts", "creature-codex"},
	})
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

func TestService_ListStatBlocks_Open5eSourceFilterSRDOnlyStillExcludesOpen5e(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkCreature("goblin", "Goblin", "humanoid", "Small", "1/4"),
		mkOpen5eCreature("open5e_bearfolk", "Bearfolk", "humanoid", "Medium", "1", "tome-of-beasts"),
	}}
	svc := NewService(store)
	entries, err := svc.ListStatBlocks(context.Background(), StatBlockFilter{
		Source:               SourceSRD,
		EnabledOpen5eSources: []string{"tome-of-beasts"},
	})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Goblin", entries[0].Name)
}

func TestService_GetStatBlock_Open5eHiddenWhenSourceNotEnabled(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkOpen5eCreature("open5e_bearfolk", "Bearfolk", "humanoid", "Medium", "1", "tome-of-beasts"),
	}}
	svc := NewService(store)
	_, err := svc.GetStatBlock(context.Background(), "open5e_bearfolk", uuid.Nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_GetStatBlockWithSources_Open5eVisible(t *testing.T) {
	store := &fakeStore{creatures: []refdata.Creature{
		mkOpen5eCreature("open5e_bearfolk", "Bearfolk", "humanoid", "Medium", "1", "tome-of-beasts"),
	}}
	svc := NewService(store)
	got, err := svc.GetStatBlockWithSources(
		context.Background(), "open5e_bearfolk", uuid.Nil, []string{"tome-of-beasts"},
	)
	require.NoError(t, err)
	assert.Equal(t, "Bearfolk", got.Name)
}

// --- parseCR helper tested directly for edge cases ---

func TestParseCR(t *testing.T) {
	cases := map[string]float64{
		"0":   0,
		"1/8": 0.125,
		"1/4": 0.25,
		"1/2": 0.5,
		"1":   1,
		"17":  17,
		"":    0,
		"??":  0,
		"1/0": 0, // divide-by-zero safety
	}
	for in, want := range cases {
		got := parseCR(in)
		assert.Equal(t, want, got, "parseCR(%q)", in)
	}
}
