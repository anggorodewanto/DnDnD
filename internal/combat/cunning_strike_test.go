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

func TestReduceDiceCount(t *testing.T) {
	tests := []struct {
		expr string
		by   int
		want string
	}{
		{"3d6", 1, "2d6"},
		{"3d6", 0, "3d6"},
		{"2d6", 2, "0d6"},
		{"3d6", 5, "0d6"}, // floor at 0
		{"1d8", 1, "0d8"}, // die size preserved
		{"garbage", 1, "garbage"},
		{"", 1, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, reduceDiceCount(tt.expr, tt.by), "%q by %d", tt.expr, tt.by)
	}
}

func TestCunningStrikeRiders(t *testing.T) {
	trip, ok := cunningStrikeRiders["trip"]
	require.True(t, ok)
	assert.Equal(t, 1, trip.diceCost)
	assert.Equal(t, "dex", trip.saveAbility)
	assert.Equal(t, "prone", trip.condition)

	poison, ok := cunningStrikeRiders["poison"]
	require.True(t, ok)
	assert.Equal(t, 1, poison.diceCost)
	assert.Equal(t, "con", poison.saveAbility)
	assert.Equal(t, "poisoned", poison.condition)

	_, ok = cunningStrikeRiders["withdraw"]
	assert.False(t, ok, "Withdraw not wired (needs a movement/OA trigger)")
}

// TestReduceSneakAttackDice: only the Sneak Attack extra-damage dice are
// reduced; a sibling extra-damage rider (Hex/Hunter's Mark) is untouched.
func TestReduceSneakAttackDice(t *testing.T) {
	features := []FeatureDefinition{
		SneakAttackFeature(5), // 3d6
		{Name: "Hex", Effects: []Effect{{Type: EffectExtraDamageDice, Dice: "1d6"}}},
	}
	reduceSneakAttackDice(features, 1)
	assert.Equal(t, "2d6", features[0].Effects[0].Dice, "Sneak Attack reduced 3d6→2d6")
	assert.Equal(t, "1d6", features[1].Effects[0].Dice, "sibling rider untouched")
}

func TestSneakAttackDealt(t *testing.T) {
	assert.True(t, sneakAttackDealt(AttackResult{OncePerTurnEffectNames: []string{"Sneak Attack"}}))
	assert.False(t, sneakAttackDealt(AttackResult{OncePerTurnEffectNames: []string{"Hex"}}))
	assert.False(t, sneakAttackDealt(AttackResult{}))
}

// TestRecordCunningStrike: the chosen effect is recorded only when it is a known
// rider, the attack hit, AND Sneak Attack actually dealt damage.
func TestRecordCunningStrike(t *testing.T) {
	saHit := AttackResult{Hit: true, OncePerTurnEffectNames: []string{"Sneak Attack"}}

	r := saHit
	recordCunningStrike(&r, AttackInput{CunningStrike: "trip"}, 14)
	assert.Equal(t, "trip", r.CunningStrikeChoice, "recorded → choice gates the rider")
	assert.Equal(t, 14, r.CunningStrikeSaveDC)

	// A second effect records identically.
	p := saHit
	recordCunningStrike(&p, AttackInput{CunningStrike: "poison"}, 15)
	assert.Equal(t, "poison", p.CunningStrikeChoice)
	assert.Equal(t, 15, p.CunningStrikeSaveDC)

	// Sneak Attack did not fire → not recorded.
	noSA := AttackResult{Hit: true, OncePerTurnEffectNames: []string{"Hex"}}
	recordCunningStrike(&noSA, AttackInput{CunningStrike: "trip"}, 14)
	assert.Empty(t, noSA.CunningStrikeChoice)

	// Unknown / empty choice → not recorded.
	unknown := saHit
	recordCunningStrike(&unknown, AttackInput{CunningStrike: "withdraw"}, 14)
	assert.Empty(t, unknown.CunningStrikeChoice)

	// Miss → not recorded.
	miss := AttackResult{Hit: false, OncePerTurnEffectNames: []string{"Sneak Attack"}}
	recordCunningStrike(&miss, AttackInput{CunningStrike: "trip"}, 14)
	assert.Empty(t, miss.CunningStrikeChoice)
}

// --- Service.Attack end-to-end ----------------------------------------------

// cunningStrikeMockStore builds a level-5 rogue (Sneak Attack 3d6) wielding a
// rapier plus the target-condition sink the Trip rider needs. hasFeature toggles
// the cunning_strike feature. Returns the store and a pointer to the slice of
// condition slugs applied to any target.
func cunningStrikeMockStore(t *testing.T, charID uuid.UUID, hasFeature bool) (*mockStore, *[]string) {
	t.Helper()
	classes := []CharacterClass{{Class: "Rogue", Level: 5}}
	feats := []CharacterFeature{{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"}}
	if hasFeature {
		feats = append(feats, CharacterFeature{Name: "Cunning Strike", MechanicalEffect: "cunning_strike"})
	}
	char := makeCharacterWithFeats(10, 16, 3, "rapier", feats, classes)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) { return makeRapier(), nil }
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	// ApplyCondition reads the target's current conditions before writing.
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	var applied []string
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		for _, c := range conds {
			applied = append(applied, c.Condition)
		}
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}
	return ms, &applied
}

func cunningRogueAttacker(charID, attackerID, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Snik",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

// cunningProneGoblin is prone + within 5ft so the rogue attacks with advantage
// (satisfying Sneak Attack's AdvantageOrAllyWithin condition). It is a plain NPC
// (no creature ref) so its DEX save bonus resolves to +0, making the DC math
// deterministic.
func cunningProneGoblin(targetID, encounterID uuid.UUID) refdata.Combatant {
	proneCond, _ := json.Marshal([]CombatCondition{{Condition: "prone"}})
	return refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Goblin",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          12,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(proneCond),
	}
}

// TestServiceAttack_CunningStrikeTrip_FailedSaveAppliesProne: a Rogue-5 with the
// feature opts into cunning:trip, deals Sneak Attack damage (with one die
// forgone), and the target fails its DEX save → Prone.
func TestServiceAttack_CunningStrikeTrip_FailedSaveAppliesProne(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID := uuid.New(), uuid.New()

	ms, applied := cunningStrikeMockStore(t, charID, true)
	svc := NewService(ms)

	// Attack rolls twice (advantage) — both 18 (hit); the DEX save rolls once — 1 (fail).
	d20calls := 0
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			d20calls++
			if d20calls <= 2 {
				return 18 // attack hits
			}
			return 1 // DEX save fails
		}
		return 5 // all damage dice
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:      cunningRogueAttacker(charID, attackerID, encounterID),
		Target:        cunningProneGoblin(targetID, encounterID),
		Turn:          turn,
		CunningStrike: "trip",
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, "trip", result.CunningStrikeChoice)
	assert.Equal(t, 14, result.CunningStrikeSaveDC, "trip fired: 8 + prof 3 + DEX 3")
	assert.False(t, result.CunningStrikeSaved, "low roll → failed save")
	assert.Contains(t, *applied, "prone", "failed save knocks the target prone")
	// One SA die forgone: 1d8(5) + DEX(3) + 2d6(10) = 18 (vs 23 at the full 3d6).
	assert.Equal(t, 18, result.DamageTotal, "Cunning Strike forgoes one Sneak Attack die (3d6→2d6)")
}

// TestServiceAttack_CunningStrikeTrip_SuccessfulSaveNoProne: the target makes the
// DEX save → no Prone, but the die is still forgone (the cost is paid on use).
func TestServiceAttack_CunningStrikeTrip_SuccessfulSaveNoProne(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID := uuid.New(), uuid.New()

	ms, applied := cunningStrikeMockStore(t, charID, true)
	svc := NewService(ms)

	d20calls := 0
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			d20calls++
			if d20calls <= 2 {
				return 18 // attack hits
			}
			return 20 // DEX save succeeds
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:      cunningRogueAttacker(charID, attackerID, encounterID),
		Target:        cunningProneGoblin(targetID, encounterID),
		Turn:          turn,
		CunningStrike: "trip",
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, 14, result.CunningStrikeSaveDC, "trip fired")
	assert.True(t, result.CunningStrikeSaved, "high roll → made save")
	assert.NotContains(t, *applied, "prone", "made save → no prone")
	assert.Equal(t, 18, result.DamageTotal, "die still forgone on a made save")
}

// TestServiceAttack_CunningStrikeTrip_NoFeatureIgnored: without the feature,
// cunning:trip is inert — full Sneak Attack dice, no save, no prone.
func TestServiceAttack_CunningStrikeTrip_NoFeatureIgnored(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID := uuid.New(), uuid.New()

	ms, applied := cunningStrikeMockStore(t, charID, false)
	svc := NewService(ms)

	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // attack hits (advantage)
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:      cunningRogueAttacker(charID, attackerID, encounterID),
		Target:        cunningProneGoblin(targetID, encounterID),
		Turn:          turn,
		CunningStrike: "trip",
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Empty(t, result.CunningStrikeChoice, "no feature → trip not attempted")
	assert.NotContains(t, *applied, "prone")
	// Full 3d6 Sneak Attack: 1d8(5) + DEX(3) + 3d6(15) = 23.
	assert.Equal(t, 23, result.DamageTotal, "no feature → full Sneak Attack dice")
}

// TestServiceAttack_CunningStrikePoison_FailedSaveAppliesPoisoned: a Rogue-5 with
// the feature opts into cunning:poison; the target fails its CON save → Poisoned,
// and one Sneak Attack die is forgone (same seam as Trip, different save+condition).
func TestServiceAttack_CunningStrikePoison_FailedSaveAppliesPoisoned(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID := uuid.New(), uuid.New()

	ms, applied := cunningStrikeMockStore(t, charID, true)
	svc := NewService(ms)

	// Attack rolls twice (advantage) — both 18 (hit); the CON save rolls once — 1 (fail).
	d20calls := 0
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			d20calls++
			if d20calls <= 2 {
				return 18 // attack hits
			}
			return 1 // CON save fails
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:      cunningRogueAttacker(charID, attackerID, encounterID),
		Target:        cunningProneGoblin(targetID, encounterID),
		Turn:          turn,
		CunningStrike: "poison",
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, "poison", result.CunningStrikeChoice)
	assert.Equal(t, 14, result.CunningStrikeSaveDC, "poison DC is also 8 + prof 3 + DEX 3")
	assert.False(t, result.CunningStrikeSaved, "low roll → failed save")
	assert.Contains(t, *applied, "poisoned", "failed CON save poisons the target")
	assert.Equal(t, 18, result.DamageTotal, "poison also forgoes one Sneak Attack die (3d6→2d6)")
}

// TestFormatAttackLog_CunningStrike surfaces each effect's outcome (saved vs the
// rider's condition) as a player-visible line, labelled by effect.
func TestFormatAttackLog_CunningStrike(t *testing.T) {
	base := AttackResult{
		AttackerName: "Snik", TargetName: "Goblin", WeaponName: "Rapier",
		IsMelee: true, Hit: true, DamageTotal: 18, DamageType: "piercing", DamageDice: "1d8",
		D20Roll: dice.D20Result{Total: 18, Chosen: 15, Modifier: 3},
	}

	tripped := base
	tripped.CunningStrikeChoice = "trip"
	tripped.CunningStrikeSaveDC = 14
	log := FormatAttackLog(tripped)
	assert.Contains(t, log, "Cunning Strike (Trip)")
	assert.Contains(t, log, "knocked Prone")

	poisoned := base
	poisoned.CunningStrikeChoice = "poison"
	poisoned.CunningStrikeSaveDC = 14
	plog := FormatAttackLog(poisoned)
	assert.Contains(t, plog, "Cunning Strike (Poison)")
	assert.Contains(t, plog, "Poisoned")

	saved := base
	saved.CunningStrikeChoice = "trip"
	saved.CunningStrikeSaveDC = 14
	saved.CunningStrikeSaved = true
	assert.Contains(t, FormatAttackLog(saved), "saves")
}
