package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// SR-018: end-to-end service tests for Help action advantage. The
// help_advantage condition lives on the attacker and is consumed after a
// single attack vs the named target.
//
// Three cases match the SR acceptance criteria:
//   - attack #1 vs the named target rolls with advantage and clears the condition
//   - attack #2 vs the same target (after consume) rolls without advantage
//   - an attack vs a DIFFERENT target leaves the condition in place and rolls without advantage

// helpAdvSetup builds a mockStore wired for Service.Attack so we can observe
// the conditions JSONB written back per UpdateCombatantConditions call.
type helpAdvSetup struct {
	ms        *mockStore
	attacker  refdata.Combatant
	target    refdata.Combatant
	other     refdata.Combatant
	turn      refdata.Turn
	mu        sync.Mutex
	condsSeen map[uuid.UUID]json.RawMessage
}

func newHelpAdvSetup(t *testing.T, attackerConditions json.RawMessage) *helpAdvSetup {
	t.Helper()
	encounterID := uuid.New()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	otherID := uuid.New()

	char := makeCharacter(16, 14, 2, "longsword")
	char.ID = charID

	hs := &helpAdvSetup{
		condsSeen: make(map[uuid.UUID]json.RawMessage),
	}

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		if id == "longsword" {
			return makeLongsword(), nil
		}
		return refdata.Weapon{}, sql.ErrNoRows
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		hs.mu.Lock()
		hs.condsSeen[arg.ID] = arg.Conditions
		hs.mu.Unlock()
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	// Encounter/zone lookups used by resolveObscurement; default-empty.
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}

	hs.ms = ms
	hs.attacker = refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  attackerConditions,
	}
	hs.target = refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Goblin",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	hs.other = refdata.Combatant{
		ID:          otherID,
		EncounterID: encounterID,
		DisplayName: "Orc",
		PositionCol: "B",
		PositionRow: 2,
		Ac:          13,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	hs.turn = refdata.Turn{
		ID:               uuid.New(),
		EncounterID:      encounterID,
		CombatantID:      attackerID,
		AttacksRemaining: 2,
	}
	return hs
}

func helpAdvCondition(t *testing.T, targetID uuid.UUID) json.RawMessage {
	t.Helper()
	conds := []CombatCondition{{
		Condition:         "help_advantage",
		DurationRounds:    1,
		StartedRound:      1,
		SourceCombatantID: uuid.New().String(), // helper, distinct from attacker
		TargetCombatantID: targetID.String(),
		ExpiresOn:         "start_of_turn",
	}}
	raw, err := json.Marshal(conds)
	require.NoError(t, err)
	return raw
}

// rollerSeq returns a roller that cycles through the supplied d20 values.
// Non-d20 rolls return max-1 deterministically (cf. helpers in standard_actions_test.go).
func rollerSeq(d20s ...int) (*dice.Roller, *int) {
	idx := 0
	r := dice.NewRoller(func(max int) int {
		if max == 20 {
			v := d20s[idx%len(d20s)]
			idx++
			return v
		}
		return max - 1
	})
	return r, &idx
}

// Cycle 1: First attack vs the help target rolls with Advantage and clears
// the condition on the attacker.
func TestHelpAdvantage_FirstAttackHasAdvantage_ConditionConsumed(t *testing.T) {
	hs := newHelpAdvSetup(t, json.RawMessage(`[]`))
	hs.attacker.Conditions = helpAdvCondition(t, hs.target.ID)

	roller, _ := rollerSeq(18)

	result, err := NewService(hs.ms).Attack(context.Background(), AttackCommand{
		Attacker: hs.attacker,
		Target:   hs.target,
		Turn:     hs.turn,
	}, roller)
	require.NoError(t, err)

	assert.Equal(t, dice.Advantage, result.RollMode, "first attack vs help target must roll with advantage")
	assert.Contains(t, result.AdvantageReasons, "help advantage")

	hs.mu.Lock()
	stored, ok := hs.condsSeen[hs.attacker.ID]
	hs.mu.Unlock()
	require.True(t, ok, "expected attacker conditions to be updated (help_advantage consumed)")
	assert.False(t, HasCondition(stored, "help_advantage"), "help_advantage must be cleared after first attack vs the named target")
}

// Cycle 2: A second attack vs the same target after the first consume has no
// advantage. This simulates the consumed state: the in-memory attacker row no
// longer carries the condition.
func TestHelpAdvantage_SecondAttackSameTarget_NoAdvantage(t *testing.T) {
	hs := newHelpAdvSetup(t, json.RawMessage(`[]`)) // condition already consumed

	roller, _ := rollerSeq(18)

	result, err := NewService(hs.ms).Attack(context.Background(), AttackCommand{
		Attacker: hs.attacker,
		Target:   hs.target,
		Turn:     hs.turn,
	}, roller)
	require.NoError(t, err)

	assert.Equal(t, dice.Normal, result.RollMode, "second attack vs same target must not have advantage")
	assert.NotContains(t, result.AdvantageReasons, "help advantage")
}

// Cycle 3: An attack vs a DIFFERENT target (not the one named by Help) rolls
// without advantage AND leaves the help_advantage condition in place so the
// helped creature can still spend it on the named target later.
func TestHelpAdvantage_DifferentTarget_NoAdvantage_ConditionPreserved(t *testing.T) {
	hs := newHelpAdvSetup(t, json.RawMessage(`[]`))
	hs.attacker.Conditions = helpAdvCondition(t, hs.target.ID) // help vs Goblin

	roller, _ := rollerSeq(18)

	result, err := NewService(hs.ms).Attack(context.Background(), AttackCommand{
		Attacker: hs.attacker,
		Target:   hs.other, // attack Orc instead
		Turn:     hs.turn,
	}, roller)
	require.NoError(t, err)

	assert.Equal(t, dice.Normal, result.RollMode, "attack vs different target must not have advantage")
	assert.NotContains(t, result.AdvantageReasons, "help advantage")

	hs.mu.Lock()
	_, condUpdated := hs.condsSeen[hs.attacker.ID]
	hs.mu.Unlock()
	assert.False(t, condUpdated, "help_advantage scoped to a different target must NOT be cleared by an attack vs another combatant")
}
