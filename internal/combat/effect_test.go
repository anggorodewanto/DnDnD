package combat

import (
	"encoding/json"
	"testing"

	"github.com/ab/dndnd/internal/dice"
)

func TestEffectType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		et       EffectType
		wantValid bool
	}{
		{"modify_attack_roll", EffectModifyAttackRoll, true},
		{"modify_damage_roll", EffectModifyDamageRoll, true},
		{"extra_damage_dice", EffectExtraDamageDice, true},
		{"modify_ac", EffectModifyAC, true},
		{"modify_save", EffectModifySave, true},
		{"modify_check", EffectModifyCheck, true},
		{"modify_speed", EffectModifySpeed, true},
		{"grant_resistance", EffectGrantResistance, true},
		{"grant_immunity", EffectGrantImmunity, true},
		{"extra_attack", EffectExtraAttack, true},
		{"modify_hp", EffectModifyHP, true},
		{"conditional_advantage", EffectConditionalAdvantage, true},
		{"resource_on_hit", EffectResourceOnHit, true},
		{"reaction_trigger", EffectReactionTrigger, true},
		{"aura", EffectAura, true},
		{"replace_roll", EffectReplaceRoll, true},
		{"grant_proficiency", EffectGrantProficiency, true},
		{"modify_range", EffectModifyRange, true},
		{"dm_resolution", EffectDMResolution, true},
		{"invalid_effect", EffectType("bogus"), false},
		{"empty", EffectType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.et.IsValid()
			if got != tt.wantValid {
				t.Errorf("EffectType(%q).IsValid() = %v, want %v", tt.et, got, tt.wantValid)
			}
		})
	}
}

func TestTriggerPoint_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		tp        TriggerPoint
		wantValid bool
	}{
		{"on_attack_roll", TriggerOnAttackRoll, true},
		{"on_damage_roll", TriggerOnDamageRoll, true},
		{"on_take_damage", TriggerOnTakeDamage, true},
		{"on_save", TriggerOnSave, true},
		{"on_check", TriggerOnCheck, true},
		{"on_turn_start", TriggerOnTurnStart, true},
		{"on_turn_end", TriggerOnTurnEnd, true},
		{"on_rest", TriggerOnRest, true},
		{"bogus", TriggerPoint("bogus"), false},
		{"empty", TriggerPoint(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tp.IsValid()
			if got != tt.wantValid {
				t.Errorf("TriggerPoint(%q).IsValid() = %v, want %v", tt.tp, got, tt.wantValid)
			}
		})
	}
}

func TestEffect_MatchesTrigger(t *testing.T) {
	e := Effect{
		Type:    EffectModifyAttackRoll,
		Trigger: TriggerOnAttackRoll,
	}

	if !e.MatchesTrigger(TriggerOnAttackRoll) {
		t.Error("expected effect to match on_attack_roll trigger")
	}
	if e.MatchesTrigger(TriggerOnDamageRoll) {
		t.Error("expected effect not to match on_damage_roll trigger")
	}
}

func TestEvaluateConditions(t *testing.T) {
	tests := []struct {
		name string
		effect Effect
		ctx    EffectContext
		want   bool
	}{
		{
			name:   "no conditions always passes",
			effect: Effect{Type: EffectModifyAttackRoll},
			ctx:    EffectContext{},
			want:   true,
		},
		{
			name:   "when_raging passes if raging",
			effect: Effect{Type: EffectModifyDamageRoll, Conditions: EffectConditions{WhenRaging: true}},
			ctx:    EffectContext{IsRaging: true},
			want:   true,
		},
		{
			name:   "when_raging fails if not raging",
			effect: Effect{Type: EffectModifyDamageRoll, Conditions: EffectConditions{WhenRaging: true}},
			ctx:    EffectContext{IsRaging: false},
			want:   false,
		},
		{
			name:   "when_concentrating passes",
			effect: Effect{Type: EffectModifyAC, Conditions: EffectConditions{WhenConcentrating: true}},
			ctx:    EffectContext{IsConcentrating: true},
			want:   true,
		},
		{
			name:   "when_concentrating fails",
			effect: Effect{Type: EffectModifyAC, Conditions: EffectConditions{WhenConcentrating: true}},
			ctx:    EffectContext{IsConcentrating: false},
			want:   false,
		},
		{
			name:   "attack_type melee matches",
			effect: Effect{Type: EffectModifyDamageRoll, Conditions: EffectConditions{AttackType: "melee"}},
			ctx:    EffectContext{AttackType: "melee"},
			want:   true,
		},
		{
			name:   "attack_type melee mismatch",
			effect: Effect{Type: EffectModifyDamageRoll, Conditions: EffectConditions{AttackType: "melee"}},
			ctx:    EffectContext{AttackType: "ranged"},
			want:   false,
		},
		{
			name:   "ability_used str matches",
			effect: Effect{Type: EffectModifyDamageRoll, Conditions: EffectConditions{AbilityUsed: "str"}},
			ctx:    EffectContext{AbilityUsed: "str"},
			want:   true,
		},
		{
			name:   "ability_used str mismatch",
			effect: Effect{Type: EffectModifyDamageRoll, Conditions: EffectConditions{AbilityUsed: "str"}},
			ctx:    EffectContext{AbilityUsed: "dex"},
			want:   false,
		},
		{
			name:   "weapon_property finesse matches",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{WeaponProperty: "finesse"}},
			ctx:    EffectContext{WeaponProperty: "finesse"},
			want:   true,
		},
		{
			name:   "weapon_property mismatch",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{WeaponProperty: "finesse"}},
			ctx:    EffectContext{WeaponProperty: "heavy"},
			want:   false,
		},
		{
			name:   "target_condition frightened matches",
			effect: Effect{Type: EffectModifyAttackRoll, Conditions: EffectConditions{TargetCondition: "frightened"}},
			ctx:    EffectContext{TargetCondition: "frightened"},
			want:   true,
		},
		{
			name:   "target_condition mismatch",
			effect: Effect{Type: EffectModifyAttackRoll, Conditions: EffectConditions{TargetCondition: "frightened"}},
			ctx:    EffectContext{TargetCondition: ""},
			want:   false,
		},
		{
			name:   "ally_within 5 passes when ally at 5ft",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{AllyWithin: 5}},
			ctx:    EffectContext{AllyWithinFt: 5},
			want:   true,
		},
		{
			name:   "ally_within 5 fails when no ally nearby",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{AllyWithin: 5}},
			ctx:    EffectContext{AllyWithinFt: 10},
			want:   false,
		},
		{
			name:   "has_advantage passes",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{HasAdvantage: true}},
			ctx:    EffectContext{HasAdvantage: true},
			want:   true,
		},
		{
			name:   "has_advantage fails",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{HasAdvantage: true}},
			ctx:    EffectContext{HasAdvantage: false},
			want:   false,
		},
		{
			name:   "once_per_turn passes first time",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{OncePerTurn: true}},
			ctx:    EffectContext{UsedThisTurn: map[string]bool{}},
			want:   true,
		},
		{
			name:   "once_per_turn fails if already used",
			effect: Effect{Type: EffectExtraDamageDice, Conditions: EffectConditions{OncePerTurn: true}},
			ctx:    EffectContext{UsedThisTurn: map[string]bool{"extra_damage_dice": true}},
			want:   false,
		},
		{
			name:   "uses_remaining passes",
			effect: Effect{Type: EffectReplaceRoll, Conditions: EffectConditions{UsesRemaining: true}},
			ctx:    EffectContext{UsesRemaining: map[string]int{"replace_roll": 2}},
			want:   true,
		},
		{
			name:   "uses_remaining fails with 0",
			effect: Effect{Type: EffectReplaceRoll, Conditions: EffectConditions{UsesRemaining: true}},
			ctx:    EffectContext{UsesRemaining: map[string]int{"replace_roll": 0}},
			want:   false,
		},
		{
			name:   "uses_remaining fails with nil map",
			effect: Effect{Type: EffectReplaceRoll, Conditions: EffectConditions{UsesRemaining: true}},
			ctx:    EffectContext{},
			want:   false,
		},
		{
			name: "multiple conditions all must pass",
			effect: Effect{
				Type: EffectModifyDamageRoll,
				Conditions: EffectConditions{
					WhenRaging:  true,
					AttackType:  "melee",
					AbilityUsed: "str",
				},
			},
			ctx:  EffectContext{IsRaging: true, AttackType: "melee", AbilityUsed: "str"},
			want: true,
		},
		{
			name: "multiple conditions one fails",
			effect: Effect{
				Type: EffectModifyDamageRoll,
				Conditions: EffectConditions{
					WhenRaging:  true,
					AttackType:  "melee",
					AbilityUsed: "str",
				},
			},
			ctx:  EffectContext{IsRaging: true, AttackType: "ranged", AbilityUsed: "str"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateConditions(tt.effect, tt.ctx)
			if got != tt.want {
				t.Errorf("EvaluateConditions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEffectPriority(t *testing.T) {
	tests := []struct {
		name     string
		et       EffectType
		wantPri  ResolutionPriority
	}{
		{"immunity is priority 1", EffectGrantImmunity, PriorityImmunity},
		{"resistance is priority 2", EffectGrantResistance, PriorityResistance},
		{"modify_attack_roll is flat mod", EffectModifyAttackRoll, PriorityFlatModifier},
		{"modify_damage_roll is flat mod", EffectModifyDamageRoll, PriorityFlatModifier},
		{"modify_ac is flat mod", EffectModifyAC, PriorityFlatModifier},
		{"modify_save is flat mod", EffectModifySave, PriorityFlatModifier},
		{"modify_check is flat mod", EffectModifyCheck, PriorityFlatModifier},
		{"modify_speed is flat mod", EffectModifySpeed, PriorityFlatModifier},
		{"extra_damage_dice is dice mod", EffectExtraDamageDice, PriorityDiceModifier},
		{"conditional_advantage is priority 5", EffectConditionalAdvantage, PriorityAdvantage},
		{"dm_resolution is flat mod", EffectDMResolution, PriorityFlatModifier},
		{"replace_roll is flat mod", EffectReplaceRoll, PriorityFlatModifier},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectPriority(tt.et)
			if got != tt.wantPri {
				t.Errorf("EffectPriority(%q) = %d, want %d", tt.et, got, tt.wantPri)
			}
		})
	}
}

func TestSortByPriority(t *testing.T) {
	effects := []ResolvedEffect{
		{FeatureName: "Reckless Attack", Effect: Effect{Type: EffectConditionalAdvantage}, Priority: PriorityAdvantage},
		{FeatureName: "Sneak Attack", Effect: Effect{Type: EffectExtraDamageDice}, Priority: PriorityDiceModifier},
		{FeatureName: "Rage Resistance", Effect: Effect{Type: EffectGrantResistance}, Priority: PriorityResistance},
		{FeatureName: "Archery", Effect: Effect{Type: EffectModifyAttackRoll}, Priority: PriorityFlatModifier},
		{FeatureName: "Immunity", Effect: Effect{Type: EffectGrantImmunity}, Priority: PriorityImmunity},
	}

	SortByPriority(effects)

	expectedOrder := []string{"Immunity", "Rage Resistance", "Archery", "Sneak Attack", "Reckless Attack"}
	for i, want := range expectedOrder {
		if effects[i].FeatureName != want {
			t.Errorf("position %d: got %q, want %q", i, effects[i].FeatureName, want)
		}
	}
}

func TestSortByPriority_Empty(t *testing.T) {
	var effects []ResolvedEffect
	SortByPriority(effects) // should not panic
}

func TestSortByPriority_Single(t *testing.T) {
	effects := []ResolvedEffect{
		{FeatureName: "X", Priority: PriorityFlatModifier},
	}
	SortByPriority(effects)
	if effects[0].FeatureName != "X" {
		t.Error("single element sort failed")
	}
}

func TestCollectEffects_FiltersAndMatches(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Rage",
			Effects: []Effect{
				{
					Type:     EffectModifyDamageRoll,
					Trigger:  TriggerOnDamageRoll,
					Modifier: 2,
					Conditions: EffectConditions{
						WhenRaging:  true,
						AttackType:  "melee",
						AbilityUsed: "str",
					},
				},
				{
					Type:        EffectGrantResistance,
					Trigger:     TriggerOnTakeDamage,
					DamageTypes: []string{"bludgeoning", "piercing", "slashing"},
					Conditions:  EffectConditions{WhenRaging: true},
				},
			},
		},
		{
			Name: "Archery",
			Effects: []Effect{
				{
					Type:     EffectModifyAttackRoll,
					Trigger:  TriggerOnAttackRoll,
					Modifier: 2,
				},
			},
		},
	}

	t.Run("on_damage_roll while raging with melee str", func(t *testing.T) {
		ctx := EffectContext{IsRaging: true, AttackType: "melee", AbilityUsed: "str"}
		results := CollectEffects(features, TriggerOnDamageRoll, ctx)
		if len(results) != 1 {
			t.Fatalf("expected 1 effect, got %d", len(results))
		}
		if results[0].FeatureName != "Rage" {
			t.Errorf("expected feature Rage, got %q", results[0].FeatureName)
		}
	})

	t.Run("on_damage_roll not raging", func(t *testing.T) {
		ctx := EffectContext{IsRaging: false, AttackType: "melee", AbilityUsed: "str"}
		results := CollectEffects(features, TriggerOnDamageRoll, ctx)
		if len(results) != 0 {
			t.Fatalf("expected 0 effects, got %d", len(results))
		}
	})

	t.Run("on_attack_roll gets archery", func(t *testing.T) {
		ctx := EffectContext{}
		results := CollectEffects(features, TriggerOnAttackRoll, ctx)
		if len(results) != 1 {
			t.Fatalf("expected 1 effect, got %d", len(results))
		}
		if results[0].FeatureName != "Archery" {
			t.Errorf("expected feature Archery, got %q", results[0].FeatureName)
		}
	})

	t.Run("on_take_damage while raging gets resistance", func(t *testing.T) {
		ctx := EffectContext{IsRaging: true}
		results := CollectEffects(features, TriggerOnTakeDamage, ctx)
		if len(results) != 1 {
			t.Fatalf("expected 1 effect, got %d", len(results))
		}
		if results[0].Effect.Type != EffectGrantResistance {
			t.Errorf("expected grant_resistance, got %q", results[0].Effect.Type)
		}
	})

	t.Run("no matching trigger returns empty", func(t *testing.T) {
		ctx := EffectContext{}
		results := CollectEffects(features, TriggerOnRest, ctx)
		if len(results) != 0 {
			t.Fatalf("expected 0 effects, got %d", len(results))
		}
	})
}

func TestProcessEffects_RageBarbarianOnDamageRoll(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Rage",
			Effects: []Effect{
				{
					Type:     EffectModifyDamageRoll,
					Trigger:  TriggerOnDamageRoll,
					Modifier: 2,
					Conditions: EffectConditions{
						WhenRaging:  true,
						AttackType:  "melee",
						AbilityUsed: "str",
					},
				},
			},
		},
	}

	ctx := EffectContext{IsRaging: true, AttackType: "melee", AbilityUsed: "str"}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)

	if result.FlatModifier != 2 {
		t.Errorf("expected flat modifier 2, got %d", result.FlatModifier)
	}
	if len(result.AppliedEffects) != 1 {
		t.Errorf("expected 1 applied effect, got %d", len(result.AppliedEffects))
	}
}

func TestProcessEffects_ArcheryOnAttackRoll(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Archery",
			Effects: []Effect{
				{Type: EffectModifyAttackRoll, Trigger: TriggerOnAttackRoll, Modifier: 2},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{})
	if result.FlatModifier != 2 {
		t.Errorf("expected flat modifier 2, got %d", result.FlatModifier)
	}
}

func TestProcessEffects_SneakAttackExtraDice(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Sneak Attack",
			Effects: []Effect{
				{
					Type:    EffectExtraDamageDice,
					Trigger: TriggerOnDamageRoll,
					Dice:    "3d6",
					Conditions: EffectConditions{
						WeaponProperty: "finesse",
						HasAdvantage:   true,
					},
				},
			},
		},
	}

	ctx := EffectContext{WeaponProperty: "finesse", HasAdvantage: true}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)

	if len(result.ExtraDice) != 1 || result.ExtraDice[0] != "3d6" {
		t.Errorf("expected extra dice [3d6], got %v", result.ExtraDice)
	}
}

func TestProcessEffects_GrantResistance(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Rage",
			Effects: []Effect{
				{
					Type:        EffectGrantResistance,
					Trigger:     TriggerOnTakeDamage,
					DamageTypes: []string{"bludgeoning", "piercing", "slashing"},
					Conditions:  EffectConditions{WhenRaging: true},
				},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTakeDamage, EffectContext{IsRaging: true})
	if len(result.Resistances) != 3 {
		t.Errorf("expected 3 resistances, got %d", len(result.Resistances))
	}
}

func TestProcessEffects_GrantImmunity(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Aura of Courage",
			Effects: []Effect{
				{
					Type:    EffectGrantImmunity,
					Trigger: TriggerOnTakeDamage,
					On:      "frightened",
				},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTakeDamage, EffectContext{})
	if len(result.Immunities) != 1 || result.Immunities[0] != "frightened" {
		t.Errorf("expected immunity to frightened, got %v", result.Immunities)
	}
}

func TestProcessEffects_ModifyAC(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Shield of Faith",
			Effects: []Effect{
				{Type: EffectModifyAC, Trigger: TriggerOnAttackRoll, Modifier: 2},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{})
	if result.ACModifier != 2 {
		t.Errorf("expected AC modifier 2, got %d", result.ACModifier)
	}
}

func TestProcessEffects_ModifySave(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Aura of Protection",
			Effects: []Effect{
				{Type: EffectModifySave, Trigger: TriggerOnSave, Modifier: 3},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnSave, EffectContext{})
	if result.FlatModifier != 3 {
		t.Errorf("expected flat modifier 3, got %d", result.FlatModifier)
	}
}

func TestProcessEffects_ModifyCheck(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Jack of All Trades",
			Effects: []Effect{
				{Type: EffectModifyCheck, Trigger: TriggerOnCheck, Modifier: 1},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnCheck, EffectContext{})
	if result.FlatModifier != 1 {
		t.Errorf("expected flat modifier 1, got %d", result.FlatModifier)
	}
}

func TestProcessEffects_ModifySpeed(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Fast Movement",
			Effects: []Effect{
				{Type: EffectModifySpeed, Trigger: TriggerOnTurnStart, Modifier: 10},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{})
	if result.SpeedModifier != 10 {
		t.Errorf("expected speed modifier 10, got %d", result.SpeedModifier)
	}
}

func TestProcessEffects_ModifyHP(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Tough",
			Effects: []Effect{
				{Type: EffectModifyHP, Trigger: TriggerOnRest, Modifier: 20},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnRest, EffectContext{})
	if result.HPModifier != 20 {
		t.Errorf("expected HP modifier 20, got %d", result.HPModifier)
	}
}

func TestProcessEffects_ModifyRange(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Spell Sniper",
			Effects: []Effect{
				{Type: EffectModifyRange, Trigger: TriggerOnAttackRoll, Modifier: 60},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{})
	if result.RangeModifier != 60 {
		t.Errorf("expected range modifier 60, got %d", result.RangeModifier)
	}
}

func TestProcessEffects_ExtraAttack(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Extra Attack",
			Effects: []Effect{
				{Type: EffectExtraAttack, Trigger: TriggerOnTurnStart, Modifier: 1},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{})
	if result.ExtraAttacks != 1 {
		t.Errorf("expected 1 extra attack, got %d", result.ExtraAttacks)
	}
}

func TestProcessEffects_ExtraAttack_DefaultToOne(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Extra Attack",
			Effects: []Effect{
				{Type: EffectExtraAttack, Trigger: TriggerOnTurnStart},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{})
	if result.ExtraAttacks != 1 {
		t.Errorf("expected 1 extra attack (default), got %d", result.ExtraAttacks)
	}
}

func TestProcessEffects_ReplaceRoll(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Portent",
			Effects: []Effect{
				{
					Type:         EffectReplaceRoll,
					Trigger:      TriggerOnAttackRoll,
					ReplaceValue: 17,
					Conditions:   EffectConditions{UsesRemaining: true},
				},
			},
		},
	}

	ctx := EffectContext{UsesRemaining: map[string]int{"replace_roll": 1}}
	result := ProcessEffects(features, TriggerOnAttackRoll, ctx)

	if result.ReplacedRoll == nil || *result.ReplacedRoll != 17 {
		t.Errorf("expected replaced roll 17, got %v", result.ReplacedRoll)
	}
}

func TestProcessEffects_GrantProficiency(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Remarkable Athlete",
			Effects: []Effect{
				{Type: EffectGrantProficiency, Trigger: TriggerOnCheck, On: "athletics"},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnCheck, EffectContext{})
	if len(result.Proficiencies) != 1 || result.Proficiencies[0] != "athletics" {
		t.Errorf("expected proficiency in athletics, got %v", result.Proficiencies)
	}
}

func TestProcessEffects_ResourceOnHit(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Divine Smite",
			Effects: []Effect{
				{Type: EffectResourceOnHit, Trigger: TriggerOnDamageRoll},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnDamageRoll, EffectContext{})
	if len(result.ResourceTriggers) != 1 || result.ResourceTriggers[0] != "Divine Smite" {
		t.Errorf("expected resource trigger Divine Smite, got %v", result.ResourceTriggers)
	}
}

func TestProcessEffects_ReactionTrigger(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Shield",
			Effects: []Effect{
				{Type: EffectReactionTrigger, Trigger: TriggerOnTakeDamage},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTakeDamage, EffectContext{})
	if len(result.ReactionTriggers) != 1 || result.ReactionTriggers[0] != "Shield" {
		t.Errorf("expected reaction trigger Shield, got %v", result.ReactionTriggers)
	}
}

func TestProcessEffects_Aura(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Aura of Protection",
			Effects: []Effect{
				{
					Type:    EffectAura,
					Trigger: TriggerOnSave,
					Conditions: EffectConditions{
						AuraRadius: 10,
						Target:     "self_and_allies",
					},
				},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnSave, EffectContext{})
	if len(result.AuraEffects) != 1 {
		t.Errorf("expected 1 aura effect, got %d", len(result.AuraEffects))
	}
}

func TestProcessEffects_DMResolution(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Wild Magic Surge",
			Effects: []Effect{
				{
					Type:        EffectDMResolution,
					Trigger:     TriggerOnTurnStart,
					Description: "Roll on Wild Magic Surge table",
				},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTurnStart, EffectContext{})
	if len(result.DMResolutions) != 1 {
		t.Errorf("expected 1 DM resolution, got %d", len(result.DMResolutions))
	}
	if result.DMResolutions[0].FeatureName != "Wild Magic Surge" {
		t.Errorf("expected Wild Magic Surge, got %q", result.DMResolutions[0].FeatureName)
	}
}

func TestProcessEffects_ConditionalAdvantage(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Reckless Attack",
			Effects: []Effect{
				{
					Type:    EffectConditionalAdvantage,
					Trigger: TriggerOnAttackRoll,
					On:      "advantage",
					Conditions: EffectConditions{
						WhenRaging:  true,
						AbilityUsed: "str",
					},
				},
			},
		},
	}

	ctx := EffectContext{IsRaging: true, AbilityUsed: "str"}
	result := ProcessEffects(features, TriggerOnAttackRoll, ctx)

	if result.RollMode != dice.Advantage {
		t.Errorf("expected advantage, got %v", result.RollMode)
	}
}

func TestProcessEffects_ConditionalDisadvantage(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Sunlight Sensitivity",
			Effects: []Effect{
				{Type: EffectConditionalAdvantage, Trigger: TriggerOnAttackRoll, On: "disadvantage"},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{})
	if result.RollMode != dice.Disadvantage {
		t.Errorf("expected disadvantage, got %v", result.RollMode)
	}
}

func TestProcessEffects_AdvantageAndDisadvantageCancel(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Reckless Attack",
			Effects: []Effect{
				{Type: EffectConditionalAdvantage, Trigger: TriggerOnAttackRoll, On: "advantage"},
			},
		},
		{
			Name: "Sunlight Sensitivity",
			Effects: []Effect{
				{Type: EffectConditionalAdvantage, Trigger: TriggerOnAttackRoll, On: "disadvantage"},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnAttackRoll, EffectContext{})
	if result.RollMode != dice.AdvantageAndDisadvantage {
		t.Errorf("expected advantage+disadvantage, got %v", result.RollMode)
	}
}

func TestProcessEffects_MultipleSimultaneousEffects(t *testing.T) {
	// Simulate a raging barbarian with Archery fighting style attacking with a melee STR weapon
	features := []FeatureDefinition{
		{
			Name: "Rage",
			Effects: []Effect{
				{
					Type:     EffectModifyDamageRoll,
					Trigger:  TriggerOnDamageRoll,
					Modifier: 2,
					Conditions: EffectConditions{
						WhenRaging:  true,
						AttackType:  "melee",
						AbilityUsed: "str",
					},
				},
				{
					Type:        EffectGrantResistance,
					Trigger:     TriggerOnTakeDamage,
					DamageTypes: []string{"bludgeoning", "piercing", "slashing"},
					Conditions:  EffectConditions{WhenRaging: true},
				},
			},
		},
		{
			Name: "Great Weapon Master",
			Effects: []Effect{
				{
					Type:     EffectModifyDamageRoll,
					Trigger:  TriggerOnDamageRoll,
					Modifier: 10,
					Conditions: EffectConditions{
						AttackType:  "melee",
					},
				},
			},
		},
		{
			Name: "Sneak Attack",
			Effects: []Effect{
				{
					Type:    EffectExtraDamageDice,
					Trigger: TriggerOnDamageRoll,
					Dice:    "3d6",
					Conditions: EffectConditions{
						WeaponProperty: "finesse",
					},
				},
			},
		},
	}

	ctx := EffectContext{
		IsRaging:       true,
		AttackType:     "melee",
		AbilityUsed:    "str",
		WeaponProperty: "heavy", // Not finesse, so Sneak Attack should not apply
	}
	result := ProcessEffects(features, TriggerOnDamageRoll, ctx)

	// Rage +2 + GWM +10 = 12 flat modifier
	if result.FlatModifier != 12 {
		t.Errorf("expected flat modifier 12, got %d", result.FlatModifier)
	}
	// Sneak Attack should NOT apply (weapon is heavy, not finesse)
	if len(result.ExtraDice) != 0 {
		t.Errorf("expected no extra dice, got %v", result.ExtraDice)
	}
	// Should have 2 applied effects (Rage damage + GWM damage)
	if len(result.AppliedEffects) != 2 {
		t.Errorf("expected 2 applied effects, got %d", len(result.AppliedEffects))
	}
}

func TestProcessEffects_PriorityOrder(t *testing.T) {
	// Test that effects are applied in priority order:
	// immunities -> resistances -> flat mods -> dice mods -> advantage
	features := []FeatureDefinition{
		{
			Name: "Reckless",
			Effects: []Effect{
				{Type: EffectConditionalAdvantage, Trigger: TriggerOnTakeDamage, On: "advantage"},
			},
		},
		{
			Name: "Sneak Extra",
			Effects: []Effect{
				{Type: EffectExtraDamageDice, Trigger: TriggerOnTakeDamage, Dice: "2d6"},
			},
		},
		{
			Name: "Flat Bonus",
			Effects: []Effect{
				{Type: EffectModifyDamageRoll, Trigger: TriggerOnTakeDamage, Modifier: 5},
			},
		},
		{
			Name: "Resist",
			Effects: []Effect{
				{Type: EffectGrantResistance, Trigger: TriggerOnTakeDamage, DamageTypes: []string{"fire"}},
			},
		},
		{
			Name: "Immune",
			Effects: []Effect{
				{Type: EffectGrantImmunity, Trigger: TriggerOnTakeDamage, On: "poison"},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTakeDamage, EffectContext{})

	if len(result.AppliedEffects) != 5 {
		t.Fatalf("expected 5 applied effects, got %d", len(result.AppliedEffects))
	}

	// Verify ordering: immunity first, then resistance, flat mod, dice mod, advantage
	expectedTypes := []EffectType{
		EffectGrantImmunity,
		EffectGrantResistance,
		EffectModifyDamageRoll,
		EffectExtraDamageDice,
		EffectConditionalAdvantage,
	}
	for i, et := range expectedTypes {
		if result.AppliedEffects[i].Effect.Type != et {
			t.Errorf("position %d: expected %q, got %q", i, et, result.AppliedEffects[i].Effect.Type)
		}
	}
}

func TestProcessEffects_NoFeatures(t *testing.T) {
	result := ProcessEffects(nil, TriggerOnAttackRoll, EffectContext{})
	if result.FlatModifier != 0 {
		t.Errorf("expected 0 flat modifier, got %d", result.FlatModifier)
	}
	if len(result.AppliedEffects) != 0 {
		t.Errorf("expected 0 applied effects, got %d", len(result.AppliedEffects))
	}
	if result.RollMode != dice.Normal {
		t.Errorf("expected normal roll mode, got %v", result.RollMode)
	}
}

func TestProcessEffects_EmptyFeatures(t *testing.T) {
	result := ProcessEffects([]FeatureDefinition{}, TriggerOnAttackRoll, EffectContext{})
	if len(result.AppliedEffects) != 0 {
		t.Errorf("expected 0 applied effects, got %d", len(result.AppliedEffects))
	}
}

func TestFeatureDefinition_JSONRoundTrip(t *testing.T) {
	original := FeatureDefinition{
		Name:   "Rage",
		Source: "barbarian",
		Effects: []Effect{
			{
				Type:     EffectModifyDamageRoll,
				Trigger:  TriggerOnDamageRoll,
				Modifier: 2,
				Conditions: EffectConditions{
					WhenRaging:  true,
					AttackType:  "melee",
					AbilityUsed: "str",
				},
			},
			{
				Type:        EffectGrantResistance,
				Trigger:     TriggerOnTakeDamage,
				DamageTypes: []string{"bludgeoning", "piercing", "slashing"},
				Conditions:  EffectConditions{WhenRaging: true},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded FeatureDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("name: got %q, want %q", decoded.Name, original.Name)
	}
	if len(decoded.Effects) != len(original.Effects) {
		t.Fatalf("effects count: got %d, want %d", len(decoded.Effects), len(original.Effects))
	}
	if decoded.Effects[0].Type != EffectModifyDamageRoll {
		t.Errorf("effect[0].Type: got %q, want %q", decoded.Effects[0].Type, EffectModifyDamageRoll)
	}
	if decoded.Effects[0].Trigger != TriggerOnDamageRoll {
		t.Errorf("effect[0].Trigger: got %q, want %q", decoded.Effects[0].Trigger, TriggerOnDamageRoll)
	}
	if !decoded.Effects[0].Conditions.WhenRaging {
		t.Error("effect[0].Conditions.WhenRaging should be true")
	}
	if len(decoded.Effects[1].DamageTypes) != 3 {
		t.Errorf("effect[1].DamageTypes: got %d, want 3", len(decoded.Effects[1].DamageTypes))
	}
}

func TestProcessEffects_GrantImmunityWithDamageTypes(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Fire Immunity",
			Effects: []Effect{
				{
					Type:        EffectGrantImmunity,
					Trigger:     TriggerOnTakeDamage,
					DamageTypes: []string{"fire", "poison"},
				},
			},
		},
	}

	result := ProcessEffects(features, TriggerOnTakeDamage, EffectContext{})
	if len(result.Immunities) != 2 {
		t.Errorf("expected 2 immunities, got %d: %v", len(result.Immunities), result.Immunities)
	}
}

func TestCollectEffects_MultipleFeatures(t *testing.T) {
	features := []FeatureDefinition{
		{
			Name: "Aura of Protection",
			Effects: []Effect{
				{Type: EffectModifySave, Trigger: TriggerOnSave, Modifier: 3},
			},
		},
		{
			Name: "Bless",
			Effects: []Effect{
				{Type: EffectModifySave, Trigger: TriggerOnSave, Modifier: 0, Dice: "1d4"},
			},
		},
	}

	results := CollectEffects(features, TriggerOnSave, EffectContext{})
	if len(results) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(results))
	}
}

func TestEffectPriority_UnknownDefaultsToFlatModifier(t *testing.T) {
	got := EffectPriority(EffectType("unknown_type"))
	if got != PriorityFlatModifier {
		t.Errorf("expected PriorityFlatModifier for unknown type, got %d", got)
	}
}
