package combat

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// A creature with structured attacks + a recharge ability surfaces both, so the
// DM can run its turn from the Console without opening the stat block (ISSUE-027).
func TestBuildCreatureTurnSummary_AttacksAndRecharge(t *testing.T) {
	creature := refdata.Creature{
		ID:   "young-red-dragon",
		Name: "Young Red Dragon",
		Attacks: json.RawMessage(`[
			{"name":"Bite","to_hit":10,"damage":"2d10+6","damage_type":"piercing","reach_ft":10},
			{"name":"Claw","to_hit":10,"damage":"2d6+6","damage_type":"slashing","reach_ft":5}
		]`),
		Abilities: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`[{"name":"Fire Breath (Recharge 5-6)","description":"Exhales fire in a 30-foot cone."}]`),
			Valid:      true,
		},
	}

	s := BuildCreatureTurnSummary(creature)

	require.Len(t, s.Attacks, 2)
	assert.Equal(t, "Bite", s.Attacks[0].Name)
	assert.Equal(t, 10, s.Attacks[0].ToHit)
	assert.Equal(t, "2d10+6", s.Attacks[0].Damage)
	assert.Equal(t, "piercing", s.Attacks[0].DamageType)
	assert.Equal(t, 10, s.Attacks[0].ReachFt)

	require.Len(t, s.RechargeAbilities, 1)
	assert.Equal(t, "Fire Breath (Recharge 5-6)", s.RechargeAbilities[0].Name)
	assert.Equal(t, 5, s.RechargeAbilities[0].RechargeMin)

	assert.False(t, s.IsEmpty())
}

// Legendary + lair headers surface as availability flags (with the legendary
// action budget parsed from the header prose).
func TestBuildCreatureTurnSummary_LegendaryAndLair(t *testing.T) {
	creature := refdata.Creature{
		ID:      "adult-red-dragon",
		Name:    "Adult Red Dragon",
		Attacks: json.RawMessage(`[{"name":"Bite","to_hit":14,"damage":"2d10+8","damage_type":"piercing","reach_ft":10}]`),
		Abilities: pqtype.NullRawMessage{
			RawMessage: json.RawMessage(`[
				{"name":"Legendary Actions","description":"The dragon can take 3 legendary actions."},
				{"name":"Detect","description":"The dragon makes a Wisdom (Perception) check."},
				{"name":"Tail Attack (Costs 2 Actions)","description":"The dragon makes a tail attack."},
				{"name":"Lair Actions","description":"On initiative count 20 the dragon takes a lair action."}
			]`),
			Valid: true,
		},
	}

	s := BuildCreatureTurnSummary(creature)

	assert.True(t, s.HasLegendary)
	assert.Equal(t, 3, s.LegendaryBudget)
	assert.True(t, s.HasLair)
}

// A plain creature with no attacks/abilities yields an empty summary, so the
// adapter can omit creature_summary entirely.
func TestBuildCreatureTurnSummary_EmptyForStatelessCreature(t *testing.T) {
	s := BuildCreatureTurnSummary(refdata.Creature{ID: "commoner", Name: "Commoner"})

	assert.True(t, s.IsEmpty())
	assert.Empty(t, s.Attacks)
	assert.Empty(t, s.RechargeAbilities)
	assert.False(t, s.HasLegendary)
	assert.Zero(t, s.LegendaryBudget)
	assert.False(t, s.HasLair)
}

// Malformed attack JSON is tolerated (best-effort, like the rest of the Console
// adapter) rather than failing the whole situation payload.
func TestBuildCreatureTurnSummary_MalformedAttacksTolerated(t *testing.T) {
	s := BuildCreatureTurnSummary(refdata.Creature{
		ID:      "broken",
		Name:    "Broken",
		Attacks: json.RawMessage(`{not valid json`),
	})

	assert.Empty(t, s.Attacks)
	assert.True(t, s.IsEmpty())
}

// open5e:* rows hold prose, not structured attacks, so no structured attacks
// are surfaced (mirrors ParseCreatureAttacksWithSource / the turn builder).
func TestBuildCreatureTurnSummary_Open5eProseHasNoStructuredAttacks(t *testing.T) {
	s := BuildCreatureTurnSummary(refdata.Creature{
		ID:      "kobold",
		Name:    "Kobold",
		Source:  sql.NullString{String: "open5e:tome-of-beasts", Valid: true},
		Attacks: json.RawMessage(`[{"name":"Dagger","desc":"Melee weapon attack."}]`),
	})

	assert.Empty(t, s.Attacks)
}
