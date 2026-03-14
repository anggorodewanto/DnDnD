package refdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const (
	// SRD reference data counts.
	WeaponCount    = 37
	ArmorCount     = 13
	ConditionCount = 15
	ClassCount     = 12
	RaceCount      = 9
	FeatCount      = 41
	SpellCount     = 358
	CreatureCount  = 327
	MagicItemCount = 70
)

// SeedAll populates all SRD reference data (weapons, armor, conditions, classes, races, feats, spells, creatures, magic items).
func SeedAll(ctx context.Context, db DBTX) error {
	if db == nil {
		return fmt.Errorf("database connection must not be nil")
	}
	q := New(db)

	if err := seedWeapons(ctx, q); err != nil {
		return fmt.Errorf("seeding weapons: %w", err)
	}
	if err := seedArmor(ctx, q); err != nil {
		return fmt.Errorf("seeding armor: %w", err)
	}
	if err := seedConditions(ctx, q); err != nil {
		return fmt.Errorf("seeding conditions: %w", err)
	}
	if err := seedClasses(ctx, q); err != nil {
		return fmt.Errorf("seeding classes: %w", err)
	}
	if err := seedRaces(ctx, q); err != nil {
		return fmt.Errorf("seeding races: %w", err)
	}
	if err := seedFeats(ctx, q); err != nil {
		return fmt.Errorf("seeding feats: %w", err)
	}
	if err := seedSpells(ctx, q); err != nil {
		return fmt.Errorf("seeding spells: %w", err)
	}
	if err := seedCreatures(ctx, q); err != nil {
		return fmt.Errorf("seeding creatures: %w", err)
	}
	if err := seedMagicItems(ctx, q); err != nil {
		return fmt.Errorf("seeding magic_items: %w", err)
	}

	return nil
}

func seedEntities[P any](ctx context.Context, items []P, upsert func(context.Context, P) error, kind string) error {
	for _, item := range items {
		if err := upsert(ctx, item); err != nil {
			return fmt.Errorf("upserting %s: %w", kind, err)
		}
	}
	return nil
}

func optFloat(v float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: v, Valid: true}
}

func optInt(v int32) sql.NullInt32 {
	return sql.NullInt32{Int32: v, Valid: true}
}

func optStr(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

func optBool(v bool) sql.NullBool {
	return sql.NullBool{Bool: v, Valid: true}
}

func seedWeapons(ctx context.Context, q *Queries) error {
	weapons := []UpsertWeaponParams{
		// Simple Melee
		{ID: "club", Name: "Club", Damage: "1d4", DamageType: "bludgeoning", WeightLb: optFloat(2), Properties: []string{"light"}, WeaponType: "simple_melee"},
		{ID: "dagger", Name: "Dagger", Damage: "1d4", DamageType: "piercing", WeightLb: optFloat(1), Properties: []string{"finesse", "light", "thrown"}, RangeNormalFt: optInt(20), RangeLongFt: optInt(60), WeaponType: "simple_melee"},
		{ID: "greatclub", Name: "Greatclub", Damage: "1d8", DamageType: "bludgeoning", WeightLb: optFloat(10), Properties: []string{"two-handed"}, WeaponType: "simple_melee"},
		{ID: "handaxe", Name: "Handaxe", Damage: "1d6", DamageType: "slashing", WeightLb: optFloat(2), Properties: []string{"light", "thrown"}, RangeNormalFt: optInt(20), RangeLongFt: optInt(60), WeaponType: "simple_melee"},
		{ID: "javelin", Name: "Javelin", Damage: "1d6", DamageType: "piercing", WeightLb: optFloat(2), Properties: []string{"thrown"}, RangeNormalFt: optInt(30), RangeLongFt: optInt(120), WeaponType: "simple_melee"},
		{ID: "light-hammer", Name: "Light hammer", Damage: "1d4", DamageType: "bludgeoning", WeightLb: optFloat(2), Properties: []string{"light", "thrown"}, RangeNormalFt: optInt(20), RangeLongFt: optInt(60), WeaponType: "simple_melee"},
		{ID: "mace", Name: "Mace", Damage: "1d6", DamageType: "bludgeoning", WeightLb: optFloat(4), Properties: []string{}, WeaponType: "simple_melee"},
		{ID: "quarterstaff", Name: "Quarterstaff", Damage: "1d6", DamageType: "bludgeoning", WeightLb: optFloat(4), Properties: []string{"versatile"}, VersatileDamage: optStr("1d8"), WeaponType: "simple_melee"},
		{ID: "sickle", Name: "Sickle", Damage: "1d4", DamageType: "slashing", WeightLb: optFloat(2), Properties: []string{"light"}, WeaponType: "simple_melee"},
		{ID: "spear", Name: "Spear", Damage: "1d6", DamageType: "piercing", WeightLb: optFloat(3), Properties: []string{"thrown", "versatile"}, RangeNormalFt: optInt(20), RangeLongFt: optInt(60), VersatileDamage: optStr("1d8"), WeaponType: "simple_melee"},
		// Simple Ranged
		{ID: "light-crossbow", Name: "Light crossbow", Damage: "1d8", DamageType: "piercing", WeightLb: optFloat(5), Properties: []string{"ammunition", "loading", "two-handed"}, RangeNormalFt: optInt(80), RangeLongFt: optInt(320), WeaponType: "simple_ranged"},
		{ID: "dart", Name: "Dart", Damage: "1d4", DamageType: "piercing", WeightLb: optFloat(0.25), Properties: []string{"finesse", "thrown"}, RangeNormalFt: optInt(20), RangeLongFt: optInt(60), WeaponType: "simple_ranged"},
		{ID: "shortbow", Name: "Shortbow", Damage: "1d6", DamageType: "piercing", WeightLb: optFloat(2), Properties: []string{"ammunition", "two-handed"}, RangeNormalFt: optInt(80), RangeLongFt: optInt(320), WeaponType: "simple_ranged"},
		{ID: "sling", Name: "Sling", Damage: "1d4", DamageType: "bludgeoning", WeightLb: optFloat(0), Properties: []string{"ammunition"}, RangeNormalFt: optInt(30), RangeLongFt: optInt(120), WeaponType: "simple_ranged"},
		// Martial Melee
		{ID: "battleaxe", Name: "Battleaxe", Damage: "1d8", DamageType: "slashing", WeightLb: optFloat(4), Properties: []string{"versatile"}, VersatileDamage: optStr("1d10"), WeaponType: "martial_melee"},
		{ID: "flail", Name: "Flail", Damage: "1d8", DamageType: "bludgeoning", WeightLb: optFloat(2), Properties: []string{}, WeaponType: "martial_melee"},
		{ID: "glaive", Name: "Glaive", Damage: "1d10", DamageType: "slashing", WeightLb: optFloat(6), Properties: []string{"heavy", "reach", "two-handed"}, WeaponType: "martial_melee"},
		{ID: "greataxe", Name: "Greataxe", Damage: "1d12", DamageType: "slashing", WeightLb: optFloat(7), Properties: []string{"heavy", "two-handed"}, WeaponType: "martial_melee"},
		{ID: "greatsword", Name: "Greatsword", Damage: "2d6", DamageType: "slashing", WeightLb: optFloat(6), Properties: []string{"heavy", "two-handed"}, WeaponType: "martial_melee"},
		{ID: "halberd", Name: "Halberd", Damage: "1d10", DamageType: "slashing", WeightLb: optFloat(6), Properties: []string{"heavy", "reach", "two-handed"}, WeaponType: "martial_melee"},
		{ID: "lance", Name: "Lance", Damage: "1d12", DamageType: "piercing", WeightLb: optFloat(6), Properties: []string{"reach", "special"}, WeaponType: "martial_melee"},
		{ID: "longsword", Name: "Longsword", Damage: "1d8", DamageType: "slashing", WeightLb: optFloat(3), Properties: []string{"versatile"}, VersatileDamage: optStr("1d10"), WeaponType: "martial_melee"},
		{ID: "maul", Name: "Maul", Damage: "2d6", DamageType: "bludgeoning", WeightLb: optFloat(10), Properties: []string{"heavy", "two-handed"}, WeaponType: "martial_melee"},
		{ID: "morningstar", Name: "Morningstar", Damage: "1d8", DamageType: "piercing", WeightLb: optFloat(4), Properties: []string{}, WeaponType: "martial_melee"},
		{ID: "pike", Name: "Pike", Damage: "1d10", DamageType: "piercing", WeightLb: optFloat(18), Properties: []string{"heavy", "reach", "two-handed"}, WeaponType: "martial_melee"},
		{ID: "rapier", Name: "Rapier", Damage: "1d8", DamageType: "piercing", WeightLb: optFloat(2), Properties: []string{"finesse"}, WeaponType: "martial_melee"},
		{ID: "scimitar", Name: "Scimitar", Damage: "1d6", DamageType: "slashing", WeightLb: optFloat(3), Properties: []string{"finesse", "light"}, WeaponType: "martial_melee"},
		{ID: "shortsword", Name: "Shortsword", Damage: "1d6", DamageType: "piercing", WeightLb: optFloat(2), Properties: []string{"finesse", "light"}, WeaponType: "martial_melee"},
		{ID: "trident", Name: "Trident", Damage: "1d6", DamageType: "piercing", WeightLb: optFloat(4), Properties: []string{"thrown", "versatile"}, RangeNormalFt: optInt(20), RangeLongFt: optInt(60), VersatileDamage: optStr("1d8"), WeaponType: "martial_melee"},
		{ID: "war-pick", Name: "War pick", Damage: "1d8", DamageType: "piercing", WeightLb: optFloat(2), Properties: []string{}, WeaponType: "martial_melee"},
		{ID: "warhammer", Name: "Warhammer", Damage: "1d8", DamageType: "bludgeoning", WeightLb: optFloat(2), Properties: []string{"versatile"}, VersatileDamage: optStr("1d10"), WeaponType: "martial_melee"},
		{ID: "whip", Name: "Whip", Damage: "1d4", DamageType: "slashing", WeightLb: optFloat(3), Properties: []string{"finesse", "reach"}, WeaponType: "martial_melee"},
		// Martial Ranged
		{ID: "blowgun", Name: "Blowgun", Damage: "1", DamageType: "piercing", WeightLb: optFloat(1), Properties: []string{"ammunition", "loading"}, RangeNormalFt: optInt(25), RangeLongFt: optInt(100), WeaponType: "martial_ranged"},
		{ID: "hand-crossbow", Name: "Hand crossbow", Damage: "1d6", DamageType: "piercing", WeightLb: optFloat(3), Properties: []string{"ammunition", "light", "loading"}, RangeNormalFt: optInt(30), RangeLongFt: optInt(120), WeaponType: "martial_ranged"},
		{ID: "heavy-crossbow", Name: "Heavy crossbow", Damage: "1d10", DamageType: "piercing", WeightLb: optFloat(18), Properties: []string{"ammunition", "heavy", "loading", "two-handed"}, RangeNormalFt: optInt(100), RangeLongFt: optInt(400), WeaponType: "martial_ranged"},
		{ID: "longbow", Name: "Longbow", Damage: "1d8", DamageType: "piercing", WeightLb: optFloat(2), Properties: []string{"ammunition", "heavy", "two-handed"}, RangeNormalFt: optInt(150), RangeLongFt: optInt(600), WeaponType: "martial_ranged"},
		{ID: "net", Name: "Net", Damage: "0", DamageType: "none", WeightLb: optFloat(3), Properties: []string{"special", "thrown"}, RangeNormalFt: optInt(5), RangeLongFt: optInt(15), WeaponType: "martial_ranged"},
	}

	return seedEntities(ctx, weapons, q.UpsertWeapon, "weapon")
}

func seedArmor(ctx context.Context, q *Queries) error {
	armor := []UpsertArmorParams{
		// Light
		{ID: "padded", Name: "Padded", AcBase: 11, AcDexBonus: optBool(true), ArmorType: "light", WeightLb: optFloat(8), StealthDisadv: optBool(true)},
		{ID: "leather", Name: "Leather", AcBase: 11, AcDexBonus: optBool(true), ArmorType: "light", WeightLb: optFloat(10)},
		{ID: "studded-leather", Name: "Studded leather", AcBase: 12, AcDexBonus: optBool(true), ArmorType: "light", WeightLb: optFloat(13)},
		// Medium
		{ID: "hide", Name: "Hide", AcBase: 12, AcDexBonus: optBool(true), AcDexMax: optInt(2), ArmorType: "medium", WeightLb: optFloat(12)},
		{ID: "chain-shirt", Name: "Chain shirt", AcBase: 13, AcDexBonus: optBool(true), AcDexMax: optInt(2), ArmorType: "medium", WeightLb: optFloat(20)},
		{ID: "scale-mail", Name: "Scale mail", AcBase: 14, AcDexBonus: optBool(true), AcDexMax: optInt(2), ArmorType: "medium", WeightLb: optFloat(45), StealthDisadv: optBool(true)},
		{ID: "breastplate", Name: "Breastplate", AcBase: 14, AcDexBonus: optBool(true), AcDexMax: optInt(2), ArmorType: "medium", WeightLb: optFloat(20)},
		{ID: "half-plate", Name: "Half plate", AcBase: 15, AcDexBonus: optBool(true), AcDexMax: optInt(2), ArmorType: "medium", WeightLb: optFloat(40), StealthDisadv: optBool(true)},
		// Heavy
		{ID: "ring-mail", Name: "Ring mail", AcBase: 14, AcDexBonus: optBool(false), ArmorType: "heavy", WeightLb: optFloat(40), StealthDisadv: optBool(true)},
		{ID: "chain-mail", Name: "Chain mail", AcBase: 16, AcDexBonus: optBool(false), StrengthReq: optInt(13), ArmorType: "heavy", WeightLb: optFloat(55), StealthDisadv: optBool(true)},
		{ID: "splint", Name: "Splint", AcBase: 17, AcDexBonus: optBool(false), StrengthReq: optInt(15), ArmorType: "heavy", WeightLb: optFloat(60), StealthDisadv: optBool(true)},
		{ID: "plate", Name: "Plate", AcBase: 18, AcDexBonus: optBool(false), StrengthReq: optInt(15), ArmorType: "heavy", WeightLb: optFloat(65), StealthDisadv: optBool(true)},
		// Shield
		{ID: "shield", Name: "Shield", AcBase: 2, AcDexBonus: optBool(false), ArmorType: "shield", WeightLb: optFloat(6)},
	}

	return seedEntities(ctx, armor, q.UpsertArmor, "armor")
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return b
}

// MechanicalEffect represents a single mechanical effect of a condition.
type MechanicalEffect struct {
	EffectType  string `json:"effect_type"`
	Description string `json:"description,omitempty"`
	Target      string `json:"target,omitempty"`
	Condition   string `json:"condition,omitempty"`
	Value       string `json:"value,omitempty"`
}

func seedConditions(ctx context.Context, q *Queries) error {
	conditions := []UpsertConditionParams{
		{
			ID: "blinded", Name: "Blinded",
			Description: "A blinded creature can't see and automatically fails any ability check that requires sight. Attack rolls against the creature have advantage, and the creature's attack rolls have disadvantage.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "cant_see"},
				{EffectType: "auto_fail_ability_check", Condition: "requires_sight"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against"},
				{EffectType: "impose_disadvantage", Target: "attack_rolls"},
			}),
		},
		{
			ID: "charmed", Name: "Charmed",
			Description: "A charmed creature can't attack the charmer or target the charmer with harmful abilities or magical effects. The charmer has advantage on any ability check to interact socially with the creature.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "cant_attack", Target: "charmer"},
				{EffectType: "cant_target_harmful", Target: "charmer"},
				{EffectType: "grant_advantage", Target: "social_ability_checks", Condition: "charmer_interacting"},
			}),
		},
		{
			ID: "deafened", Name: "Deafened",
			Description: "A deafened creature can't hear and automatically fails any ability check that requires hearing.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "cant_hear"},
				{EffectType: "auto_fail_ability_check", Condition: "requires_hearing"},
			}),
		},
		{
			ID: "exhaustion", Name: "Exhaustion",
			Description: "Exhaustion is measured in six levels. An effect can give a creature one or more levels of exhaustion. Level 1: Disadvantage on ability checks. Level 2: Speed halved. Level 3: Disadvantage on attack rolls and saving throws. Level 4: Hit point maximum halved. Level 5: Speed reduced to 0. Level 6: Death.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "exhaustion_level", Value: "1", Description: "Disadvantage on ability checks"},
				{EffectType: "exhaustion_level", Value: "2", Description: "Speed halved"},
				{EffectType: "exhaustion_level", Value: "3", Description: "Disadvantage on attack rolls and saving throws"},
				{EffectType: "exhaustion_level", Value: "4", Description: "Hit point maximum halved"},
				{EffectType: "exhaustion_level", Value: "5", Description: "Speed reduced to 0"},
				{EffectType: "exhaustion_level", Value: "6", Description: "Death"},
			}),
		},
		{
			ID: "frightened", Name: "Frightened",
			Description: "A frightened creature has disadvantage on ability checks and attack rolls while the source of its fear is within line of sight. The creature can't willingly move closer to the source of its fear.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "impose_disadvantage", Target: "ability_checks", Condition: "source_in_sight"},
				{EffectType: "impose_disadvantage", Target: "attack_rolls", Condition: "source_in_sight"},
				{EffectType: "cant_move_closer", Target: "fear_source"},
			}),
		},
		{
			ID: "grappled", Name: "Grappled",
			Description: "A grappled creature's speed becomes 0, and it can't benefit from any bonus to its speed. The condition ends if the grappler is incapacitated or if an effect removes the grappled creature from the reach of the grappler.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "set_speed", Value: "0"},
				{EffectType: "no_speed_bonus"},
			}),
		},
		{
			ID: "incapacitated", Name: "Incapacitated",
			Description: "An incapacitated creature can't take actions or reactions.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "cant_take_actions"},
				{EffectType: "cant_take_reactions"},
			}),
		},
		{
			ID: "invisible", Name: "Invisible",
			Description: "An invisible creature is impossible to see without the aid of magic or a special sense. For the purpose of hiding, the creature is heavily obscured. The creature's location can be detected by any noise it makes or any tracks it leaves. Attack rolls against the creature have disadvantage, and the creature's attack rolls have advantage.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "cant_be_seen"},
				{EffectType: "heavily_obscured"},
				{EffectType: "grant_disadvantage", Target: "attack_rolls_against"},
				{EffectType: "grant_advantage", Target: "attack_rolls"},
			}),
		},
		{
			ID: "paralyzed", Name: "Paralyzed",
			Description: "A paralyzed creature is incapacitated and can't move or speak. The creature automatically fails Strength and Dexterity saving throws. Attack rolls against the creature have advantage. Any attack that hits the creature is a critical hit if the attacker is within 5 feet of the creature.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "incapacitated"},
				{EffectType: "cant_move"},
				{EffectType: "cant_speak"},
				{EffectType: "auto_fail_saving_throw", Target: "strength"},
				{EffectType: "auto_fail_saving_throw", Target: "dexterity"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against"},
				{EffectType: "auto_crit", Condition: "attacker_within_5ft"},
			}),
		},
		{
			ID: "petrified", Name: "Petrified",
			Description: "A petrified creature is transformed, along with any nonmagical object it is wearing or carrying, into a solid inanimate substance. Its weight increases by a factor of ten, and it ceases aging. The creature is incapacitated, can't move or speak, and is unaware of its surroundings. Attack rolls against the creature have advantage. The creature automatically fails Strength and Dexterity saving throws. The creature has resistance to all damage. The creature is immune to poison and disease.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "incapacitated"},
				{EffectType: "cant_move"},
				{EffectType: "cant_speak"},
				{EffectType: "unaware"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against"},
				{EffectType: "auto_fail_saving_throw", Target: "strength"},
				{EffectType: "auto_fail_saving_throw", Target: "dexterity"},
				{EffectType: "resistance", Target: "all_damage"},
				{EffectType: "immunity", Target: "poison"},
				{EffectType: "immunity", Target: "disease"},
			}),
		},
		{
			ID: "poisoned", Name: "Poisoned",
			Description: "A poisoned creature has disadvantage on attack rolls and ability checks.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "impose_disadvantage", Target: "attack_rolls"},
				{EffectType: "impose_disadvantage", Target: "ability_checks"},
			}),
		},
		{
			ID: "prone", Name: "Prone",
			Description: "A prone creature's only movement option is to crawl, unless it stands up and thereby ends the condition. The creature has disadvantage on attack rolls. An attack roll against the creature has advantage if the attacker is within 5 feet of the creature. Otherwise, the attack roll has disadvantage.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "movement_crawl_only"},
				{EffectType: "impose_disadvantage", Target: "attack_rolls"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against", Condition: "attacker_within_5ft"},
				{EffectType: "grant_disadvantage", Target: "attack_rolls_against", Condition: "attacker_beyond_5ft"},
			}),
		},
		{
			ID: "restrained", Name: "Restrained",
			Description: "A restrained creature's speed becomes 0, and it can't benefit from any bonus to its speed. Attack rolls against the creature have advantage, and the creature's attack rolls have disadvantage. The creature has disadvantage on Dexterity saving throws.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "set_speed", Value: "0"},
				{EffectType: "no_speed_bonus"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against"},
				{EffectType: "impose_disadvantage", Target: "attack_rolls"},
				{EffectType: "impose_disadvantage", Target: "dexterity_saving_throws"},
			}),
		},
		{
			ID: "stunned", Name: "Stunned",
			Description: "A stunned creature is incapacitated, can't move, and can speak only falteringly. The creature automatically fails Strength and Dexterity saving throws. Attack rolls against the creature have advantage.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "incapacitated"},
				{EffectType: "cant_move"},
				{EffectType: "speak_falteringly"},
				{EffectType: "auto_fail_saving_throw", Target: "strength"},
				{EffectType: "auto_fail_saving_throw", Target: "dexterity"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against"},
			}),
		},
		{
			ID: "unconscious", Name: "Unconscious",
			Description: "An unconscious creature is incapacitated, can't move or speak, and is unaware of its surroundings. The creature drops whatever it's holding and falls prone. The creature automatically fails Strength and Dexterity saving throws. Attack rolls against the creature have advantage. Any attack that hits the creature is a critical hit if the attacker is within 5 feet of the creature.",
			MechanicalEffects: mustJSON([]MechanicalEffect{
				{EffectType: "incapacitated"},
				{EffectType: "cant_move"},
				{EffectType: "cant_speak"},
				{EffectType: "unaware"},
				{EffectType: "drop_held_items"},
				{EffectType: "fall_prone"},
				{EffectType: "auto_fail_saving_throw", Target: "strength"},
				{EffectType: "auto_fail_saving_throw", Target: "dexterity"},
				{EffectType: "grant_advantage", Target: "attack_rolls_against"},
				{EffectType: "auto_crit", Condition: "attacker_within_5ft"},
			}),
		},
	}

	return seedEntities(ctx, conditions, q.UpsertCondition, "condition")
}
