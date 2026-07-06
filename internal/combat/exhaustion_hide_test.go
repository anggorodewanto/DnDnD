package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// Hide rolls a Stealth check — a 2024 D20 Test — so exhaustion's -2×level
// penalty applies to the roll. resolveHide is shared by both /action hide and
// /bonus cunning-action hide, so folding it there covers both entrypoints.
// These cases pin fixed dice (deterministic shows 19) that WITHOUT exhaustion
// beat the hostile's passive Perception, then flip the outcome purely by
// raising the hider's ExhaustionLevel.
func TestHide_Exhaustion(t *testing.T) {
	// Perception 15 on the goblin → passive Perception 25.
	watchfulGoblin := func(context.Context, string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":10,"cha":8}`),
			Skills:        pqtype.NullRawMessage{RawMessage: json.RawMessage(`{"perception":15}`), Valid: true},
		}, nil
	}

	// DEX 14 (+2), proficiency 3, expertise in stealth → +2 + 3*2 = +8.
	// deterministic 19 → base Stealth 27.
	setupHider := func() (HideCommand, *mockStore) {
		encounterID, combatantID, charID, ms := makeStdTestSetup()
		combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
		char := makeBasicChar(charID, 30)
		char.AbilityScores = json.RawMessage(`{"str":10,"dex":14,"con":12,"int":10,"wis":10,"cha":8}`)
		char.ProficiencyBonus = 3
		char.Proficiencies = profJSON([]string{"stealth"}, []string{"stealth"}, false)
		ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) { return char, nil }
		ms.getCreatureFn = watchfulGoblin
		setupUpdateTurnActions(ms)
		hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")
		cmd := HideCommand{
			Combatant: combatant,
			Turn:      makeBasicTurn(),
			Encounter: makeBasicEncounter(encounterID, 1),
			Hostiles:  []refdata.Combatant{hostile},
		}
		return cmd, ms
	}

	t.Run("no exhaustion — beats passive Perception", func(t *testing.T) {
		cmd, ms := setupHider()
		result, err := NewService(ms).Hide(context.Background(), cmd, dice.NewRoller(deterministic))
		require.NoError(t, err)
		assert.Equal(t, 27, result.StealthRoll)
		assert.True(t, result.Success) // 27 > 25
	})

	t.Run("exhaustion 2 lowers the roll and flips to spotted", func(t *testing.T) {
		cmd, ms := setupHider()
		cmd.Combatant.ExhaustionLevel = 2 // -4 to the Stealth D20 Test
		result, err := NewService(ms).Hide(context.Background(), cmd, dice.NewRoller(deterministic))
		require.NoError(t, err)
		assert.Equal(t, 23, result.StealthRoll) // 27 - 4
		assert.False(t, result.Success)         // 23 < 25
	})
}

// TestCunningActionHide_Exhaustion pins that /bonus cunning-action hide, which
// routes through the same resolveHide, also folds the exhaustion penalty.
func TestCunningActionHide_Exhaustion(t *testing.T) {
	encounterID, combatantID, charID, ms := makeStdTestSetup()
	combatant := makePCCombatant(combatantID, encounterID, charID, "Shadow")
	combatant.ExhaustionLevel = 3 // -6 to the Stealth D20 Test
	char := makeRogueChar(charID, 5)
	char.Proficiencies = profJSON([]string{"stealth"}, nil, false) // stealth mod = DEX +4 + PB 3 = +7
	ms.getCharacterFn = func(context.Context, uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getCreatureFn = func(context.Context, string) (refdata.Creature, error) {
		return refdata.Creature{
			ID:            "goblin",
			AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`), // PP 9
		}, nil
	}
	setupUpdateTurnActions(ms)
	hostile := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin", "goblin")

	cmd := CunningActionCommand{
		Combatant: combatant,
		Turn:      makeBasicTurn(),
		Encounter: makeBasicEncounter(encounterID, 1),
		Action:    "hide",
		Hostiles:  []refdata.Combatant{hostile},
	}
	result, err := NewService(ms).CunningAction(context.Background(), cmd, dice.NewRoller(deterministic))
	require.NoError(t, err)
	// deterministic 19 + stealth 7 - 6 exhaustion = 20.
	assert.Equal(t, 20, result.HideResult.StealthRoll)
}
