package magicitem

import (
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
)

// ItemFeatures converts a magic inventory item's MagicBonus into
// FeatureDefinition structs for the Feature Effect System.
// Returns nil for non-magic, unequipped, or zero-bonus items.
func ItemFeatures(item character.InventoryItem) []combat.FeatureDefinition {
	if !item.IsMagic || !item.Equipped || item.MagicBonus == 0 {
		return nil
	}

	var effects []combat.Effect
	switch item.Type {
	case "weapon":
		effects = []combat.Effect{
			{Type: combat.EffectModifyAttackRoll, Trigger: combat.TriggerOnAttackRoll, Modifier: item.MagicBonus},
			{Type: combat.EffectModifyDamageRoll, Trigger: combat.TriggerOnDamageRoll, Modifier: item.MagicBonus},
		}
	case "armor":
		effects = []combat.Effect{
			{Type: combat.EffectModifyAC, Trigger: combat.TriggerOnAttackRoll, Modifier: item.MagicBonus},
		}
	default:
		return nil
	}

	return []combat.FeatureDefinition{{
		Name:    item.Name,
		Source:  "magic_item",
		Effects: effects,
	}}
}

// CollectItemFeatures gathers FeatureDefinitions from all equipped magic items.
// Items that require attunement are only included if they appear in attunement slots.
// If an item has MagicProperties (passive effects JSON), those are used.
// Otherwise, MagicBonus is converted to effects based on item type.
func CollectItemFeatures(items []character.InventoryItem, attunement []character.AttunementSlot) []combat.FeatureDefinition {
	attunedSet := make(map[string]bool, len(attunement))
	for _, a := range attunement {
		attunedSet[a.ItemID] = true
	}

	var features []combat.FeatureDefinition
	for _, item := range items {
		if !item.IsMagic || !item.Equipped {
			continue
		}
		if item.RequiresAttunement && !attunedSet[item.ItemID] {
			continue
		}

		// Prefer passive effects JSON if available
		if item.MagicProperties != "" {
			effects, err := ParsePassiveEffects(item.MagicProperties)
			if err == nil && len(effects) > 0 {
				features = append(features, combat.FeatureDefinition{
					Name:    item.Name,
					Source:  "magic_item",
					Effects: effects,
				})
				continue
			}
		}

		// Fall back to MagicBonus-based effects
		itemFeatures := ItemFeatures(item)
		features = append(features, itemFeatures...)
	}
	return features
}

// passiveEffect represents a single passive effect entry in the magic_items JSONB.
type passiveEffect struct {
	Type       string `json:"type"`
	Modifier   int    `json:"modifier,omitempty"`
	DamageType string `json:"damage_type,omitempty"`
	Dice       string `json:"dice,omitempty"`
}

// ParsePassiveEffects parses a JSON array of passive effects from the magic_items table
// into combat.Effect structs suitable for inclusion in a FeatureDefinition.
func ParsePassiveEffects(raw string) ([]combat.Effect, error) {
	if raw == "" {
		return nil, nil
	}

	var entries []passiveEffect
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("parsing passive effects: %w", err)
	}

	var effects []combat.Effect
	for _, e := range entries {
		effect, ok := convertPassiveEffect(e)
		if !ok {
			continue
		}
		effects = append(effects, effect)
	}
	return effects, nil
}

// convertPassiveEffect maps a passive effect JSON entry to a combat.Effect.
func convertPassiveEffect(pe passiveEffect) (combat.Effect, bool) {
	switch pe.Type {
	case "modify_attack":
		return combat.Effect{
			Type:     combat.EffectModifyAttackRoll,
			Trigger:  combat.TriggerOnAttackRoll,
			Modifier: pe.Modifier,
		}, true
	case "modify_damage":
		return combat.Effect{
			Type:     combat.EffectModifyDamageRoll,
			Trigger:  combat.TriggerOnDamageRoll,
			Modifier: pe.Modifier,
		}, true
	case "modify_ac":
		return combat.Effect{
			Type:     combat.EffectModifyAC,
			Trigger:  combat.TriggerOnAttackRoll,
			Modifier: pe.Modifier,
		}, true
	case "modify_saving_throw", "modify_save":
		return combat.Effect{
			Type:     combat.EffectModifySave,
			Trigger:  combat.TriggerOnSave,
			Modifier: pe.Modifier,
		}, true
	case "resistance", "grant_resistance":
		return combat.Effect{
			Type:        combat.EffectGrantResistance,
			Trigger:     combat.TriggerOnTakeDamage,
			DamageTypes: []string{pe.DamageType},
		}, true
	case "bonus_damage":
		return combat.Effect{
			Type:        combat.EffectExtraDamageDice,
			Trigger:     combat.TriggerOnDamageRoll,
			Dice:        pe.Dice,
			DamageTypes: []string{pe.DamageType},
		}, true
	case "modify_speed":
		return combat.Effect{
			Type:     combat.EffectModifySpeed,
			Trigger:  combat.TriggerOnTurnStart,
			Modifier: pe.Modifier,
		}, true
	default:
		return combat.Effect{}, false
	}
}
