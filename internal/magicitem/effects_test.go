package magicitem

import (
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/combat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMagicArmorEffects_PlusTwoArmor(t *testing.T) {
	item := character.InventoryItem{
		ItemID:     "armor-plus-2",
		Name:       "+2 Chain Mail",
		Type:       "armor",
		IsMagic:    true,
		MagicBonus: 2,
		Equipped:   true,
	}

	features := ItemFeatures(item)
	require.Len(t, features, 1)

	f := features[0]
	assert.Equal(t, "+2 Chain Mail", f.Name)
	assert.Equal(t, "magic_item", f.Source)
	require.Len(t, f.Effects, 1)

	assert.Equal(t, combat.EffectModifyAC, f.Effects[0].Type)
	assert.Equal(t, 2, f.Effects[0].Modifier)
}

func TestItemFeatures_NonMagicReturnsNil(t *testing.T) {
	item := character.InventoryItem{
		ItemID: "longsword",
		Name:   "Longsword",
		Type:   "weapon",
	}

	features := ItemFeatures(item)
	assert.Nil(t, features)
}

func TestItemFeatures_UnequippedReturnsNil(t *testing.T) {
	item := character.InventoryItem{
		ItemID:     "longsword-plus-1",
		Name:       "+1 Longsword",
		Type:       "weapon",
		IsMagic:    true,
		MagicBonus: 1,
		Equipped:   false,
	}

	features := ItemFeatures(item)
	assert.Nil(t, features)
}

func TestParsePassiveEffects_ModifyACAndSave(t *testing.T) {
	raw := `[{"type": "modify_ac", "modifier": 1}, {"type": "modify_saving_throw", "modifier": 1}]`

	effects, err := ParsePassiveEffects(raw)
	require.NoError(t, err)
	require.Len(t, effects, 2)

	assert.Equal(t, combat.EffectModifyAC, effects[0].Type)
	assert.Equal(t, combat.TriggerOnAttackRoll, effects[0].Trigger)
	assert.Equal(t, 1, effects[0].Modifier)

	assert.Equal(t, combat.EffectModifySave, effects[1].Type)
	assert.Equal(t, combat.TriggerOnSave, effects[1].Trigger)
	assert.Equal(t, 1, effects[1].Modifier)
}

func TestParsePassiveEffects_Resistance(t *testing.T) {
	raw := `[{"type": "resistance", "damage_type": "fire"}]`

	effects, err := ParsePassiveEffects(raw)
	require.NoError(t, err)
	require.Len(t, effects, 1)

	assert.Equal(t, combat.EffectGrantResistance, effects[0].Type)
	assert.Equal(t, combat.TriggerOnTakeDamage, effects[0].Trigger)
	assert.Equal(t, []string{"fire"}, effects[0].DamageTypes)
}

func TestParsePassiveEffects_AttackAndDamage(t *testing.T) {
	raw := `[{"type": "modify_attack", "modifier": 2}, {"type": "modify_damage", "modifier": 2}]`

	effects, err := ParsePassiveEffects(raw)
	require.NoError(t, err)
	require.Len(t, effects, 2)

	assert.Equal(t, combat.EffectModifyAttackRoll, effects[0].Type)
	assert.Equal(t, combat.TriggerOnAttackRoll, effects[0].Trigger)
	assert.Equal(t, 2, effects[0].Modifier)

	assert.Equal(t, combat.EffectModifyDamageRoll, effects[1].Type)
	assert.Equal(t, combat.TriggerOnDamageRoll, effects[1].Trigger)
	assert.Equal(t, 2, effects[1].Modifier)
}

func TestParsePassiveEffects_EmptyString(t *testing.T) {
	effects, err := ParsePassiveEffects("")
	require.NoError(t, err)
	assert.Nil(t, effects)
}

func TestParsePassiveEffects_InvalidJSON(t *testing.T) {
	_, err := ParsePassiveEffects("{bad json")
	assert.Error(t, err)
}

func TestCollectItemFeatures_SkipsUnattuned(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:             "cloak-of-protection",
			Name:               "Cloak of Protection",
			Type:               "magic_item",
			IsMagic:            true,
			Equipped:           true,
			RequiresAttunement: true,
			MagicProperties:    `[{"type": "modify_ac", "modifier": 1}, {"type": "modify_saving_throw", "modifier": 1}]`,
		},
	}
	attunement := []character.AttunementSlot{} // not attuned

	features := CollectItemFeatures(items, attunement)
	assert.Empty(t, features) // requires attunement but not attuned
}

func TestCollectItemFeatures_IncludesAttuned(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:             "cloak-of-protection",
			Name:               "Cloak of Protection",
			Type:               "magic_item",
			IsMagic:            true,
			Equipped:           true,
			RequiresAttunement: true,
			MagicProperties:    `[{"type": "modify_ac", "modifier": 1}, {"type": "modify_saving_throw", "modifier": 1}]`,
		},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	}

	features := CollectItemFeatures(items, attunement)
	require.Len(t, features, 1)
	assert.Equal(t, "Cloak of Protection", features[0].Name)
	assert.Len(t, features[0].Effects, 2)
}

func TestCollectItemFeatures_WeaponBonusAndPassiveEffects(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:          "longsword-plus-1",
			Name:            "+1 Longsword",
			Type:            "weapon",
			IsMagic:         true,
			MagicBonus:      1,
			Equipped:        true,
			MagicProperties: `[{"type": "modify_attack", "modifier": 1}, {"type": "modify_damage", "modifier": 1}]`,
		},
	}

	features := CollectItemFeatures(items, nil)
	// Should get the bonus feature (from MagicBonus) but NOT duplicate from passive effects
	// when passive effects overlap with magic bonus. The approach: use passive effects if available,
	// otherwise fall back to MagicBonus.
	require.Len(t, features, 1)
	assert.Equal(t, "+1 Longsword", features[0].Name)
}

func TestCollectItemFeatures_NoAttunementRequired(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:     "armor-plus-1",
			Name:       "+1 Chain Mail",
			Type:       "armor",
			IsMagic:    true,
			MagicBonus: 1,
			Equipped:   true,
		},
	}

	features := CollectItemFeatures(items, nil)
	require.Len(t, features, 1)
	assert.Equal(t, "+1 Chain Mail", features[0].Name)
}

func TestIntegration_MagicWeaponBonusAppliedToAttack(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:     "longsword-plus-2",
			Name:       "+2 Longsword",
			Type:       "weapon",
			IsMagic:    true,
			MagicBonus: 2,
			Equipped:   true,
		},
	}

	features := CollectItemFeatures(items, nil)
	result := combat.ProcessEffects(features, combat.TriggerOnAttackRoll, combat.EffectContext{})
	assert.Equal(t, 2, result.FlatModifier)

	dmgResult := combat.ProcessEffects(features, combat.TriggerOnDamageRoll, combat.EffectContext{})
	assert.Equal(t, 2, dmgResult.FlatModifier)
}

func TestIntegration_MagicArmorBonusAppliedToAC(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:     "armor-plus-1",
			Name:       "+1 Chain Mail",
			Type:       "armor",
			IsMagic:    true,
			MagicBonus: 1,
			Equipped:   true,
		},
	}

	features := CollectItemFeatures(items, nil)
	result := combat.ProcessEffects(features, combat.TriggerOnAttackRoll, combat.EffectContext{})
	assert.Equal(t, 1, result.ACModifier)
}

func TestIntegration_CloakOfProtection(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:             "cloak-of-protection",
			Name:               "Cloak of Protection",
			Type:               "magic_item",
			IsMagic:            true,
			Equipped:           true,
			RequiresAttunement: true,
			MagicProperties:    `[{"type": "modify_ac", "modifier": 1}, {"type": "modify_saving_throw", "modifier": 1}]`,
		},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "cloak-of-protection", Name: "Cloak of Protection"},
	}

	features := CollectItemFeatures(items, attunement)

	acResult := combat.ProcessEffects(features, combat.TriggerOnAttackRoll, combat.EffectContext{})
	assert.Equal(t, 1, acResult.ACModifier)

	saveResult := combat.ProcessEffects(features, combat.TriggerOnSave, combat.EffectContext{})
	assert.Equal(t, 1, saveResult.FlatModifier)
}

func TestIntegration_RingOfResistance(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:             "ring-of-fire-resistance",
			Name:               "Ring of Fire Resistance",
			Type:               "magic_item",
			IsMagic:            true,
			Equipped:           true,
			RequiresAttunement: true,
			MagicProperties:    `[{"type": "resistance", "damage_type": "fire"}]`,
		},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "ring-of-fire-resistance", Name: "Ring of Fire Resistance"},
	}

	features := CollectItemFeatures(items, attunement)
	result := combat.ProcessEffects(features, combat.TriggerOnTakeDamage, combat.EffectContext{})
	assert.Contains(t, result.Resistances, "fire")
}

func TestIntegration_FrostBrandPassiveEffects(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:             "frost-brand",
			Name:               "Frost Brand",
			Type:               "weapon",
			IsMagic:            true,
			Equipped:           true,
			RequiresAttunement: true,
			MagicProperties:    `[{"type": "bonus_damage", "damage_type": "cold", "dice": "1d6"}, {"type": "resistance", "damage_type": "fire"}]`,
		},
	}
	attunement := []character.AttunementSlot{
		{ItemID: "frost-brand", Name: "Frost Brand"},
	}

	features := CollectItemFeatures(items, attunement)

	dmgResult := combat.ProcessEffects(features, combat.TriggerOnDamageRoll, combat.EffectContext{})
	assert.Contains(t, dmgResult.ExtraDice, "1d6")

	resResult := combat.ProcessEffects(features, combat.TriggerOnTakeDamage, combat.EffectContext{})
	assert.Contains(t, resResult.Resistances, "fire")
}

func TestParsePassiveEffects_BonusDamage(t *testing.T) {
	raw := `[{"type": "bonus_damage", "damage_type": "cold", "dice": "1d6"}]`

	effects, err := ParsePassiveEffects(raw)
	require.NoError(t, err)
	require.Len(t, effects, 1)

	assert.Equal(t, combat.EffectExtraDamageDice, effects[0].Type)
	assert.Equal(t, combat.TriggerOnDamageRoll, effects[0].Trigger)
	assert.Equal(t, "1d6", effects[0].Dice)
	assert.Equal(t, []string{"cold"}, effects[0].DamageTypes)
}

func TestParsePassiveEffects_ModifySpeed(t *testing.T) {
	raw := `[{"type": "modify_speed", "modifier": 10}]`

	effects, err := ParsePassiveEffects(raw)
	require.NoError(t, err)
	require.Len(t, effects, 1)

	assert.Equal(t, combat.EffectModifySpeed, effects[0].Type)
	assert.Equal(t, combat.TriggerOnTurnStart, effects[0].Trigger)
	assert.Equal(t, 10, effects[0].Modifier)
}

func TestParsePassiveEffects_UnknownTypeSkipped(t *testing.T) {
	raw := `[{"type": "bonus_hp", "modifier": 1, "per_level": true}, {"type": "modify_ac", "modifier": 1}]`

	effects, err := ParsePassiveEffects(raw)
	require.NoError(t, err)
	require.Len(t, effects, 1) // bonus_hp is skipped
	assert.Equal(t, combat.EffectModifyAC, effects[0].Type)
}

func TestItemFeatures_PlusThreeWeapon(t *testing.T) {
	item := character.InventoryItem{
		ItemID:     "longsword-plus-3",
		Name:       "+3 Longsword",
		Type:       "weapon",
		IsMagic:    true,
		MagicBonus: 3,
		Equipped:   true,
	}

	features := ItemFeatures(item)
	require.Len(t, features, 1)
	assert.Equal(t, 3, features[0].Effects[0].Modifier)
	assert.Equal(t, 3, features[0].Effects[1].Modifier)
}

func TestItemFeatures_MagicItemTypeWithBonus(t *testing.T) {
	// magic_item type with a bonus should not produce weapon/armor effects
	item := character.InventoryItem{
		ItemID:     "some-ring",
		Name:       "Ring of Something",
		Type:       "magic_item",
		IsMagic:    true,
		MagicBonus: 1,
		Equipped:   true,
	}

	features := ItemFeatures(item)
	assert.Empty(t, features) // magic_item type with bonus doesn't match weapon/armor
}

func TestItemFeatures_ZeroMagicBonus(t *testing.T) {
	item := character.InventoryItem{
		ItemID:   "flame-tongue",
		Name:     "Flame Tongue",
		Type:     "weapon",
		IsMagic:  true,
		Equipped: true,
	}

	features := ItemFeatures(item)
	assert.Empty(t, features) // no magic bonus, no features from ItemFeatures
}

func TestCollectItemFeatures_MultipleItems(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:     "longsword-plus-1",
			Name:       "+1 Longsword",
			Type:       "weapon",
			IsMagic:    true,
			MagicBonus: 1,
			Equipped:   true,
		},
		{
			ItemID:     "armor-plus-1",
			Name:       "+1 Chain Mail",
			Type:       "armor",
			IsMagic:    true,
			MagicBonus: 1,
			Equipped:   true,
		},
		{
			ItemID: "rope",
			Name:   "Rope",
			Type:   "other",
		},
	}

	features := CollectItemFeatures(items, nil)
	require.Len(t, features, 2)
}

func TestCollectItemFeatures_PrefersMagicPropertiesOverBonus(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:          "longsword-plus-1",
			Name:            "+1 Longsword",
			Type:            "weapon",
			IsMagic:         true,
			MagicBonus:      1,
			Equipped:        true,
			MagicProperties: `[{"type": "modify_attack", "modifier": 1}, {"type": "modify_damage", "modifier": 1}]`,
		},
	}

	features := CollectItemFeatures(items, nil)
	require.Len(t, features, 1)
	assert.Equal(t, "magic_item", features[0].Source)
	assert.Len(t, features[0].Effects, 2)
}

func TestCollectItemFeatures_EmptySlice(t *testing.T) {
	features := CollectItemFeatures(nil, nil)
	assert.Empty(t, features)
}

func TestCollectItemFeatures_InvalidMagicPropertiesFallsBackToBonus(t *testing.T) {
	items := []character.InventoryItem{
		{
			ItemID:          "longsword-plus-1",
			Name:            "+1 Longsword",
			Type:            "weapon",
			IsMagic:         true,
			MagicBonus:      1,
			Equipped:        true,
			MagicProperties: `{bad json`,
		},
	}

	features := CollectItemFeatures(items, nil)
	require.Len(t, features, 1) // falls back to MagicBonus
	assert.Equal(t, "+1 Longsword", features[0].Name)
}

func TestMagicWeaponEffects_PlusOneWeapon(t *testing.T) {
	item := character.InventoryItem{
		ItemID:     "longsword-plus-1",
		Name:       "+1 Longsword",
		Type:       "weapon",
		IsMagic:    true,
		MagicBonus: 1,
		Equipped:   true,
	}

	features := ItemFeatures(item)
	require.Len(t, features, 1)

	f := features[0]
	assert.Equal(t, "+1 Longsword", f.Name)
	assert.Equal(t, "magic_item", f.Source)
	require.Len(t, f.Effects, 2)

	// Attack bonus
	assert.Equal(t, combat.EffectModifyAttackRoll, f.Effects[0].Type)
	assert.Equal(t, combat.TriggerOnAttackRoll, f.Effects[0].Trigger)
	assert.Equal(t, 1, f.Effects[0].Modifier)

	// Damage bonus
	assert.Equal(t, combat.EffectModifyDamageRoll, f.Effects[1].Type)
	assert.Equal(t, combat.TriggerOnDamageRoll, f.Effects[1].Trigger)
	assert.Equal(t, 1, f.Effects[1].Modifier)
}

func TestParsePassiveEffects_ModifySaveAlias(t *testing.T) {
	canonical := `[{"type": "modify_saving_throw", "modifier": 2}]`
	alias := `[{"type": "modify_save", "modifier": 2}]`

	canonicalEffects, err := ParsePassiveEffects(canonical)
	require.NoError(t, err)

	aliasEffects, err := ParsePassiveEffects(alias)
	require.NoError(t, err)
	require.Len(t, aliasEffects, 1, `"modify_save" should be recognized`)

	assert.Equal(t, canonicalEffects, aliasEffects)
}

func TestParsePassiveEffects_GrantResistanceAlias(t *testing.T) {
	canonical := `[{"type": "resistance", "damage_type": "fire"}]`
	alias := `[{"type": "grant_resistance", "damage_type": "fire"}]`

	canonicalEffects, err := ParsePassiveEffects(canonical)
	require.NoError(t, err)

	aliasEffects, err := ParsePassiveEffects(alias)
	require.NoError(t, err)
	require.Len(t, aliasEffects, 1, `"grant_resistance" should be recognized`)

	assert.Equal(t, canonicalEffects, aliasEffects)
}
