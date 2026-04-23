package open5e

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- fake store ---

type fakeCacheStore struct {
	creatures         []refdata.UpsertCreatureParams
	spells            []refdata.UpsertSpellParams
	upsertCreatureErr error
	upsertSpellErr    error
}

func (f *fakeCacheStore) UpsertCreature(ctx context.Context, arg refdata.UpsertCreatureParams) error {
	if f.upsertCreatureErr != nil {
		return f.upsertCreatureErr
	}
	f.creatures = append(f.creatures, arg)
	return nil
}

func (f *fakeCacheStore) UpsertSpell(ctx context.Context, arg refdata.UpsertSpellParams) error {
	if f.upsertSpellErr != nil {
		return f.upsertSpellErr
	}
	f.spells = append(f.spells, arg)
	return nil
}

// --- Cycle 1: CacheMonster maps fields correctly ---

func TestCache_CacheMonster_StoresWithOpen5eSource(t *testing.T) {
	store := &fakeCacheStore{}
	cache := NewCache(store)
	m := Monster{
		Slug: "aboleth", Name: "Aboleth",
		Size: "Large", Type: "aberration",
		Alignment:  "lawful evil",
		ArmorClass: 17,
		ArmorDesc:  "natural armor",
		HitPoints:  135,
		HitDice:    "18d10+36",
		Speed:      json.RawMessage(`{"walk":10}`),
		Strength:   21, Dexterity: 9, Constitution: 15,
		Intelligence: 18, Wisdom: 15, Charisma: 18,
		ChallengeRating: "10",
		Actions:         json.RawMessage(`[{"name":"Tentacle"}]`),
		DocumentSlug:    "tome-of-beasts",
	}

	id, err := cache.CacheMonster(context.Background(), m)
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	require.Len(t, store.creatures, 1)
	c := store.creatures[0]
	assert.Equal(t, "Aboleth", c.Name)
	assert.Equal(t, "Large", c.Size)
	assert.Equal(t, "aberration", c.Type)
	assert.Equal(t, int32(17), c.Ac)
	assert.Equal(t, int32(135), c.HpAverage)
	assert.Equal(t, "18d10+36", c.HpFormula)
	assert.Equal(t, "10", c.Cr)
	// source = open5e:<document_slug>
	require.True(t, c.Source.Valid)
	assert.Equal(t, "open5e:tome-of-beasts", c.Source.String)
	// Global row (no campaign_id)
	assert.False(t, c.CampaignID.Valid)
	// Not homebrew
	require.True(t, c.Homebrew.Valid)
	assert.False(t, c.Homebrew.Bool)
	// ID prefixed with open5e_ to avoid clashing with SRD ids
	assert.Equal(t, "open5e_aboleth", id)
	assert.Equal(t, "open5e_aboleth", c.ID)
}

func TestCache_CacheMonster_EmptyDocumentSlugDefaultsToOpen5e(t *testing.T) {
	store := &fakeCacheStore{}
	cache := NewCache(store)
	m := Monster{Slug: "x", Name: "X", Size: "Medium", Type: "beast", ChallengeRating: "0"}
	_, err := cache.CacheMonster(context.Background(), m)
	require.NoError(t, err)
	require.Len(t, store.creatures, 1)
	require.True(t, store.creatures[0].Source.Valid)
	assert.Equal(t, "open5e:unknown", store.creatures[0].Source.String)
}

func TestCache_CacheMonster_EmptyRequiredFieldsApplyDefaults(t *testing.T) {
	store := &fakeCacheStore{}
	cache := NewCache(store)
	// Minimal monster — no size, no type, no CR. Cache must apply
	// sensible defaults rather than bomb.
	m := Monster{Slug: "mystery", Name: "Mystery", DocumentSlug: "creature-codex"}
	_, err := cache.CacheMonster(context.Background(), m)
	require.NoError(t, err)
	require.Len(t, store.creatures, 1)
	c := store.creatures[0]
	assert.NotEmpty(t, c.Size)
	assert.NotEmpty(t, c.Type)
	assert.NotEmpty(t, c.Cr)
	// Speed still valid JSON even when Open5e omitted it
	assert.NotEmpty(t, c.Speed)
	// Ability scores default to {10,10,10,10,10,10} valid JSON
	assert.NotEmpty(t, c.AbilityScores)
}

func TestCache_CacheMonster_EmptySlugReturnsError(t *testing.T) {
	store := &fakeCacheStore{}
	cache := NewCache(store)
	_, err := cache.CacheMonster(context.Background(), Monster{Slug: "", Name: "X"})
	require.Error(t, err)
	assert.Empty(t, store.creatures)
}

func TestCache_CacheMonster_UpsertError(t *testing.T) {
	store := &fakeCacheStore{upsertCreatureErr: errors.New("db down")}
	cache := NewCache(store)
	_, err := cache.CacheMonster(context.Background(), Monster{Slug: "aboleth", Name: "Aboleth", DocumentSlug: "wotc-srd"})
	require.Error(t, err)
}

// --- Cycle 2: CacheSpell ---

func TestCache_CacheSpell_StoresWithOpen5eSource(t *testing.T) {
	store := &fakeCacheStore{}
	cache := NewCache(store)
	sp := Spell{
		Slug:          "fireball",
		Name:          "Fireball",
		LevelInt:      3,
		School:        "evocation",
		CastingTime:   "1 action",
		Range:         "150 feet",
		Duration:      "Instantaneous",
		Components:    "V, S, M",
		Material:      "a tiny ball of bat guano",
		Ritual:        false,
		Concentration: false,
		Description:   "Boom.",
		HigherLevel:   "More fire.",
		DndClass:      "Sorcerer, Wizard",
		DocumentSlug:  "deep-magic",
	}
	id, err := cache.CacheSpell(context.Background(), sp)
	require.NoError(t, err)
	assert.Equal(t, "open5e_fireball", id)
	require.Len(t, store.spells, 1)
	s := store.spells[0]
	assert.Equal(t, "Fireball", s.Name)
	assert.Equal(t, int32(3), s.Level)
	assert.Equal(t, "evocation", s.School)
	assert.Equal(t, "1 action", s.CastingTime)
	assert.ElementsMatch(t, []string{"V", "S", "M"}, s.Components)
	assert.ElementsMatch(t, []string{"sorcerer", "wizard"}, s.Classes)
	require.True(t, s.Source.Valid)
	assert.Equal(t, "open5e:deep-magic", s.Source.String)
	assert.False(t, s.CampaignID.Valid)
	require.True(t, s.Homebrew.Valid)
	assert.False(t, s.Homebrew.Bool)
}

func TestCache_CacheSpell_EmptySlugReturnsError(t *testing.T) {
	store := &fakeCacheStore{}
	cache := NewCache(store)
	_, err := cache.CacheSpell(context.Background(), Spell{})
	require.Error(t, err)
}

func TestCache_CacheSpell_UpsertError(t *testing.T) {
	store := &fakeCacheStore{upsertSpellErr: errors.New("boom")}
	cache := NewCache(store)
	_, err := cache.CacheSpell(context.Background(), Spell{Slug: "fireball", Name: "Fireball", DocumentSlug: "wotc-srd"})
	require.Error(t, err)
}

// --- Cycle 3: components and class parsing ---

func TestParseComponents(t *testing.T) {
	cases := map[string][]string{
		"V":       {"V"},
		"V, S, M": {"V", "S", "M"},
		"":        {},
		"v,s":     {"V", "S"},
	}
	for in, want := range cases {
		assert.ElementsMatch(t, want, parseComponents(in), "parseComponents(%q)", in)
	}
}

func TestParseClasses(t *testing.T) {
	assert.ElementsMatch(t, []string{"wizard"}, parseClasses("Wizard"))
	assert.ElementsMatch(t, []string{"sorcerer", "wizard"}, parseClasses("Sorcerer, Wizard"))
	assert.Empty(t, parseClasses(""))
	assert.ElementsMatch(t, []string{"bard", "cleric"}, parseClasses("Bard,cleric"))
}
