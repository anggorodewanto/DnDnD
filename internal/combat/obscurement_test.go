package combat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func TestZoneObscurement_HeavyObscurement(t *testing.T) {
	assert.Equal(t, HeavilyObscured, ZoneObscurement("heavy_obscurement"))
}

func TestZoneObscurement_MagicalDarkness(t *testing.T) {
	assert.Equal(t, HeavilyObscured, ZoneObscurement("magical_darkness"))
}

func TestZoneObscurement_DimLight(t *testing.T) {
	assert.Equal(t, LightlyObscured, ZoneObscurement("dim_light"))
}

func TestZoneObscurement_LightObscurement(t *testing.T) {
	assert.Equal(t, LightlyObscured, ZoneObscurement("light_obscurement"))
}

func TestZoneObscurement_Darkness(t *testing.T) {
	assert.Equal(t, HeavilyObscured, ZoneObscurement("darkness"))
}

func TestZoneObscurement_UnknownType(t *testing.T) {
	assert.Equal(t, NotObscured, ZoneObscurement("damage"))
	assert.Equal(t, NotObscured, ZoneObscurement("control"))
	assert.Equal(t, NotObscured, ZoneObscurement(""))
}

// --- EffectiveObscurement tests ---

func TestEffectiveObscurement_NoObscurement(t *testing.T) {
	vision := VisionCapabilities{}
	assert.Equal(t, NotObscured, EffectiveObscurement(NotObscured, "damage", vision, 30))
}

func TestEffectiveObscurement_DarkvisionInDarkness(t *testing.T) {
	// Darkvision treats darkness as dim light (lightly obscured)
	vision := VisionCapabilities{DarkvisionFt: 60}
	assert.Equal(t, LightlyObscured, EffectiveObscurement(HeavilyObscured, "darkness", vision, 30))
}

func TestEffectiveObscurement_DarkvisionInDarkness_OutOfRange(t *testing.T) {
	// Darkvision only works within range
	vision := VisionCapabilities{DarkvisionFt: 30}
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "darkness", vision, 60))
}

func TestEffectiveObscurement_DarkvisionInDimLight(t *testing.T) {
	// Darkvision treats dim light as bright (no penalty)
	vision := VisionCapabilities{DarkvisionFt: 60}
	assert.Equal(t, NotObscured, EffectiveObscurement(LightlyObscured, "dim_light", vision, 30))
}

func TestEffectiveObscurement_DarkvisionInMagicalDarkness(t *testing.T) {
	// Darkvision does NOT help in magical darkness
	vision := VisionCapabilities{DarkvisionFt: 60}
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 30))
}

func TestEffectiveObscurement_DarkvisionInFog(t *testing.T) {
	// Darkvision does NOT help in fog/heavy obscurement (fog is not darkness)
	vision := VisionCapabilities{DarkvisionFt: 60}
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "heavy_obscurement", vision, 30))
}

func TestEffectiveObscurement_BlindsightPenetratesAll(t *testing.T) {
	vision := VisionCapabilities{BlindsightFt: 60}
	// Blindsight penetrates magical darkness
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 30))
	// Blindsight penetrates fog
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "heavy_obscurement", vision, 30))
	// Blindsight penetrates normal darkness
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "darkness", vision, 30))
}

func TestEffectiveObscurement_BlindsightOutOfRange(t *testing.T) {
	vision := VisionCapabilities{BlindsightFt: 20}
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 30))
}

func TestEffectiveObscurement_TruesightPenetratesAll(t *testing.T) {
	vision := VisionCapabilities{TruesightFt: 60}
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 30))
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "heavy_obscurement", vision, 30))
}

func TestEffectiveObscurement_DevilsSightInMagicalDarkness(t *testing.T) {
	vision := VisionCapabilities{HasDevilsSight: true}
	// Devil's Sight penetrates magical darkness (120ft range by RAW)
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 120))
}

func TestEffectiveObscurement_DevilsSightBeyondRange(t *testing.T) {
	vision := VisionCapabilities{HasDevilsSight: true}
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 125))
}

func TestEffectiveObscurement_DarkvisionInLightObscurement(t *testing.T) {
	// Darkvision has no special interaction with light_obscurement
	vision := VisionCapabilities{DarkvisionFt: 60}
	assert.Equal(t, LightlyObscured, EffectiveObscurement(LightlyObscured, "light_obscurement", vision, 30))
}

// --- DetectAdvantage obscurement integration tests ---

func TestDetectAdvantage_AttackerHeavilyObscured(t *testing.T) {
	input := AdvantageInput{
		AttackerObscurement: HeavilyObscured,
	}
	mode, _, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Contains(t, disadvReasons, "heavily obscured (blinded)")
}

func TestDetectAdvantage_TargetHeavilyObscured(t *testing.T) {
	input := AdvantageInput{
		TargetObscurement: HeavilyObscured,
	}
	mode, advReasons, _ := DetectAdvantage(input)
	assert.Equal(t, dice.Advantage, mode)
	assert.Contains(t, advReasons, "target heavily obscured (blinded)")
}

func TestDetectAdvantage_BothHeavilyObscured(t *testing.T) {
	input := AdvantageInput{
		AttackerObscurement: HeavilyObscured,
		TargetObscurement:   HeavilyObscured,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.AdvantageAndDisadvantage, mode)
	assert.Contains(t, advReasons, "target heavily obscured (blinded)")
	assert.Contains(t, disadvReasons, "heavily obscured (blinded)")
}

func TestDetectAdvantage_LightlyObscured_NoAttackEffect(t *testing.T) {
	// Lightly obscured only affects perception, not attacks
	input := AdvantageInput{
		AttackerObscurement: LightlyObscured,
		TargetObscurement:   LightlyObscured,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, advReasons)
	assert.Empty(t, disadvReasons)
}

// --- ObscurementCheckEffect tests ---

func TestObscurementCheckEffect_LightlyObscured_Perception(t *testing.T) {
	mode, reason := ObscurementCheckEffect(LightlyObscured, "perception")
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Equal(t, "lightly obscured: disadvantage on Perception", reason)
}

func TestObscurementCheckEffect_LightlyObscured_NonPerception(t *testing.T) {
	mode, reason := ObscurementCheckEffect(LightlyObscured, "athletics")
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reason)
}

func TestObscurementCheckEffect_HeavilyObscured_Perception(t *testing.T) {
	// Heavily obscured means effectively blinded — auto-fail on sight-based checks
	// is handled via the blinded condition; here we return disadvantage as a baseline
	mode, reason := ObscurementCheckEffect(HeavilyObscured, "perception")
	assert.Equal(t, dice.Disadvantage, mode)
	assert.Equal(t, "heavily obscured: disadvantage on Perception", reason)
}

func TestObscurementCheckEffect_NotObscured(t *testing.T) {
	mode, reason := ObscurementCheckEffect(NotObscured, "perception")
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reason)
}

// --- ObscurementAllowsHide tests ---

func TestObscurementAllowsHide_LightlyObscured(t *testing.T) {
	assert.True(t, ObscurementAllowsHide(LightlyObscured))
}

func TestObscurementAllowsHide_HeavilyObscured(t *testing.T) {
	assert.True(t, ObscurementAllowsHide(HeavilyObscured))
}

func TestObscurementAllowsHide_NotObscured(t *testing.T) {
	assert.False(t, ObscurementAllowsHide(NotObscured))
}

// --- ObscurementReasonString tests ---

func TestObscurementReasonString_DimLightNoDarkvision(t *testing.T) {
	reason := ObscurementReasonString("dim_light", VisionCapabilities{}, 30)
	assert.Equal(t, "dim light, no darkvision", reason)
}

func TestObscurementReasonString_DarkvisionDowngradesDarkness(t *testing.T) {
	reason := ObscurementReasonString("darkness", VisionCapabilities{DarkvisionFt: 60}, 30)
	assert.Equal(t, "darkness \u2192 dim (darkvision)", reason)
}

func TestObscurementReasonString_MagicalDarkness(t *testing.T) {
	reason := ObscurementReasonString("magical_darkness", VisionCapabilities{}, 30)
	assert.Equal(t, "magical darkness", reason)
}

func TestObscurementReasonString_MagicalDarknessWithDarkvision(t *testing.T) {
	// Darkvision doesn't help with magical darkness
	reason := ObscurementReasonString("magical_darkness", VisionCapabilities{DarkvisionFt: 60}, 30)
	assert.Equal(t, "magical darkness", reason)
}

func TestObscurementReasonString_DarknessNoDarkvision(t *testing.T) {
	reason := ObscurementReasonString("darkness", VisionCapabilities{}, 30)
	assert.Equal(t, "darkness, no darkvision", reason)
}

func TestObscurementReasonString_HeavyObscurement(t *testing.T) {
	reason := ObscurementReasonString("heavy_obscurement", VisionCapabilities{}, 30)
	assert.Equal(t, "heavy obscurement", reason)
}

func TestObscurementReasonString_LightObscurement(t *testing.T) {
	reason := ObscurementReasonString("light_obscurement", VisionCapabilities{}, 30)
	assert.Equal(t, "light obscurement", reason)
}

func TestObscurementReasonString_NoObscurement(t *testing.T) {
	reason := ObscurementReasonString("damage", VisionCapabilities{}, 30)
	assert.Equal(t, "", reason)
}

func TestObscurementReasonString_DimLightWithDarkvision(t *testing.T) {
	// Darkvision in dim light: treats as bright, so no reason string
	reason := ObscurementReasonString("dim_light", VisionCapabilities{DarkvisionFt: 60}, 30)
	assert.Equal(t, "", reason)
}

func TestObscurementReasonString_BlindsightNegates(t *testing.T) {
	reason := ObscurementReasonString("magical_darkness", VisionCapabilities{BlindsightFt: 60}, 30)
	assert.Equal(t, "", reason)
}

// --- ObscurementLevel.String() tests ---

func TestObscurementLevel_String(t *testing.T) {
	assert.Equal(t, "none", NotObscured.String())
	assert.Equal(t, "lightly obscured", LightlyObscured.String())
	assert.Equal(t, "heavily obscured", HeavilyObscured.String())
}

// --- IsMagicalDarkness helper ---

func TestIsMagicalDarkness(t *testing.T) {
	assert.True(t, IsMagicalDarkness("magical_darkness"))
	assert.False(t, IsMagicalDarkness("darkness"))
	assert.False(t, IsMagicalDarkness("heavy_obscurement"))
	assert.False(t, IsMagicalDarkness("dim_light"))
}

// --- Edge cases ---

func TestEffectiveObscurement_ExactBoundaryDistance(t *testing.T) {
	// Darkvision at exact range should still work
	vision := VisionCapabilities{DarkvisionFt: 30}
	assert.Equal(t, LightlyObscured, EffectiveObscurement(HeavilyObscured, "darkness", vision, 30))
	// One foot beyond should NOT work
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "darkness", vision, 31))
}

func TestEffectiveObscurement_DevilsSightInNormalDarkness(t *testing.T) {
	vision := VisionCapabilities{HasDevilsSight: true}
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "darkness", vision, 120))
}

func TestEffectiveObscurement_DevilsSightInDimLight(t *testing.T) {
	vision := VisionCapabilities{HasDevilsSight: true}
	assert.Equal(t, NotObscured, EffectiveObscurement(LightlyObscured, "dim_light", vision, 60))
}

func TestEffectiveObscurement_DevilsSightDoesNotHelpFog(t *testing.T) {
	vision := VisionCapabilities{HasDevilsSight: true}
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "heavy_obscurement", vision, 30))
}

func TestEffectiveObscurement_TruesightExactBoundary(t *testing.T) {
	vision := VisionCapabilities{TruesightFt: 30}
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 30))
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 31))
}

func TestEffectiveObscurement_CombinedVision_BlindsightTakesPriority(t *testing.T) {
	vision := VisionCapabilities{DarkvisionFt: 60, BlindsightFt: 30}
	// Within blindsight range: fully negated even in magical darkness
	assert.Equal(t, NotObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 30))
	// Beyond blindsight but within darkvision: darkvision doesn't help with magical darkness
	assert.Equal(t, HeavilyObscured, EffectiveObscurement(HeavilyObscured, "magical_darkness", vision, 50))
}

func TestObscurementCheckEffect_HeavilyObscured_Investigation(t *testing.T) {
	// Non-perception checks are not affected
	mode, reason := ObscurementCheckEffect(HeavilyObscured, "investigation")
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, reason)
}

func TestObscurementReasonString_DevilsSightInDarkness(t *testing.T) {
	vision := VisionCapabilities{HasDevilsSight: true}
	reason := ObscurementReasonString("darkness", vision, 60)
	assert.Equal(t, "", reason)
}

func TestObscurementReasonString_TruesightInFog(t *testing.T) {
	vision := VisionCapabilities{TruesightFt: 60}
	reason := ObscurementReasonString("heavy_obscurement", vision, 30)
	assert.Equal(t, "", reason)
}

// --- CombatantObscurement tests ---

func TestCombatantObscurement_NoZones(t *testing.T) {
	level := CombatantObscurement(2, 3, nil, VisionCapabilities{})
	assert.Equal(t, NotObscured, level)
}

func TestCombatantObscurement_NotInAnyZone(t *testing.T) {
	zones := []ZoneInfo{
		{
			ZoneType:  "darkness",
			Shape:     "circle",
			OriginCol: "F",
			OriginRow: 6,
			Dimensions: json.RawMessage(`{"radius_ft":10}`),
		},
	}
	level := CombatantObscurement(0, 0, zones, VisionCapabilities{}) // A1, far from F6
	assert.Equal(t, NotObscured, level)
}

func TestCombatantObscurement_InDarknessZone_NoDarkvision(t *testing.T) {
	zones := []ZoneInfo{
		{
			ZoneType:  "darkness",
			Shape:     "circle",
			OriginCol: "C",
			OriginRow: 3,
			Dimensions: json.RawMessage(`{"radius_ft":15}`),
		},
	}
	// C3 origin, radius 15ft => covers C3 area. Combatant at col=2 (C), row=2 (row 3, 0-based)
	level := CombatantObscurement(2, 2, zones, VisionCapabilities{})
	assert.Equal(t, HeavilyObscured, level)
}

func TestCombatantObscurement_InDarknessZone_WithDarkvision(t *testing.T) {
	zones := []ZoneInfo{
		{
			ZoneType:  "darkness",
			Shape:     "circle",
			OriginCol: "C",
			OriginRow: 3,
			Dimensions: json.RawMessage(`{"radius_ft":15}`),
		},
	}
	vision := VisionCapabilities{DarkvisionFt: 60}
	level := CombatantObscurement(2, 2, zones, vision)
	assert.Equal(t, LightlyObscured, level) // Darkvision downgrades darkness to dim
}

func TestCombatantObscurement_InMagicalDarkness_WithDarkvision(t *testing.T) {
	zones := []ZoneInfo{
		{
			ZoneType:  "magical_darkness",
			Shape:     "circle",
			OriginCol: "C",
			OriginRow: 3,
			Dimensions: json.RawMessage(`{"radius_ft":15}`),
		},
	}
	vision := VisionCapabilities{DarkvisionFt: 60}
	level := CombatantObscurement(2, 2, zones, vision)
	assert.Equal(t, HeavilyObscured, level) // Darkvision doesn't help with magical darkness
}

func TestCombatantObscurement_WorstZoneWins(t *testing.T) {
	// Combatant in both dim_light and darkness zone — darkness wins
	zones := []ZoneInfo{
		{
			ZoneType:  "dim_light",
			Shape:     "circle",
			OriginCol: "C",
			OriginRow: 3,
			Dimensions: json.RawMessage(`{"radius_ft":15}`),
		},
		{
			ZoneType:  "darkness",
			Shape:     "circle",
			OriginCol: "C",
			OriginRow: 3,
			Dimensions: json.RawMessage(`{"radius_ft":15}`),
		},
	}
	level := CombatantObscurement(2, 2, zones, VisionCapabilities{})
	assert.Equal(t, HeavilyObscured, level)
}

func TestCombatantObscurement_DamageZoneIgnored(t *testing.T) {
	zones := []ZoneInfo{
		{
			ZoneType:  "damage",
			Shape:     "circle",
			OriginCol: "C",
			OriginRow: 3,
			Dimensions: json.RawMessage(`{"radius_ft":15}`),
		},
	}
	level := CombatantObscurement(2, 2, zones, VisionCapabilities{})
	assert.Equal(t, NotObscured, level)
}

// --- AttackInput obscurement wiring tests ---

func TestResolveAttack_AttackerInHeavyObscurement_GetsDisadvantage(t *testing.T) {
	roller := dice.NewRoller(func(n int) int { return n - 1 }) // always max
	input := AttackInput{
		AttackerName:        "Thorn",
		TargetName:          "Goblin",
		TargetAC:            12,
		Weapon:              refdata.Weapon{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeaponType: "martial_melee"},
		Scores:              AbilityScores{Str: 16, Dex: 12, Con: 14, Int: 10, Wis: 12, Cha: 8},
		ProfBonus:           2,
		DistanceFt:          5,
		AttackerObscurement: HeavilyObscured,
	}
	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Disadvantage, result.RollMode)
	assert.Contains(t, result.DisadvantageReasons, "heavily obscured (blinded)")
}

func TestResolveAttack_TargetInHeavyObscurement_GetsAdvantage(t *testing.T) {
	roller := dice.NewRoller(func(n int) int { return n - 1 })
	input := AttackInput{
		AttackerName:       "Aria",
		TargetName:         "Goblin",
		TargetAC:           12,
		Weapon:             refdata.Weapon{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeaponType: "martial_melee"},
		Scores:             AbilityScores{Str: 16, Dex: 12, Con: 14, Int: 10, Wis: 12, Cha: 8},
		ProfBonus:          2,
		DistanceFt:         5,
		TargetObscurement:  HeavilyObscured,
	}
	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Advantage, result.RollMode)
	assert.Contains(t, result.AdvantageReasons, "target heavily obscured (blinded)")
}

func TestResolveAttack_LightObscurement_NoAttackEffect(t *testing.T) {
	roller := dice.NewRoller(func(n int) int { return n - 1 })
	input := AttackInput{
		AttackerName:        "Thorn",
		TargetName:          "Goblin",
		TargetAC:            12,
		Weapon:              refdata.Weapon{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeaponType: "martial_melee"},
		Scores:              AbilityScores{Str: 16, Dex: 12, Con: 14, Int: 10, Wis: 12, Cha: 8},
		ProfBonus:           2,
		DistanceFt:          5,
		AttackerObscurement: LightlyObscured,
	}
	result, err := ResolveAttack(input, roller)
	require.NoError(t, err)
	assert.Equal(t, dice.Normal, result.RollMode)
}

func TestDetectAdvantage_NotObscured_NoEffect(t *testing.T) {
	input := AdvantageInput{
		AttackerObscurement: NotObscured,
		TargetObscurement:   NotObscured,
	}
	mode, advReasons, disadvReasons := DetectAdvantage(input)
	assert.Equal(t, dice.Normal, mode)
	assert.Empty(t, advReasons)
	assert.Empty(t, disadvReasons)
}
