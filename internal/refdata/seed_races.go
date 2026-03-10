package refdata

import (
	"context"
)

func seedRaces(ctx context.Context, q *Queries) error {
	races := []UpsertRaceParams{
		{
			ID: "dwarf", Name: "Dwarf", SpeedFt: 25, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]int{"con": 2}),
			DarkvisionFt:   60,
			Traits: mustJSON([]map[string]string{
				{"name": "Dwarven Resilience", "description": "You have advantage on saving throws against poison, and you have resistance against poison damage.", "mechanical_effect": "advantage_saves_poison,resistance_poison"},
				{"name": "Dwarven Combat Training", "description": "You have proficiency with the battleaxe, handaxe, light hammer, and warhammer.", "mechanical_effect": "proficiency_battleaxe_handaxe_light_hammer_warhammer"},
				{"name": "Tool Proficiency", "description": "You gain proficiency with the artisan's tools of your choice: smith's tools, brewer's supplies, or mason's tools.", "mechanical_effect": "choose_tool_proficiency_smiths_brewers_masons"},
				{"name": "Stonecunning", "description": "Whenever you make an Intelligence (History) check related to the origin of stonework, you are considered proficient in the History skill and add double your proficiency bonus to the check.", "mechanical_effect": "double_proficiency_history_stonework"},
			}),
			Languages: []string{"Common", "Dwarvish"},
			Subraces: optJSON([]map[string]any{
				{
					"id": "hill-dwarf", "name": "Hill Dwarf",
					"ability_bonuses": map[string]int{"wis": 1},
					"traits": []map[string]string{
						{"name": "Dwarven Toughness", "description": "Your hit point maximum increases by 1, and it increases by 1 every time you gain a level.", "mechanical_effect": "hp_plus_1_per_level"},
					},
				},
			}),
		},
		{
			ID: "elf", Name: "Elf", SpeedFt: 30, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]int{"dex": 2}),
			DarkvisionFt:   60,
			Traits: mustJSON([]map[string]string{
				{"name": "Keen Senses", "description": "You have proficiency in the Perception skill.", "mechanical_effect": "proficiency_perception"},
				{"name": "Fey Ancestry", "description": "You have advantage on saving throws against being charmed, and magic can't put you to sleep.", "mechanical_effect": "advantage_saves_charmed,immune_magical_sleep"},
				{"name": "Trance", "description": "Elves don't need to sleep. Instead, they meditate deeply for 4 hours a day.", "mechanical_effect": "long_rest_4_hours"},
			}),
			Languages: []string{"Common", "Elvish"},
			Subraces: optJSON([]map[string]any{
				{
					"id": "high-elf", "name": "High Elf",
					"ability_bonuses": map[string]int{"int": 1},
					"traits": []map[string]string{
						{"name": "Elf Weapon Training", "description": "You have proficiency with the longsword, shortsword, shortbow, and longbow.", "mechanical_effect": "proficiency_longsword_shortsword_shortbow_longbow"},
						{"name": "Cantrip", "description": "You know one cantrip of your choice from the wizard spell list.", "mechanical_effect": "learn_1_wizard_cantrip"},
						{"name": "Extra Language", "description": "You can speak, read, and write one extra language of your choice.", "mechanical_effect": "learn_1_language"},
					},
				},
			}),
		},
		{
			ID: "halfling", Name: "Halfling", SpeedFt: 25, Size: "Small",
			AbilityBonuses: mustJSON(map[string]int{"dex": 2}),
			DarkvisionFt:   0,
			Traits: mustJSON([]map[string]string{
				{"name": "Lucky", "description": "When you roll a 1 on the d20 for an attack roll, ability check, or saving throw, you can reroll the die and must use the new roll.", "mechanical_effect": "reroll_natural_1"},
				{"name": "Brave", "description": "You have advantage on saving throws against being frightened.", "mechanical_effect": "advantage_saves_frightened"},
				{"name": "Halfling Nimbleness", "description": "You can move through the space of any creature that is of a size larger than yours.", "mechanical_effect": "move_through_larger_creatures"},
			}),
			Languages: []string{"Common", "Halfling"},
			Subraces: optJSON([]map[string]any{
				{
					"id": "lightfoot", "name": "Lightfoot Halfling",
					"ability_bonuses": map[string]int{"cha": 1},
					"traits": []map[string]string{
						{"name": "Naturally Stealthy", "description": "You can attempt to hide even when you are obscured only by a creature that is at least one size larger than you.", "mechanical_effect": "hide_behind_larger_creatures"},
					},
				},
			}),
		},
		{
			ID: "human", Name: "Human", SpeedFt: 30, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]int{"str": 1, "dex": 1, "con": 1, "int": 1, "wis": 1, "cha": 1}),
			DarkvisionFt:   0,
			Traits: mustJSON([]map[string]string{
				{"name": "Extra Language", "description": "You can speak, read, and write one extra language of your choice.", "mechanical_effect": "learn_1_language"},
			}),
			Languages: []string{"Common"},
		},
		{
			ID: "dragonborn", Name: "Dragonborn", SpeedFt: 30, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]int{"str": 2, "cha": 1}),
			DarkvisionFt:   0,
			Traits: mustJSON([]map[string]string{
				{"name": "Draconic Ancestry", "description": "You have draconic ancestry. Choose one type of dragon from the Draconic Ancestry table.", "mechanical_effect": "choose_draconic_ancestry"},
				{"name": "Breath Weapon", "description": "You can use your action to exhale destructive energy. Your draconic ancestry determines the size, shape, and damage type of the exhalation.", "mechanical_effect": "breath_weapon_2d6_scaling"},
				{"name": "Damage Resistance", "description": "You have resistance to the damage type associated with your draconic ancestry.", "mechanical_effect": "resistance_draconic_ancestry_damage_type"},
			}),
			Languages: []string{"Common", "Draconic"},
		},
		{
			ID: "gnome", Name: "Gnome", SpeedFt: 25, Size: "Small",
			AbilityBonuses: mustJSON(map[string]int{"int": 2}),
			DarkvisionFt:   60,
			Traits: mustJSON([]map[string]string{
				{"name": "Gnome Cunning", "description": "You have advantage on all Intelligence, Wisdom, and Charisma saving throws against magic.", "mechanical_effect": "advantage_int_wis_cha_saves_vs_magic"},
			}),
			Languages: []string{"Common", "Gnomish"},
			Subraces: optJSON([]map[string]any{
				{
					"id": "rock-gnome", "name": "Rock Gnome",
					"ability_bonuses": map[string]int{"con": 1},
					"traits": []map[string]string{
						{"name": "Artificer's Lore", "description": "Whenever you make an Intelligence (History) check related to magic items, alchemical objects, or technological devices, you can add twice your proficiency bonus.", "mechanical_effect": "double_proficiency_history_magic_items_alchemy_tech"},
						{"name": "Tinker", "description": "You have proficiency with artisan's tools (tinker's tools). Using those tools, you can spend 1 hour and 10 gp worth of materials to construct a Tiny clockwork device.", "mechanical_effect": "proficiency_tinkers_tools,construct_clockwork_devices"},
					},
				},
			}),
		},
		{
			ID: "half-elf", Name: "Half-Elf", SpeedFt: 30, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]any{"cha": 2, "choose": map[string]any{"count": 2, "amount": 1, "from": []string{"str", "dex", "con", "int", "wis"}}}),
			DarkvisionFt:   60,
			Traits: mustJSON([]map[string]string{
				{"name": "Fey Ancestry", "description": "You have advantage on saving throws against being charmed, and magic can't put you to sleep.", "mechanical_effect": "advantage_saves_charmed,immune_magical_sleep"},
				{"name": "Skill Versatility", "description": "You gain proficiency in two skills of your choice.", "mechanical_effect": "gain_2_skill_proficiencies"},
			}),
			Languages: []string{"Common", "Elvish"},
		},
		{
			ID: "half-orc", Name: "Half-Orc", SpeedFt: 30, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]int{"str": 2, "con": 1}),
			DarkvisionFt:   60,
			Traits: mustJSON([]map[string]string{
				{"name": "Menacing", "description": "You gain proficiency in the Intimidation skill.", "mechanical_effect": "proficiency_intimidation"},
				{"name": "Relentless Endurance", "description": "When you are reduced to 0 hit points but not killed outright, you can drop to 1 hit point instead. You can't use this feature again until you finish a long rest.", "mechanical_effect": "drop_to_1hp_instead_of_0_once_per_long_rest"},
				{"name": "Savage Attacks", "description": "When you score a critical hit with a melee weapon attack, you can roll one of the weapon's damage dice one additional time and add it to the extra damage of the critical hit.", "mechanical_effect": "extra_crit_damage_die"},
			}),
			Languages: []string{"Common", "Orc"},
		},
		{
			ID: "tiefling", Name: "Tiefling", SpeedFt: 30, Size: "Medium",
			AbilityBonuses: mustJSON(map[string]int{"cha": 2, "int": 1}),
			DarkvisionFt:   60,
			Traits: mustJSON([]map[string]string{
				{"name": "Hellish Resistance", "description": "You have resistance to fire damage.", "mechanical_effect": "resistance_fire"},
				{"name": "Infernal Legacy", "description": "You know the thaumaturgy cantrip. When you reach 3rd level, you can cast the hellish rebuke spell as a 2nd-level spell once with this trait. When you reach 5th level, you can cast the darkness spell once with this trait. Charisma is your spellcasting ability for these spells.", "mechanical_effect": "cantrip_thaumaturgy,level3_hellish_rebuke,level5_darkness"},
			}),
			Languages: []string{"Common", "Infernal"},
		},
	}

	return seedEntities(ctx, races, q.UpsertRace, "race")
}
