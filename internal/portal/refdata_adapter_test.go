package portal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueries implements the subset of refdata.Queries methods needed.
type mockQueries struct {
	races   []refdata.Race
	classes []refdata.Class
	spells  []refdata.Spell
	weapons []refdata.Weapon
	armor   []refdata.Armor
}

func (m *mockQueries) ListRaces(_ context.Context) ([]refdata.Race, error) {
	return m.races, nil
}

func (m *mockQueries) ListClasses(_ context.Context) ([]refdata.Class, error) {
	return m.classes, nil
}

func (m *mockQueries) ListSpellsByClass(_ context.Context, class string) ([]refdata.Spell, error) {
	return m.spells, nil
}

func (m *mockQueries) ListWeapons(_ context.Context) ([]refdata.Weapon, error) {
	return m.weapons, nil
}

func (m *mockQueries) ListArmor(_ context.Context) ([]refdata.Armor, error) {
	return m.armor, nil
}

func TestRefDataAdapter_ListRaces(t *testing.T) {
	bonuses := json.RawMessage(`{"str": 2}`)
	mq := &mockQueries{
		races: []refdata.Race{
			{ID: "dwarf", Name: "Dwarf", SpeedFt: 25, Size: "Medium", DarkvisionFt: 60, AbilityBonuses: bonuses, Languages: []string{"Common", "Dwarvish"}},
		},
	}
	adapter := portal.NewRefDataAdapter(mq)

	races, err := adapter.ListRaces(context.Background())
	require.NoError(t, err)
	assert.Len(t, races, 1)
	assert.Equal(t, "dwarf", races[0].ID)
	assert.Equal(t, "Dwarf", races[0].Name)
	assert.Equal(t, 25, races[0].SpeedFt)
	assert.Equal(t, 60, races[0].DarkvisionFt)
}

func TestRefDataAdapter_ListClasses(t *testing.T) {
	skillChoices := pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"count":2,"from":["athletics","perception"]}`), Valid: true}
	mq := &mockQueries{
		classes: []refdata.Class{
			{ID: "fighter", Name: "Fighter", HitDie: "d10", PrimaryAbility: "str",
				SaveProficiencies:   []string{"str", "con"},
				ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
				WeaponProficiencies: []string{"simple", "martial"},
				SkillChoices:        skillChoices,
				WeaponMasteryCount:  3,
				SubclassLevel:       3, Subclasses: json.RawMessage(`[]`)},
		},
	}
	adapter := portal.NewRefDataAdapter(mq)

	classes, err := adapter.ListClasses(context.Background())
	require.NoError(t, err)
	assert.Len(t, classes, 1)
	assert.Equal(t, "fighter", classes[0].ID)
	assert.Equal(t, "d10", classes[0].HitDie)
	assert.Equal(t, 3, classes[0].SubclassLevel)
	assert.Equal(t, []string{"light", "medium", "heavy", "shields"}, classes[0].ArmorProficiencies)
	assert.Equal(t, []string{"simple", "martial"}, classes[0].WeaponProficiencies)
	assert.Equal(t, 3, classes[0].WeaponMasteryCount)
}

func TestRefDataAdapter_ListRaces_Error(t *testing.T) {
	mq := &errorQueries{}
	adapter := portal.NewRefDataAdapter(mq)
	_, err := adapter.ListRaces(context.Background())
	assert.Error(t, err)
}

func TestRefDataAdapter_ListClasses_Error(t *testing.T) {
	mq := &errorQueries{}
	adapter := portal.NewRefDataAdapter(mq)
	_, err := adapter.ListClasses(context.Background())
	assert.Error(t, err)
}

func TestRefDataAdapter_ListSpellsByClass_Error(t *testing.T) {
	mq := &errorQueries{}
	adapter := portal.NewRefDataAdapter(mq)
	_, err := adapter.ListSpellsByClass(context.Background(), "wizard", "")
	assert.Error(t, err)
}

type errorQueries struct{}

func (e *errorQueries) ListRaces(_ context.Context) ([]refdata.Race, error) {
	return nil, errors.New("db error")
}
func (e *errorQueries) ListClasses(_ context.Context) ([]refdata.Class, error) {
	return nil, errors.New("db error")
}
func (e *errorQueries) ListSpellsByClass(_ context.Context, _ string) ([]refdata.Spell, error) {
	return nil, errors.New("db error")
}
func (e *errorQueries) ListWeapons(_ context.Context) ([]refdata.Weapon, error) {
	return nil, errors.New("db error")
}
func (e *errorQueries) ListArmor(_ context.Context) ([]refdata.Armor, error) {
	return nil, errors.New("db error")
}

func TestRefDataAdapter_ListEquipment(t *testing.T) {
	mq := &mockQueries{
		weapons: []refdata.Weapon{
			{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeaponType: "martial-melee", Properties: []string{"versatile"}, Mastery: "sap"},
		},
		armor: []refdata.Armor{
			{ID: "chain-mail", Name: "Chain Mail", AcBase: 16, ArmorType: "heavy"},
		},
	}
	adapter := portal.NewRefDataAdapter(mq)

	items, err := adapter.ListEquipment(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "longsword", items[0].ID)
	assert.Equal(t, "weapon", items[0].Category)
	assert.Equal(t, "1d8", items[0].Damage)
	assert.Equal(t, []string{"versatile"}, items[0].Properties)
	assert.Equal(t, "sap", items[0].Mastery)
	assert.Equal(t, "chain-mail", items[1].ID)
	assert.Equal(t, "armor", items[1].Category)
	assert.Equal(t, 16, items[1].ACBase)
}

func TestRefDataAdapter_ListEquipment_WeaponsError(t *testing.T) {
	mq := &errorQueries{}
	adapter := portal.NewRefDataAdapter(mq)
	_, err := adapter.ListEquipment(context.Background(), "")
	assert.Error(t, err)
}

func TestRefDataAdapter_ListEquipment_ArmorError(t *testing.T) {
	// Weapons succeed but armor fails
	mq := &armorErrorQueries{}
	adapter := portal.NewRefDataAdapter(mq)
	_, err := adapter.ListEquipment(context.Background(), "")
	assert.Error(t, err)
}

// armorErrorQueries returns an error only for ListArmor.
type armorErrorQueries struct{ mockQueries }

func (e *armorErrorQueries) ListArmor(_ context.Context) ([]refdata.Armor, error) {
	return nil, errors.New("armor db error")
}

func TestRefDataAdapter_ListSpellsByClass(t *testing.T) {
	mq := &mockQueries{
		spells: []refdata.Spell{
			{
				ID: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "evocation",
				RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 120, Valid: true},
				Components: []string{"V", "S"}, Classes: []string{"wizard"},
			},
		},
	}
	adapter := portal.NewRefDataAdapter(mq)

	spells, err := adapter.ListSpellsByClass(context.Background(), "wizard", "")
	require.NoError(t, err)
	assert.Len(t, spells, 1)
	assert.Equal(t, "Fire Bolt", spells[0].Name)
	assert.Equal(t, "120ft", spells[0].Range)
	assert.Equal(t, []string{"V", "S"}, spells[0].Components)
}

func TestRefDataAdapter_ListSpellsByClass_RangeFormatting(t *testing.T) {
	cases := []struct {
		name      string
		rangeType string
		rangeFt   sql.NullInt32
		want      string
	}{
		{"ranged with feet", "ranged", sql.NullInt32{Int32: 30, Valid: true}, "30ft"},
		{"ranged without feet falls back to type", "ranged", sql.NullInt32{}, "ranged"},
		{"self", "self", sql.NullInt32{}, "Self"},
		{"touch", "touch", sql.NullInt32{}, "Touch"},
		{"sight", "sight", sql.NullInt32{}, "sight"},
		{"unlimited", "unlimited", sql.NullInt32{}, "unlimited"},
		{"empty type", "", sql.NullInt32{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mq := &mockQueries{
				spells: []refdata.Spell{
					{ID: "s", Name: "S", School: "evocation", RangeType: tc.rangeType, RangeFt: tc.rangeFt, Classes: []string{"wizard"}},
				},
			}
			adapter := portal.NewRefDataAdapter(mq)
			spells, err := adapter.ListSpellsByClass(context.Background(), "wizard", "")
			require.NoError(t, err)
			require.Len(t, spells, 1)
			assert.Equal(t, tc.want, spells[0].Range)
		})
	}
}

func TestRefDataAdapter_ListEquipment_CampaignFiltering(t *testing.T) {
	campA := uuid.New()
	campB := uuid.New()
	mq := &mockQueries{
		weapons: []refdata.Weapon{
			{ID: "longsword", Name: "Longsword"},
			{ID: "hb-a", Name: "Homebrew A", CampaignID: uuid.NullUUID{UUID: campA, Valid: true}},
			{ID: "hb-b", Name: "Homebrew B", CampaignID: uuid.NullUUID{UUID: campB, Valid: true}},
		},
		armor: []refdata.Armor{
			{ID: "chain-mail", Name: "Chain Mail", AcBase: 16, ArmorType: "heavy"},
		},
	}
	adapter := portal.NewRefDataAdapter(mq)

	// With campaign A — should see global + campA, not campB
	items, err := adapter.ListEquipment(context.Background(), campA.String())
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	assert.True(t, ids["longsword"])
	assert.True(t, ids["hb-a"])
	assert.False(t, ids["hb-b"])
	assert.True(t, ids["chain-mail"])
}

func TestRefDataAdapter_ListSpellsByClass_CampaignFiltering(t *testing.T) {
	campA := uuid.New()
	campB := uuid.New()
	mq := &mockQueries{
		spells: []refdata.Spell{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "evocation", Classes: []string{"wizard"}},
			{ID: "hb-spell-a", Name: "Homebrew Spell A", Level: 1, School: "abjuration", Classes: []string{"wizard"}, CampaignID: uuid.NullUUID{UUID: campA, Valid: true}},
			{ID: "hb-spell-b", Name: "Homebrew Spell B", Level: 1, School: "necromancy", Classes: []string{"wizard"}, CampaignID: uuid.NullUUID{UUID: campB, Valid: true}},
		},
	}
	adapter := portal.NewRefDataAdapter(mq)

	spells, err := adapter.ListSpellsByClass(context.Background(), "wizard", campA.String())
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, s := range spells {
		ids[s.ID] = true
	}
	assert.True(t, ids["fire-bolt"])
	assert.True(t, ids["hb-spell-a"])
	assert.False(t, ids["hb-spell-b"])
}
