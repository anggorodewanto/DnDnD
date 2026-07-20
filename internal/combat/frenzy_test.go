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

// --- pure helpers -----------------------------------------------------------

func TestFrenzyFeature(t *testing.T) {
	f := FrenzyFeature(3, "slashing")
	assert.Equal(t, "Frenzy", f.Name)
	require.Len(t, f.Effects, 1)
	assert.Equal(t, EffectExtraDamageDice, f.Effects[0].Type)
	assert.Equal(t, TriggerOnDamageRoll, f.Effects[0].Trigger)
	assert.Equal(t, "3d6", f.Effects[0].Dice, "Nd6 where N = Rage Damage bonus")
	assert.Equal(t, []string{"slashing"}, f.Effects[0].DamageTypes)

	// The dice count tracks the Rage Damage bonus, not a fixed value.
	assert.Equal(t, "2d6", FrenzyFeature(2, "bludgeoning").Effects[0].Dice)
	assert.Equal(t, []string{"bludgeoning"}, FrenzyFeature(2, "bludgeoning").Effects[0].DamageTypes)
}

// frenzyFeats is the feature list of a Berserker who has Frenzy.
var frenzyFeats = []CharacterFeature{{Name: "Frenzy"}}

// TestFrenzyEligible covers every gate in the 2024 text: using Reckless Attack
// (declared or via the transient marker), Rage active, the feature present, a
// Strength-based attack, and the once-per-turn allowance still unspent.
func TestFrenzyEligible(t *testing.T) {
	noConds := refdata.Combatant{Conditions: json.RawMessage(`[]`)}
	markerConds, _ := json.Marshal([]CombatCondition{{Condition: "reckless"}})

	declared := AttackCommand{Reckless: true, Attacker: noConds}
	viaMarker := AttackCommand{Attacker: refdata.Combatant{Conditions: markerConds}}
	notReckless := AttackCommand{Attacker: noConds}

	tests := []struct {
		name        string
		cmd         AttackCommand
		feats       []CharacterFeature
		abilityUsed string
		isRaging    bool
		used        map[string]bool
		want        bool
	}{
		{"declared reckless + raging + feature + str", declared, frenzyFeats, "str", true, nil, true},
		{"reckless marker from an earlier attack this turn", viaMarker, frenzyFeats, "str", true, nil, true},
		{"rage not active", declared, frenzyFeats, "str", false, nil, false},
		{"not using reckless attack", notReckless, frenzyFeats, "str", true, nil, false},
		{"feature absent", declared, nil, "str", true, nil, false},
		{"dex-based attack", declared, frenzyFeats, "dex", true, nil, false},
		{"already fired this turn", declared, frenzyFeats, "str", true, map[string]bool{frenzyUsedEffect: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, frenzyEligible(tt.cmd, tt.feats, tt.abilityUsed, tt.isRaging, tt.used))
		})
	}
}

// --- ResolveAttack ----------------------------------------------------------

// frenzyResolveInput is a STR greataxe swing with the Frenzy rider already
// injected (populateAttackFES's job), so ResolveAttack's half can be tested
// without the service.
func frenzyResolveInput(rageBonus int) AttackInput {
	return AttackInput{
		AttackerName: "Forge",
		TargetName:   "Ogre",
		TargetAC:     14,
		Weapon:       makeGreataxe(),
		Scores:       AbilityScores{Str: 16, Dex: 10},
		ProfBonus:    2,
		DistanceFt:   5,
		AbilityUsed:  "str",
		IsRaging:     true,
		Frenzy:       true,
		Features:     []FeatureDefinition{FrenzyFeature(rageBonus, "slashing")},
	}
}

// frenzyRoller: d20 drives the hit, d12 the greataxe, d6 the Frenzy dice.
func frenzyRoller(d20, d12, d6 int) *dice.Roller {
	return dice.NewRoller(func(maxN int) int {
		switch maxN {
		case 20:
			return d20
		case 12:
			return d12
		default:
			return d6
		}
	})
}

func TestResolveAttack_Frenzy_HitRecordsOncePerTurnKey(t *testing.T) {
	result, err := ResolveAttack(frenzyResolveInput(2), frenzyRoller(18, 10, 4))
	require.NoError(t, err)
	require.True(t, result.Hit)
	// 1d12(10) + STR(3) + Frenzy 2d6(8) = 21.
	assert.Equal(t, 21, result.DamageTotal)
	assert.Contains(t, result.OncePerTurnEffectsFired, frenzyUsedEffect)
}

func TestResolveAttack_Frenzy_MissDoesNotSpendIt(t *testing.T) {
	// d20 2 + (STR 3 + prof 2) = 7 < AC 14 → miss.
	result, err := ResolveAttack(frenzyResolveInput(2), frenzyRoller(2, 10, 4))
	require.NoError(t, err)
	require.False(t, result.Hit)
	assert.Zero(t, result.DamageTotal)
	assert.NotContains(t, result.OncePerTurnEffectsFired, frenzyUsedEffect,
		"Frenzy is spent on the first target you HIT — a miss keeps it available")
}

// --- Service.Attack end-to-end ----------------------------------------------

// frenzyMockStore wires a Berserker barbarian of the given level carrying the
// given features and wielding `weapon`.
func frenzyMockStore(barbLevel int, feats []CharacterFeature, weapon refdata.Weapon) (*mockStore, uuid.UUID) {
	charID := uuid.New()
	char := makeCharacterWithFeats(16, 10, 2, weapon.ID, feats,
		[]CharacterClass{{Class: "Barbarian", Level: barbLevel}})
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return weapon, nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	return ms, charID
}

// frenzyAttacker is a raging barbarian already carrying the transient reckless
// marker (Reckless Attack declared on an earlier attack this turn).
func frenzyAttacker(charID, attackerID, encounterID uuid.UUID, raging, reckless bool) refdata.Combatant {
	conds := json.RawMessage(`[]`)
	if reckless {
		marker, _ := json.Marshal([]CombatCondition{{Condition: "reckless"}})
		conds = marker
	}
	return refdata.Combatant{
		ID: attackerID, EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Forge", PositionCol: "A", PositionRow: 1,
		IsAlive: true, IsVisible: true, IsRaging: raging, Conditions: conds,
	}
}

func frenzyTarget(encounterID uuid.UUID, ac int32) refdata.Combatant {
	return refdata.Combatant{
		ID: uuid.New(), EncounterID: encounterID,
		DisplayName: "Ogre", PositionCol: "B", PositionRow: 1, Ac: ac,
		IsAlive: true, IsNpc: true, IsVisible: true, Conditions: json.RawMessage(`[]`),
	}
}

// frenzyComponent returns the Frenzy entry of a result's damage breakdown, or
// the zero value when the rider did not fire.
func frenzyComponent(result AttackResult) DamageComponent {
	for _, c := range result.DamageBreakdown {
		if c.SourceName == "Frenzy" {
			return c
		}
	}
	return DamageComponent{}
}

// runFrenzyAttack drives one Service.Attack with the given gates and returns the
// result. AC 14 with a +5 to-hit means a d20 of 18 always hits.
func runFrenzyAttack(t *testing.T, barbLevel int, feats []CharacterFeature, weapon refdata.Weapon, raging, reckless bool, roller *dice.Roller) AttackResult {
	t.Helper()
	ms, charID := frenzyMockStore(barbLevel, feats, weapon)
	svc := NewService(ms)

	encounterID, attackerID := uuid.New(), uuid.New()
	turn := refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: frenzyAttacker(charID, attackerID, encounterID, raging, reckless),
		Target:   frenzyTarget(encounterID, 14),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	return result
}

// TestServiceAttack_Frenzy_HappyPath: a raging level-3 Berserker using Reckless
// Attack lands a STR greataxe hit → +2d6 slashing (Rage Damage bonus 2).
func TestServiceAttack_Frenzy_HappyPath(t *testing.T) {
	result := runFrenzyAttack(t, 3, frenzyFeats, makeGreataxe(), true, true, frenzyRoller(18, 10, 4))
	require.True(t, result.Hit)
	// 1d12(10) + STR(3) + Frenzy 2d6(8) = 21.
	assert.Equal(t, 21, result.DamageTotal)
	assert.Equal(t, DamageComponent{SourceName: "Frenzy", Amount: 8, DamageType: "slashing"}, frenzyComponent(result))
}

// TestServiceAttack_Frenzy_RequiresActiveRage: the same swing without Rage.
func TestServiceAttack_Frenzy_RequiresActiveRage(t *testing.T) {
	result := runFrenzyAttack(t, 3, frenzyFeats, makeGreataxe(), false, true, frenzyRoller(18, 10, 4))
	require.True(t, result.Hit)
	assert.Equal(t, 13, result.DamageTotal, "1d12(10) + STR(3), no Frenzy")
	assert.Zero(t, frenzyComponent(result).Amount)
}

// TestServiceAttack_Frenzy_RequiresReckless: raging, but Reckless Attack was
// never used this turn.
func TestServiceAttack_Frenzy_RequiresReckless(t *testing.T) {
	result := runFrenzyAttack(t, 3, frenzyFeats, makeGreataxe(), true, false, frenzyRoller(18, 10, 4))
	require.True(t, result.Hit)
	assert.Equal(t, 13, result.DamageTotal, "1d12(10) + STR(3), no Frenzy")
	assert.Zero(t, frenzyComponent(result).Amount)
}

// TestServiceAttack_Frenzy_RequiresFeature: a Berserker-less barbarian.
func TestServiceAttack_Frenzy_RequiresFeature(t *testing.T) {
	result := runFrenzyAttack(t, 3, nil, makeGreataxe(), true, true, frenzyRoller(18, 10, 4))
	require.True(t, result.Hit)
	assert.Equal(t, 13, result.DamageTotal, "1d12(10) + STR(3), no Frenzy")
	assert.Zero(t, frenzyComponent(result).Amount)
}

// TestServiceAttack_Frenzy_RequiresStrengthAttack: a longbow is a DEX attack, so
// Frenzy does not ride even while raging with the reckless marker up. (The gate
// is Strength-based, NOT melee-only — a thrown STR weapon would still qualify.)
func TestServiceAttack_Frenzy_RequiresStrengthAttack(t *testing.T) {
	// d20 18 + DEX(0) + prof(2) = 20 ≥ AC 14 → hit. 1d8 longbow.
	roller := dice.NewRoller(func(maxN int) int {
		if maxN == 20 {
			return 18
		}
		return 4
	})
	result := runFrenzyAttack(t, 3, frenzyFeats, makeLongbow(), true, true, roller)
	require.True(t, result.Hit)
	assert.Zero(t, frenzyComponent(result).Amount, "Frenzy needs a Strength-based attack")
}

// TestServiceAttack_Frenzy_MissNoExtraDamage: a miss deals nothing at all.
func TestServiceAttack_Frenzy_MissNoExtraDamage(t *testing.T) {
	result := runFrenzyAttack(t, 3, frenzyFeats, makeGreataxe(), true, true, frenzyRoller(2, 10, 4))
	require.False(t, result.Hit)
	assert.Zero(t, result.DamageTotal)
	assert.Zero(t, frenzyComponent(result).Amount)
}

// TestServiceAttack_Frenzy_OncePerTurn: "the first target you hit on your turn"
// — the second hit in the same turn gets no Frenzy dice.
func TestServiceAttack_Frenzy_OncePerTurn(t *testing.T) {
	ms, charID := frenzyMockStore(3, frenzyFeats, makeGreataxe())
	svc := NewService(ms)
	roller := frenzyRoller(18, 10, 4)

	encounterID, attackerID := uuid.New(), uuid.New()
	attacker := frenzyAttacker(charID, attackerID, encounterID, true, true)
	target := frenzyTarget(encounterID, 14)
	turnID := uuid.New()

	first, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker, Target: target,
		Turn: refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 2},
	}, roller)
	require.NoError(t, err)
	require.True(t, first.Hit)
	assert.Equal(t, 8, frenzyComponent(first).Amount, "first hit gets Frenzy 2d6")

	second, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: attacker, Target: target,
		Turn: refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}, roller)
	require.NoError(t, err)
	require.True(t, second.Hit)
	assert.Zero(t, frenzyComponent(second).Amount, "Frenzy is once per turn")
	assert.Equal(t, 13, second.DamageTotal, "1d12(10) + STR(3) only")
}

// TestServiceAttack_Frenzy_DeclaredRecklessThisAttack: the other reckless path —
// Reckless Attack declared on THIS attack rather than carried by the marker.
func TestServiceAttack_Frenzy_DeclaredRecklessThisAttack(t *testing.T) {
	ms, charID := frenzyMockStore(3, frenzyFeats, makeGreataxe())
	svc := NewService(ms)

	encounterID, attackerID := uuid.New(), uuid.New()
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: frenzyAttacker(charID, attackerID, encounterID, true, false),
		Target:   frenzyTarget(encounterID, 14),
		Turn:     refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
		Reckless: true,
	}, frenzyRoller(18, 10, 4))
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, frenzyComponent(result).Amount, "declared Reckless also arms Frenzy")
}

// TestServiceAttack_Frenzy_DamageTypeMatchesWeapon: the extra dice take the
// weapon's damage type — bludgeoning for a maul, slashing for a greataxe.
func TestServiceAttack_Frenzy_DamageTypeMatchesWeapon(t *testing.T) {
	maul := runFrenzyAttack(t, 3, frenzyFeats, makeBrutalMaul(), true, true, frenzyRoller(18, 10, 4))
	require.True(t, maul.Hit)
	assert.Equal(t, "bludgeoning", frenzyComponent(maul).DamageType)

	axe := runFrenzyAttack(t, 3, frenzyFeats, makeGreataxe(), true, true, frenzyRoller(18, 10, 4))
	require.True(t, axe.Hit)
	assert.Equal(t, "slashing", frenzyComponent(axe).DamageType)
}

// TestServiceAttack_Frenzy_ScalesWithRageDamageBonus: a level-9 barbarian's Rage
// Damage bonus is 3, so Frenzy rolls 3d6 (not 2d6).
func TestServiceAttack_Frenzy_ScalesWithRageDamageBonus(t *testing.T) {
	require.Equal(t, 3, RageDamageBonus(9), "level-9 Rage Damage bonus")

	result := runFrenzyAttack(t, 9, frenzyFeats, makeGreataxe(), true, true, frenzyRoller(18, 10, 4))
	require.True(t, result.Hit)
	assert.Equal(t, 12, frenzyComponent(result).Amount, "3d6 at 4 each")
	assert.Equal(t, 25, result.DamageTotal, "1d12(10) + STR(3) + 3d6(12)")
}

// TestServiceAttack_Frenzy_MulticlassUsesBarbarianLevel: a Barbarian 3 / Fighter 6
// is character level 9 but Rage Damage bonus 2 — the dice follow the BARBARIAN
// level, so 2d6 not 3d6.
func TestServiceAttack_Frenzy_MulticlassUsesBarbarianLevel(t *testing.T) {
	charID := uuid.New()
	char := makeCharacterWithFeats(16, 10, 2, "greataxe", frenzyFeats,
		[]CharacterClass{{Class: "Barbarian", Level: 3}, {Class: "Fighter", Level: 6}})
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(_ context.Context, _ uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(_ context.Context, _ string) (refdata.Weapon, error) { return makeGreataxe(), nil }
	ms.updateTurnActionsFn = func(_ context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	svc := NewService(ms)

	encounterID, attackerID := uuid.New(), uuid.New()
	result, err := svc.Attack(context.Background(), AttackCommand{
		Attacker: frenzyAttacker(charID, attackerID, encounterID, true, true),
		Target:   frenzyTarget(encounterID, 14),
		Turn:     refdata.Turn{ID: uuid.New(), EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1},
	}, frenzyRoller(18, 10, 4))
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 8, frenzyComponent(result).Amount, "Barbarian 3 → Rage Damage bonus 2 → 2d6")
}
