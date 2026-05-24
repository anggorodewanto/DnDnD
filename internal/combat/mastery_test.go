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

// charDataWithMasteries builds a NullRawMessage character_data column carrying
// the supplied weapon_masteries JSON body.
func charDataWithMasteries(body string) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{RawMessage: json.RawMessage(body), Valid: true}
}

// --- fixtures ---

// makeGrazeGreatsword returns a greatsword whose mastery property is "graze".
func makeGrazeGreatsword() refdata.Weapon {
	return refdata.Weapon{
		ID:         "greatsword",
		Name:       "Greatsword",
		Damage:     "2d6",
		DamageType: "slashing",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "two-handed"},
		Mastery:    "graze",
	}
}

// makeToppleMaul returns a maul whose mastery property is "topple".
func makeToppleMaul() refdata.Weapon {
	return refdata.Weapon{
		ID:         "maul",
		Name:       "Maul",
		Damage:     "2d6",
		DamageType: "bludgeoning",
		WeaponType: "martial_melee",
		Properties: []string{"heavy", "two-handed"},
		Mastery:    "topple",
	}
}

// --- masteryActive ---

func TestMasteryActive(t *testing.T) {
	tests := []struct {
		name     string
		weapon   refdata.Weapon
		known    []string
		expected bool
	}{
		{
			name:     "weapon has no mastery",
			weapon:   makeLongsword(),
			known:    []string{"longsword"},
			expected: false,
		},
		{
			name:     "mastery weapon and attacker knows it",
			weapon:   makeGrazeGreatsword(),
			known:    []string{"greatsword"},
			expected: true,
		},
		{
			name:     "mastery weapon but attacker does not know it",
			weapon:   makeGrazeGreatsword(),
			known:    []string{"longsword"},
			expected: false,
		},
		{
			name:     "mastery weapon but empty known list",
			weapon:   makeGrazeGreatsword(),
			known:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := masteryActive(AttackInput{Weapon: tt.weapon, WeaponMasteries: tt.known})
			assert.Equal(t, tt.expected, got)
		})
	}
}

// --- parseWeaponMasteries ---

func TestParseWeaponMasteries(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected []string
	}{
		{
			name:     "empty raw",
			raw:      "",
			expected: nil,
		},
		{
			name:     "missing key",
			raw:      `{"prepared_spells":["bless"]}`,
			expected: nil,
		},
		{
			name:     "valid list",
			raw:      `{"weapon_masteries":["greatsword","maul"]}`,
			expected: []string{"greatsword", "maul"},
		},
		{
			name:     "invalid json tolerated",
			raw:      `{not json`,
			expected: nil,
		},
		{
			name:     "wrong type tolerated",
			raw:      `{"weapon_masteries":"greatsword"}`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWeaponMasteries(json.RawMessage(tt.raw))
			assert.Equal(t, tt.expected, got)
		})
	}
}

// --- Graze (pure ResolveAttack) ---

func TestResolveAttack_GrazeMissDealsAbilityMod(t *testing.T) {
	// d20 rolls 2 (+3 STR +2 prof = 7 < AC 20) → miss.
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 2
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        20,
		Weapon:          makeGrazeGreatsword(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"greatsword"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "graze", result.MasteryProperty)
	// STR 16 → +3 damage modifier, no dice.
	assert.Equal(t, 3, result.DamageTotal)
}

func TestResolveAttack_GrazeMissUnknownMasteryNoDamage(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 2
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        20,
		Weapon:          makeGrazeGreatsword(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: nil, // attacker does NOT know greatsword mastery
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
	assert.Equal(t, 0, result.DamageTotal)
}

func TestResolveAttack_GrazeNegativeModClampsToZero(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 2
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        20,
		Weapon:          makeGrazeGreatsword(),
		Scores:          AbilityScores{Str: 6, Dex: 6}, // -2 mod
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"greatsword"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "graze", result.MasteryProperty)
	assert.Equal(t, 0, result.DamageTotal) // max(0, -2)
}

// --- Topple (pure ResolveAttack) ---

func TestResolveAttack_ToppleHitSetsSaveDC(t *testing.T) {
	// d20 rolls 18 (+3 STR +2 prof = 23 >= AC 13) → hit.
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        13,
		Weapon:          makeToppleMaul(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"maul"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "topple", result.MasteryProperty)
	// DC = 8 + prof(2) + STR mod(3) = 13
	assert.Equal(t, 13, result.MasteryToppleSaveDC)
}

func TestResolveAttack_ToppleHitUnknownMasteryNoSave(t *testing.T) {
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 6
	})

	input := AttackInput{
		AttackerName:    "Aria",
		TargetName:      "Goblin #1",
		TargetAC:        13,
		Weapon:          makeToppleMaul(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: nil,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
	assert.Equal(t, 0, result.MasteryToppleSaveDC)
}

// --- Graze (service-level) ---

// grazeTestSetup wires a mock store for a melee miss with a graze weapon and
// records the HP write the service performs against the target.
func TestServiceAttack_GrazeMissDealsDamageToTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "greatsword")
	char.ID = charID
	// Attacker knows the greatsword mastery.
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["greatsword"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGrazeGreatsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	var hpWrites []int32
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpWrites = append(hpWrites, arg.HpCurrent)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	// d20 rolls 1 → guaranteed miss.
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 1
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          18,
		HpCurrent:   10,
		HpMax:       10,
		IsAlive:     true,
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "graze", result.MasteryProperty)
	assert.Equal(t, 3, result.DamageTotal) // STR 16 → +3
	// Target HP must drop by 3 (10 → 7).
	require.NotEmpty(t, hpWrites, "expected an HP write on graze miss")
	assert.Equal(t, int32(7), hpWrites[len(hpWrites)-1])
}

func TestServiceAttack_GrazeMissUnknownMasteryNoDamage(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "greatsword")
	char.ID = charID
	// No weapon_masteries → attacker does not know it.
	char.CharacterData = charDataWithMasteries(`{}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeGrazeGreatsword(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	var hpWrites []int32
	ms.updateCombatantHPFn = func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
		hpWrites = append(hpWrites, arg.HpCurrent)
		return refdata.Combatant{ID: arg.ID, HpCurrent: arg.HpCurrent, Conditions: json.RawMessage(`[]`)}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 1
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	target := refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          18,
		HpCurrent:   10,
		HpMax:       10,
		IsAlive:     true,
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
	assert.Empty(t, hpWrites, "no HP write expected when mastery is unknown")
}

// --- Topple (service-level) ---

func toppleAttacker(charID, attackerID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          attackerID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A",
		PositionRow: 1,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func toppleTarget(targetID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          targetID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          10,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsVisible:   true, // visible so the attack rolls Normal (no hidden-target disadvantage)
		IsNpc:       true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func TestServiceAttack_ToppleHitFailedSaveAppliesProne(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "maul")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["maul"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeToppleMaul(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	// GetCombatant is used by ApplyCondition; return a clean target.
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return toppleTarget(targetID), nil
	}
	var appliedConditions []string
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		for _, c := range conds {
			appliedConditions = append(appliedConditions, c.Condition)
		}
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	// d20=18 (hit), then d20=1 for the CON save (target has +0 CON → 1 < DC 13 → fail).
	d20calls := 0
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			d20calls++
			if d20calls == 1 {
				return 18 // attack hits
			}
			return 1 // CON save fails
		}
		return 6
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: toppleAttacker(charID, attackerID),
		Target:   toppleTarget(targetID),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "topple", result.MasteryProperty)
	assert.Equal(t, 13, result.MasteryToppleSaveDC)
	assert.Contains(t, appliedConditions, "prone")
}

func TestServiceAttack_ToppleHitSuccessfulSaveNoProne(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "maul")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["maul"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeToppleMaul(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return toppleTarget(targetID), nil
	}
	var appliedConditions []string
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		for _, c := range conds {
			appliedConditions = append(appliedConditions, c.Condition)
		}
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	// d20=18 (hit), then d20=20 for CON save (20 >= DC 13 → success).
	d20calls := 0
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			d20calls++
			if d20calls == 1 {
				return 18
			}
			return 20
		}
		return 6
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: toppleAttacker(charID, attackerID),
		Target:   toppleTarget(targetID),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "topple", result.MasteryProperty)
	assert.NotContains(t, appliedConditions, "prone")
}

func TestServiceAttack_ToppleUnknownMasteryNoSave(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "maul")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{}`) // does not know maul mastery

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeToppleMaul(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return toppleTarget(targetID), nil
	}
	var appliedConditions []string
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		for _, c := range conds {
			appliedConditions = append(appliedConditions, c.Condition)
		}
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18
		}
		return 6
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: toppleAttacker(charID, attackerID),
		Target:   toppleTarget(targetID),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
	assert.NotContains(t, appliedConditions, "prone")
}
