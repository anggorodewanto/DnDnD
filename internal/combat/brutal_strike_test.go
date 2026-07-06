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

// makeBrutalMaul is a plain Strength melee weapon (no mastery) so a Brutal Strike
// test observes only the brutal effects, not a weapon-mastery rider.
func makeBrutalMaul() refdata.Weapon {
	return refdata.Weapon{
		ID:         "maul",
		Name:       "Maul",
		Damage:     "2d6",
		DamageType: "bludgeoning",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "two-handed"},
	}
}

var brutalStrikeFeatsJSON = []byte(`[{"name":"Brutal Strike","mechanical_effect":"brutal_strike"}]`)

// --- pure helpers -----------------------------------------------------------

func TestBrutalStrikeFeature(t *testing.T) {
	f := BrutalStrikeFeature("slashing")
	assert.Equal(t, "Brutal Strike", f.Name)
	require.Len(t, f.Effects, 1)
	assert.Equal(t, EffectExtraDamageDice, f.Effects[0].Type)
	assert.Equal(t, TriggerOnDamageRoll, f.Effects[0].Trigger)
	assert.Equal(t, "1d10", f.Effects[0].Dice)
	assert.Equal(t, []string{"slashing"}, f.Effects[0].DamageTypes)
}

// TestDetectAdvantage_ForgoAdvantage: ForgoAdvantage clears ALL advantage sources
// on the roll (not just Reckless), while disadvantage still applies.
func TestDetectAdvantage_ForgoAdvantage(t *testing.T) {
	// Reckless → advantage; ForgoAdvantage removes it.
	mode, adv, _ := DetectAdvantage(AdvantageInput{Reckless: true})
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, adv, "Reckless Attack")

	mode, adv, _ = DetectAdvantage(AdvantageInput{Reckless: true, ForgoAdvantage: true})
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, adv)

	// Forgoes a non-Reckless advantage source too (attacker hidden).
	mode, _, _ = DetectAdvantage(AdvantageInput{AttackerHidden: true, ForgoAdvantage: true})
	assert.Equal(t, dice.Normal, mode)

	// Disadvantage survives ForgoAdvantage (advantage gone → disadvantage stands).
	mode, _, dis := DetectAdvantage(AdvantageInput{Reckless: true, TargetHidden: true, ForgoAdvantage: true})
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, dis, "target hidden")
}

// TestBrutalStrikeEligible covers the gate: a known effect + the feature + using
// Reckless (declared or via the marker) + a STR melee attack.
func TestBrutalStrikeEligible(t *testing.T) {
	feats := pqtype.NullRawMessage{RawMessage: brutalStrikeFeatsJSON, Valid: true}
	maul := makeBrutalMaul()
	noConds := refdata.Combatant{Conditions: json.RawMessage(`[]`)}
	markerConds, _ := json.Marshal([]CombatCondition{{Condition: "reckless"}})

	declared := AttackCommand{BrutalStrike: "forceful", Reckless: true, Attacker: noConds}
	viaMarker := AttackCommand{BrutalStrike: "forceful", Attacker: refdata.Combatant{Conditions: markerConds}}
	notReckless := AttackCommand{BrutalStrike: "forceful", Attacker: noConds}

	assert.True(t, brutalStrikeEligible(declared, feats, maul, "str"), "declared reckless")
	assert.True(t, brutalStrikeEligible(viaMarker, feats, maul, "str"), "reckless marker")
	assert.False(t, brutalStrikeEligible(notReckless, feats, maul, "str"), "not using reckless")
	assert.False(t, brutalStrikeEligible(declared, pqtype.NullRawMessage{}, maul, "str"), "no feature")
	assert.False(t, brutalStrikeEligible(declared, feats, maul, "dex"), "not a STR attack")
	assert.False(t, brutalStrikeEligible(AttackCommand{BrutalStrike: "hamstring", Reckless: true, Attacker: noConds}, feats, maul, "str"), "unknown effect")
	assert.False(t, brutalStrikeEligible(AttackCommand{BrutalStrike: "", Reckless: true, Attacker: noConds}, feats, maul, "str"), "empty choice")
}

// --- Service.Attack end-to-end ----------------------------------------------

// brutalStrikeMockStore wires a level-9 Barbarian carrying the Brutal Strike
// feature, wielding a plain STR maul, on a 10x10 map. hasFeature toggles the
// feature. Returns the store and a pointer to captured position writes.
func brutalStrikeMockStore(t *testing.T, charID, mapID uuid.UUID, hasFeature bool) (*mockStore, *[]refdata.UpdateCombatantPositionParams) {
	t.Helper()
	classes := []CharacterClass{{Class: "Barbarian", Level: 9}}
	var feats []CharacterFeature
	if hasFeature {
		feats = []CharacterFeature{{Name: "Brutal Strike", MechanicalEffect: "brutal_strike"}}
	}
	char := makeCharacterWithFeats(16, 10, 4, "maul", feats, classes)
	char.ID = charID

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) { return char, nil }
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) { return makeBrutalMaul(), nil }
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	ms.getEncounterFn = func(ctx context.Context, id uuid.UUID) (refdata.Encounter, error) {
		return refdata.Encounter{ID: id, MapID: uuid.NullUUID{UUID: mapID, Valid: true}}, nil
	}
	ms.getMapByIDUncheckedFn = func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
		return refdata.Map{ID: id, WidthSquares: 10, HeightSquares: 10}, nil
	}
	ms.getCreatureFn = func(ctx context.Context, id string) (refdata.Creature, error) {
		return refdata.Creature{ID: id, Size: "Medium"}, nil
	}
	var posWrites []refdata.UpdateCombatantPositionParams
	ms.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		posWrites = append(posWrites, arg)
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}
	return ms, &posWrites
}

// brutalRecklessAttacker is a barbarian already carrying the transient reckless
// marker (an earlier attack this turn), at col A row 3.
func brutalRecklessAttacker(charID, attackerID, encounterID uuid.UUID) refdata.Combatant {
	marker, _ := json.Marshal([]CombatCondition{{Condition: "reckless"}})
	return refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Grog",
		PositionCol: "A",
		PositionRow: 3,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(marker),
	}
}

// TestServiceAttack_BrutalStrikeForcefulBlow: a Barbarian-9 using Reckless (marker)
// opts into brutal:forceful — the advantage is forgone (normal roll), the hit deals
// +1d10, and the target is pushed 15 ft (3 squares).
func TestServiceAttack_BrutalStrikeForcefulBlow(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites := brutalStrikeMockStore(t, charID, mapID, true)
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{brutalRecklessAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 5 // 2d6 weapon + 1d10 brutal
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:     brutalRecklessAttacker(charID, attackerID, encounterID),
		Target:       pushTarget(targetID, encounterID, "goblin"),
		Turn:         turn,
		BrutalStrike: "forceful",
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, dice.Normal, result.RollMode, "Brutal Strike forgoes the Reckless advantage")
	assert.Equal(t, "forceful", result.BrutalStrikeChoice)
	// 2d6(10) + STR(3) + 1d10(5) = 18 (the +1d10 rider fired).
	assert.Equal(t, 18, result.DamageTotal, "Brutal Strike adds +1d10")
	// Pushed 3 squares from col B(2) away from A(1) → col E(5), same row.
	require.NotEmpty(t, *posWrites, "Forceful Blow must push the target")
	last := (*posWrites)[len(*posWrites)-1]
	assert.Equal(t, targetID, last.ID)
	assert.Equal(t, "E", last.PositionCol, "15 ft = 3 squares: B→E")
	assert.Equal(t, int32(3), last.PositionRow)
}

// TestServiceAttack_BrutalStrike_NoFeatureInert: without the feature, brutal:forceful
// is inert — the Reckless advantage is kept, no +1d10, no push.
func TestServiceAttack_BrutalStrike_NoFeatureInert(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites := brutalStrikeMockStore(t, charID, mapID, false)
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{brutalRecklessAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit (advantage rolls twice, both 18)
		}
		return 5
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker:     brutalRecklessAttacker(charID, attackerID, encounterID),
		Target:       pushTarget(targetID, encounterID, "goblin"),
		Turn:         turn,
		BrutalStrike: "forceful",
	}, roller)
	require.NoError(t, err)
	require.True(t, result.Hit)
	assert.Equal(t, dice.Advantage, result.RollMode, "no feature → Reckless advantage kept")
	assert.Empty(t, result.BrutalStrikeChoice, "no feature → brutal inert")
	// 2d6(10) + STR(3) = 13 (no +1d10).
	assert.Equal(t, 13, result.DamageTotal, "no feature → no Brutal +1d10")
	assert.Empty(t, *posWrites, "no feature → no push")
}

// TestFormatAttackLog_BrutalStrike surfaces the forgone Advantage + the effect.
func TestFormatAttackLog_BrutalStrike(t *testing.T) {
	base := AttackResult{
		AttackerName: "Grog", TargetName: "Goblin", WeaponName: "Maul",
		IsMelee: true, Hit: true, DamageTotal: 18, DamageType: "bludgeoning", DamageDice: "2d6",
		D20Roll: dice.D20Result{Total: 18, Chosen: 15, Modifier: 3},
	}

	hit := base
	hit.BrutalStrikeChoice = "forceful"
	log := FormatAttackLog(hit)
	assert.Contains(t, log, "Brutal Strike (Forceful Blow)")
	assert.Contains(t, log, "pushed up to 15 ft")

	miss := base
	miss.Hit = false
	miss.BrutalStrikeChoice = "forceful"
	assert.Contains(t, FormatAttackLog(miss), "Advantage forgone")
}
