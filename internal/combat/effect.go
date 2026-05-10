package combat

import (
	"sort"

	"github.com/ab/dndnd/internal/dice"
)

// EffectType represents the kind of mechanical effect a feature provides.
type EffectType string

const (
	EffectModifyAttackRoll    EffectType = "modify_attack_roll"
	EffectModifyDamageRoll    EffectType = "modify_damage_roll"
	EffectExtraDamageDice     EffectType = "extra_damage_dice"
	EffectModifyAC            EffectType = "modify_ac"
	EffectModifySave          EffectType = "modify_save"
	EffectModifyCheck         EffectType = "modify_check"
	EffectModifySpeed         EffectType = "modify_speed"
	EffectGrantResistance     EffectType = "grant_resistance"
	EffectGrantImmunity       EffectType = "grant_immunity"
	EffectExtraAttack         EffectType = "extra_attack"
	EffectModifyHP            EffectType = "modify_hp"
	EffectConditionalAdvantage EffectType = "conditional_advantage"
	EffectResourceOnHit       EffectType = "resource_on_hit"
	EffectReactionTrigger     EffectType = "reaction_trigger"
	EffectAura                EffectType = "aura"
	EffectReplaceRoll         EffectType = "replace_roll"
	EffectGrantProficiency    EffectType = "grant_proficiency"
	EffectModifyRange         EffectType = "modify_range"
	EffectDMResolution        EffectType = "dm_resolution"
)

// IsValid returns true if the effect type is a recognized value.
func (et EffectType) IsValid() bool {
	switch et {
	case EffectModifyAttackRoll, EffectModifyDamageRoll, EffectExtraDamageDice,
		EffectModifyAC, EffectModifySave, EffectModifyCheck, EffectModifySpeed,
		EffectGrantResistance, EffectGrantImmunity, EffectExtraAttack,
		EffectModifyHP, EffectConditionalAdvantage, EffectResourceOnHit,
		EffectReactionTrigger, EffectAura, EffectReplaceRoll,
		EffectGrantProficiency, EffectModifyRange, EffectDMResolution:
		return true
	default:
		return false
	}
}

// TriggerPoint represents when an effect is evaluated during gameplay.
type TriggerPoint string

const (
	TriggerOnAttackRoll TriggerPoint = "on_attack_roll"
	TriggerOnDamageRoll TriggerPoint = "on_damage_roll"
	TriggerOnTakeDamage TriggerPoint = "on_take_damage"
	TriggerOnSave       TriggerPoint = "on_save"
	TriggerOnCheck      TriggerPoint = "on_check"
	TriggerOnTurnStart  TriggerPoint = "on_turn_start"
	TriggerOnTurnEnd    TriggerPoint = "on_turn_end"
	TriggerOnRest       TriggerPoint = "on_rest"
)

// IsValid returns true if the trigger point is a recognized value.
func (tp TriggerPoint) IsValid() bool {
	switch tp {
	case TriggerOnAttackRoll, TriggerOnDamageRoll, TriggerOnTakeDamage,
		TriggerOnSave, TriggerOnCheck, TriggerOnTurnStart,
		TriggerOnTurnEnd, TriggerOnRest:
		return true
	default:
		return false
	}
}

// EffectConditions holds the filters that must be true for an effect to apply.
type EffectConditions struct {
	WhenRaging             bool     `json:"when_raging,omitempty"`
	WhenConcentrating      bool     `json:"when_concentrating,omitempty"`
	WeaponProperty         string   `json:"weapon_property,omitempty"`
	WeaponProperties       []string `json:"weapon_properties,omitempty"`
	AttackType             string   `json:"attack_type,omitempty"`
	AbilityUsed            string   `json:"ability_used,omitempty"`
	TargetCondition        string   `json:"target_condition,omitempty"`
	AllyWithin             int      `json:"ally_within,omitempty"`
	UsesRemaining          bool     `json:"uses_remaining,omitempty"`
	OncePerTurn            bool     `json:"once_per_turn,omitempty"`
	HasAdvantage           bool     `json:"has_advantage,omitempty"`
	AdvantageOrAllyWithin  int      `json:"advantage_or_ally_within,omitempty"`
	WearingArmor           bool     `json:"wearing_armor,omitempty"`
	NotWearingArmor        bool     `json:"not_wearing_armor,omitempty"`
	OneHandedMeleeOnly     bool     `json:"one_handed_melee_only,omitempty"`
	AuraRadius             int      `json:"aura_radius,omitempty"`
	Target                 string   `json:"target,omitempty"`
}

// Effect represents a single mechanical effect declared by a feature.
type Effect struct {
	Type         EffectType       `json:"type"`
	Trigger      TriggerPoint     `json:"trigger"`
	Modifier     int              `json:"modifier,omitempty"`
	Dice         string           `json:"dice,omitempty"`
	DamageTypes  []string         `json:"damage_types,omitempty"`
	On           string           `json:"on,omitempty"`
	Description  string           `json:"description,omitempty"`
	ReplaceValue int              `json:"replace_value,omitempty"`
	Conditions   EffectConditions `json:"conditions,omitzero"`
}

// MatchesTrigger returns true if this effect's trigger matches the given trigger point.
func (e Effect) MatchesTrigger(tp TriggerPoint) bool {
	return e.Trigger == tp
}

// FeatureDefinition represents a named feature with its declared effects.
type FeatureDefinition struct {
	Name    string   `json:"feature"`
	Source  string   `json:"source,omitempty"`
	Effects []Effect `json:"effects"`
}

// ResolutionPriority defines the order in which effect types are resolved.
// Lower number = higher priority (applied first).
type ResolutionPriority int

const (
	PriorityImmunity     ResolutionPriority = 1
	PriorityResistance   ResolutionPriority = 2
	PriorityFlatModifier ResolutionPriority = 3
	PriorityDiceModifier ResolutionPriority = 4
	PriorityAdvantage    ResolutionPriority = 5
)

// EffectPriority returns the resolution priority for an effect type.
func EffectPriority(et EffectType) ResolutionPriority {
	switch et {
	case EffectGrantImmunity:
		return PriorityImmunity
	case EffectGrantResistance:
		return PriorityResistance
	case EffectModifyAttackRoll, EffectModifyDamageRoll, EffectModifyAC,
		EffectModifySave, EffectModifyCheck, EffectModifySpeed,
		EffectModifyHP, EffectModifyRange, EffectExtraAttack,
		EffectGrantProficiency, EffectResourceOnHit, EffectReactionTrigger,
		EffectAura, EffectReplaceRoll, EffectDMResolution:
		return PriorityFlatModifier
	case EffectExtraDamageDice:
		return PriorityDiceModifier
	case EffectConditionalAdvantage:
		return PriorityAdvantage
	default:
		return PriorityFlatModifier
	}
}

// EffectContext provides the current combat state for condition filtering.
type EffectContext struct {
	IsRaging           bool
	IsConcentrating    bool
	WeaponProperty     string
	WeaponProperties   []string
	AttackType         string
	AbilityUsed        string
	TargetCondition    string
	AllyWithinFt       int
	HasAdvantage       bool
	WearingArmor       bool
	OneHandedMeleeOnly bool
	UsedThisTurn       map[string]bool
	UsesRemaining      map[string]int
}

// matchesAnyProperty returns true if any required property matches the context's
// WeaponProperty or is found in the context's WeaponProperties list.
func matchesAnyProperty(required []string, ctx EffectContext) bool {
	for _, p := range required {
		if p == ctx.WeaponProperty {
			return true
		}
		for _, wp := range ctx.WeaponProperties {
			if p == wp {
				return true
			}
		}
	}
	return false
}

// EvaluateConditions checks whether an effect's conditions are satisfied
// by the current context. Returns true if all conditions pass.
func EvaluateConditions(e Effect, ctx EffectContext) bool {
	c := e.Conditions

	if c.WhenRaging && !ctx.IsRaging {
		return false
	}
	if c.WhenConcentrating && !ctx.IsConcentrating {
		return false
	}
	if c.AttackType != "" && c.AttackType != ctx.AttackType {
		return false
	}
	if c.AbilityUsed != "" && c.AbilityUsed != ctx.AbilityUsed {
		return false
	}
	if c.WeaponProperty != "" && c.WeaponProperty != ctx.WeaponProperty {
		return false
	}
	// WeaponProperties: OR match — weapon must have at least one of the listed properties
	if len(c.WeaponProperties) > 0 && !matchesAnyProperty(c.WeaponProperties, ctx) {
		return false
	}
	if c.TargetCondition != "" && c.TargetCondition != ctx.TargetCondition {
		return false
	}
	if c.AllyWithin > 0 && ctx.AllyWithinFt > c.AllyWithin {
		return false
	}
	if c.HasAdvantage && !ctx.HasAdvantage {
		return false
	}
	// AdvantageOrAllyWithin: OR condition — advantage or ally within N ft
	if c.AdvantageOrAllyWithin > 0 {
		if !ctx.HasAdvantage && ctx.AllyWithinFt > c.AdvantageOrAllyWithin {
			return false
		}
	}
	if c.WearingArmor && !ctx.WearingArmor {
		return false
	}
	if c.NotWearingArmor && ctx.WearingArmor {
		return false
	}
	if c.OneHandedMeleeOnly && !ctx.OneHandedMeleeOnly {
		return false
	}
	if c.OncePerTurn && ctx.UsedThisTurn != nil && ctx.UsedThisTurn[string(e.Type)] {
		return false
	}
	if c.UsesRemaining && (ctx.UsesRemaining == nil || ctx.UsesRemaining[string(e.Type)] <= 0) {
		return false
	}

	return true
}

// ResolvedEffect is an effect that passed condition filtering and is ready
// to apply, along with metadata about the source feature.
type ResolvedEffect struct {
	FeatureName string
	Effect      Effect
	Priority    ResolutionPriority
}

// ProcessorResult holds the output of the single-pass effect processor.
type ProcessorResult struct {
	FlatModifier     int
	ExtraDice        []string
	Resistances      []string
	Immunities       []string
	RollMode         dice.RollMode
	ReplacedRoll     *int
	ExtraAttacks     int
	HPModifier       int
	SpeedModifier    int
	ACModifier       int
	RangeModifier    int
	Proficiencies    []string
	ResourceTriggers []string
	ReactionTriggers []string
	AuraEffects      []ResolvedEffect
	DMResolutions    []ResolvedEffect
	AppliedEffects   []ResolvedEffect
}

// CollectEffects gathers all effects from the given features that match the
// trigger point and pass condition filtering.
func CollectEffects(features []FeatureDefinition, trigger TriggerPoint, ctx EffectContext) []ResolvedEffect {
	var resolved []ResolvedEffect
	for _, f := range features {
		for _, e := range f.Effects {
			if !e.MatchesTrigger(trigger) {
				continue
			}
			if !EvaluateConditions(e, ctx) {
				continue
			}
			resolved = append(resolved, ResolvedEffect{
				FeatureName: f.Name,
				Effect:      e,
				Priority:    EffectPriority(e.Type),
			})
		}
	}
	return resolved
}

// SortByPriority sorts resolved effects by their resolution priority
// (immunities first, then R/V, flat mods, dice mods, adv/disadv).
func SortByPriority(effects []ResolvedEffect) {
	sort.SliceStable(effects, func(i, j int) bool {
		return effects[i].Priority < effects[j].Priority
	})
}

// ProcessEffects is the single-pass processor: collect active effects,
// filter by conditions, sort by priority, and apply in order.
func ProcessEffects(features []FeatureDefinition, trigger TriggerPoint, ctx EffectContext) ProcessorResult {
	effects := CollectEffects(features, trigger, ctx)
	SortByPriority(effects)

	result := ProcessorResult{
		RollMode: dice.Normal,
	}

	var advReasons, disadvReasons []string

	for _, re := range effects {
		e := re.Effect
		result.AppliedEffects = append(result.AppliedEffects, re)

		switch e.Type {
		case EffectGrantImmunity:
			result.Immunities = append(result.Immunities, e.DamageTypes...)
			if e.On != "" {
				result.Immunities = append(result.Immunities, e.On)
			}

		case EffectGrantResistance:
			result.Resistances = append(result.Resistances, e.DamageTypes...)

		case EffectModifyAttackRoll, EffectModifyDamageRoll, EffectModifySave, EffectModifyCheck:
			result.FlatModifier += e.Modifier

		case EffectModifyAC:
			result.ACModifier += e.Modifier

		case EffectModifySpeed:
			result.SpeedModifier += e.Modifier

		case EffectModifyHP:
			result.HPModifier += e.Modifier

		case EffectModifyRange:
			result.RangeModifier += e.Modifier

		case EffectExtraDamageDice:
			if e.Dice != "" {
				result.ExtraDice = append(result.ExtraDice, e.Dice)
			}

		case EffectExtraAttack:
			result.ExtraAttacks += e.Modifier
			if e.Modifier == 0 {
				result.ExtraAttacks++
			}

		case EffectConditionalAdvantage:
			if e.On == "disadvantage" {
				disadvReasons = append(disadvReasons, re.FeatureName)
			} else {
				advReasons = append(advReasons, re.FeatureName)
			}

		case EffectReplaceRoll:
			val := e.ReplaceValue
			result.ReplacedRoll = &val

		case EffectGrantProficiency:
			if e.On != "" {
				result.Proficiencies = append(result.Proficiencies, e.On)
			}

		case EffectResourceOnHit:
			result.ResourceTriggers = append(result.ResourceTriggers, re.FeatureName)

		case EffectReactionTrigger:
			result.ReactionTriggers = append(result.ReactionTriggers, re.FeatureName)

		case EffectAura:
			// Reserved API: collected for future Aura-of-Protection /
			// Bless surfacing. No production consumer today.
			result.AuraEffects = append(result.AuraEffects, re)

		case EffectDMResolution:
			// Reserved API: collected for future "DM clarifies" flow.
			// No production consumer today.
			result.DMResolutions = append(result.DMResolutions, re)
		}
	}

	// Resolve advantage/disadvantage
	result.RollMode = resolveMode(advReasons, disadvReasons)

	return result
}
