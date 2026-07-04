package combat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// makeSecondWindChar builds a Fighter character (reusing makeFighterChar) with
// a seeded Second Wind pool instead of the default Action Surge pool.
func makeSecondWindChar(charID uuid.UUID, fighterLevel, uses int) refdata.Character {
	char := makeFighterChar(charID, fighterLevel, 0)
	featureUsesJSON, _ := json.Marshal(map[string]character.FeatureUse{
		FeatureKeySecondWind: {Current: uses, Max: 1, Recharge: "short"},
	})
	char.FeatureUses = pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true}
	return char
}

// makeWoundedFighter builds a fighter combatant (reusing makeFighterCombatant)
// wounded to the given current HP.
func makeWoundedFighter(combatantID, encounterID, charID uuid.UUID, hpCurrent int32) refdata.Combatant {
	c := makeFighterCombatant(combatantID, encounterID, charID)
	c.HpCurrent = hpCurrent
	return c
}

// secondWindStore wires a mockStore for the Second Wind happy path and captures
// the HP write.
func secondWindStore(char refdata.Character) (*mockStore, *[]refdata.UpdateCombatantHPParams) {
	ms := defaultMockStore()
	hpWrites := &[]refdata.UpdateCombatantHPParams{}
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.updateCharacterFeatureUsesFn = func(_ context.Context, arg refdata.UpdateCharacterFeatureUsesParams) (refdata.Character, error) {
		return refdata.Character{ID: arg.ID, FeatureUses: arg.FeatureUses}, nil
	}
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	ms.updateCombatantHPFn = func(_ context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		*hpWrites = append(*hpWrites, arg)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, TempHp: arg.TempHp, IsAlive: arg.IsAlive}, nil
	}
	return ms, hpWrites
}

func TestSecondWind_HappyPath_HealsAndSpendsBonusAction(t *testing.T) {
	encounterID, combatantID, charID := uuid.New(), uuid.New(), uuid.New()
	char := makeSecondWindChar(charID, 5, 1)
	fighter := makeWoundedFighter(combatantID, encounterID, charID, 10)
	ms, hpWrites := secondWindStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(_ int) int { return 7 }) // d10 -> 7; +5 level = 12

	turn := refdata.Turn{ID: uuid.New()}
	result, err := svc.SecondWind(context.Background(), SecondWindCommand{Fighter: fighter, Turn: turn}, roller)
	require.NoError(t, err)

	assert.Equal(t, int32(12), result.HPRestored)
	assert.Equal(t, int32(22), result.HPAfter)
	assert.Equal(t, 0, result.UsesRemaining)
	assert.True(t, result.Turn.BonusActionUsed, "bonus action must be consumed")
	assert.Contains(t, result.CombatLog, "Second Wind")
	assert.Contains(t, result.CombatLog, "12 HP")

	require.Len(t, *hpWrites, 1)
	assert.Equal(t, int32(22), (*hpWrites)[0].HpCurrent)
}

func TestSecondWind_CapsAtMaxHP(t *testing.T) {
	encounterID, combatantID, charID := uuid.New(), uuid.New(), uuid.New()
	char := makeSecondWindChar(charID, 5, 1)
	fighter := makeWoundedFighter(combatantID, encounterID, charID, 40) // 40/44
	ms, hpWrites := secondWindStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(_ int) int { return 10 }) // d10 -> 10; +5 = 15, but only 4 headroom

	result, err := svc.SecondWind(context.Background(), SecondWindCommand{Fighter: fighter, Turn: refdata.Turn{ID: uuid.New()}}, roller)
	require.NoError(t, err)

	assert.Equal(t, int32(4), result.HPRestored)
	assert.Equal(t, int32(44), result.HPAfter)
	require.Len(t, *hpWrites, 1)
	assert.Equal(t, int32(44), (*hpWrites)[0].HpCurrent)
}

func TestSecondWind_NoUsesRemaining(t *testing.T) {
	encounterID, combatantID, charID := uuid.New(), uuid.New(), uuid.New()
	char := makeSecondWindChar(charID, 5, 0) // pool empty
	fighter := makeWoundedFighter(combatantID, encounterID, charID, 10)
	ms, hpWrites := secondWindStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(_ int) int { return 7 })

	_, err := svc.SecondWind(context.Background(), SecondWindCommand{Fighter: fighter, Turn: refdata.Turn{ID: uuid.New()}}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Second Wind uses remaining")
	assert.Empty(t, *hpWrites, "no HP write when pool empty")
}

func TestSecondWind_BonusActionAlreadyUsed(t *testing.T) {
	encounterID, combatantID, charID := uuid.New(), uuid.New(), uuid.New()
	char := makeSecondWindChar(charID, 5, 1)
	fighter := makeWoundedFighter(combatantID, encounterID, charID, 10)
	ms, hpWrites := secondWindStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(_ int) int { return 7 })

	turn := refdata.Turn{ID: uuid.New(), BonusActionUsed: true}
	_, err := svc.SecondWind(context.Background(), SecondWindCommand{Fighter: fighter, Turn: turn}, roller)
	require.Error(t, err)
	assert.Empty(t, *hpWrites, "no HP write when bonus action unavailable")
}

func TestSecondWind_NotAFighter(t *testing.T) {
	encounterID, combatantID, charID := uuid.New(), uuid.New(), uuid.New()
	classesJSON, _ := json.Marshal([]CharacterClass{{Class: "Wizard", Level: 5}})
	char := makeSecondWindChar(charID, 5, 1)
	char.Classes = classesJSON // override to non-fighter
	fighter := makeWoundedFighter(combatantID, encounterID, charID, 10)
	ms, hpWrites := secondWindStore(char)

	svc := NewService(ms)
	roller := dice.NewRoller(func(_ int) int { return 7 })

	_, err := svc.SecondWind(context.Background(), SecondWindCommand{Fighter: fighter, Turn: refdata.Turn{ID: uuid.New()}}, roller)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "fighter")
	assert.Empty(t, *hpWrites)
}

func TestSecondWind_NPCRejected(t *testing.T) {
	encounterID, combatantID := uuid.New(), uuid.New()
	npc := makeWoundedFighter(combatantID, encounterID, uuid.New(), 10)
	npc.CharacterID = uuid.NullUUID{} // NPC has no character
	npc.IsNpc = true

	ms := defaultMockStore()
	svc := NewService(ms)
	roller := dice.NewRoller(func(_ int) int { return 7 })

	_, err := svc.SecondWind(context.Background(), SecondWindCommand{Fighter: npc, Turn: refdata.Turn{ID: uuid.New()}}, roller)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NPC")
}

func TestSecondWindHealDice(t *testing.T) {
	assert.Equal(t, "1d10+1", SecondWindHealDice(1))
	assert.Equal(t, "1d10+9", SecondWindHealDice(9))
}
