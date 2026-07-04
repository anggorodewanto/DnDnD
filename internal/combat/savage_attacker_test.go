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

func TestSavageAttackerEligible(t *testing.T) {
	tests := []struct {
		name    string
		input   AttackInput
		isMelee bool
		want    bool
	}{
		{"feat + melee + unused", AttackInput{SavageAttacker: true}, true, true},
		{"no feat", AttackInput{SavageAttacker: false}, true, false},
		{"ranged (not a melee weapon attack)", AttackInput{SavageAttacker: true}, false, false},
		{"already used this turn", AttackInput{SavageAttacker: true, UsedThisTurn: map[string]bool{savageAttackerUsedEffect: true}}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, savageAttackerEligible(tt.input, tt.isMelee))
		})
	}
}

// d8SeqRoller returns the given values on successive d8 (maxN=8) rolls, 0 otherwise.
func d8SeqRoller(seq ...int) *dice.Roller {
	i := 0
	return dice.NewRoller(func(maxN int) int {
		if maxN != 8 {
			return 0
		}
		v := seq[i%len(seq)]
		i++
		return v
	})
}

func TestRollWeaponDamageSavage_KeepsHigher(t *testing.T) {
	weapon := makeLongsword() // 1d8

	// savage=false: single roll, keep the first (2).
	dmg, _, _ := rollWeaponDamageSavage(false, weapon, 0, false, false, d8SeqRoller(2, 7), 0)
	assert.Equal(t, 2, dmg, "no reroll → first d8 (2)")

	// savage=true: roll twice (2 then 7), keep the higher (7).
	dmg2, _, _ := rollWeaponDamageSavage(true, weapon, 0, false, false, d8SeqRoller(2, 7), 0)
	assert.Equal(t, 7, dmg2, "reroll → keep the higher d8 (7)")

	// savage=true but the first roll is already higher: keep it, don't downgrade.
	dmg3, _, _ := rollWeaponDamageSavage(true, weapon, 0, false, false, d8SeqRoller(8, 1), 0)
	assert.Equal(t, 8, dmg3, "reroll worse → keep the original (8)")
}

func savageAttackInput(weapon refdata.Weapon, used map[string]bool) AttackInput {
	return AttackInput{
		AttackerName:   "Grog",
		TargetName:     "Ogre",
		TargetAC:       12,
		Weapon:         weapon,
		Scores:         AbilityScores{Str: 16, Dex: 16},
		ProfBonus:      3,
		DistanceFt:     5,
		SavageAttacker: true,
		UsedThisTurn:   used,
	}
}

// savageRoller: d20 hits (15); the d8 sequence drives the two weapon rolls.
func savageRoller(d8seq ...int) *dice.Roller {
	i := 0
	return dice.NewRoller(func(maxN int) int {
		switch maxN {
		case 20:
			return 15
		case 8:
			v := d8seq[i%len(d8seq)]
			i++
			return v
		default:
			return 2
		}
	})
}

func TestResolveAttack_SavageAttacker_RerollsMeleeDamage(t *testing.T) {
	result, err := ResolveAttack(savageAttackInput(makeLongsword(), nil), savageRoller(2, 7))
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.True(t, result.SavageAttackerUsed)
	assert.Equal(t, 10, result.DamageTotal, "higher d8 (7) + STR (3)")
	assert.Contains(t, result.OncePerTurnEffectsFired, savageAttackerUsedEffect)
}

func TestResolveAttack_SavageAttacker_RangedNoReroll(t *testing.T) {
	input := savageAttackInput(makeLongbow(), nil) // ranged 1d8
	input.DistanceFt = 30
	result, err := ResolveAttack(input, savageRoller(2, 7))
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.False(t, result.SavageAttackerUsed, "ranged is not a melee weapon attack")
	assert.Equal(t, 5, result.DamageTotal, "single d8 (2) + DEX (3), no reroll")
	assert.NotContains(t, result.OncePerTurnEffectsFired, savageAttackerUsedEffect)
}

func TestResolveAttack_SavageAttacker_AlreadyUsedThisTurn(t *testing.T) {
	input := savageAttackInput(makeLongsword(), map[string]bool{savageAttackerUsedEffect: true})
	result, err := ResolveAttack(input, savageRoller(2, 7))
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.False(t, result.SavageAttackerUsed, "reroll already spent this turn")
	assert.Equal(t, 5, result.DamageTotal, "single d8 (2) + STR (3)")
}

func TestSavageAttackerTag(t *testing.T) {
	assert.Equal(t, "", savageAttackerTag(AttackResult{}))
	assert.Contains(t, savageAttackerTag(AttackResult{SavageAttackerUsed: true}), "Savage Attacker")
}

// End-to-end through Service.Attack: a character carrying the feat by name has
// populateAttackFES set the flag, so the melee hit rerolls and keeps the higher
// roll; the once-per-turn key is recorded on the result for marking.
func TestServiceAttack_SavageAttacker_RerollsMeleeDamage(t *testing.T) {
	charID := uuid.New()
	char := makeCharacterWithFeats(16, 12, 3, "longsword",
		[]CharacterFeature{{Name: "Savage Attacker"}},
		[]CharacterClass{{Class: "Fighter", Level: 5}})
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeLongsword(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	encounterID := uuid.New()
	attackerID := uuid.New()
	attacker := refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Grog", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: 14,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(context.Background(), AttackCommand{Attacker: attacker, Target: target, Turn: turn}, savageRoller(2, 7))
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.True(t, result.SavageAttackerUsed, "character has the feat → melee hit rerolls")
	assert.Equal(t, 10, result.DamageTotal, "higher d8 (7) + STR (3)")
}
