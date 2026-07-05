package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// TestIsPolearmButtWeapon pins the 2014 butt-strike weapon set: glaive,
// halberd, quarterstaff, spear. Pike grants only the opportunity-attack half of
// Polearm Master, not the bonus-action butt strike, so it must NOT qualify.
func TestIsPolearmButtWeapon(t *testing.T) {
	for _, id := range []string{"glaive", "halberd", "quarterstaff", "spear"} {
		assert.True(t, IsPolearmButtWeapon(refdata.Weapon{ID: id}), "%s should qualify", id)
	}
	for _, id := range []string{"pike", "longsword", "greataxe", "dagger"} {
		assert.False(t, IsPolearmButtWeapon(refdata.Weapon{ID: id}), "%s should not qualify", id)
	}
}

// polearmChar builds a level-5 fighter (prof +3, STR 16 → +3) with the Polearm
// Master feat and a glaive equipped.
func polearmChar() refdata.Character {
	feats := []CharacterFeature{{Name: "Polearm Master", MechanicalEffect: `[{"effect_type":"bonus_action_butt_attack_d4"},{"effect_type":"opportunity_attack_on_enter_reach"}]`}}
	classes := []CharacterClass{{Class: "Fighter", Level: 5}}
	return makeCharacterWithFeats(16, 10, 3, "glaive", feats, classes)
}

func polearmMockStore(char refdata.Character, weapon refdata.Weapon) *mockStore {
	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return weapon, nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, BonusActionUsed: arg.BonusActionUsed}, nil
	}
	return ms
}

func polearmCombatants(charID uuid.UUID) (refdata.Combatant, refdata.Combatant, refdata.Turn) {
	encounterID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: targetID, EncounterID: encounterID,
		DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 13,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1, ActionUsed: true}
	return attacker, target, turn
}

// d20=15 → 15 + STR(3) + prof(3) = 21 vs AC 13 → hit. Butt die 1d4 → 3, + STR(3)
// = 6 bludgeoning (NOT the glaive's 1d10 slashing).
func polearmRoller() *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 15
		}
		return 3
	})
}

func TestServicePolearmMasterBonusAttack_HappyPath(t *testing.T) {
	ctx := context.Background()
	char := polearmChar()
	char.ID = uuid.New()
	svc := NewService(polearmMockStore(char, makeGlaive()))
	attacker, target, turn := polearmCombatants(char.ID)

	result, err := svc.PolearmMasterBonusAttack(ctx, PolearmMasterBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, polearmRoller())
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 6, result.DamageTotal, "1d4(3)+STR(3) = 6 — the butt strike, not the glaive's 1d10")
	assert.Equal(t, "bludgeoning", result.DamageType, "the opposite end deals bludgeoning, not the glaive's slashing")
}

func TestServicePolearmMasterBonusAttack_RequiresFeat(t *testing.T) {
	ctx := context.Background()
	char := makeCharacterWithFeats(16, 10, 3, "glaive", nil, []CharacterClass{{Class: "Fighter", Level: 5}})
	char.ID = uuid.New()
	svc := NewService(polearmMockStore(char, makeGlaive()))
	attacker, target, turn := polearmCombatants(char.ID)

	_, err := svc.PolearmMasterBonusAttack(ctx, PolearmMasterBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, polearmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Polearm Master feat")
}

func TestServicePolearmMasterBonusAttack_RequiresAttackAction(t *testing.T) {
	ctx := context.Background()
	char := polearmChar()
	char.ID = uuid.New()
	svc := NewService(polearmMockStore(char, makeGlaive()))
	attacker, target, turn := polearmCombatants(char.ID)
	turn.ActionUsed = false

	_, err := svc.PolearmMasterBonusAttack(ctx, PolearmMasterBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, polearmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Attack action")
}

func TestServicePolearmMasterBonusAttack_RequiresPolearm(t *testing.T) {
	ctx := context.Background()
	char := polearmChar()
	char.ID = uuid.New()
	// Feat present but a longsword equipped — not a polearm.
	longsword := refdata.Weapon{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeaponType: "martial_melee"}
	svc := NewService(polearmMockStore(char, longsword))
	attacker, target, turn := polearmCombatants(char.ID)

	_, err := svc.PolearmMasterBonusAttack(ctx, PolearmMasterBonusAttackCommand{
		Attacker: attacker, Target: target, Turn: turn,
	}, polearmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "glaive, halberd, quarterstaff, or spear")
}

func TestServicePolearmMasterBonusAttack_NotACharacter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	_, err := svc.PolearmMasterBonusAttack(ctx, PolearmMasterBonusAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), IsNpc: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New(), ActionUsed: true},
	}, polearmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a character")
}

func TestServicePolearmMasterBonusAttack_BonusActionAlreadyUsed(t *testing.T) {
	ctx := context.Background()
	svc := NewService(defaultMockStore())
	_, err := svc.PolearmMasterBonusAttack(ctx, PolearmMasterBonusAttackCommand{
		Attacker: refdata.Combatant{ID: uuid.New(), CharacterID: uuid.NullUUID{UUID: uuid.New(), Valid: true}, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), Ac: 13, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New(), BonusActionUsed: true, ActionUsed: true},
	}, polearmRoller())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bonus action")
}
