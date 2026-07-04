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

func makeClub() refdata.Weapon {
	return refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
	}
}

func TestEffectiveAbilityMod_PactBladeUsesCHAWhenHigher(t *testing.T) {
	// Warlock: CHA 18 (+4), STR 8 (-1). Non-finesse club normally uses STR.
	input := AttackInput{
		Scores:       AbilityScores{Str: 8, Cha: 18},
		Weapon:       makeClub(),
		PactBladeCHA: true,
	}
	assert.Equal(t, 4, effectiveAbilityMod(input))
}

func TestEffectiveAbilityMod_PactBladeKeepsHigherWeaponAbility(t *testing.T) {
	// CHA 8 (-1) is worse than STR 18 (+4): "can use CHA" is player-optimal, so
	// the higher weapon ability is kept.
	input := AttackInput{
		Scores:       AbilityScores{Str: 18, Cha: 8},
		Weapon:       makeClub(),
		PactBladeCHA: true,
	}
	assert.Equal(t, 4, effectiveAbilityMod(input))
}

func TestEffectiveAbilityMod_NoPactBladeIgnoresCHA(t *testing.T) {
	// Without the boon, CHA never enters the selection.
	input := AttackInput{
		Scores: AbilityScores{Str: 8, Cha: 18},
		Weapon: makeClub(),
	}
	assert.Equal(t, -1, effectiveAbilityMod(input))
}

func TestResolveAttack_PactBladeUsesCHAForAttackAndDamage(t *testing.T) {
	// d20 rolls 10, damage d4 rolls 3.
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 3
	})

	input := AttackInput{
		AttackerName: "Vale",
		TargetName:   "Goblin #1",
		TargetAC:     10,
		Weapon:       makeClub(),
		Scores:       AbilityScores{Str: 8, Cha: 18}, // STR -1, CHA +4
		ProfBonus:    2,
		DistanceFt:   5,
		Cover:        CoverNone,
		PactBladeCHA: true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// 10 (d20) + 4 (CHA, not -1 STR) + 2 (prof) = 16
	assert.Equal(t, 16, result.D20Roll.Total)
	// 3 (d4) + 4 (CHA) = 7
	assert.Equal(t, 7, result.DamageTotal)
}

func TestResolveAttack_PactBladeKeepsWeaponAbilityWhenHigher(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 3
	})

	input := AttackInput{
		AttackerName: "Vale",
		TargetName:   "Goblin #1",
		TargetAC:     10,
		Weapon:       makeClub(),
		Scores:       AbilityScores{Str: 18, Cha: 8}, // STR +4, CHA -1
		ProfBonus:    2,
		DistanceFt:   5,
		Cover:        CoverNone,
		PactBladeCHA: true,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	// 10 + 4 (STR kept, CHA -1 rejected) + 2 = 16
	assert.Equal(t, 16, result.D20Roll.Total)
	assert.Equal(t, 7, result.DamageTotal) // 3 + 4 STR
}

// pactBladeAttackFixture builds a level-5 warlock (STR 8/-1, CHA 18/+4) wielding
// a club (non-finesse melee) and attacking a goblin, carrying the given boon
// feature slug. It exercises the real Service.Attack → populateAttackFES set-site
// so the pact-blade gate is proven end-to-end (not just the ResolveAttack flag).
func pactBladeAttackFixture(t *testing.T, boonSlug, boonName string) (*Service, AttackCommand, *dice.Roller) {
	t.Helper()
	charID := uuid.New()
	attackerID := uuid.New()
	encounterID := uuid.New()

	feats := []CharacterFeature{{Name: boonName, MechanicalEffect: boonSlug}}
	classes := []CharacterClass{{Class: "Warlock", Level: 5}}
	char := makeCharacterWithFeats(8, 8, 3, "club", feats, classes)
	char.ID = charID
	scores := AbilityScores{Str: 8, Dex: 8, Con: 10, Int: 10, Wis: 10, Cha: 18}
	char.AbilityScores, _ = json.Marshal(scores)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) { return makeClub(), nil }
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}

	cmd := AttackCommand{
		Attacker: refdata.Combatant{ID: attackerID, EncounterID: encounterID, CharacterID: uuid.NullUUID{UUID: charID, Valid: true}, DisplayName: "Vale", PositionCol: "A", PositionRow: 1, IsAlive: true, IsVisible: true, Conditions: json.RawMessage(`[]`)},
		Target:   refdata.Combatant{ID: uuid.New(), EncounterID: encounterID, DisplayName: "Goblin", PositionCol: "B", PositionRow: 1, Ac: 12, IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`)},
		Turn:     refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}
	// d20 → 10, damage d4 → 3
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 10
		}
		return 3
	})
	return NewService(ms), cmd, roller
}

func TestServiceAttack_PactOfTheBlade_UsesCHA(t *testing.T) {
	svc, cmd, roller := pactBladeAttackFixture(t, "pact_of_the_blade", "Pact of the Blade")
	result, err := svc.Attack(context.Background(), cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	// 10 (d20) + 4 (CHA) + 3 (prof) = 17
	assert.Equal(t, 17, result.D20Roll.Total)
	// 3 (d4) + 4 (CHA) = 7
	assert.Equal(t, 7, result.DamageTotal)
}

func TestServiceAttack_PactOfTheTome_NoCHAOverride(t *testing.T) {
	// Negative control: a non-Blade boon must not trigger the CHA substitution;
	// the club falls back to STR (-1).
	svc, cmd, roller := pactBladeAttackFixture(t, "pact_of_the_tome", "Pact of the Tome")
	result, err := svc.Attack(context.Background(), cmd, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	// 10 + (-1 STR) + 3 (prof) = 12
	assert.Equal(t, 12, result.D20Roll.Total)
	// 3 (d4) + (-1 STR) = 2
	assert.Equal(t, 2, result.DamageTotal)
}
