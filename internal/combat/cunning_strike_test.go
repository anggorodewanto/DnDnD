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

func TestCunningStrikeDiceCost(t *testing.T) {
	assert.Equal(t, 1, cunningStrikeDiceCost("trip"))
	assert.Equal(t, 0, cunningStrikeDiceCost(""))
	assert.Equal(t, 0, cunningStrikeDiceCost("unknown"))
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

// TestRecordCunningStrikeTrip: the trip signal is recorded only when the option
// is "trip", the attack hit, AND Sneak Attack actually dealt damage.
func TestRecordCunningStrikeTrip(t *testing.T) {
	saHit := AttackResult{Hit: true, OncePerTurnEffectNames: []string{"Sneak Attack"}}
	tripInput := AttackInput{CunningStrike: "trip"}

	r := saHit
	recordCunningStrikeTrip(&r, tripInput, 14)
	assert.Equal(t, 14, r.CunningStrikeTripDC, "recorded → non-zero DC gates the trip")

	// Sneak Attack did not fire → not recorded.
	noSA := AttackResult{Hit: true, OncePerTurnEffectNames: []string{"Hex"}}
	recordCunningStrikeTrip(&noSA, tripInput, 14)
	assert.Zero(t, noSA.CunningStrikeTripDC)

	// Not a trip choice → not recorded.
	noChoice := saHit
	recordCunningStrikeTrip(&noChoice, AttackInput{}, 14)
	assert.Zero(t, noChoice.CunningStrikeTripDC)

	// Miss → not recorded.
	miss := AttackResult{Hit: false, OncePerTurnEffectNames: []string{"Sneak Attack"}}
	recordCunningStrikeTrip(&miss, tripInput, 14)
	assert.Zero(t, miss.CunningStrikeTripDC)
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
	assert.Equal(t, 14, result.CunningStrikeTripDC, "trip fired: 8 + prof 3 + DEX 3")
	assert.False(t, result.CunningStrikeTripSaved, "low roll → failed save")
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
	assert.Equal(t, 14, result.CunningStrikeTripDC, "trip fired")
	assert.True(t, result.CunningStrikeTripSaved, "high roll → made save")
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
	assert.Zero(t, result.CunningStrikeTripDC, "no feature → trip not attempted")
	assert.NotContains(t, *applied, "prone")
	// Full 3d6 Sneak Attack: 1d8(5) + DEX(3) + 3d6(15) = 23.
	assert.Equal(t, 23, result.DamageTotal, "no feature → full Sneak Attack dice")
}

// TestFormatAttackLog_CunningStrikeTrip surfaces the Trip outcome (saved vs
// knocked prone) as a player-visible line.
func TestFormatAttackLog_CunningStrikeTrip(t *testing.T) {
	base := AttackResult{
		AttackerName: "Snik", TargetName: "Goblin", WeaponName: "Rapier",
		IsMelee: true, Hit: true, DamageTotal: 18, DamageType: "piercing", DamageDice: "1d8",
		D20Roll: dice.D20Result{Total: 18, Chosen: 15, Modifier: 3},
	}

	tripped := base
	tripped.CunningStrikeTripDC = 14
	log := FormatAttackLog(tripped)
	assert.Contains(t, log, "Cunning Strike")
	assert.Contains(t, log, "Prone")

	saved := base
	saved.CunningStrikeTripDC = 14
	saved.CunningStrikeTripSaved = true
	assert.Contains(t, FormatAttackLog(saved), "saves")
}
