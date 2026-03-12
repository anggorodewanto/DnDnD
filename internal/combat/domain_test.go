package combat

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// --- TDD Cycle 1: CombatantFromCreature copies stats correctly ---

func TestCombatantFromCreature(t *testing.T) {
	creature := refdata.Creature{
		ID:        "goblin",
		Name:      "Goblin",
		Ac:        15,
		HpAverage: 7,
		Speed:     json.RawMessage(`{"walk":30}`),
	}

	params := CombatantFromCreature(creature, "G1", "Goblin 1", "B", 3)

	assert.Equal(t, "goblin", params.CreatureRefID)
	assert.Equal(t, "G1", params.ShortID)
	assert.Equal(t, "Goblin 1", params.DisplayName)
	assert.Equal(t, int32(15), params.AC)
	assert.Equal(t, int32(7), params.HPMax)
	assert.Equal(t, int32(7), params.HPCurrent)
	assert.Equal(t, "B", params.PositionCol)
	assert.Equal(t, int32(3), params.PositionRow)
	assert.True(t, params.IsNPC)
	assert.True(t, params.IsAlive)
	assert.True(t, params.IsVisible)
	assert.Equal(t, int32(30), params.SpeedFt)
}

// --- TDD Cycle 2: CombatantFromCreature with no walk speed defaults to 30 ---

func TestCombatantFromCreature_NoWalkSpeed(t *testing.T) {
	creature := refdata.Creature{
		ID:        "ooze",
		Name:      "Ooze",
		Ac:        8,
		HpAverage: 22,
		Speed:     json.RawMessage(`{"swim":20}`),
	}

	params := CombatantFromCreature(creature, "O1", "Ooze 1", "A", 1)
	assert.Equal(t, int32(30), params.SpeedFt)
}

// --- TDD Cycle 3: CombatantFromCreature with invalid speed JSON defaults to 30 ---

func TestCombatantFromCreature_InvalidSpeedJSON(t *testing.T) {
	creature := refdata.Creature{
		ID:        "weird",
		Name:      "Weird",
		Ac:        10,
		HpAverage: 5,
		Speed:     json.RawMessage(`invalid`),
	}

	params := CombatantFromCreature(creature, "W1", "Weird 1", "C", 2)
	assert.Equal(t, int32(30), params.SpeedFt)
}

// --- TDD Cycle 4: CombatantFromCharacter copies stats correctly ---

func TestCombatantFromCharacter(t *testing.T) {
	charID := uuid.New()
	char := refdata.Character{
		ID:        charID,
		Name:      "Aragorn",
		HpMax:     45,
		HpCurrent: 42,
		TempHp:    5,
		Ac:        16,
		SpeedFt:   30,
	}

	params := CombatantFromCharacter(char, "AR", "A", 5)

	assert.Equal(t, charID.String(), params.CharacterID)
	assert.Equal(t, "AR", params.ShortID)
	assert.Equal(t, "Aragorn", params.DisplayName)
	assert.Equal(t, int32(45), params.HPMax)
	assert.Equal(t, int32(42), params.HPCurrent)
	assert.Equal(t, int32(5), params.TempHP)
	assert.Equal(t, int32(16), params.AC)
	assert.Equal(t, int32(30), params.SpeedFt)
	assert.Equal(t, "A", params.PositionCol)
	assert.Equal(t, int32(5), params.PositionRow)
	assert.False(t, params.IsNPC)
	assert.True(t, params.IsAlive)
	assert.True(t, params.IsVisible)
	// PCs should have death saves initialized
	require.NotNil(t, params.DeathSaves)
	var ds DeathSaves
	err := json.Unmarshal(params.DeathSaves, &ds)
	require.NoError(t, err)
	assert.Equal(t, 0, ds.Successes)
	assert.Equal(t, 0, ds.Failures)
}

// --- TDD Cycle 5: ParseTemplateCreatures ---

func TestParseTemplateCreatures(t *testing.T) {
	raw := json.RawMessage(`[
		{"creature_ref_id":"goblin","short_id":"G1","display_name":"Goblin 1","position_col":"A","position_row":1,"quantity":2},
		{"creature_ref_id":"ogre","short_id":"O1","display_name":"Ogre Boss","position_col":"C","position_row":3,"quantity":1}
	]`)

	creatures, err := ParseTemplateCreatures(raw)
	require.NoError(t, err)
	require.Len(t, creatures, 2)
	assert.Equal(t, "goblin", creatures[0].CreatureRefID)
	assert.Equal(t, 2, creatures[0].Quantity)
	assert.Equal(t, "ogre", creatures[1].CreatureRefID)
}

func TestParseTemplateCreatures_InvalidJSON(t *testing.T) {
	_, err := ParseTemplateCreatures(json.RawMessage(`invalid`))
	require.Error(t, err)
}

func TestParseTemplateCreatures_Empty(t *testing.T) {
	creatures, err := ParseTemplateCreatures(json.RawMessage(`[]`))
	require.NoError(t, err)
	assert.Empty(t, creatures)
}

func TestParseTemplateCreatures_Nil(t *testing.T) {
	creatures, err := ParseTemplateCreatures(nil)
	require.NoError(t, err)
	assert.Empty(t, creatures)
}
