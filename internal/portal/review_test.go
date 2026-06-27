package portal

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fullReviewCharacter builds a refdata.Character populated across every column
// and character_data bag field the DM review projection reads.
func fullReviewCharacter() refdata.Character {
	return refdata.Character{
		ID:            uuid.New(),
		CampaignID:    uuid.New(),
		Name:          "Thorin",
		Race:          "dwarf",
		Classes:       json.RawMessage(`[{"class":"fighter","subclass":"champion","level":3,"is_primary":true},{"class":"rogue","level":1}]`),
		Level:         4,
		AbilityScores: json.RawMessage(`{"str":16,"dex":14,"con":15,"int":10,"wis":12,"cha":8}`),
		HpMax:         34,
		Ac:            18,
		SpeedFt:       25,
		Languages:     []string{"dwarvish", "common"},
		Proficiencies: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"saves":["str","con"],"skills":["intimidation","athletics"],"expertise":["athletics"]}`),
			Valid:      true,
		},
		Features: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`[{"name":"Second Wind"},{"name":"Action Surge"}]`),
			Valid:      true,
		},
		Inventory: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`[{"item_id":"shield","name":"Shield"},{"item_id":"longsword","name":"Longsword"}]`),
			Valid:      true,
		},
		CharacterData: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"subrace":"hill-dwarf","background":"soldier","spells":["mage-hand","fireball"],"weapon_masteries":["longsword"],"appearance":"Stocky and bearded.","backstory":"Exiled heir."}`),
			Valid:      true,
		},
	}
}

func TestProjectReview_FullCharacter(t *testing.T) {
	got := ProjectReview(fullReviewCharacter())

	assert.Equal(t, "Thorin", got.Name)
	assert.Equal(t, "dwarf", got.Race)
	assert.Equal(t, "hill-dwarf", got.Subrace)
	assert.Equal(t, "soldier", got.Background)
	// Classes render as readable labels, primary first (stored order preserved).
	assert.Equal(t, []string{"Fighter 3 (Champion)", "Rogue 1"}, got.Classes)
	assert.Equal(t, int32(4), got.Level)
	assert.Equal(t, character.AbilityScores{STR: 16, DEX: 14, CON: 15, INT: 10, WIS: 12, CHA: 8}, got.AbilityScores)
	assert.Equal(t, int32(34), got.HPMax)
	assert.Equal(t, int32(18), got.AC)
	assert.Equal(t, int32(25), got.SpeedFt)
	// Set-like lists are sorted so re-ordering never shows as a spurious diff.
	assert.Equal(t, []string{"athletics", "intimidation"}, got.Skills)
	assert.Equal(t, []string{"athletics"}, got.Expertise)
	assert.Equal(t, []string{"con", "str"}, got.Saves)
	assert.Equal(t, []string{"common", "dwarvish"}, got.Languages)
	assert.Equal(t, []string{"Longsword", "Shield"}, got.Equipment)
	assert.Equal(t, []string{"fireball", "mage-hand"}, got.Spells)
	assert.Equal(t, []string{"longsword"}, got.WeaponMasteries)
	assert.Equal(t, []string{"Action Surge", "Second Wind"}, got.Features)
	assert.Equal(t, "Stocky and bearded.", got.Appearance)
	assert.Equal(t, "Exiled heir.", got.Backstory)
}

func TestProjectReview_EmptyOptionalFields(t *testing.T) {
	ch := refdata.Character{
		Name:          "Blank",
		Race:          "human",
		Classes:       json.RawMessage(`[]`),
		AbilityScores: json.RawMessage(`{"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10}`),
		Level:         1,
	}
	got := ProjectReview(ch)

	assert.Equal(t, "Blank", got.Name)
	assert.Empty(t, got.Subrace)
	assert.Empty(t, got.Background)
	// Slices must be non-nil so they marshal to [] (not null) and never diff
	// spuriously against an empty before-snapshot.
	assert.NotNil(t, got.Classes)
	assert.NotNil(t, got.Skills)
	assert.NotNil(t, got.Saves)
	assert.NotNil(t, got.Languages)
	assert.NotNil(t, got.Equipment)
	assert.NotNil(t, got.Spells)
	assert.NotNil(t, got.Features)
	assert.Empty(t, got.Classes)
	assert.Empty(t, got.Skills)
}

func TestProjectReview_SortsSetLikeLists(t *testing.T) {
	ch := refdata.Character{
		Name:          "Sorter",
		AbilityScores: json.RawMessage(`{}`),
		Languages:     []string{"zzz", "aaa", "mmm"},
		Proficiencies: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"skills":["stealth","acrobatics"],"saves":["wis","dex"]}`),
			Valid:      true,
		},
		CharacterData: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`{"spells":["zone-of-truth","aid"]}`),
			Valid:      true,
		},
	}
	got := ProjectReview(ch)

	assert.Equal(t, []string{"aaa", "mmm", "zzz"}, got.Languages)
	assert.Equal(t, []string{"acrobatics", "stealth"}, got.Skills)
	assert.Equal(t, []string{"dex", "wis"}, got.Saves)
	assert.Equal(t, []string{"aid", "zone-of-truth"}, got.Spells)
}

// The projection must marshal to JSON whose top-level keys match what the
// frontend diff (diffStates) compares, with arrays as [] rather than null.
func TestProjectReview_MarshalsStableJSON(t *testing.T) {
	blob, err := json.Marshal(ProjectReview(fullReviewCharacter()))
	require.NoError(t, err)

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(blob, &decoded))
	for _, key := range []string{"name", "race", "classes", "ability_scores", "skills", "saves", "languages", "equipment", "spells", "features"} {
		_, ok := decoded[key]
		assert.True(t, ok, "expected JSON key %q in review projection", key)
	}
}
