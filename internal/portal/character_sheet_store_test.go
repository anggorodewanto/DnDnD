package portal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/portal"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCharacterQuerier implements portal.CharacterQuerier for unit tests.
type mockCharacterQuerier struct {
	character    refdata.Character
	charErr      error
	pc           refdata.PlayerCharacter
	pcErr        error
	spells       []refdata.Spell
	spellsErr    error
	campaign     refdata.Campaign
	campaignErr  error
	viewerPC     refdata.PlayerCharacter
	viewerPCErr  error
	weapons      []refdata.Weapon
	weaponsErr   error
	armor        []refdata.Armor
	armorErr     error
	combatant    *refdata.Combatant
	combatantErr error
}

func (m *mockCharacterQuerier) GetCharacter(_ context.Context, id uuid.UUID) (refdata.Character, error) {
	return m.character, m.charErr
}

func (m *mockCharacterQuerier) GetPlayerCharacterByCharacter(_ context.Context, _ refdata.GetPlayerCharacterByCharacterParams) (refdata.PlayerCharacter, error) {
	return m.pc, m.pcErr
}

func (m *mockCharacterQuerier) GetSpellsByIDs(_ context.Context, ids []string) ([]refdata.Spell, error) {
	return m.spells, m.spellsErr
}

func (m *mockCharacterQuerier) GetActiveCombatantByCharacterID(_ context.Context, _ uuid.NullUUID) (refdata.Combatant, error) {
	if m.combatantErr != nil {
		return refdata.Combatant{}, m.combatantErr
	}
	if m.combatant != nil {
		return *m.combatant, nil
	}
	return refdata.Combatant{}, sql.ErrNoRows // default: not in combat
}

func (m *mockCharacterQuerier) GetCampaignByID(_ context.Context, _ uuid.UUID) (refdata.Campaign, error) {
	return m.campaign, m.campaignErr
}

func (m *mockCharacterQuerier) GetPlayerCharacterByDiscordUser(_ context.Context, _ refdata.GetPlayerCharacterByDiscordUserParams) (refdata.PlayerCharacter, error) {
	return m.viewerPC, m.viewerPCErr
}

func (m *mockCharacterQuerier) ListWeapons(_ context.Context) ([]refdata.Weapon, error) {
	return m.weapons, m.weaponsErr
}

func (m *mockCharacterQuerier) ListArmor(_ context.Context) ([]refdata.Armor, error) {
	return m.armor, m.armorErr
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_DM(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()
	q := &mockCharacterQuerier{
		character:   refdata.Character{ID: charID, CampaignID: campID},
		campaign:    refdata.Campaign{ID: campID, DmUserID: "dm-1"},
		viewerPCErr: sql.ErrNoRows, // DM has no PC in the campaign
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	ok, err := store.CanViewCharacter(context.Background(), charID.String(), "dm-1")

	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_CampaignMember(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()
	q := &mockCharacterQuerier{
		character: refdata.Character{ID: charID, CampaignID: campID},
		campaign:  refdata.Campaign{ID: campID, DmUserID: "dm-1"},
		viewerPC:  refdata.PlayerCharacter{CampaignID: campID, DiscordUserID: "player-2", Status: "approved"},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	ok, err := store.CanViewCharacter(context.Background(), charID.String(), "player-2")

	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_Stranger(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()
	q := &mockCharacterQuerier{
		character:   refdata.Character{ID: charID, CampaignID: campID},
		campaign:    refdata.Campaign{ID: campID, DmUserID: "dm-1"},
		viewerPCErr: sql.ErrNoRows, // not a player in this campaign
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	ok, err := store.CanViewCharacter(context.Background(), charID.String(), "stranger")

	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_InvalidID(t *testing.T) {
	store := portal.NewCharacterSheetStoreAdapter(&mockCharacterQuerier{})
	_, err := store.CanViewCharacter(context.Background(), "not-a-uuid", "user-1")
	require.Error(t, err)
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_CharNotFound(t *testing.T) {
	q := &mockCharacterQuerier{charErr: sql.ErrNoRows}
	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.CanViewCharacter(context.Background(), uuid.New().String(), "user-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrCharacterNotFound)
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_CampaignLookupError(t *testing.T) {
	charID := uuid.New()
	q := &mockCharacterQuerier{
		character:   refdata.Character{ID: charID, CampaignID: uuid.New()},
		campaignErr: errors.New("db down"),
	}
	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.CanViewCharacter(context.Background(), charID.String(), "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_ViewerLookupError(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()
	q := &mockCharacterQuerier{
		character:   refdata.Character{ID: charID, CampaignID: campID},
		campaign:    refdata.Campaign{ID: campID, DmUserID: "dm-1"},
		viewerPCErr: errors.New("db down"),
	}
	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.CanViewCharacter(context.Background(), charID.String(), "player-2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestCharacterSheetStoreAdapter_CanViewCharacter_CampaignMissingButMember(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()
	q := &mockCharacterQuerier{
		character:   refdata.Character{ID: charID, CampaignID: campID},
		campaignErr: sql.ErrNoRows, // campaign row gone; fall through to membership
		viewerPC:    refdata.PlayerCharacter{CampaignID: campID, DiscordUserID: "player-2"},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	ok, err := store.CanViewCharacter(context.Background(), charID.String(), "player-2")

	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()

	q := &mockCharacterQuerier{
		character: refdata.Character{ID: charID, CampaignID: campID},
		pc: refdata.PlayerCharacter{
			CharacterID:   charID,
			CampaignID:    campID,
			DiscordUserID: "user-123",
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	ownerID, err := store.GetCharacterOwner(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "user-123", ownerID)
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner_InvalidID(t *testing.T) {
	store := portal.NewCharacterSheetStoreAdapter(&mockCharacterQuerier{})
	_, err := store.GetCharacterOwner(context.Background(), "not-a-uuid")

	require.Error(t, err)
}

func TestCharacterSheetStoreAdapter_GetCharacterOwner_CharNotFound(t *testing.T) {
	q := &mockCharacterQuerier{
		charErr: sql.ErrNoRows,
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.GetCharacterOwner(context.Background(), uuid.New().String())

	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrCharacterNotFound)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet(t *testing.T) {
	charID := uuid.New()
	campID := uuid.New()

	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 12, INT: 10, WIS: 8, CHA: 13}
	scoresJSON, _ := json.Marshal(scores)

	classes := []character.ClassEntry{{Class: "Fighter", Level: 3}}
	classesJSON, _ := json.Marshal(classes)

	profs := character.Proficiencies{Saves: []string{"str", "con"}, Skills: []string{"athletics"}}
	profsJSON, _ := json.Marshal(profs)

	features := []character.Feature{{Name: "Second Wind", Source: "Fighter", Level: 1, Description: "Heal"}}
	featuresJSON, _ := json.Marshal(features)

	inventory := []character.InventoryItem{{ItemID: "longsword", Name: "Longsword", Quantity: 1, Equipped: true, Type: "weapon"}}
	inventoryJSON, _ := json.Marshal(inventory)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:               charID,
			CampaignID:       campID,
			Name:             "Thorn",
			Race:             "Human",
			Level:            3,
			Classes:          classesJSON,
			AbilityScores:    scoresJSON,
			HpMax:            28,
			HpCurrent:        25,
			TempHp:           5,
			Ac:               18,
			SpeedFt:          30,
			ProficiencyBonus: 2,
			EquippedMainHand: sql.NullString{String: "Longsword", Valid: true},
			Proficiencies:    pqtype.NullRawMessage{RawMessage: profsJSON, Valid: true},
			Features:         pqtype.NullRawMessage{RawMessage: featuresJSON, Valid: true},
			Languages:        []string{"Common", "Elvish"},
			Inventory:        pqtype.NullRawMessage{RawMessage: inventoryJSON, Valid: true},
			Gold:             50,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "Thorn", data.Name)
	assert.Equal(t, "Human", data.Race)
	assert.Equal(t, 3, data.Level)
	assert.Equal(t, 28, data.HpMax)
	assert.Equal(t, 25, data.HpCurrent)
	assert.Equal(t, 5, data.TempHP)
	assert.Equal(t, 18, data.AC)
	assert.Equal(t, 30, data.SpeedFt)
	assert.Equal(t, 2, data.ProficiencyBonus)
	assert.Equal(t, "Longsword", data.EquippedMainHand.Name)
	assert.Equal(t, 16, data.AbilityScores.STR)
	assert.Len(t, data.Classes, 1)
	assert.Equal(t, "Fighter", data.Classes[0].Class)
	assert.Equal(t, []string{"str", "con"}, data.Proficiencies.Saves)
	assert.Len(t, data.Features, 1)
	assert.Equal(t, "Second Wind", data.Features[0].Name)
	assert.Len(t, data.Inventory, 1)
	assert.Equal(t, "Longsword", data.Inventory[0].Name)
	assert.Equal(t, []string{"Common", "Elvish"}, data.Languages)
	assert.Equal(t, 50, data.Gold)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_PortalSpells(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Wizard", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	// Portal format: array of spell ID strings
	charData := map[string]any{"spells": []string{"fire-bolt", "magic-missile", "shield"}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "Evocation", CastingTime: "1 action", RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 120, Valid: true}},
			{ID: "magic-missile", Name: "Magic Missile", Level: 1, School: "Evocation", CastingTime: "1 action", RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 120, Valid: true}},
			{ID: "shield", Name: "Shield", Level: 1, School: "Abjuration", CastingTime: "1 reaction", RangeType: "self"},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 3)

	// Spells should be enriched from reference table
	// Results ordered by level then name from the query
	byID := make(map[string]portal.SpellDisplayEntry)
	for _, s := range data.Spells {
		byID[s.ID] = s
	}
	assert.Equal(t, "Fire Bolt", byID["fire-bolt"].Name)
	assert.Equal(t, 0, byID["fire-bolt"].Level)
	assert.Equal(t, "Evocation", byID["fire-bolt"].School)
	assert.Equal(t, "1 action", byID["fire-bolt"].CastingTime)
	assert.Equal(t, "120ft", byID["fire-bolt"].Range)

	assert.Equal(t, "Magic Missile", byID["magic-missile"].Name)
	assert.Equal(t, 1, byID["magic-missile"].Level)

	assert.Equal(t, "Shield", byID["shield"].Name)
	assert.Equal(t, 1, byID["shield"].Level)
	assert.Equal(t, "Abjuration", byID["shield"].School)
	assert.Equal(t, "1 reaction", byID["shield"].CastingTime)
	assert.Equal(t, "Self", byID["shield"].Range)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_SpellDetails(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Wizard", Level: 5}})
	charDataJSON, _ := json.Marshal(map[string]any{"spells": []string{"fireball"}})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{
				ID: "fireball", Name: "Fireball", Level: 3, School: "evocation",
				CastingTime:   "1 action",
				RangeType:     "ranged",
				RangeFt:       sql.NullInt32{Int32: 150, Valid: true},
				Components:    []string{"V", "S", "M"},
				Duration:      "Instantaneous",
				Concentration: sql.NullBool{Bool: false, Valid: true},
				Description:   "A bright streak flashes to a point you choose, then blossoms into flame.",
			},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 1)
	s := data.Spells[0]
	assert.Equal(t, []string{"V", "S", "M"}, s.Components)
	assert.Equal(t, "Instantaneous", s.Duration)
	assert.Equal(t, "A bright streak flashes to a point you choose, then blossoms into flame.", s.Description)
	assert.False(t, s.Concentration)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_ConcentrationSpell(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Wizard", Level: 5}})
	charDataJSON, _ := json.Marshal(map[string]any{"spells": []string{"haste"}})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{
				ID: "haste", Name: "Haste", Level: 3, School: "transmutation",
				CastingTime:   "1 action",
				Duration:      "Concentration, up to 1 minute",
				Concentration: sql.NullBool{Bool: true, Valid: true},
			},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 1)
	assert.True(t, data.Spells[0].Concentration)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_Description(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 14, INT: 12, WIS: 10, CHA: 16})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Warlock", Level: 3}})
	charDataJSON, _ := json.Marshal(map[string]any{
		"background": "entertainer",
		"appearance": "tall tiefling, ember eyes",
		"backstory":  "ran from a pact gone wrong",
	})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Vale",
			Race:          "Tiefling",
			Level:         3,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, "tall tiefling, ember eyes", data.Appearance)
	assert.Equal(t, "ran from a pact gone wrong", data.Backstory)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_NoDescription(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Forge",
			Race:          "Dwarf",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Empty(t, data.Appearance)
	assert.Empty(t, data.Backstory)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_DDBSpells(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Wizard", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	// DDB format: array of objects with name, level, source
	type ddbSpell struct {
		Name   string `json:"name"`
		Level  int    `json:"level"`
		Source string `json:"source"`
	}
	charData := map[string]any{"spells": []ddbSpell{
		{Name: "Fireball", Level: 3, Source: "class"},
		{Name: "Fire Bolt", Level: 0, Source: "class"},
	}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{ID: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "Evocation", CastingTime: "1 action", RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 120, Valid: true}},
			{ID: "fireball", Name: "Fireball", Level: 3, School: "Evocation", CastingTime: "1 action", RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 150, Valid: true}},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 2)

	// DDB spells should be enriched with school, casting_time, range from reference table
	byID := make(map[string]portal.SpellDisplayEntry)
	for _, s := range data.Spells {
		byID[s.ID] = s
	}
	assert.Equal(t, "Fireball", byID["fireball"].Name)
	assert.Equal(t, 3, byID["fireball"].Level)
	assert.Equal(t, "Evocation", byID["fireball"].School)
	assert.Equal(t, "1 action", byID["fireball"].CastingTime)
	assert.Equal(t, "150ft", byID["fireball"].Range)
	assert.Equal(t, "class", byID["fireball"].Source) // Source should be preserved

	assert.Equal(t, "Fire Bolt", byID["fire-bolt"].Name)
	assert.Equal(t, 0, byID["fire-bolt"].Level)
	assert.Equal(t, "Evocation", byID["fire-bolt"].School)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_DDBSpellHomebrewTags(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Wizard", Level: 3}}
	classesJSON, _ := json.Marshal(classes)

	type ddbSpell struct {
		Name     string `json:"name"`
		Level    int    `json:"level"`
		Source   string `json:"source"`
		Homebrew bool   `json:"homebrew"`
		OffList  bool   `json:"off_list"`
	}
	charData := map[string]any{"spells": []ddbSpell{
		{Name: "Cure Wounds", Level: 1, Source: "class", Homebrew: true, OffList: true},
	}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Mira",
			Race:          "Human",
			Level:         3,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, School: "Evocation", CastingTime: "1 action", RangeType: "touch"},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 1)
	assert.True(t, data.Spells[0].Homebrew)
	assert.True(t, data.Spells[0].OffList)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_SpellRangeTouch(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Cleric", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charData := map[string]any{"spells": []string{"cure-wounds"}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Healer",
			Race:          "Human",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, School: "Evocation", CastingTime: "1 action", RangeType: "touch"},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 1)
	assert.Equal(t, "Touch", data.Spells[0].Range)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_PreparedSpells(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Cleric", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charData := map[string]any{
		"spells":          []string{"bless", "cure-wounds", "shield-of-faith"},
		"prepared_spells": []string{"bless", "cure-wounds"},
	}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Cleric",
			Race:          "Human",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{
			{ID: "bless", Name: "Bless", Level: 1, School: "Enchantment", CastingTime: "1 action", RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 30, Valid: true}},
			{ID: "cure-wounds", Name: "Cure Wounds", Level: 1, School: "Evocation", CastingTime: "1 action", RangeType: "touch"},
			{ID: "shield-of-faith", Name: "Shield of Faith", Level: 1, School: "Abjuration", CastingTime: "1 bonus action", RangeType: "ranged", RangeFt: sql.NullInt32{Int32: 60, Valid: true}},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 3)

	// Check prepared indicators are preserved after enrichment
	preparedByID := make(map[string]bool)
	for _, s := range data.Spells {
		preparedByID[s.ID] = s.Prepared
	}
	assert.True(t, preparedByID["bless"])
	assert.True(t, preparedByID["cure-wounds"])
	assert.False(t, preparedByID["shield-of-faith"])

	// Also check enrichment happened
	byID := make(map[string]portal.SpellDisplayEntry)
	for _, s := range data.Spells {
		byID[s.ID] = s
	}
	assert.Equal(t, "Bless", byID["bless"].Name)
	assert.Equal(t, 1, byID["bless"].Level)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_PortalSpells_NoRefData(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Wizard", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charData := map[string]any{"spells": []string{"unknown-spell"}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spells: []refdata.Spell{}, // no matches
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	require.Len(t, data.Spells, 1)
	// Falls back to ID as name when not enriched
	assert.Equal(t, "unknown-spell", data.Spells[0].Name)
	assert.Equal(t, "unknown-spell", data.Spells[0].ID)
	assert.Equal(t, 0, data.Spells[0].Level)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_SpellEnrichmentError(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 8, DEX: 14, CON: 12, INT: 18, WIS: 13, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Wizard", Level: 5}}
	classesJSON, _ := json.Marshal(classes)

	charData := map[string]any{"spells": []string{"fire-bolt"}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Gandalf",
			Race:          "Elf",
			Level:         5,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
		spellsErr: fmt.Errorf("db connection error"),
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	// Should still succeed, just without enrichment
	require.NoError(t, err)
	require.Len(t, data.Spells, 1)
	assert.Equal(t, "fire-bolt", data.Spells[0].Name) // falls back to ID
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_NoSpells(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 16, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Fighter", Level: 1}}
	classesJSON, _ := json.Marshal(classes)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Fighter",
			Race:          "Human",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Empty(t, data.Spells)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_InvalidCharacterData(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Fighter", Level: 1}}
	classesJSON, _ := json.Marshal(classes)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Broken",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: []byte(`not-json`), Valid: true},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Empty(t, data.Spells)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_EmptySpellsArray(t *testing.T) {
	charID := uuid.New()

	scores := character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10}
	scoresJSON, _ := json.Marshal(scores)
	classes := []character.ClassEntry{{Class: "Fighter", Level: 1}}
	classesJSON, _ := json.Marshal(classes)

	charData := map[string]any{"spells": []string{}}
	charDataJSON, _ := json.Marshal(charData)

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Empty",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Empty(t, data.Spells)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_InvalidID(t *testing.T) {
	store := portal.NewCharacterSheetStoreAdapter(&mockCharacterQuerier{})
	_, err := store.GetCharacterForSheet(context.Background(), "not-a-uuid")

	require.Error(t, err)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_NotFound(t *testing.T) {
	q := &mockCharacterQuerier{
		charErr: sql.ErrNoRows,
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	_, err := store.GetCharacterForSheet(context.Background(), uuid.New().String())

	require.Error(t, err)
	assert.ErrorIs(t, err, portal.ErrCharacterNotFound)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_PersistentConditionsAndExhaustion(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})
	charDataJSON, _ := json.Marshal(map[string]any{"exhaustion_level": 2})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Cursed",
			Race:          "Human",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
			Conditions:    json.RawMessage(`[{"condition":"poisoned"},{"condition":"prone"}]`),
		},
		// no combatant set: character is out of combat
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Equal(t, []string{"poisoned", "prone"}, data.Conditions)
	assert.Equal(t, 2, data.ExhaustionLevel)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_NoPersistentConditions(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Clean",
			Race:          "Human",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			// nil Conditions, no character_data
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	assert.Empty(t, data.Conditions)
	assert.Equal(t, 0, data.ExhaustionLevel)
}

func TestCharacterSheetStoreAdapter_GetCharacterForSheet_InCombatReplacesConditions(t *testing.T) {
	charID := uuid.New()

	scoresJSON, _ := json.Marshal(character.AbilityScores{STR: 10, DEX: 10, CON: 10, INT: 10, WIS: 10, CHA: 10})
	classesJSON, _ := json.Marshal([]character.ClassEntry{{Class: "Fighter", Level: 1}})
	charDataJSON, _ := json.Marshal(map[string]any{"exhaustion_level": 1})

	q := &mockCharacterQuerier{
		character: refdata.Character{
			ID:            charID,
			Name:          "Brawler",
			Race:          "Human",
			Level:         1,
			Classes:       classesJSON,
			AbilityScores: scoresJSON,
			CharacterData: pqtype.NullRawMessage{RawMessage: charDataJSON, Valid: true},
			// Persistent condition carried into combat.
			Conditions: json.RawMessage(`[{"condition":"poisoned"}]`),
		},
		combatant: &refdata.Combatant{
			// Live combat state: poisoned (carried in) + grappled (new).
			Conditions:      json.RawMessage(`[{"condition":"poisoned"},{"condition":"grappled"}]`),
			ExhaustionLevel: 3,
		},
	}

	store := portal.NewCharacterSheetStoreAdapter(q)
	data, err := store.GetCharacterForSheet(context.Background(), charID.String())

	require.NoError(t, err)
	// Combatant is the live source of truth: conditions are replaced, not appended.
	assert.Equal(t, []string{"poisoned", "grappled"}, data.Conditions)

	// "poisoned" exists both persistently and on the combatant; it must appear exactly once.
	count := 0
	for _, c := range data.Conditions {
		if c == "poisoned" {
			count++
		}
	}
	assert.Equal(t, 1, count)

	// Exhaustion is overlaid from the combatant during combat.
	assert.Equal(t, 3, data.ExhaustionLevel)
}
