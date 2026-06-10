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

// makeVexRapier returns a rapier whose mastery property is "vex".
func makeVexRapier() refdata.Weapon {
	return refdata.Weapon{
		ID:         "rapier",
		Name:       "Rapier",
		Damage:     "1d8",
		DamageType: "piercing",
		WeaponType: "martial_melee",
		Properties: []string{"finesse"},
		Mastery:    "vex",
	}
}

// makeSapMace returns a mace whose mastery property is "sap".
func makeSapMace() refdata.Weapon {
	return refdata.Weapon{
		ID:         "mace",
		Name:       "Mace",
		Damage:     "1d6",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
		Properties: []string{},
		Mastery:    "sap",
	}
}

// makeSlowClub returns a club whose mastery property is "slow". The 2024 Slow
// mastery lives on light melee weapons (club, dagger, sickle, etc.) and the
// sling; a melee club keeps the pure-resolve / service tests at 5ft so no
// ranged-range or ammunition wiring is involved.
func makeSlowClub() refdata.Weapon {
	return refdata.Weapon{
		ID:         "club",
		Name:       "Club",
		Damage:     "1d4",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
		Properties: []string{"light"},
		Mastery:    "slow",
	}
}

// makePushGreatclub returns a greatclub whose mastery property is "push".
func makePushGreatclub() refdata.Weapon {
	return refdata.Weapon{
		ID:         "greatclub",
		Name:       "Greatclub",
		Damage:     "1d8",
		DamageType: "bludgeoning",
		WeaponType: "simple_melee",
		Properties: []string{"two-handed"},
		Mastery:    "push",
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

// --- Vex (pure ResolveAttack) ---

func TestResolveAttack_VexHitSetsMasteryProperty(t *testing.T) {
	// d20 rolls 18 (+3 STR/DEX +2 prof = 23 >= AC 13) → hit.
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
		Weapon:          makeVexRapier(),
		Scores:          AbilityScores{Str: 10, Dex: 16},
		ProfBonus:       2,
		DistanceFt:      5,
		AbilityUsed:     "dex",
		WeaponMasteries: []string{"rapier"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "vex", result.MasteryProperty)
}

func TestResolveAttack_VexHitUnknownMasteryNoProperty(t *testing.T) {
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
		Weapon:          makeVexRapier(),
		Scores:          AbilityScores{Str: 10, Dex: 16},
		ProfBonus:       2,
		DistanceFt:      5,
		AbilityUsed:     "dex",
		WeaponMasteries: nil, // attacker does NOT know rapier mastery
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
}

func TestResolveAttack_VexMissNoProperty(t *testing.T) {
	// d20 rolls 2 (+3 DEX +2 prof = 7 < AC 20) → miss.
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
		Weapon:          makeVexRapier(),
		Scores:          AbilityScores{Str: 10, Dex: 16},
		ProfBonus:       2,
		DistanceFt:      5,
		AbilityUsed:     "dex",
		WeaponMasteries: []string{"rapier"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty, "vex only fires on a hit")
}

// --- Sap (pure ResolveAttack) ---

func TestResolveAttack_SapHitSetsMasteryProperty(t *testing.T) {
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
		Weapon:          makeSapMace(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"mace"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "sap", result.MasteryProperty)
}

func TestResolveAttack_SapHitUnknownMasteryNoProperty(t *testing.T) {
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
		Weapon:          makeSapMace(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: nil,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
}

func TestResolveAttack_SapMissNoProperty(t *testing.T) {
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
		Weapon:          makeSapMace(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"mace"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty, "sap only fires on a hit")
}

// --- Slow (pure ResolveAttack) ---

func TestResolveAttack_SlowHitSetsMasteryProperty(t *testing.T) {
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
		Weapon:          makeSlowClub(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"club"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "slow", result.MasteryProperty)
}

func TestResolveAttack_SlowHitUnknownMasteryNoProperty(t *testing.T) {
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
		Weapon:          makeSlowClub(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: nil, // attacker does NOT know club mastery
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
}

func TestResolveAttack_SlowMissNoProperty(t *testing.T) {
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
		Weapon:          makeSlowClub(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"club"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty, "slow only fires on a hit")
}

// --- Push (pure ResolveAttack) ---

func TestResolveAttack_PushHitSetsMasteryProperty(t *testing.T) {
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
		Weapon:          makePushGreatclub(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"greatclub"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "push", result.MasteryProperty)
}

func TestResolveAttack_PushHitUnknownMasteryNoProperty(t *testing.T) {
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
		Weapon:          makePushGreatclub(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: nil,
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
}

func TestResolveAttack_PushMissNoProperty(t *testing.T) {
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
		Weapon:          makePushGreatclub(),
		Scores:          AbilityScores{Str: 16, Dex: 10},
		ProfBonus:       2,
		DistanceFt:      5,
		WeaponMasteries: []string{"greatclub"},
	}

	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.False(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty, "push only fires on a hit")
}

// --- Slow speed-reduction unit (EffectiveSpeed) ---

func slowedCondition() []CombatCondition {
	return []CombatCondition{{Condition: "slowed", DurationRounds: 1, ExpiresOn: "start_of_turn"}}
}

func TestEffectiveSpeed_SlowedReducesBy10(t *testing.T) {
	assert.Equal(t, 20, EffectiveSpeed(30, slowedCondition()))
}

func TestEffectiveSpeed_SlowedFlooredAtZero(t *testing.T) {
	assert.Equal(t, 0, EffectiveSpeed(5, slowedCondition()))
}

func TestEffectiveSpeed_NotSlowedUnchanged(t *testing.T) {
	assert.Equal(t, 30, EffectiveSpeed(30, []CombatCondition{}))
}

func TestEffectiveSpeed_GrappledStillZeroWithSlow(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "grappled"},
		{Condition: "slowed"},
	}
	assert.Equal(t, 0, EffectiveSpeed(30, conds))
}

// --- Push forced-move helper unit (computePushSquares) ---

func TestComputePushSquares_MovesTwoSquaresAway(t *testing.T) {
	// Attacker at col 1 (A), target at col 2 (B), same row → away vector is +col.
	col, row := computePushSquares(1, 3, 2, 3, 2, 10, 10, nil)
	assert.Equal(t, 4, col) // 2 + 2 squares
	assert.Equal(t, 3, row)
}

func TestComputePushSquares_DiagonalAway(t *testing.T) {
	// Attacker at (1,1), target at (2,2) → away vector is (+1,+1).
	col, row := computePushSquares(1, 1, 2, 2, 2, 10, 10, nil)
	assert.Equal(t, 4, col)
	assert.Equal(t, 4, row)
}

func TestComputePushSquares_ClampedAtMapEdge(t *testing.T) {
	// Target one square from the right edge (width 5 → cols 1..5).
	// Pushing +col would reach col 6 then 7; clamp keeps it at the last
	// in-bounds square it could reach (col 5).
	col, row := computePushSquares(3, 3, 4, 3, 2, 5, 5, nil)
	assert.Equal(t, 5, col)
	assert.Equal(t, 3, row)
}

func TestComputePushSquares_StopsBeforeOccupiedSquare(t *testing.T) {
	// An occupant sits at col 4, the first square the push would enter.
	// The target must stop in its current square (no movement possible).
	occupied := map[[2]int]bool{{4, 3}: true}
	col, row := computePushSquares(1, 3, 3, 3, 2, 10, 10, occupied)
	assert.Equal(t, 3, col) // could not advance past the occupied square
	assert.Equal(t, 3, row)
}

func TestComputePushSquares_StopsAtFirstSquareWhenSecondOccupied(t *testing.T) {
	// First square (col 4) free, second square (col 5) occupied → stop at col 4.
	occupied := map[[2]int]bool{{5, 3}: true}
	col, row := computePushSquares(1, 3, 3, 3, 2, 10, 10, occupied)
	assert.Equal(t, 4, col)
	assert.Equal(t, 3, row)
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

// --- Vex (service-level) ---

// TestServiceAttack_VexHitAppliesVexAdvantageToAttacker verifies a hit with a
// known vex weapon applies a vex_advantage condition to the ATTACKER, scoped
// to the target (mirrors help_advantage). The condition write lands on the
// attacker's combatant ID.
func TestServiceAttack_VexHitAppliesVexAdvantageToAttacker(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(10, 16, 2, "rapier")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["rapier"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeVexRapier(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	condWrites := make(map[uuid.UUID][]CombatCondition)
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		condWrites[arg.ID] = conds
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
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
		EncounterID: encounterID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "vex", result.MasteryProperty)

	attackerConds, ok := condWrites[attackerID]
	require.True(t, ok, "expected a condition write on the attacker for vex_advantage")
	var vex *CombatCondition
	for i := range attackerConds {
		if attackerConds[i].Condition == "vex_advantage" {
			vex = &attackerConds[i]
		}
	}
	require.NotNil(t, vex, "expected vex_advantage condition on attacker")
	assert.Equal(t, targetID.String(), vex.TargetCombatantID, "vex_advantage must be scoped to the target")
	// The vex_advantage condition must NOT be applied to the target.
	if targetConds, ok := condWrites[targetID]; ok {
		for _, c := range targetConds {
			assert.NotEqual(t, "vex_advantage", c.Condition, "vex_advantage must not land on the target")
		}
	}
}

// --- Sap (service-level) ---

// TestServiceAttack_SapHitAppliesSapDisadvantageToTarget verifies a hit with a
// known sap weapon applies a sap_disadvantage condition to the TARGET, so the
// target's next attack rolls at disadvantage.
func TestServiceAttack_SapHitAppliesSapDisadvantageToTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "mace")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["mace"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeSapMace(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	condWrites := make(map[uuid.UUID][]CombatCondition)
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		condWrites[arg.ID] = conds
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
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
		EncounterID: encounterID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "sap", result.MasteryProperty)

	targetConds, ok := condWrites[targetID]
	require.True(t, ok, "expected a condition write on the target for sap_disadvantage")
	var found bool
	for _, c := range targetConds {
		if c.Condition == "sap_disadvantage" {
			found = true
		}
	}
	assert.True(t, found, "expected sap_disadvantage condition on the target")
	// sap_disadvantage must NOT land on the attacker.
	if attackerConds, ok := condWrites[attackerID]; ok {
		for _, c := range attackerConds {
			assert.NotEqual(t, "sap_disadvantage", c.Condition, "sap_disadvantage must not land on the attacker")
		}
	}
}

// --- Slow (service-level) ---

// TestServiceAttack_SlowHitAppliesSlowedToTarget verifies a hit with a known
// slow weapon applies a "slowed" condition to the TARGET (single round, expires
// at the start of the attacker's next turn).
func TestServiceAttack_SlowHitAppliesSlowedToTarget(t *testing.T) {
	ctx := context.Background()
	charID := uuid.New()
	attackerID := uuid.New()
	targetID := uuid.New()
	turnID := uuid.New()
	encounterID := uuid.New()

	char := makeCharacter(16, 10, 2, "club")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["club"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makeSlowClub(), nil
	}
	ms.updateTurnActionsFn = func(ctx context.Context, arg refdata.UpdateTurnActionsParams) (refdata.Turn, error) {
		return refdata.Turn{ID: arg.ID, AttacksRemaining: arg.AttacksRemaining}, nil
	}
	ms.getCombatantFn = func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
		return refdata.Combatant{ID: id, Conditions: json.RawMessage(`[]`)}, nil
	}
	condWrites := make(map[uuid.UUID][]CombatCondition)
	ms.updateCombatantConditionsFn = func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
		var conds []CombatCondition
		_ = json.Unmarshal(arg.Conditions, &conds)
		condWrites[arg.ID] = conds
		return refdata.Combatant{ID: arg.ID, Conditions: arg.Conditions}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})

	attacker := refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
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
		EncounterID: encounterID,
		DisplayName: "Goblin #1",
		PositionCol: "B",
		PositionRow: 1,
		Ac:          13,
		HpCurrent:   20,
		HpMax:       20,
		IsAlive:     true,
		IsNpc:       true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}

	result, err := svc.Attack(ctx, AttackCommand{Attacker: attacker, Target: target, Turn: turn}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "slow", result.MasteryProperty)

	targetConds, ok := condWrites[targetID]
	require.True(t, ok, "expected a condition write on the target for slowed")
	var slowed *CombatCondition
	for i := range targetConds {
		if targetConds[i].Condition == "slowed" {
			slowed = &targetConds[i]
		}
	}
	require.NotNil(t, slowed, "expected slowed condition on the target")
	assert.Equal(t, 1, slowed.DurationRounds)
	assert.Equal(t, "start_of_turn", slowed.ExpiresOn)
	assert.Equal(t, attackerID.String(), slowed.SourceCombatantID, "slowed must expire on the attacker's turn")
	// slowed must NOT land on the attacker.
	if attackerConds, ok := condWrites[attackerID]; ok {
		for _, c := range attackerConds {
			assert.NotEqual(t, "slowed", c.Condition, "slowed must not land on the attacker")
		}
	}
}

// --- Push (service-level) ---

// pushMockStore wires a mock store for a push-mastery service test on a 10x10
// map with no other occupants. It returns the store plus a pointer to capture
// the position write performed on the target.
func pushMockStore(t *testing.T, charID, _ uuid.UUID, mapID uuid.UUID, _ string, creatureSize string) (*mockStore, *[]refdata.UpdateCombatantPositionParams) {
	t.Helper()
	char := makeCharacter(16, 10, 2, "greatclub")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{"weapon_masteries":["greatclub"]}`)

	ms := defaultMockStore()
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.getWeaponFn = func(ctx context.Context, id string) (refdata.Weapon, error) {
		return makePushGreatclub(), nil
	}
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
		return refdata.Creature{ID: id, Size: creatureSize}, nil
	}
	var posWrites []refdata.UpdateCombatantPositionParams
	ms.updateCombatantPositionFn = func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
		posWrites = append(posWrites, arg)
		return refdata.Combatant{ID: arg.ID, PositionCol: arg.PositionCol, PositionRow: arg.PositionRow, Conditions: json.RawMessage(`[]`)}, nil
	}
	return ms, &posWrites
}

func pushAttacker(charID, attackerID, encounterID uuid.UUID) refdata.Combatant {
	return refdata.Combatant{
		ID:          attackerID,
		EncounterID: encounterID,
		CharacterID: uuid.NullUUID{UUID: charID, Valid: true},
		DisplayName: "Aria",
		PositionCol: "A", // col 1
		PositionRow: 3,
		IsAlive:     true,
		IsVisible:   true,
		Conditions:  json.RawMessage(`[]`),
	}
}

func pushTarget(targetID, encounterID uuid.UUID, creatureRef string) refdata.Combatant {
	return refdata.Combatant{
		ID:            targetID,
		EncounterID:   encounterID,
		CreatureRefID: nullString(creatureRef),
		DisplayName:   "Goblin #1",
		PositionCol:   "B", // col 2, adjacent to attacker at col 1
		PositionRow:   3,
		Ac:            13,
		HpCurrent:     20,
		HpMax:         20,
		IsAlive:       true,
		IsNpc:         true,
		IsVisible:     true,
		Conditions:    json.RawMessage(`[]`),
	}
}

func TestServiceAttack_PushHitMovesTargetTwoSquaresAway(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites := pushMockStore(t, charID, targetID, mapID, "goblin", "Medium")
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pushAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
	}

	svc := NewService(ms)
	roller := dice.NewRoller(func(max int) int {
		if max == 20 {
			return 18 // hit
		}
		return 6
	})

	turn := refdata.Turn{ID: turnID, EncounterID: encounterID, CombatantID: attackerID, AttacksRemaining: 1}
	result, err := svc.Attack(ctx, AttackCommand{
		Attacker: pushAttacker(charID, attackerID, encounterID),
		Target:   pushTarget(targetID, encounterID, "goblin"),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "push", result.MasteryProperty)

	require.NotEmpty(t, *posWrites, "expected a position write for the pushed target")
	last := (*posWrites)[len(*posWrites)-1]
	assert.Equal(t, targetID, last.ID)
	// Attacker col 1, target col 2 → away vector +col, 2 squares → col 4 ("D").
	assert.Equal(t, "D", last.PositionCol)
	assert.Equal(t, int32(3), last.PositionRow)
}

func TestServiceAttack_PushClampedAtMapEdge(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites := pushMockStore(t, charID, targetID, mapID, "goblin", "Medium")
	// 3x3 map: cols/rows 1..3. Attacker at col1, target at col2 → push would
	// reach col4 (out of bounds) → clamp keeps it at col 3 ("C").
	ms.getMapByIDUncheckedFn = func(ctx context.Context, id uuid.UUID) (refdata.Map, error) {
		return refdata.Map{ID: id, WidthSquares: 3, HeightSquares: 3}, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pushAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
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
		Attacker: pushAttacker(charID, attackerID, encounterID),
		Target:   pushTarget(targetID, encounterID, "goblin"),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)

	require.NotEmpty(t, *posWrites, "expected a clamped position write")
	last := (*posWrites)[len(*posWrites)-1]
	assert.Equal(t, "C", last.PositionCol) // col 3, last in-bounds square
	assert.Equal(t, int32(3), last.PositionRow)
}

func TestServiceAttack_PushHugeTargetNotMoved(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites := pushMockStore(t, charID, targetID, mapID, "giant", "Huge")
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pushAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "giant")}, nil
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
		Attacker: pushAttacker(charID, attackerID, encounterID),
		Target:   pushTarget(targetID, encounterID, "giant"),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "push", result.MasteryProperty)
	assert.Empty(t, *posWrites, "Huge targets must not be pushed")
}

func TestServiceAttack_PushUnknownMasteryNotMoved(t *testing.T) {
	ctx := context.Background()
	charID, attackerID, targetID := uuid.New(), uuid.New(), uuid.New()
	turnID, encounterID, mapID := uuid.New(), uuid.New(), uuid.New()

	ms, posWrites := pushMockStore(t, charID, targetID, mapID, "goblin", "Medium")
	// Attacker does NOT know the greatclub mastery.
	char := makeCharacter(16, 10, 2, "greatclub")
	char.ID = charID
	char.CharacterData = charDataWithMasteries(`{}`)
	ms.getCharacterFn = func(ctx context.Context, id uuid.UUID) (refdata.Character, error) {
		return char, nil
	}
	ms.listCombatantsByEncounterIDFn = func(ctx context.Context, eid uuid.UUID) ([]refdata.Combatant, error) {
		return []refdata.Combatant{pushAttacker(charID, attackerID, encounterID), pushTarget(targetID, encounterID, "goblin")}, nil
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
		Attacker: pushAttacker(charID, attackerID, encounterID),
		Target:   pushTarget(targetID, encounterID, "goblin"),
		Turn:     turn,
	}, roller)
	require.NoError(t, err)
	assert.True(t, result.Hit)
	assert.Equal(t, "", result.MasteryProperty)
	assert.Empty(t, *posWrites, "no push without a known mastery")
}
