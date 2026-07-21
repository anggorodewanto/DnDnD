package combat

import "testing"

// TestFastMovementFeature locks the FeatureDefinition shape for the 2024
// Barbarian L5 Fast Movement feature: a flat +10 ft turn-start speed bonus that
// applies only while NOT wearing Heavy armor (medium/light/none all qualify).
func TestFastMovementFeature(t *testing.T) {
	fd := FastMovementFeature()
	if fd.Name != "Fast Movement" {
		t.Errorf("Name = %q, want %q", fd.Name, "Fast Movement")
	}
	if fd.Source != "barbarian" {
		t.Errorf("Source = %q, want %q", fd.Source, "barbarian")
	}
	if len(fd.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(fd.Effects))
	}
	e := fd.Effects[0]
	if e.Type != EffectModifySpeed {
		t.Errorf("Type = %q, want %q", e.Type, EffectModifySpeed)
	}
	if e.Trigger != TriggerOnTurnStart {
		t.Errorf("Trigger = %q, want %q", e.Trigger, TriggerOnTurnStart)
	}
	if e.Modifier != 10 {
		t.Errorf("Modifier = %d, want 10", e.Modifier)
	}
	if !e.Conditions.NotWearingHeavyArmor {
		t.Error("expected NotWearingHeavyArmor condition")
	}
	// Fast Movement is gated ONLY on heavy armor, unlike Monk Unarmored Movement
	// which is gated on any armor + shield. Guard against copy-paste drift.
	if e.Conditions.NotWearingArmor {
		t.Error("Fast Movement must NOT gate on NotWearingArmor (medium/light armor still qualify)")
	}
	if e.Conditions.NotUsingShield {
		t.Error("Fast Movement must NOT gate on NotUsingShield (a shield does not block it)")
	}
}

// TestFastMovement_HeavyArmorGating drives the effect through ProcessEffects at
// TriggerOnTurnStart across the armor spectrum. Only Heavy armor blocks the bonus.
func TestFastMovement_HeavyArmorGating(t *testing.T) {
	features := []FeatureDefinition{FastMovementFeature()}

	t.Run("no armor gets +10", func(t *testing.T) {
		result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: false, WearingHeavyArmor: false})
		if result.SpeedModifier != 10 {
			t.Errorf("no armor: SpeedModifier = %d, want 10", result.SpeedModifier)
		}
	})
	t.Run("medium/light armor still gets +10", func(t *testing.T) {
		// WearingArmor true but not heavy: 2024 Fast Movement still applies.
		result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: true, WearingHeavyArmor: false})
		if result.SpeedModifier != 10 {
			t.Errorf("medium/light armor: SpeedModifier = %d, want 10", result.SpeedModifier)
		}
	})
	t.Run("shield does not block the bonus", func(t *testing.T) {
		result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: false, HasShield: true})
		if result.SpeedModifier != 10 {
			t.Errorf("with shield: SpeedModifier = %d, want 10", result.SpeedModifier)
		}
	})
	t.Run("heavy armor blocks the bonus", func(t *testing.T) {
		result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{WearingArmor: true, WearingHeavyArmor: true})
		if result.SpeedModifier != 0 {
			t.Errorf("heavy armor: SpeedModifier = %d, want 0", result.SpeedModifier)
		}
	})
}

// TestNotWearingHeavyArmorCondition locks the new EffectConditions predicate in
// EvaluateConditions, mirroring TestNotWearingArmorCondition.
func TestNotWearingHeavyArmorCondition(t *testing.T) {
	effect := Effect{
		Type:     EffectModifySpeed,
		Trigger:  TriggerOnTurnStart,
		Modifier: 10,
		Conditions: EffectConditions{
			NotWearingHeavyArmor: true,
		},
	}

	t.Run("not wearing heavy armor passes", func(t *testing.T) {
		if !EvaluateConditions(effect, EffectContext{WearingHeavyArmor: false}) {
			t.Error("should pass without heavy armor")
		}
	})
	t.Run("wearing medium armor still passes", func(t *testing.T) {
		if !EvaluateConditions(effect, EffectContext{WearingArmor: true, WearingHeavyArmor: false}) {
			t.Error("should pass with non-heavy armor")
		}
	})
	t.Run("wearing heavy armor fails", func(t *testing.T) {
		if EvaluateConditions(effect, EffectContext{WearingArmor: true, WearingHeavyArmor: true}) {
			t.Error("should fail with heavy armor")
		}
	})
}

// TestBuildFeatureDefinitions_BarbarianFastMovement locks the seed→FES mapping:
// a Barbarian L5+ carrying the `fast_movement` mechanical effect gains the Fast
// Movement definition; a sub-L5 barbarian does not (level-gated, mirrors the
// AuraOfProtection precedent so a mis-seeded feature can't leak the bonus).
func TestBuildFeatureDefinitions_BarbarianFastMovement(t *testing.T) {
	features := []CharacterFeature{
		{Name: "Fast Movement", MechanicalEffect: "fast_movement"},
	}

	t.Run("level 5 barbarian gains Fast Movement", func(t *testing.T) {
		classes := []CharacterClass{{Class: "Barbarian", Level: 5}}
		defs := BuildFeatureDefinitions(classes, features)
		if !hasFeatureNamed(defs, "Fast Movement") {
			t.Errorf("expected Fast Movement feature for L5 barbarian, got %+v", featureNames(defs))
		}
	})

	t.Run("level 4 barbarian does not gain Fast Movement", func(t *testing.T) {
		classes := []CharacterClass{{Class: "Barbarian", Level: 4}}
		defs := BuildFeatureDefinitions(classes, features)
		if hasFeatureNamed(defs, "Fast Movement") {
			t.Errorf("L4 barbarian must NOT gain Fast Movement, got %+v", featureNames(defs))
		}
	})
}

func hasFeatureNamed(defs []FeatureDefinition, name string) bool {
	for _, d := range defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

func featureNames(defs []FeatureDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return names
}
