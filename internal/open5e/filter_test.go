package open5e

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ab/dndnd/internal/refdata"
)

func TestIsOpen5eSource(t *testing.T) {
	assert.True(t, IsOpen5eSource("open5e:tome-of-beasts"))
	assert.True(t, IsOpen5eSource("open5e:"))
	assert.False(t, IsOpen5eSource("srd"))
	assert.False(t, IsOpen5eSource("homebrew"))
	assert.False(t, IsOpen5eSource(""))
}

func TestDocumentSlug(t *testing.T) {
	assert.Equal(t, "tome-of-beasts", DocumentSlug("open5e:tome-of-beasts"))
	assert.Equal(t, "", DocumentSlug("srd"))
	assert.Equal(t, "", DocumentSlug(""))
}

func TestFilterSpellsByOpen5eSources(t *testing.T) {
	srd := refdata.Spell{ID: "fireball", Source: sql.NullString{String: "srd", Valid: true}}
	homebrew := refdata.Spell{ID: "hb_boom", Source: sql.NullString{String: "homebrew", Valid: true}}
	tomeBeasts := refdata.Spell{ID: "open5e_flame-arrows", Source: sql.NullString{String: "open5e:tome-of-beasts", Valid: true}}
	deepMagic := refdata.Spell{ID: "open5e_rune-ward", Source: sql.NullString{String: "open5e:deep-magic", Valid: true}}
	unset := refdata.Spell{ID: "unset", Source: sql.NullString{}}

	all := []refdata.Spell{srd, homebrew, tomeBeasts, deepMagic, unset}
	got := FilterSpellsByOpen5eSources(all, []string{"tome-of-beasts"})
	ids := make([]string, len(got))
	for i, s := range got {
		ids[i] = s.ID
	}
	assert.ElementsMatch(t, []string{"fireball", "hb_boom", "open5e_flame-arrows", "unset"}, ids)
}

func TestFilterSpellsByOpen5eSources_EmptyEnabledHidesAllOpen5e(t *testing.T) {
	spells := []refdata.Spell{
		{ID: "srd", Source: sql.NullString{String: "srd", Valid: true}},
		{ID: "open5e_x", Source: sql.NullString{String: "open5e:foo", Valid: true}},
	}
	got := FilterSpellsByOpen5eSources(spells, nil)
	assert.Len(t, got, 1)
	assert.Equal(t, "srd", got[0].ID)
}
