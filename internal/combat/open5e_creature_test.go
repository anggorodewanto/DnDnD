package combat

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pqtypeNullRaw(raw json.RawMessage) pqtype.NullRawMessage {
	if len(raw) == 0 {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

// Phase 111 (iteration 2): Open5e creatures store their `actions` and
// `special_abilities` as Open5e prose payloads with the shape
// [{"name":"X","desc":"Y"}] (note: `desc`, not `description`). The combat
// turn builder previously unmarshalled them as structured
// CreatureAttackEntry/CreatureAbilityEntry, which silently produced empty
// ToHit/Damage/Description fields and broke NPC turn planning for any
// Open5e-sourced creature. The tests below lock in the Open5e-tolerant
// decode path that is exercised when a creature's Source column begins
// with "open5e:".

func TestParseCreatureAttacksWithSource_Open5eProse_ReturnsNil(t *testing.T) {
	prose := json.RawMessage(`[{"name":"Tentacle","desc":"Melee Weapon Attack: +7 to hit, reach 10 ft., one target. Hit: 10 (2d6+3) bludgeoning damage."}]`)
	source := sql.NullString{String: "open5e:tome-of-beasts", Valid: true}

	attacks, err := ParseCreatureAttacksWithSource(prose, source)

	require.NoError(t, err)
	assert.Nil(t, attacks, "open5e prose payload should not produce structured attacks")
}

func TestParseCreatureAttacksWithSource_SRDStructured_DecodesNormally(t *testing.T) {
	raw := json.RawMessage(`[{"name":"Bite","to_hit":5,"damage":"1d6+3","damage_type":"piercing","reach_ft":5}]`)
	source := sql.NullString{String: "SRD", Valid: true}

	attacks, err := ParseCreatureAttacksWithSource(raw, source)

	require.NoError(t, err)
	require.Len(t, attacks, 1)
	assert.Equal(t, "Bite", attacks[0].Name)
	assert.Equal(t, 5, attacks[0].ToHit)
}

func TestParseCreatureAttacksWithSource_NullSource_DecodesNormally(t *testing.T) {
	raw := json.RawMessage(`[{"name":"Bite","to_hit":5,"damage":"1d6+3","damage_type":"piercing","reach_ft":5}]`)
	attacks, err := ParseCreatureAttacksWithSource(raw, sql.NullString{})
	require.NoError(t, err)
	require.Len(t, attacks, 1)
	assert.Equal(t, "Bite", attacks[0].Name)
}

func TestParseCreatureAbilitiesFromCreature_Open5e_DecodesProseDesc(t *testing.T) {
	prose := json.RawMessage(`[{"name":"Keen Smell","desc":"The creature has advantage on Wisdom (Perception) checks that rely on smell."}]`)
	creature := refdata.Creature{
		Source: sql.NullString{String: "open5e:tome-of-beasts", Valid: true},
	}
	creature.Abilities.RawMessage = prose
	creature.Abilities.Valid = true

	got := parseCreatureAbilitiesFromCreature(creature)
	require.Len(t, got, 1)
	assert.Equal(t, "Keen Smell", got[0].Name)
	assert.Contains(t, got[0].Description, "advantage on Wisdom", "Open5e `desc` field should map to Description")
}

func TestParseCreatureAbilitiesFromCreature_Open5e_IncludesActionsAsAbilities(t *testing.T) {
	// When an open5e creature has attacks stored as prose, the turn builder
	// should still be able to surface them to the DM — we surface them via
	// the abilities list so the DM sees the raw text instead of silent
	// zero-value attack structs.
	actions := json.RawMessage(`[{"name":"Tentacle","desc":"Reach 10 ft."}]`)
	creature := refdata.Creature{
		Source: sql.NullString{String: "open5e:tome-of-beasts", Valid: true},
	}
	creature.Attacks = actions
	creature.Abilities = pqtypeNullRaw(json.RawMessage(`[]`))

	got := parseCreatureAbilitiesFromCreature(creature)
	require.Len(t, got, 1)
	assert.Equal(t, "Tentacle", got[0].Name)
	assert.Equal(t, "Reach 10 ft.", got[0].Description)
}

func TestParseCreatureAbilitiesFromCreature_SRDShapeUnchanged(t *testing.T) {
	structured := json.RawMessage(`[{"name":"Pack Tactics","description":"Advantage when ally within 5 ft."}]`)
	creature := refdata.Creature{
		Source: sql.NullString{String: "SRD", Valid: true},
	}
	creature.Abilities = pqtypeNullRaw(structured)

	got := parseCreatureAbilitiesFromCreature(creature)
	require.Len(t, got, 1)
	assert.Equal(t, "Pack Tactics", got[0].Name)
	assert.Equal(t, "Advantage when ally within 5 ft.", got[0].Description)
}
