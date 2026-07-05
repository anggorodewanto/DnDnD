package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// shieldMasterChar builds a level-5 fighter (STR 16 → +3) with the Shield Master
// feat and a shield equipped in the off hand.
func shieldMasterChar(charID uuid.UUID) refdata.Character {
	char := makeBasicChar(charID, 30)
	feats := []CharacterFeature{{Name: "Shield Master", MechanicalEffect: `[{"effect_type":"bonus_action_shield_shove"}]`}}
	featsJSON, _ := json.Marshal(feats)
	char.Features = pqtype.NullRawMessage{RawMessage: featsJSON, Valid: true}
	char.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	return char
}

// shieldMasterFixture wires a shover (Shield Master, shield equipped, attack
// already made) adjacent to a goblin, plus the store fns the shove path needs.
func shieldMasterFixture() (*mockStore, refdata.Combatant, refdata.Combatant, refdata.Turn, uuid.UUID) {
	encounterID := uuid.New()
	charID := uuid.New()
	ms := defaultMockStore()

	shover := makePCCombatant(uuid.New(), encounterID, charID, "Aria")
	shover.PositionCol = "C"
	shover.PositionRow = 3
	target := makeNPCCombatantWithCreature(uuid.New(), encounterID, "Goblin #1", "goblin")
	target.PositionCol = "D"
	target.PositionRow = 3

	char := shieldMasterChar(charID)
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getCreatureFn = func(_ context.Context, _ string) (refdata.Creature, error) {
		return refdata.Creature{ID: "goblin", Size: "Small", AbilityScores: json.RawMessage(`{"str":8,"dex":14,"con":10,"int":10,"wis":8,"cha":8}`)}, nil
	}
	ms.getArmorFn = func(_ context.Context, _ string) (refdata.Armor, error) {
		return refdata.Armor{ID: "shield", ArmorType: "shield", AcBase: 2}, nil
	}
	ms.updateCombatantConditionsFn = func(_ context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions, DisplayName: "Goblin #1"}, nil
	}
	ms.updateCombatantPositionFn = func(_ context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, DisplayName: "Goblin #1"}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(_ context.Context, _ uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{shover, target}, nil
	}
	setupUpdateTurnActions(ms)

	turn := makeBasicTurn()
	turn.AttacksRemaining = 0 // an attack was already made this turn (the Attack action)
	return ms, shover, target, turn, encounterID
}

// shoverRoll 12 (+STR 3 = 15) beats the goblin's Acrobatics 10 (+DEX 2 = 12).
func shieldMasterRoller() *dice.Roller { return dice.NewRoller(fixedRand(12, 10)) }

func TestShieldMasterShove_ProneSuccess(t *testing.T) {
	ms, shover, target, turn, encounterID := shieldMasterFixture()
	svc := NewService(ms)

	result, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShoveProne,
	}, shieldMasterRoller())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "Knocked prone!")
	assert.True(t, HasCondition(result.Target.Conditions, "prone"))
	// The shove spends the BONUS action, not the action.
	assert.True(t, result.Turn.BonusActionUsed, "Shield Master shove spends the bonus action")
	assert.False(t, result.Turn.ActionUsed, "Shield Master shove must NOT spend the action")
}

func TestShieldMasterShove_PushSuccess(t *testing.T) {
	ms, shover, target, turn, encounterID := shieldMasterFixture()
	svc := NewService(ms)

	result, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover:    shover,
		Target:    target,
		Turn:      turn,
		Encounter: makeBasicEncounter(encounterID, 1),
		Mode:      ShovePush,
	}, shieldMasterRoller())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.CombatLog, "Pushed to")
	assert.True(t, result.Turn.BonusActionUsed)
	assert.False(t, result.Turn.ActionUsed)
}

func TestShieldMasterShove_RequiresFeat(t *testing.T) {
	ms, shover, target, turn, encounterID := shieldMasterFixture()
	// Character without the Shield Master feat (but shield still equipped).
	charID := shover.CharacterID.UUID
	noFeat := makeBasicChar(charID, 30)
	noFeat.EquippedOffHand = sql.NullString{String: "shield", Valid: true}
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return noFeat, nil }
	svc := NewService(ms)

	_, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover: shover, Target: target, Turn: turn,
		Encounter: makeBasicEncounter(encounterID, 1), Mode: ShoveProne,
	}, shieldMasterRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Shield Master feat")
}

func TestShieldMasterShove_RequiresShield(t *testing.T) {
	ms, shover, target, turn, encounterID := shieldMasterFixture()
	// Feat present but no shield equipped (empty off hand).
	charID := shover.CharacterID.UUID
	char := shieldMasterChar(charID)
	char.EquippedOffHand = sql.NullString{}
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	svc := NewService(ms)

	_, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover: shover, Target: target, Turn: turn,
		Encounter: makeBasicEncounter(encounterID, 1), Mode: ShoveProne,
	}, shieldMasterRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shield")
}

func TestShieldMasterShove_RequiresAttackAction(t *testing.T) {
	ms, shover, target, turn, encounterID := shieldMasterFixture()
	turn.AttacksRemaining = 5 // no attack made this turn
	svc := NewService(ms)

	_, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover: shover, Target: target, Turn: turn,
		Encounter: makeBasicEncounter(encounterID, 1), Mode: ShoveProne,
	}, shieldMasterRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Attack action")
}

func TestShieldMasterShove_NotACharacter(t *testing.T) {
	ms, _, target, turn, encounterID := shieldMasterFixture()
	svc := NewService(ms)
	npc := makeNPCCombatantWithCreature(uuid.New(), target.EncounterID, "Ogre", "ogre")

	_, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover: npc, Target: target, Turn: turn,
		Encounter: makeBasicEncounter(encounterID, 1), Mode: ShoveProne,
	}, shieldMasterRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestShieldMasterShove_BonusActionAlreadyUsed(t *testing.T) {
	ms, shover, target, turn, encounterID := shieldMasterFixture()
	turn.BonusActionUsed = true
	svc := NewService(ms)

	_, err := svc.ShieldMasterShove(context.Background(), ShoveCommand{
		Shover: shover, Target: target, Turn: turn,
		Encounter: makeBasicEncounter(encounterID, 1), Mode: ShoveProne,
	}, shieldMasterRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}
