package combat

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

// diceRollsFor wraps encoded swings into a NullRawMessage the way an
// action_log row would carry them.
func diceRollsFor(t *testing.T, swings ...attackSwing) pqtype.NullRawMessage {
	t.Helper()
	raw := encodeAttackSwings(swings)
	require.NotNil(t, raw)
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func TestEncodeAttackSwings_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, encodeAttackSwings(nil))
	assert.Nil(t, encodeAttackSwings([]attackSwing{}))
}

func TestAttackResultSwings(t *testing.T) {
	tid := uuid.New()

	t.Run("plain hit", func(t *testing.T) {
		got := attackResultSwings(AttackResult{Hit: true, DamageTotal: 7}, tid)
		require.Len(t, got, 1)
		assert.Equal(t, attackSwing{Target: tid.String(), Hit: true, Damage: 7}, got[0])
	})

	t.Run("critical hit", func(t *testing.T) {
		got := attackResultSwings(AttackResult{Hit: true, CriticalHit: true, DamageTotal: 18}, tid)
		require.Len(t, got, 1)
		assert.True(t, got[0].Crit)
		assert.Equal(t, 18, got[0].Damage)
	})

	t.Run("auto crit counts as crit", func(t *testing.T) {
		got := attackResultSwings(AttackResult{Hit: true, AutoCrit: true, DamageTotal: 12}, tid)
		require.Len(t, got, 1)
		assert.True(t, got[0].Crit)
	})

	t.Run("miss carries target but no damage", func(t *testing.T) {
		got := attackResultSwings(AttackResult{Hit: false}, tid)
		require.Len(t, got, 1)
		assert.Equal(t, attackSwing{Target: tid.String(), Hit: false}, got[0])
	})

	t.Run("cleave adds a second untargeted swing", func(t *testing.T) {
		got := attackResultSwings(AttackResult{
			Hit: true, DamageTotal: 5,
			CleaveAttack: &AttackResult{Hit: true, DamageTotal: 3},
		}, tid)
		require.Len(t, got, 2)
		assert.Equal(t, 5, got[0].Damage)
		assert.Equal(t, "", got[1].Target)
		assert.Equal(t, 3, got[1].Damage)
		assert.True(t, got[1].Hit)
	})

	t.Run("nil target omits target", func(t *testing.T) {
		got := attackResultSwings(AttackResult{Hit: true, DamageTotal: 4}, uuid.Nil)
		require.Len(t, got, 1)
		assert.Equal(t, "", got[0].Target)
	})
}

func TestEnemyTurnSwings(t *testing.T) {
	tid := uuid.New()
	plan := TurnPlan{Steps: []TurnStep{
		{Type: StepTypeMovement},
		{Type: StepTypeAttack, Attack: &AttackStep{
			TargetID:   tid,
			RollResult: &AttackRollResult{Hit: true, Critical: true, DamageTotal: 9, FinalDamage: 4, DamageResolved: true},
		}},
		{Type: StepTypeAttack, Attack: &AttackStep{
			TargetID:   tid,
			RollResult: &AttackRollResult{Hit: false},
		}},
		{Type: StepTypeAttack, Attack: &AttackStep{TargetID: tid, RollResult: nil}}, // unrolled → skipped
		{Type: StepTypeAttack, Attack: nil},                                         // no attack → skipped
	}}

	got := enemyTurnSwings(plan)
	require.Len(t, got, 2)
	// resolved damage preferred over rolled total
	assert.Equal(t, attackSwing{Target: tid.String(), Hit: true, Crit: true, Damage: 4}, got[0])
	assert.Equal(t, attackSwing{Target: tid.String(), Hit: false}, got[1])
}

func TestEnemyTurnSwings_UnresolvedFallsBackToRolledTotal(t *testing.T) {
	tid := uuid.New()
	plan := TurnPlan{Steps: []TurnStep{
		{Type: StepTypeAttack, Attack: &AttackStep{
			TargetID:   tid,
			RollResult: &AttackRollResult{Hit: true, DamageTotal: 9, DamageResolved: false},
		}},
	}}
	got := enemyTurnSwings(plan)
	require.Len(t, got, 1)
	assert.Equal(t, 9, got[0].Damage)
}

func TestAggregateCombatStats(t *testing.T) {
	aID := uuid.New() // Aragorn (PC)
	gID := uuid.New() // Goblin (NPC)
	combatants := []refdata.Combatant{
		{ID: aID, DisplayName: "Aragorn"},
		{ID: gID, DisplayName: "Goblin"},
	}

	rows := []refdata.ActionLog{
		// Aragorn's turn: hit 7, crit 18, miss vs Goblin
		{
			ActionType: actionTypeAttack,
			ActorID:    aID,
			TargetID:   uuid.NullUUID{UUID: gID, Valid: true},
			DiceRolls: diceRollsFor(t,
				attackSwing{Target: gID.String(), Hit: true, Damage: 7},
				attackSwing{Target: gID.String(), Hit: true, Crit: true, Damage: 18},
				attackSwing{Target: gID.String(), Hit: false},
			),
		},
		// Goblin's enemy turn: hit 5, miss vs Aragorn
		{
			ActionType: "enemy_turn",
			ActorID:    gID,
			DiceRolls: diceRollsFor(t,
				attackSwing{Target: aID.String(), Hit: true, Damage: 5},
				attackSwing{Target: aID.String(), Hit: false},
			),
		},
		// A row with no structured stats — skipped.
		{ActionType: "cast", ActorID: aID},
	}

	cs := AggregateCombatStats(rows, combatants)

	assert.Equal(t, 5, cs.Total)
	assert.Equal(t, 3, cs.Hits)
	assert.Equal(t, 2, cs.Misses)
	assert.Equal(t, 1, cs.Crits)

	a := cs.ByID[aID]
	require.NotNil(t, a)
	assert.Equal(t, "Aragorn", a.Name)
	assert.Equal(t, 3, a.Attacks)
	assert.Equal(t, 2, a.Hits)
	assert.Equal(t, 1, a.Misses)
	assert.Equal(t, 1, a.Crits)
	assert.Equal(t, 25, a.TotalDamage)
	assert.Equal(t, 18, a.MaxHit)
	assert.Equal(t, 1, a.Evaded)      // dodged the goblin's one miss
	assert.Equal(t, 5, a.DamageTaken) // soaked the goblin's one landed hit

	g := cs.ByID[gID]
	require.NotNil(t, g)
	assert.Equal(t, 2, g.Attacks)
	assert.Equal(t, 5, g.TotalDamage)
	assert.Equal(t, 1, g.Evaded)       // dodged Aragorn's one miss
	assert.Equal(t, 25, g.DamageTaken) // soaked Aragorn's 7 + 18
}

func TestAggregateCombatStats_IgnoresMalformedAndForeignRows(t *testing.T) {
	aID := uuid.New()
	rows := []refdata.ActionLog{
		{ActionType: actionTypeAttack, ActorID: aID, DiceRolls: pqtype.NullRawMessage{RawMessage: []byte(`{not json`), Valid: true}},
		{ActionType: actionTypeAttack, ActorID: aID, DiceRolls: pqtype.NullRawMessage{RawMessage: []byte(`{"kind":"other"}`), Valid: true}},
		{ActionType: actionTypeAttack, ActorID: aID}, // dice_rolls NULL
	}
	cs := AggregateCombatStats(rows, nil)
	assert.Equal(t, 0, cs.Total)
}

func TestFormatCombatStats_Empty(t *testing.T) {
	assert.Equal(t, "", FormatCombatStats(AggregateCombatStats(nil, nil), "Whatever"))
}

func TestFormatCombatStats_RendersAwards(t *testing.T) {
	aID := uuid.New()
	gID := uuid.New()
	combatants := []refdata.Combatant{
		{ID: aID, DisplayName: "Aragorn"},
		{ID: gID, DisplayName: "Goblin"},
	}
	rows := []refdata.ActionLog{
		{ActionType: actionTypeAttack, ActorID: aID, DiceRolls: diceRollsFor(t,
			attackSwing{Target: gID.String(), Hit: true, Crit: true, Damage: 18},
			attackSwing{Target: gID.String(), Hit: false},
		)},
		{ActionType: "enemy_turn", ActorID: gID, DiceRolls: diceRollsFor(t,
			attackSwing{Target: aID.String(), Hit: false},
		)},
	}
	out := FormatCombatStats(AggregateCombatStats(rows, combatants), "The Goblin Ambush")

	assert.Contains(t, out, "The Goblin Ambush")
	assert.Contains(t, out, "3 attacks")
	assert.Contains(t, out, "Biggest hit")
	assert.Contains(t, out, "Aragorn")
	assert.Contains(t, out, "18")
	assert.Contains(t, out, "Most evasive")
	// Goblin soaked Aragorn's 18-dmg crit; Aragorn took nothing → Goblin wins tank award
	assert.Contains(t, out, "Most damage tanked")
	// no crits by anyone with >0 besides Aragorn → Aragorn wins the crit award
	assert.Contains(t, out, "Most crits")
	// trailing newline trimmed
	assert.False(t, strings.HasSuffix(out, "\n"))
}

func TestEncounterDisplayName_PrefersDisplayName(t *testing.T) {
	assert.Equal(t, "Fancy", encounterDisplayName(refdata.Encounter{
		Name:        "raw",
		DisplayName: sql.NullString{String: "Fancy", Valid: true},
	}))
	assert.Equal(t, "raw", encounterDisplayName(refdata.Encounter{Name: "raw"}))
}
