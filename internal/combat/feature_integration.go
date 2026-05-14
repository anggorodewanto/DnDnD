package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/refdata"
)

const (
	FeatureKeyRage              = "rage"
	FeatureKeyKi                = "ki"
	FeatureKeyWildShape         = "wild_shape"
	FeatureKeyBardicInspiration = "bardic-inspiration"
	FeatureKeyLayOnHands        = "lay-on-hands"
	FeatureKeyActionSurge       = "action-surge"
	FeatureKeySorceryPoints     = "sorcery-points"
)

// ParseFeatureUses extracts the canonical feature_uses map (keyed by feature
// name, valued by character.FeatureUse) and the named feature's remaining
// Current count. Unifies the previous flat int shape with rest/dashboard's
// {Current, Max, Recharge} struct so combat-written rows can be recharged on
// rest and dashboard-written rows can be decremented by combat (SR-009).
func ParseFeatureUses(char refdata.Character, featureKey string) (map[string]character.FeatureUse, int, error) {
	featureUses := make(map[string]character.FeatureUse)
	if !char.FeatureUses.Valid || len(char.FeatureUses.RawMessage) == 0 {
		return featureUses, 0, nil
	}
	if err := json.Unmarshal(char.FeatureUses.RawMessage, &featureUses); err != nil {
		return nil, 0, fmt.Errorf("parsing feature_uses: %w", err)
	}
	return featureUses, featureUses[featureKey].Current, nil
}

// ClassLevelFromJSON returns the level for the named class from raw JSON.
func ClassLevelFromJSON(classesJSON []byte, className string) int {
	if len(classesJSON) == 0 {
		return 0
	}
	var classes []CharacterClass
	if err := json.Unmarshal(classesJSON, &classes); err != nil {
		return 0
	}
	return classLevel(classes, className)
}

// DeductFeaturePool deducts a variable amount from a feature's pool, persists, and returns the new remaining value.
func (s *Service) DeductFeaturePool(ctx context.Context, char refdata.Character, featureKey string, featureUses map[string]character.FeatureUse, current int, amount int) (int, error) {
	if amount > current {
		return 0, fmt.Errorf("insufficient %s pool: need %d, have %d", featureKey, amount, current)
	}
	return s.SetFeaturePool(ctx, char, featureKey, featureUses, current-amount)
}

// SetFeaturePool sets a feature's Current pool to an absolute value, preserves
// the row's Max + Recharge metadata, persists the canonical shape, and returns
// the new Current value.
func (s *Service) SetFeaturePool(ctx context.Context, char refdata.Character, featureKey string, featureUses map[string]character.FeatureUse, value int) (int, error) {
	fu := featureUses[featureKey]
	fu.Current = value
	featureUses[featureKey] = fu
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return 0, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return 0, fmt.Errorf("updating feature_uses: %w", err)
	}
	return value, nil
}

// DeductFeatureUse decrements a feature's remaining uses by 1, persists, and returns the new count.
func (s *Service) DeductFeatureUse(ctx context.Context, char refdata.Character, featureKey string, featureUses map[string]character.FeatureUse, current int) (int, error) {
	return s.DeductFeaturePool(ctx, char, featureKey, featureUses, current, 1)
}

// SneakAttackFeature returns the FeatureDefinition for Sneak Attack at the given rogue level.
// Sneak Attack adds extra damage dice once per turn when using a finesse or ranged weapon
// with advantage, or when an ally is within 5ft of the target.
func SneakAttackFeature(rogueLevel int) FeatureDefinition {
	return FeatureDefinition{
		Name:   "Sneak Attack",
		Source: "rogue",
		Effects: []Effect{
			{
				Type:    EffectExtraDamageDice,
				Trigger: TriggerOnDamageRoll,
				Dice:    SneakAttackDice(rogueLevel),
				Conditions: EffectConditions{
					WeaponProperties:      []string{"finesse", "ranged"},
					AdvantageOrAllyWithin: 5,
					OncePerTurn:           true,
				},
			},
		},
	}
}

// EvasionFeature returns the FeatureDefinition for Evasion (Rogue 7+).
// On DEX save: success = no damage, fail = half damage.
func EvasionFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Evasion",
		Source: "rogue",
		Effects: []Effect{
			{
				Type:    EffectModifySave,
				Trigger: TriggerOnSave,
				On:      "evasion",
				Conditions: EffectConditions{
					AbilityUsed: "dex",
				},
			},
		},
	}
}

// ApplyEvasion applies Evasion logic to damage from a DEX save effect.
// On save success: damage is reduced to 0.
// On save failure: damage is halved (rounded down).
func ApplyEvasion(damage int, saveSuccess bool) int {
	if saveSuccess {
		return 0
	}
	return damage / 2
}

// UncannyDodgeFeature returns the FeatureDefinition for Uncanny Dodge (Rogue 5+).
// Reaction: halve damage from one visible attacker.
func UncannyDodgeFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Uncanny Dodge",
		Source: "rogue",
		Effects: []Effect{
			{
				Type:    EffectReactionTrigger,
				Trigger: TriggerOnTakeDamage,
				On:      "uncanny_dodge",
			},
		},
	}
}

// ApplyUncannyDodge halves the incoming damage (rounded down).
func ApplyUncannyDodge(damage int) int {
	return damage / 2
}

// SacredWeaponFeature returns a FeatureDefinition for the Sacred Weapon
// Channel Divinity option. Adds CHA modifier to all weapon attack rolls.
func SacredWeaponFeature(chaMod int) FeatureDefinition {
	return FeatureDefinition{
		Name:   "Sacred Weapon",
		Source: "channel_divinity",
		Effects: []Effect{
			{
				Type:     EffectModifyAttackRoll,
				Trigger:  TriggerOnAttackRoll,
				Modifier: chaMod,
			},
		},
	}
}

// ArcheryFeature returns the FeatureDefinition for the Archery fighting style.
// +2 to ranged attack rolls.
func ArcheryFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Archery",
		Source: "fighting_style",
		Effects: []Effect{
			{
				Type:     EffectModifyAttackRoll,
				Trigger:  TriggerOnAttackRoll,
				Modifier: 2,
				Conditions: EffectConditions{
					AttackType: "ranged",
				},
			},
		},
	}
}

// DefenseFeature returns the FeatureDefinition for the Defense fighting style.
// +1 AC when wearing armor.
func DefenseFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Defense",
		Source: "fighting_style",
		Effects: []Effect{
			{
				Type:     EffectModifyAC,
				Trigger:  TriggerOnAttackRoll,
				Modifier: 1,
				Conditions: EffectConditions{
					WearingArmor: true,
				},
			},
		},
	}
}

// DuelingFeature returns the FeatureDefinition for the Dueling fighting style.
// +2 damage when wielding a one-handed melee weapon with no weapon in off-hand.
func DuelingFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Dueling",
		Source: "fighting_style",
		Effects: []Effect{
			{
				Type:     EffectModifyDamageRoll,
				Trigger:  TriggerOnDamageRoll,
				Modifier: 2,
				Conditions: EffectConditions{
					AttackType:         "melee",
					OneHandedMeleeOnly: true,
				},
			},
		},
	}
}

// GreatWeaponFightingFeature returns the FeatureDefinition for the Great Weapon Fighting style.
// Reroll 1s and 2s on damage dice with two-handed/versatile weapons.
func GreatWeaponFightingFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Great Weapon Fighting",
		Source: "fighting_style",
		Effects: []Effect{
			{
				Type:    EffectReplaceRoll,
				Trigger: TriggerOnDamageRoll,
				On:      "great_weapon_fighting",
				Conditions: EffectConditions{
					WeaponProperties: []string{"heavy", "versatile"},
					AttackType:       "melee",
				},
			},
		},
	}
}

// ApplyGreatWeaponFighting rerolls 1s and 2s on damage dice.
// rerollFn takes the die sides and returns the reroll result.
func ApplyGreatWeaponFighting(rolls []int, dieSides int, rerollFn func(int) int) []int {
	result := make([]int, len(rolls))
	for i, r := range rolls {
		if r <= 2 {
			result[i] = rerollFn(dieSides)
		} else {
			result[i] = r
		}
	}
	return result
}

// AuraOfProtectionFeature returns the FeatureDefinition for Paladin Aura of
// Protection (SR-024). The paladin (and allies within the aura radius —
// gated upstream by the caller's distance check) adds the paladin's CHA mod
// to saving throws. Radius is level-gated via PaladinAuraRadiusFt and is
// applied at the call site (e.g. save_handler.nearbyPaladinAuras) rather
// than inside the effect-condition machinery, since EffectConditions has no
// concept of "ally within N of a specific other combatant."
func AuraOfProtectionFeature(chaMod int) FeatureDefinition {
	return FeatureDefinition{
		Name:   "Aura of Protection",
		Source: "paladin",
		Effects: []Effect{
			{
				Type:     EffectModifySave,
				Trigger:  TriggerOnSave,
				Modifier: chaMod,
			},
		},
	}
}

// PaladinAuraRadiusFt returns the Aura of Protection radius in feet for a
// given paladin level: 0 below L6, 10 ft at L6–L17, 30 ft at L18+ (spec
// line 2587). SR-024.
func PaladinAuraRadiusFt(paladinLevel int) int {
	if paladinLevel >= 18 {
		return 30
	}
	if paladinLevel >= 6 {
		return 10
	}
	return 0
}

// ResolvePaladinAura inspects a character's classes and, when the character
// is a Paladin at L6+, returns the FeatureDefinition for Aura of Protection
// with the given CHA modifier baked in. Returns (_, false) for non-paladins
// or sub-L6 paladins so the caller can early-return. SR-024.
func ResolvePaladinAura(classes []CharacterClass, chaMod int) (FeatureDefinition, bool) {
	level := classLevel(classes, "Paladin")
	if level < 6 {
		return FeatureDefinition{}, false
	}
	return AuraOfProtectionFeature(chaMod), true
}

// PackTacticsFeature returns the FeatureDefinition for Pack Tactics.
// Creature feature: advantage on attack when ally within 5ft of target.
func PackTacticsFeature() FeatureDefinition {
	return FeatureDefinition{
		Name:   "Pack Tactics",
		Source: "creature",
		Effects: []Effect{
			{
				Type:    EffectConditionalAdvantage,
				Trigger: TriggerOnAttackRoll,
				On:      "advantage",
				Conditions: EffectConditions{
					AllyWithin: 5,
				},
			},
		},
	}
}

// BuildFeatureDefinitions converts character classes and features into a slice
// of FeatureDefinition for the effect processor. It maps mechanical_effect strings
// to their corresponding feature definitions. Additional extra definitions (e.g.
// from magic items via magicitem.CollectItemFeatures) are appended if provided.
func BuildFeatureDefinitions(classes []CharacterClass, features []CharacterFeature, extraDefs ...[]FeatureDefinition) []FeatureDefinition {
	var defs []FeatureDefinition

	rogueLevel := classLevel(classes, "Rogue")
	barbarianLevel := classLevel(classes, "Barbarian")
	monkLevel := classLevel(classes, "Monk")
	// Druid level is checked in the service method for Wild Shape, not here.

	for _, f := range features {
		// Handle comma-separated mechanical effects (e.g., "martial_arts_d4,bonus_action_unarmed_strike")
		for _, effect := range splitMechanicalEffects(f.MechanicalEffect) {
			switch effect {
			case "rage":
				defs = append(defs, RageFeature(max(barbarianLevel, 1)))
			case "sneak_attack":
				defs = append(defs, SneakAttackFeature(max(rogueLevel, 1)))
			case "evasion":
				defs = append(defs, EvasionFeature())
			case "uncanny_dodge":
				defs = append(defs, UncannyDodgeFeature())
			case "archery":
				defs = append(defs, ArcheryFeature())
			case "defense":
				defs = append(defs, DefenseFeature())
			case "dueling":
				defs = append(defs, DuelingFeature())
			case "great_weapon_fighting":
				defs = append(defs, GreatWeaponFightingFeature())
			case "pack_tactics":
				defs = append(defs, PackTacticsFeature())
			case "aura_of_protection":
				// SR-024 — recognise the seed-class mechanical_effect string
				// so future seed_classes.go updates Just Work. CHA mod is not
				// available at this layer (BuildFeatureDefinitions only sees
				// classes+features, not ability scores), so emit a zero-mod
				// placeholder. The live CHA-scaled aura is produced by
				// ResolvePaladinAura at the save-handler layer where scores
				// are in scope. Gated on paladin L6+ to match the spec.
				if classLevel(classes, "Paladin") >= 6 {
					defs = append(defs, AuraOfProtectionFeature(0))
				}
			case "wild_shape":
				// Wild Shape is an activation command, not a passive combat effect.
				// No FeatureDefinition needed here; handled by ActivateWildShape service method.

			// Monk features
			case "martial_arts_d4":
				// Martial arts is a passive feature handled at attack resolution time.
				// No FeatureDefinition needed — the monk level is checked in ResolveAttack.
			case "bonus_action_unarmed_strike":
				// Handled by MartialArtsBonusAttack service method, not a passive feature.
			case "speed_plus_10":
				defs = append(defs, UnarmoredMovementFeature(max(monkLevel, 2)))
			}
		}
	}

	// Merge extra feature definitions (e.g. magic items)
	for _, extra := range extraDefs {
		defs = append(defs, extra...)
	}

	return defs
}

// splitMechanicalEffects splits a comma-separated mechanical_effect string into
// individual effect IDs. Single effects are returned as-is.
func splitMechanicalEffects(effect string) []string {
	parts := strings.Split(effect, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// classLevel returns the level for the given class name, or 0 if not found.
func classLevel(classes []CharacterClass, className string) int {
	for _, c := range classes {
		if strings.EqualFold(c.Class, className) {
			return c.Level
		}
	}
	return 0
}

// AttackEffectInput holds the parameters needed to build an EffectContext for attack effects.
type AttackEffectInput struct {
	Weapon             refdata.Weapon
	HasAdvantage       bool
	AllyWithinFt       int
	WearingArmor       bool
	OneHandedMeleeOnly bool
	IsRaging           bool
	AbilityUsed        string
	UsedThisTurn       map[string]bool
}

// BuildAttackEffectContext builds an EffectContext from attack parameters.
func BuildAttackEffectContext(input AttackEffectInput) EffectContext {
	isRanged := IsRangedWeapon(input.Weapon)

	attackType := "melee"
	if isRanged {
		attackType = "ranged"
	}

	weaponProperty := ""
	if HasProperty(input.Weapon, "finesse") {
		weaponProperty = "finesse"
	} else if isRanged {
		weaponProperty = "ranged"
	}

	return EffectContext{
		AttackType:         attackType,
		WeaponProperty:     weaponProperty,
		WeaponProperties:   input.Weapon.Properties,
		HasAdvantage:       input.HasAdvantage,
		AllyWithinFt:       input.AllyWithinFt,
		WearingArmor:       input.WearingArmor,
		OneHandedMeleeOnly: input.OneHandedMeleeOnly,
		IsRaging:           input.IsRaging,
		AbilityUsed:        input.AbilityUsed,
		UsedThisTurn:       input.UsedThisTurn,
	}
}

// SneakAttackDice returns the sneak attack dice expression for a given rogue level.
// 1d6 per 2 rogue levels, rounded up.
func SneakAttackDice(rogueLevel int) string {
	count := (rogueLevel + 1) / 2
	return fmt.Sprintf("%dd6", count)
}

// collectMagicItemFeatures builds the FeatureDefinition list contributed by a
// character's equipped (and attuned, when required) magic items. It mirrors
// magicitem.CollectItemFeatures but lives inside the combat package so the
// hot attack/turn paths don't have to import magicitem (which already imports
// combat — the reverse would cycle).
//
// Inventory / attunement JSON parse errors degrade silently to nil, matching
// the buildSaveFeatureEffects / populateAttackFES "drop the bonus rather than
// fail the roll" convention. Per-item passive-effect parse errors fall back to
// the MagicBonus path so a single bad magic_properties row never wipes the
// rest of the equipped magic loadout.
func collectMagicItemFeatures(char refdata.Character) []FeatureDefinition {
	items, err := character.ParseInventoryItems(char.Inventory.RawMessage, char.Inventory.Valid)
	if err != nil {
		return nil
	}
	if len(items) == 0 {
		return nil
	}
	attunement, err := character.ParseAttunementSlots(char.AttunementSlots.RawMessage, char.AttunementSlots.Valid)
	if err != nil {
		return nil
	}
	attunedSet := make(map[string]bool, len(attunement))
	for _, a := range attunement {
		attunedSet[a.ItemID] = true
	}

	var defs []FeatureDefinition
	for _, item := range items {
		if !item.IsMagic || !item.Equipped {
			continue
		}
		if item.RequiresAttunement && !attunedSet[item.ItemID] {
			continue
		}
		if item.MagicProperties != "" {
			if effects := parseMagicItemPassiveEffects(item.MagicProperties); len(effects) > 0 {
				defs = append(defs, FeatureDefinition{
					Name:    item.Name,
					Source:  "magic_item",
					Effects: effects,
				})
				continue
			}
		}
		if effects := magicBonusEffects(item); len(effects) > 0 {
			defs = append(defs, FeatureDefinition{
				Name:    item.Name,
				Source:  "magic_item",
				Effects: effects,
			})
		}
	}
	return defs
}

// magicBonusEffects converts an equipped weapon/armor's MagicBonus into combat
// Effects. Twin of magicitem.ItemFeatures, kept in-package to avoid the
// combat→magicitem cycle.
func magicBonusEffects(item character.InventoryItem) []Effect {
	if item.MagicBonus == 0 {
		return nil
	}
	switch item.Type {
	case "weapon":
		return []Effect{
			{Type: EffectModifyAttackRoll, Trigger: TriggerOnAttackRoll, Modifier: item.MagicBonus},
			{Type: EffectModifyDamageRoll, Trigger: TriggerOnDamageRoll, Modifier: item.MagicBonus},
		}
	case "armor":
		return []Effect{
			{Type: EffectModifyAC, Trigger: TriggerOnAttackRoll, Modifier: item.MagicBonus},
		}
	}
	return nil
}

// magicItemPassiveEntry mirrors magicitem.passiveEffect — kept private and
// minimal to avoid importing magicitem (which would cycle).
type magicItemPassiveEntry struct {
	Type       string `json:"type"`
	Modifier   int    `json:"modifier,omitempty"`
	DamageType string `json:"damage_type,omitempty"`
	Dice       string `json:"dice,omitempty"`
}

// parseMagicItemPassiveEffects converts a magic_properties JSON array into
// combat.Effect values. Returns nil on parse error so the caller can fall back
// to the MagicBonus path. Twin of magicitem.ParsePassiveEffects +
// convertPassiveEffect — must stay in lockstep with the magicitem package.
func parseMagicItemPassiveEffects(raw string) []Effect {
	if raw == "" {
		return nil
	}
	var entries []magicItemPassiveEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil
	}
	var effects []Effect
	for _, e := range entries {
		eff, ok := convertMagicItemPassiveEntry(e)
		if !ok {
			continue
		}
		effects = append(effects, eff)
	}
	return effects
}

// convertMagicItemPassiveEntry maps a single passive-effect entry to a
// combat.Effect. Mirror of magicitem.convertPassiveEffect.
func convertMagicItemPassiveEntry(pe magicItemPassiveEntry) (Effect, bool) {
	switch pe.Type {
	case "modify_attack":
		return Effect{Type: EffectModifyAttackRoll, Trigger: TriggerOnAttackRoll, Modifier: pe.Modifier}, true
	case "modify_damage":
		return Effect{Type: EffectModifyDamageRoll, Trigger: TriggerOnDamageRoll, Modifier: pe.Modifier}, true
	case "modify_ac":
		return Effect{Type: EffectModifyAC, Trigger: TriggerOnAttackRoll, Modifier: pe.Modifier}, true
	case "modify_saving_throw":
		return Effect{Type: EffectModifySave, Trigger: TriggerOnSave, Modifier: pe.Modifier}, true
	case "resistance":
		return Effect{Type: EffectGrantResistance, Trigger: TriggerOnTakeDamage, DamageTypes: []string{pe.DamageType}}, true
	case "bonus_damage":
		return Effect{Type: EffectExtraDamageDice, Trigger: TriggerOnDamageRoll, Dice: pe.Dice, DamageTypes: []string{pe.DamageType}}, true
	case "modify_speed":
		return Effect{Type: EffectModifySpeed, Trigger: TriggerOnTurnStart, Modifier: pe.Modifier}, true
	}
	return Effect{}, false
}
