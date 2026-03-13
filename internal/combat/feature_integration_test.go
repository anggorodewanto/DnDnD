package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

func TestSneakAttackFeature(t *testing.T) {
	fd := SneakAttackFeature(5)

	if fd.Name != "Sneak Attack" {
		t.Errorf("Name = %q, want %q", fd.Name, "Sneak Attack")
	}
	if fd.Source != "rogue" {
		t.Errorf("Source = %q, want %q", fd.Source, "rogue")
	}
	if len(fd.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(fd.Effects))
	}

	e := fd.Effects[0]
	if e.Type != EffectExtraDamageDice {
		t.Errorf("Type = %q, want %q", e.Type, EffectExtraDamageDice)
	}
	if e.Trigger != TriggerOnDamageRoll {
		t.Errorf("Trigger = %q, want %q", e.Trigger, TriggerOnDamageRoll)
	}
	if e.Dice != "3d6" {
		t.Errorf("Dice = %q, want %q", e.Dice, "3d6")
	}
	if !e.Conditions.OncePerTurn {
		t.Error("expected OncePerTurn = true")
	}
	if e.Conditions.AdvantageOrAllyWithin != 5 {
		t.Errorf("AdvantageOrAllyWithin = %d, want 5", e.Conditions.AdvantageOrAllyWithin)
	}
	if len(e.Conditions.WeaponProperties) != 2 {
		t.Errorf("WeaponProperties len = %d, want 2", len(e.Conditions.WeaponProperties))
	}
}

func TestSneakAttackWithAdvantage(t *testing.T) {
	features := []FeatureDefinition{SneakAttackFeature(5)}
	ctx := EffectContext{
		WeaponProperty: "finesse",
		HasAdvantage:   true,
	}
	// Sneak Attack should NOT apply with the basic condition filtering because
	// it only has OncePerTurn. We need SneakAttackEligible to check weapon + advantage/ally.
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if len(result.ExtraDice) != 1 || result.ExtraDice[0] != "3d6" {
		t.Errorf("expected extra dice [3d6], got %v", result.ExtraDice)
	}
}

func TestSneakAttackWithAlly(t *testing.T) {
	features := []FeatureDefinition{SneakAttackFeature(3)}
	ctx := EffectContext{
		WeaponProperty: "finesse",
		HasAdvantage:   false,
		AllyWithinFt:   5,
	}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if len(result.ExtraDice) != 1 || result.ExtraDice[0] != "2d6" {
		t.Errorf("expected extra dice [2d6], got %v", result.ExtraDice)
	}
}

func TestSneakAttackNoEligibility(t *testing.T) {
	// No advantage and no ally within range
	features := []FeatureDefinition{SneakAttackFeature(5)}
	ctx := EffectContext{
		WeaponProperty: "finesse",
		HasAdvantage:   false,
		AllyWithinFt:   999, // no ally nearby
	}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if len(result.ExtraDice) != 0 {
		t.Errorf("expected no extra dice without advantage or ally, got %v", result.ExtraDice)
	}
}

func TestSneakAttackNoFinesseOrRanged(t *testing.T) {
	features := []FeatureDefinition{SneakAttackFeature(5)}
	ctx := EffectContext{
		WeaponProperty: "heavy", // not finesse
		HasAdvantage:   true,
	}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if len(result.ExtraDice) != 0 {
		t.Errorf("expected no extra dice without finesse weapon, got %v", result.ExtraDice)
	}
}

func TestEvasionFeature(t *testing.T) {
	fd := EvasionFeature()

	if fd.Name != "Evasion" {
		t.Errorf("Name = %q, want %q", fd.Name, "Evasion")
	}
	if len(fd.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(fd.Effects))
	}
	e := fd.Effects[0]
	if e.Type != EffectModifySave {
		t.Errorf("Type = %q, want %q", e.Type, EffectModifySave)
	}
	if e.Trigger != TriggerOnSave {
		t.Errorf("Trigger = %q, want %q", e.Trigger, TriggerOnSave)
	}
	if e.On != "evasion" {
		t.Errorf("On = %q, want %q", e.On, "evasion")
	}
}

func TestApplyEvasion_SaveSuccess(t *testing.T) {
	// DEX save success with Evasion: damage should be 0
	damage := ApplyEvasion(20, true)
	if damage != 0 {
		t.Errorf("ApplyEvasion(20, success=true) = %d, want 0", damage)
	}
}

func TestApplyEvasion_SaveFail(t *testing.T) {
	// DEX save failure with Evasion: damage should be halved
	damage := ApplyEvasion(20, false)
	if damage != 10 {
		t.Errorf("ApplyEvasion(20, success=false) = %d, want 10", damage)
	}
}

func TestApplyEvasion_OddDamage(t *testing.T) {
	// DEX save failure: 21 damage halved = 10 (rounded down)
	damage := ApplyEvasion(21, false)
	if damage != 10 {
		t.Errorf("ApplyEvasion(21, success=false) = %d, want 10", damage)
	}
}

func TestUncannyDodgeFeature(t *testing.T) {
	fd := UncannyDodgeFeature()

	if fd.Name != "Uncanny Dodge" {
		t.Errorf("Name = %q, want %q", fd.Name, "Uncanny Dodge")
	}
	if len(fd.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(fd.Effects))
	}
	e := fd.Effects[0]
	if e.Type != EffectReactionTrigger {
		t.Errorf("Type = %q, want %q", e.Type, EffectReactionTrigger)
	}
	if e.Trigger != TriggerOnTakeDamage {
		t.Errorf("Trigger = %q, want %q", e.Trigger, TriggerOnTakeDamage)
	}
	if e.On != "uncanny_dodge" {
		t.Errorf("On = %q, want %q", e.On, "uncanny_dodge")
	}
}

func TestApplyUncannyDodge(t *testing.T) {
	tests := []struct {
		name   string
		damage int
		want   int
	}{
		{"even damage 20 -> 10", 20, 10},
		{"odd damage 21 -> 10", 21, 10},
		{"small damage 1 -> 0", 1, 0},
		{"zero damage", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyUncannyDodge(tt.damage)
			if got != tt.want {
				t.Errorf("ApplyUncannyDodge(%d) = %d, want %d", tt.damage, got, tt.want)
			}
		})
	}
}

func TestArcheryFeature(t *testing.T) {
	fd := ArcheryFeature()
	if fd.Name != "Archery" {
		t.Errorf("Name = %q", fd.Name)
	}
	if len(fd.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(fd.Effects))
	}
	e := fd.Effects[0]
	if e.Type != EffectModifyAttackRoll {
		t.Errorf("Type = %q", e.Type)
	}
	if e.Modifier != 2 {
		t.Errorf("Modifier = %d, want 2", e.Modifier)
	}
	if e.Conditions.AttackType != "ranged" {
		t.Errorf("AttackType = %q, want ranged", e.Conditions.AttackType)
	}
}

func TestArcheryOnlyAppliesToRanged(t *testing.T) {
	features := []FeatureDefinition{ArcheryFeature()}
	// Ranged attack: +2
	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{AttackType: "ranged"})
	if result.FlatModifier != 2 {
		t.Errorf("ranged: FlatModifier = %d, want 2", result.FlatModifier)
	}
	// Melee attack: no bonus
	result = ProcessEffects(features, TriggerOnAttackRoll, EffectContext{AttackType: "melee"})
	if result.FlatModifier != 0 {
		t.Errorf("melee: FlatModifier = %d, want 0", result.FlatModifier)
	}
}

func TestDefenseFeature(t *testing.T) {
	fd := DefenseFeature()
	if fd.Name != "Defense" {
		t.Errorf("Name = %q", fd.Name)
	}
	e := fd.Effects[0]
	if e.Type != EffectModifyAC {
		t.Errorf("Type = %q", e.Type)
	}
	if e.Modifier != 1 {
		t.Errorf("Modifier = %d, want 1", e.Modifier)
	}
	if !e.Conditions.WearingArmor {
		t.Error("expected WearingArmor condition")
	}
}

func TestDefenseOnlyWithArmor(t *testing.T) {
	features := []FeatureDefinition{DefenseFeature()}
	// Wearing armor
	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{WearingArmor: true})
	if result.ACModifier != 1 {
		t.Errorf("with armor: ACModifier = %d, want 1", result.ACModifier)
	}
	// No armor
	result = ProcessEffects(features, TriggerOnAttackRoll, EffectContext{WearingArmor: false})
	if result.ACModifier != 0 {
		t.Errorf("no armor: ACModifier = %d, want 0", result.ACModifier)
	}
}

func TestDuelingFeature(t *testing.T) {
	fd := DuelingFeature()
	if fd.Name != "Dueling" {
		t.Errorf("Name = %q", fd.Name)
	}
	e := fd.Effects[0]
	if e.Type != EffectModifyDamageRoll {
		t.Errorf("Type = %q", e.Type)
	}
	if e.Modifier != 2 {
		t.Errorf("Modifier = %d, want 2", e.Modifier)
	}
	if !e.Conditions.OneHandedMeleeOnly {
		t.Error("expected OneHandedMeleeOnly condition")
	}
}

func TestDuelingOnlyOneHandedMelee(t *testing.T) {
	features := []FeatureDefinition{DuelingFeature()}
	// One-handed melee
	result := ProcessEffects(features, TriggerOnDamageRoll, EffectContext{
		AttackType:         "melee",
		OneHandedMeleeOnly: true,
	})
	if result.FlatModifier != 2 {
		t.Errorf("one-handed melee: FlatModifier = %d, want 2", result.FlatModifier)
	}
	// Two-handed
	result = ProcessEffects(features, TriggerOnDamageRoll, EffectContext{
		AttackType:         "melee",
		OneHandedMeleeOnly: false,
	})
	if result.FlatModifier != 0 {
		t.Errorf("two-handed: FlatModifier = %d, want 0", result.FlatModifier)
	}
}

func TestGreatWeaponFightingFeature(t *testing.T) {
	fd := GreatWeaponFightingFeature()
	if fd.Name != "Great Weapon Fighting" {
		t.Errorf("Name = %q", fd.Name)
	}
	e := fd.Effects[0]
	if e.Type != EffectReplaceRoll {
		t.Errorf("Type = %q", e.Type)
	}
	if e.On != "great_weapon_fighting" {
		t.Errorf("On = %q, want great_weapon_fighting", e.On)
	}
}

func TestApplyGreatWeaponFighting(t *testing.T) {
	tests := []struct {
		name    string
		rolls   []int
		dieSide int
		want    []int
	}{
		{"reroll 1s and 2s", []int{1, 2, 5, 6}, 6, []int{4, 4, 5, 6}},
		{"no rerolls needed", []int{3, 4, 5}, 8, []int{3, 4, 5}},
		{"all 1s rerolled", []int{1, 1, 1}, 6, []int{4, 4, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a deterministic reroll value: always (dieSide/2)+1 for simplicity
			rerollIdx := 0
			rerollFn := func(sides int) int {
				rerollIdx++
				return (sides / 2) + 1 // always rerolls to middle value
			}
			got := ApplyGreatWeaponFighting(tt.rolls, tt.dieSide, rerollFn)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPackTacticsFeature(t *testing.T) {
	fd := PackTacticsFeature()
	if fd.Name != "Pack Tactics" {
		t.Errorf("Name = %q", fd.Name)
	}
	e := fd.Effects[0]
	if e.Type != EffectConditionalAdvantage {
		t.Errorf("Type = %q", e.Type)
	}
	if e.Conditions.AllyWithin != 5 {
		t.Errorf("AllyWithin = %d, want 5", e.Conditions.AllyWithin)
	}
}

func TestPackTacticsGrantsAdvantage(t *testing.T) {
	features := []FeatureDefinition{PackTacticsFeature()}
	// Ally within 5ft
	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{AllyWithinFt: 5})
	if result.RollMode != dice.Advantage {
		t.Errorf("with ally: RollMode = %v, want Advantage", result.RollMode)
	}
	// No ally nearby
	result = ProcessEffects(features, TriggerOnAttackRoll, EffectContext{AllyWithinFt: 10})
	if result.RollMode != dice.Normal {
		t.Errorf("no ally: RollMode = %v, want Normal", result.RollMode)
	}
}

func TestBuildFeatureDefinitions_Rogue5(t *testing.T) {
	classes := []CharacterClass{{Class: "Rogue", Level: 5}}
	features := []CharacterFeature{
		{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"},
		{Name: "Uncanny Dodge", MechanicalEffect: "uncanny_dodge"},
	}

	defs := BuildFeatureDefinitions(classes, features)

	found := map[string]bool{}
	for _, d := range defs {
		found[d.Name] = true
	}
	if !found["Sneak Attack"] {
		t.Error("expected Sneak Attack feature")
	}
	if !found["Uncanny Dodge"] {
		t.Error("expected Uncanny Dodge feature")
	}
}

func TestBuildFeatureDefinitions_Rogue7(t *testing.T) {
	classes := []CharacterClass{{Class: "Rogue", Level: 7}}
	features := []CharacterFeature{
		{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"},
		{Name: "Evasion", MechanicalEffect: "evasion"},
		{Name: "Uncanny Dodge", MechanicalEffect: "uncanny_dodge"},
	}

	defs := BuildFeatureDefinitions(classes, features)

	found := map[string]bool{}
	for _, d := range defs {
		found[d.Name] = true
	}
	if !found["Evasion"] {
		t.Error("expected Evasion feature")
	}
}

func TestBuildFeatureDefinitions_FightingStyles(t *testing.T) {
	classes := []CharacterClass{{Class: "Fighter", Level: 1}}
	features := []CharacterFeature{
		{Name: "Fighting Style: Archery", MechanicalEffect: "archery"},
		{Name: "Fighting Style: Defense", MechanicalEffect: "defense"},
		{Name: "Fighting Style: Dueling", MechanicalEffect: "dueling"},
		{Name: "Fighting Style: Great Weapon Fighting", MechanicalEffect: "great_weapon_fighting"},
	}

	defs := BuildFeatureDefinitions(classes, features)

	found := map[string]bool{}
	for _, d := range defs {
		found[d.Name] = true
	}
	if !found["Archery"] {
		t.Error("expected Archery feature")
	}
	if !found["Defense"] {
		t.Error("expected Defense feature")
	}
	if !found["Dueling"] {
		t.Error("expected Dueling feature")
	}
	if !found["Great Weapon Fighting"] {
		t.Error("expected Great Weapon Fighting feature")
	}
}

func TestBuildFeatureDefinitions_PackTactics(t *testing.T) {
	features := []CharacterFeature{
		{Name: "Pack Tactics", MechanicalEffect: "pack_tactics"},
	}

	defs := BuildFeatureDefinitions(nil, features)
	if len(defs) != 1 || defs[0].Name != "Pack Tactics" {
		t.Errorf("expected Pack Tactics, got %v", defs)
	}
}

func TestBuildFeatureDefinitions_Empty(t *testing.T) {
	defs := BuildFeatureDefinitions(nil, nil)
	if len(defs) != 0 {
		t.Errorf("expected empty, got %d definitions", len(defs))
	}
}

func TestBuildAttackEffectContext(t *testing.T) {
	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "rapier",
			Name:       "Rapier",
			Properties: []string{"finesse"},
			WeaponType: "martial_melee",
		},
		HasAdvantage:       true,
		AllyWithinFt:      5,
		WearingArmor:       true,
		OneHandedMeleeOnly: true,
	})

	if ctx.AttackType != "melee" {
		t.Errorf("AttackType = %q, want melee", ctx.AttackType)
	}
	if !ctx.HasAdvantage {
		t.Error("expected HasAdvantage = true")
	}
	if ctx.AllyWithinFt != 5 {
		t.Errorf("AllyWithinFt = %d, want 5", ctx.AllyWithinFt)
	}
	if !ctx.WearingArmor {
		t.Error("expected WearingArmor = true")
	}
	if !ctx.OneHandedMeleeOnly {
		t.Error("expected OneHandedMeleeOnly = true")
	}
	// Check weapon properties are available
	if ctx.WeaponProperty != "finesse" {
		t.Errorf("WeaponProperty = %q, want finesse", ctx.WeaponProperty)
	}
}

func TestBuildAttackEffectContext_Ranged(t *testing.T) {
	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "longbow",
			Name:       "Longbow",
			Properties: []string{"ammunition", "heavy", "two-handed"},
			WeaponType: "martial_ranged",
		},
	})

	if ctx.AttackType != "ranged" {
		t.Errorf("AttackType = %q, want ranged", ctx.AttackType)
	}
}

func TestIntegration_SneakAttackWithRapier(t *testing.T) {
	// Build a level 5 rogue with a rapier (finesse), advantage
	classes := []CharacterClass{{Class: "Rogue", Level: 5}}
	charFeatures := []CharacterFeature{
		{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"},
	}
	defs := BuildFeatureDefinitions(classes, charFeatures)

	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "rapier",
			Name:       "Rapier",
			Properties: []string{"finesse"},
			WeaponType: "martial_melee",
		},
		HasAdvantage: true,
	})

	result := ProcessEffects(defs, TriggerOnDamageRoll, ctx)
	if len(result.ExtraDice) != 1 || result.ExtraDice[0] != "3d6" {
		t.Errorf("expected [3d6] sneak attack dice, got %v", result.ExtraDice)
	}
}

func TestIntegration_ArcheryWithLongbow(t *testing.T) {
	classes := []CharacterClass{{Class: "Fighter", Level: 1}}
	charFeatures := []CharacterFeature{
		{Name: "Fighting Style: Archery", MechanicalEffect: "archery"},
	}
	defs := BuildFeatureDefinitions(classes, charFeatures)

	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "longbow",
			Name:       "Longbow",
			Properties: []string{"ammunition", "heavy", "two-handed"},
			WeaponType: "martial_ranged",
		},
	})

	result := ProcessEffects(defs, TriggerOnAttackRoll, ctx)
	if result.FlatModifier != 2 {
		t.Errorf("expected +2 from Archery, got %d", result.FlatModifier)
	}
}

func TestIntegration_DefenseWithArmor(t *testing.T) {
	charFeatures := []CharacterFeature{
		{Name: "Fighting Style: Defense", MechanicalEffect: "defense"},
	}
	defs := BuildFeatureDefinitions(nil, charFeatures)

	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Properties: []string{"versatile"},
			WeaponType: "martial_melee",
		},
		WearingArmor: true,
	})

	result := ProcessEffects(defs, TriggerOnAttackRoll, ctx)
	if result.ACModifier != 1 {
		t.Errorf("expected +1 AC from Defense, got %d", result.ACModifier)
	}
}

func TestIntegration_DuelingWithOneHanded(t *testing.T) {
	charFeatures := []CharacterFeature{
		{Name: "Fighting Style: Dueling", MechanicalEffect: "dueling"},
	}
	defs := BuildFeatureDefinitions(nil, charFeatures)

	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "longsword",
			Name:       "Longsword",
			Properties: []string{"versatile"},
			WeaponType: "martial_melee",
		},
		OneHandedMeleeOnly: true,
	})

	result := ProcessEffects(defs, TriggerOnDamageRoll, ctx)
	if result.FlatModifier != 2 {
		t.Errorf("expected +2 from Dueling, got %d", result.FlatModifier)
	}
}

func TestIntegration_UncannyDodgeReaction(t *testing.T) {
	charFeatures := []CharacterFeature{
		{Name: "Uncanny Dodge", MechanicalEffect: "uncanny_dodge"},
	}
	defs := BuildFeatureDefinitions(nil, charFeatures)

	result := ProcessEffects(defs, TriggerOnTakeDamage, EffectContext{})
	if len(result.ReactionTriggers) != 1 || result.ReactionTriggers[0] != "Uncanny Dodge" {
		t.Errorf("expected Uncanny Dodge reaction trigger, got %v", result.ReactionTriggers)
	}
}

func TestIntegration_EvasionOnDexSave(t *testing.T) {
	charFeatures := []CharacterFeature{
		{Name: "Evasion", MechanicalEffect: "evasion"},
	}
	defs := BuildFeatureDefinitions(nil, charFeatures)

	result := ProcessEffects(defs, TriggerOnSave, EffectContext{AbilityUsed: "dex"})
	if len(result.AppliedEffects) != 1 {
		t.Fatalf("expected 1 applied effect, got %d", len(result.AppliedEffects))
	}
	if result.AppliedEffects[0].Effect.On != "evasion" {
		t.Errorf("expected evasion marker, got %q", result.AppliedEffects[0].Effect.On)
	}
}

func TestIntegration_EvasionIgnoredOnNonDexSave(t *testing.T) {
	charFeatures := []CharacterFeature{
		{Name: "Evasion", MechanicalEffect: "evasion"},
	}
	defs := BuildFeatureDefinitions(nil, charFeatures)

	result := ProcessEffects(defs, TriggerOnSave, EffectContext{AbilityUsed: "wis"})
	if len(result.AppliedEffects) != 0 {
		t.Errorf("expected no applied effects for WIS save, got %d", len(result.AppliedEffects))
	}
}

func TestIntegration_PackTacticsWithAlly(t *testing.T) {
	charFeatures := []CharacterFeature{
		{Name: "Pack Tactics", MechanicalEffect: "pack_tactics"},
	}
	defs := BuildFeatureDefinitions(nil, charFeatures)

	ctx := BuildAttackEffectContext(AttackEffectInput{
		Weapon: refdata.Weapon{
			ID:         "bite",
			Name:       "Bite",
			WeaponType: "simple_melee",
		},
		AllyWithinFt: 5,
	})

	result := ProcessEffects(defs, TriggerOnAttackRoll, ctx)
	if result.RollMode != dice.Advantage {
		t.Errorf("expected Advantage from Pack Tactics, got %v", result.RollMode)
	}
}

func TestIntegration_MultipleFeaturesCombined(t *testing.T) {
	// Rogue 5 with Archery style and Sneak Attack, using shortbow
	classes := []CharacterClass{{Class: "Rogue", Level: 5}}
	charFeatures := []CharacterFeature{
		{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"},
		{Name: "Fighting Style: Archery", MechanicalEffect: "archery"},
	}
	defs := BuildFeatureDefinitions(classes, charFeatures)

	weapon := refdata.Weapon{
		ID:         "shortbow",
		Name:       "Shortbow",
		Properties: []string{"ammunition", "two-handed"},
		WeaponType: "simple_ranged",
	}

	// Attack roll: Archery +2
	atkCtx := BuildAttackEffectContext(AttackEffectInput{
		Weapon:       weapon,
		HasAdvantage: true,
		AllyWithinFt: 5,
	})
	atkResult := ProcessEffects(defs, TriggerOnAttackRoll, atkCtx)
	if atkResult.FlatModifier != 2 {
		t.Errorf("attack roll: expected +2 from Archery, got %d", atkResult.FlatModifier)
	}

	// Damage roll: Sneak Attack should NOT apply (shortbow is ranged but not finesse)
	// Wait — shortbow is a ranged weapon, and Sneak Attack works with finesse OR ranged weapons.
	// The WeaponProperties condition checks for "finesse" or "ranged" in weapon properties.
	// But shortbow's properties are ["ammunition", "two-handed"]. WeaponType is "simple_ranged".
	// We need to check: the weapon IS ranged, but "ranged" is not in the properties list.
	// The effect ctx WeaponProperty is "ranged" for ranged weapons. Let's check.
	dmgCtx := BuildAttackEffectContext(AttackEffectInput{
		Weapon:       weapon,
		HasAdvantage: true,
		AllyWithinFt: 5,
	})
	dmgResult := ProcessEffects(defs, TriggerOnDamageRoll, dmgCtx)
	// WeaponProperty is "ranged" for ranged weapons, which matches WeaponProperties: ["finesse", "ranged"]
	if len(dmgResult.ExtraDice) != 1 || dmgResult.ExtraDice[0] != "3d6" {
		t.Errorf("damage roll: expected [3d6] sneak attack, got %v", dmgResult.ExtraDice)
	}
}

func TestWeaponPropertiesORCondition(t *testing.T) {
	// Test the OR matching in WeaponProperties
	effect := Effect{
		Type:    EffectExtraDamageDice,
		Trigger: TriggerOnDamageRoll,
		Dice:    "1d6",
		Conditions: EffectConditions{
			WeaponProperties: []string{"finesse", "ranged"},
		},
	}

	t.Run("finesse matches", func(t *testing.T) {
		ctx := EffectContext{WeaponProperty: "finesse"}
		if !EvaluateConditions(effect, ctx) {
			t.Error("finesse should match")
		}
	})
	t.Run("ranged matches via WeaponProperty", func(t *testing.T) {
		ctx := EffectContext{WeaponProperty: "ranged"}
		if !EvaluateConditions(effect, ctx) {
			t.Error("ranged should match")
		}
	})
	t.Run("ranged matches via WeaponProperties list", func(t *testing.T) {
		ctx := EffectContext{WeaponProperties: []string{"ammunition", "finesse"}}
		if !EvaluateConditions(effect, ctx) {
			t.Error("finesse in properties list should match")
		}
	})
	t.Run("heavy does not match", func(t *testing.T) {
		ctx := EffectContext{WeaponProperty: "heavy"}
		if EvaluateConditions(effect, ctx) {
			t.Error("heavy should not match")
		}
	})
	t.Run("no properties does not match", func(t *testing.T) {
		ctx := EffectContext{}
		if EvaluateConditions(effect, ctx) {
			t.Error("empty should not match")
		}
	})
}

func TestAdvantageOrAllyWithinCondition(t *testing.T) {
	effect := Effect{
		Type:    EffectExtraDamageDice,
		Trigger: TriggerOnDamageRoll,
		Dice:    "1d6",
		Conditions: EffectConditions{
			AdvantageOrAllyWithin: 5,
		},
	}

	t.Run("advantage satisfies", func(t *testing.T) {
		ctx := EffectContext{HasAdvantage: true, AllyWithinFt: 999}
		if !EvaluateConditions(effect, ctx) {
			t.Error("advantage should satisfy")
		}
	})
	t.Run("ally within 5ft satisfies", func(t *testing.T) {
		ctx := EffectContext{HasAdvantage: false, AllyWithinFt: 5}
		if !EvaluateConditions(effect, ctx) {
			t.Error("ally within 5ft should satisfy")
		}
	})
	t.Run("neither satisfies", func(t *testing.T) {
		ctx := EffectContext{HasAdvantage: false, AllyWithinFt: 10}
		if EvaluateConditions(effect, ctx) {
			t.Error("neither advantage nor ally should fail")
		}
	})
}

func TestWearingArmorCondition(t *testing.T) {
	effect := Effect{
		Type:     EffectModifyAC,
		Trigger:  TriggerOnAttackRoll,
		Modifier: 1,
		Conditions: EffectConditions{
			WearingArmor: true,
		},
	}

	t.Run("wearing armor", func(t *testing.T) {
		if !EvaluateConditions(effect, EffectContext{WearingArmor: true}) {
			t.Error("should pass with armor")
		}
	})
	t.Run("not wearing armor", func(t *testing.T) {
		if EvaluateConditions(effect, EffectContext{WearingArmor: false}) {
			t.Error("should fail without armor")
		}
	})
}

func TestOneHandedMeleeOnlyCondition(t *testing.T) {
	effect := Effect{
		Type:     EffectModifyDamageRoll,
		Trigger:  TriggerOnDamageRoll,
		Modifier: 2,
		Conditions: EffectConditions{
			OneHandedMeleeOnly: true,
		},
	}

	t.Run("one-handed melee", func(t *testing.T) {
		if !EvaluateConditions(effect, EffectContext{OneHandedMeleeOnly: true}) {
			t.Error("should pass")
		}
	})
	t.Run("two-handed", func(t *testing.T) {
		if EvaluateConditions(effect, EffectContext{OneHandedMeleeOnly: false}) {
			t.Error("should fail")
		}
	})
}

func TestSneakAttackOncePerTurn(t *testing.T) {
	features := []FeatureDefinition{SneakAttackFeature(3)}
	ctx := EffectContext{
		WeaponProperty: "finesse",
		HasAdvantage:   true,
		UsedThisTurn:   map[string]bool{"extra_damage_dice": true},
	}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
	if len(result.ExtraDice) != 0 {
		t.Errorf("expected no extra dice (already used this turn), got %v", result.ExtraDice)
	}
}

func TestClassLevel(t *testing.T) {
	tests := []struct {
		name      string
		classes   []CharacterClass
		className string
		want      int
	}{
		{"rogue found", []CharacterClass{{Class: "Rogue", Level: 5}}, "Rogue", 5},
		{"fighter found", []CharacterClass{{Class: "Fighter", Level: 10}}, "Fighter", 10},
		{"not found", []CharacterClass{{Class: "Rogue", Level: 5}}, "Wizard", 0},
		{"case insensitive", []CharacterClass{{Class: "rogue", Level: 3}}, "Rogue", 3},
		{"nil classes", nil, "Rogue", 0},
		{"multiclass", []CharacterClass{
			{Class: "Rogue", Level: 5},
			{Class: "Fighter", Level: 3},
		}, "Rogue", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classLevel(tt.classes, tt.className)
			if got != tt.want {
				t.Errorf("classLevel() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGreatWeaponFightingConditions(t *testing.T) {
	features := []FeatureDefinition{GreatWeaponFightingFeature()}

	t.Run("heavy melee weapon matches", func(t *testing.T) {
		ctx := EffectContext{
			AttackType:       "melee",
			WeaponProperties: []string{"heavy", "two-handed"},
		}
		result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
		if len(result.AppliedEffects) != 1 {
			t.Errorf("expected 1 applied effect, got %d", len(result.AppliedEffects))
		}
	})

	t.Run("versatile melee weapon matches", func(t *testing.T) {
		ctx := EffectContext{
			AttackType:       "melee",
			WeaponProperties: []string{"versatile"},
		}
		result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
		if len(result.AppliedEffects) != 1 {
			t.Errorf("expected 1 applied effect, got %d", len(result.AppliedEffects))
		}
	})

	t.Run("ranged does not match", func(t *testing.T) {
		ctx := EffectContext{
			AttackType:       "ranged",
			WeaponProperties: []string{"heavy", "two-handed"},
		}
		result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
		if len(result.AppliedEffects) != 0 {
			t.Errorf("expected no applied effects for ranged, got %d", len(result.AppliedEffects))
		}
	})

	t.Run("light melee does not match", func(t *testing.T) {
		ctx := EffectContext{
			AttackType:       "melee",
			WeaponProperties: []string{"light", "finesse"},
		}
		result := ProcessEffects(features, TriggerOnDamageRoll, ctx)
		if len(result.AppliedEffects) != 0 {
			t.Errorf("expected no applied effects for light weapon, got %d", len(result.AppliedEffects))
		}
	})
}

func TestBuildFeatureDefinitions_SneakAttackNoRogueClass(t *testing.T) {
	// Character has sneak_attack feature but no rogue class (e.g., from multiclass or item)
	features := []CharacterFeature{
		{Name: "Sneak Attack", MechanicalEffect: "sneak_attack"},
	}
	defs := BuildFeatureDefinitions(nil, features)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	// Should default to level 1 sneak attack (1d6)
	if defs[0].Effects[0].Dice != "1d6" {
		t.Errorf("expected 1d6 (default level 1), got %q", defs[0].Effects[0].Dice)
	}
}

func TestBuildFeatureDefinitions_UnknownFeature(t *testing.T) {
	features := []CharacterFeature{
		{Name: "Some Unknown Feature", MechanicalEffect: "unknown_thing"},
	}
	defs := BuildFeatureDefinitions(nil, features)
	if len(defs) != 0 {
		t.Errorf("expected 0 defs for unknown feature, got %d", len(defs))
	}
}

func TestSneakAttackDice(t *testing.T) {
	tests := []struct {
		rogueLevel int
		want       string
	}{
		{1, "1d6"},
		{2, "1d6"},
		{3, "2d6"},
		{4, "2d6"},
		{5, "3d6"},
		{6, "3d6"},
		{7, "4d6"},
		{8, "4d6"},
		{9, "5d6"},
		{10, "5d6"},
		{11, "6d6"},
		{19, "10d6"},
		{20, "10d6"},
	}

	for _, tt := range tests {
		got := SneakAttackDice(tt.rogueLevel)
		if got != tt.want {
			t.Errorf("SneakAttackDice(%d) = %q, want %q", tt.rogueLevel, got, tt.want)
		}
	}
}
