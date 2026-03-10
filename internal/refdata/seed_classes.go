package refdata

import (
	"context"
	"encoding/json"

	"github.com/sqlc-dev/pqtype"
)

func optJSON(v any) pqtype.NullRawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("failed to marshal JSON: " + err.Error())
	}
	return pqtype.NullRawMessage{RawMessage: b, Valid: true}
}

func seedClasses(ctx context.Context, q *Queries) error {
	classes := []UpsertClassParams{
		{
			ID: "barbarian", Name: "Barbarian", HitDie: "d12", PrimaryAbility: "str",
			SaveProficiencies:  []string{"str", "con"},
			ArmorProficiencies: []string{"light", "medium", "shields"},
			WeaponProficiencies: []string{"simple", "martial"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"animal-handling", "athletics", "intimidation", "nature", "perception", "survival"}}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Rage", "description": "In battle, you fight with primal ferocity. On your turn, you can enter a rage as a bonus action.", "mechanical_effect": "advantage_str_checks_saves,resistance_bludgeoning_piercing_slashing,bonus_rage_damage"},
					{"name": "Unarmored Defense", "description": "While you are not wearing any armor, your AC equals 10 + your Dexterity modifier + your Constitution modifier.", "mechanical_effect": "ac_10_plus_dex_plus_con"},
				},
				"2": []map[string]string{
					{"name": "Reckless Attack", "description": "You can throw aside all concern for defense to attack with fierce desperation.", "mechanical_effect": "advantage_str_melee_attacks,attacks_against_have_advantage"},
					{"name": "Danger Sense", "description": "You have advantage on Dexterity saving throws against effects that you can see.", "mechanical_effect": "advantage_dex_saves_visible_effects"},
				},
				"3": []map[string]string{
					{"name": "Primal Path", "description": "You choose a path that shapes the nature of your rage.", "mechanical_effect": "subclass_choice"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1, "5": 2}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"berserker": map[string]any{
					"name": "Path of the Berserker",
					"features_by_level": map[string]any{
						"3": []map[string]string{{"name": "Frenzy", "description": "You can go into a frenzy when you rage, allowing you to make a single melee weapon attack as a bonus action on each of your turns after this one.", "mechanical_effect": "bonus_action_melee_attack_while_raging"}},
						"6": []map[string]string{{"name": "Mindless Rage", "description": "You can't be charmed or frightened while raging.", "mechanical_effect": "immune_charmed_frightened_while_raging"}},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"str": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"shields"}, "weapons": []string{"simple", "martial"}}),
		},
		{
			ID: "bard", Name: "Bard", HitDie: "d8", PrimaryAbility: "cha",
			SaveProficiencies:  []string{"dex", "cha"},
			ArmorProficiencies: []string{"light"},
			WeaponProficiencies: []string{"simple", "hand-crossbow", "longsword", "rapier", "shortsword"},
			SkillChoices: optJSON(map[string]any{"choose": 3, "from": []string{"acrobatics", "animal-handling", "arcana", "athletics", "deception", "history", "insight", "intimidation", "investigation", "medicine", "nature", "perception", "performance", "persuasion", "religion", "sleight-of-hand", "stealth", "survival"}}),
			Spellcasting: optJSON(map[string]string{"ability": "cha", "slot_progression": "full"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Bardic Inspiration", "description": "You can inspire others through stirring words or music. A creature that has a Bardic Inspiration die can roll that die and add the number rolled to one ability check, attack roll, or saving throw it makes.", "mechanical_effect": "grant_bardic_inspiration_d6"},
					{"name": "Spellcasting", "description": "You have learned to untangle and reshape the fabric of reality in harmony with your wishes and music.", "mechanical_effect": "spellcasting_cha"},
				},
				"2": []map[string]string{
					{"name": "Jack of All Trades", "description": "You can add half your proficiency bonus, rounded down, to any ability check you make that doesn't already include your proficiency bonus.", "mechanical_effect": "half_proficiency_nonproficient_checks"},
					{"name": "Song of Rest", "description": "You can use soothing music or oration to help revitalize your wounded allies during a short rest.", "mechanical_effect": "extra_1d6_healing_short_rest"},
				},
				"3": []map[string]string{
					{"name": "Bard College", "description": "You delve into the advanced techniques of a bard college of your choice.", "mechanical_effect": "subclass_choice"},
					{"name": "Expertise", "description": "Choose two of your skill proficiencies. Your proficiency bonus is doubled for any ability check you make that uses either of the chosen proficiencies.", "mechanical_effect": "double_proficiency_2_skills"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"lore": map[string]any{
					"name": "College of Lore",
					"features_by_level": map[string]any{
						"3": []map[string]string{
							{"name": "Bonus Proficiencies", "description": "You gain proficiency with three skills of your choice.", "mechanical_effect": "gain_3_skill_proficiencies"},
							{"name": "Cutting Words", "description": "You learn how to use your wit to distract, confuse, and otherwise sap the confidence and competence of others.", "mechanical_effect": "reaction_subtract_bardic_inspiration_from_attack_check_damage"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"cha": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light"}, "skills": map[string]any{"choose": 1}}),
		},
		{
			ID: "cleric", Name: "Cleric", HitDie: "d8", PrimaryAbility: "wis",
			SaveProficiencies:  []string{"wis", "cha"},
			ArmorProficiencies: []string{"light", "medium", "shields"},
			WeaponProficiencies: []string{"simple"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"history", "insight", "medicine", "persuasion", "religion"}}),
			Spellcasting: optJSON(map[string]string{"ability": "wis", "slot_progression": "full"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Spellcasting", "description": "As a conduit for divine power, you can cast cleric spells.", "mechanical_effect": "spellcasting_wis"},
					{"name": "Divine Domain", "description": "Choose one domain related to your deity.", "mechanical_effect": "subclass_choice"},
				},
				"2": []map[string]string{
					{"name": "Channel Divinity", "description": "You gain the ability to channel divine energy directly from your deity, using that energy to fuel magical effects.", "mechanical_effect": "channel_divinity_1_use"},
					{"name": "Channel Divinity: Turn Undead", "description": "As an action, you present your holy symbol and speak a prayer censuring the undead.", "mechanical_effect": "turn_undead"},
				},
				"3": []map[string]string{},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 1,
			Subclasses: mustJSON(map[string]any{
				"life": map[string]any{
					"name": "Life Domain",
					"features_by_level": map[string]any{
						"1": []map[string]string{
							{"name": "Bonus Proficiency", "description": "You gain proficiency with heavy armor.", "mechanical_effect": "proficiency_heavy_armor"},
							{"name": "Disciple of Life", "description": "Your healing spells are more effective. Whenever you use a spell of 1st level or higher to restore hit points, the creature regains additional hit points equal to 2 + the spell's level.", "mechanical_effect": "bonus_healing_2_plus_spell_level"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"wis": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light", "medium", "shields"}}),
		},
		{
			ID: "druid", Name: "Druid", HitDie: "d8", PrimaryAbility: "wis",
			SaveProficiencies:  []string{"int", "wis"},
			ArmorProficiencies: []string{"light", "medium", "shields"},
			WeaponProficiencies: []string{"club", "dagger", "dart", "javelin", "mace", "quarterstaff", "scimitar", "sickle", "sling", "spear"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"arcana", "animal-handling", "insight", "medicine", "nature", "perception", "religion", "survival"}}),
			Spellcasting: optJSON(map[string]string{"ability": "wis", "slot_progression": "full"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Druidic", "description": "You know Druidic, the secret language of druids.", "mechanical_effect": "language_druidic"},
					{"name": "Spellcasting", "description": "Drawing on the divine essence of nature itself, you can cast spells to shape that essence to your will.", "mechanical_effect": "spellcasting_wis"},
				},
				"2": []map[string]string{
					{"name": "Wild Shape", "description": "You can use your action to magically assume the shape of a beast that you have seen before.", "mechanical_effect": "wild_shape_2_uses_cr_1_4"},
					{"name": "Druid Circle", "description": "You choose to identify with a circle of druids.", "mechanical_effect": "subclass_choice"},
				},
				"3": []map[string]string{},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 2,
			Subclasses: mustJSON(map[string]any{
				"land": map[string]any{
					"name": "Circle of the Land",
					"features_by_level": map[string]any{
						"2": []map[string]string{
							{"name": "Bonus Cantrip", "description": "You learn one additional druid cantrip of your choice.", "mechanical_effect": "learn_1_druid_cantrip"},
							{"name": "Natural Recovery", "description": "You can regain some of your magical energy by sitting in meditation and communing with nature.", "mechanical_effect": "recover_spell_slots_short_rest"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"wis": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light", "medium", "shields"}}),
		},
		{
			ID: "fighter", Name: "Fighter", HitDie: "d10", PrimaryAbility: "str",
			SaveProficiencies:  []string{"str", "con"},
			ArmorProficiencies: []string{"light", "medium", "heavy", "shields"},
			WeaponProficiencies: []string{"simple", "martial"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"acrobatics", "animal-handling", "athletics", "history", "insight", "intimidation", "perception", "survival"}}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Fighting Style", "description": "You adopt a particular style of fighting as your specialty.", "mechanical_effect": "choose_fighting_style"},
					{"name": "Second Wind", "description": "You have a limited well of stamina that you can draw on to protect yourself from harm. On your turn, you can use a bonus action to regain hit points equal to 1d10 + your fighter level.", "mechanical_effect": "bonus_action_heal_1d10_plus_level"},
				},
				"2": []map[string]string{
					{"name": "Action Surge", "description": "You can push yourself beyond your normal limits for a moment. On your turn, you can take one additional action.", "mechanical_effect": "extra_action_1_use"},
				},
				"3": []map[string]string{
					{"name": "Martial Archetype", "description": "You choose an archetype that you strive to emulate in your combat styles and techniques.", "mechanical_effect": "subclass_choice"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1, "5": 2, "11": 3, "20": 4}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"champion": map[string]any{
					"name": "Champion",
					"features_by_level": map[string]any{
						"3":  []map[string]string{{"name": "Improved Critical", "description": "Your weapon attacks score a critical hit on a roll of 19 or 20.", "mechanical_effect": "crit_on_19_or_20"}},
						"7":  []map[string]string{{"name": "Remarkable Athlete", "description": "You can add half your proficiency bonus to any Strength, Dexterity, or Constitution check you make that doesn't already use your proficiency bonus.", "mechanical_effect": "half_proficiency_str_dex_con_checks"}},
						"15": []map[string]string{{"name": "Superior Critical", "description": "Your weapon attacks score a critical hit on a roll of 18-20.", "mechanical_effect": "crit_on_18_19_20"}},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"str": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light", "medium", "shields"}, "weapons": []string{"simple", "martial"}}),
		},
		{
			ID: "monk", Name: "Monk", HitDie: "d8", PrimaryAbility: "dex",
			SaveProficiencies:  []string{"str", "dex"},
			ArmorProficiencies: []string{},
			WeaponProficiencies: []string{"simple", "shortsword"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"acrobatics", "athletics", "history", "insight", "religion", "stealth"}}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Unarmored Defense", "description": "While you are wearing no armor and not wielding a shield, your AC equals 10 + your Dexterity modifier + your Wisdom modifier.", "mechanical_effect": "ac_10_plus_dex_plus_wis"},
					{"name": "Martial Arts", "description": "Your practice of martial arts gives you mastery of combat styles that use unarmed strikes and monk weapons.", "mechanical_effect": "martial_arts_d4,bonus_action_unarmed_strike"},
				},
				"2": []map[string]string{
					{"name": "Ki", "description": "Your training allows you to harness the mystic energy of ki.", "mechanical_effect": "ki_points_equal_monk_level"},
					{"name": "Unarmored Movement", "description": "Your speed increases by 10 feet while you are not wearing armor or wielding a shield.", "mechanical_effect": "speed_plus_10"},
				},
				"3": []map[string]string{
					{"name": "Monastic Tradition", "description": "You commit yourself to a monastic tradition.", "mechanical_effect": "subclass_choice"},
					{"name": "Deflect Missiles", "description": "You can use your reaction to deflect or catch the missile when you are hit by a ranged weapon attack.", "mechanical_effect": "reduce_ranged_damage_1d10_plus_dex_plus_level"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1, "5": 2}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"open-hand": map[string]any{
					"name": "Way of the Open Hand",
					"features_by_level": map[string]any{
						"3": []map[string]string{{"name": "Open Hand Technique", "description": "You can manipulate your enemy's ki when you harness your own. Whenever you hit a creature with one of the attacks granted by your Flurry of Blows, you can impose one of several effects on that target.", "mechanical_effect": "flurry_of_blows_knock_prone_or_push_or_no_reactions"}},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"dex": 13, "wis": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"weapons": []string{"simple", "shortsword"}}),
		},
		{
			ID: "paladin", Name: "Paladin", HitDie: "d10", PrimaryAbility: "str",
			SaveProficiencies:  []string{"wis", "cha"},
			ArmorProficiencies: []string{"light", "medium", "heavy", "shields"},
			WeaponProficiencies: []string{"simple", "martial"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"athletics", "insight", "intimidation", "medicine", "persuasion", "religion"}}),
			Spellcasting: optJSON(map[string]string{"ability": "cha", "slot_progression": "half"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Divine Sense", "description": "The presence of strong evil registers on your senses like a noxious odor, and powerful good rings like heavenly music in your ears.", "mechanical_effect": "detect_celestial_fiend_undead"},
					{"name": "Lay on Hands", "description": "Your blessed touch can heal wounds. You have a pool of healing power that replenishes when you take a long rest. With that pool, you can restore a total number of hit points equal to your paladin level x 5.", "mechanical_effect": "healing_pool_5x_paladin_level"},
				},
				"2": []map[string]string{
					{"name": "Fighting Style", "description": "You adopt a particular style of fighting as your specialty.", "mechanical_effect": "choose_fighting_style"},
					{"name": "Spellcasting", "description": "You have learned to draw on divine magic through meditation and prayer to cast spells.", "mechanical_effect": "spellcasting_cha"},
					{"name": "Divine Smite", "description": "When you hit a creature with a melee weapon attack, you can expend one spell slot to deal radiant damage to the target, in addition to the weapon's damage.", "mechanical_effect": "expend_spell_slot_2d8_radiant_plus_1d8_per_slot_level"},
				},
				"3": []map[string]string{
					{"name": "Sacred Oath", "description": "You swear the oath that binds you as a paladin forever.", "mechanical_effect": "subclass_choice"},
					{"name": "Divine Health", "description": "The divine magic flowing through you makes you immune to disease.", "mechanical_effect": "immune_disease"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1, "5": 2}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"devotion": map[string]any{
					"name": "Oath of Devotion",
					"features_by_level": map[string]any{
						"3": []map[string]string{
							{"name": "Sacred Weapon", "description": "As an action, you can imbue one weapon that you are holding with positive energy. For 1 minute, you add your Charisma modifier to attack rolls made with that weapon.", "mechanical_effect": "channel_divinity_add_cha_to_attack_rolls"},
							{"name": "Turn the Unholy", "description": "As an action, you present your holy symbol and each fiend or undead within 30 feet must make a Wisdom saving throw.", "mechanical_effect": "channel_divinity_turn_fiend_undead"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"str": 13, "cha": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light", "medium", "shields"}, "weapons": []string{"simple", "martial"}}),
		},
		{
			ID: "ranger", Name: "Ranger", HitDie: "d10", PrimaryAbility: "dex",
			SaveProficiencies:  []string{"str", "dex"},
			ArmorProficiencies: []string{"light", "medium", "shields"},
			WeaponProficiencies: []string{"simple", "martial"},
			SkillChoices: optJSON(map[string]any{"choose": 3, "from": []string{"animal-handling", "athletics", "insight", "investigation", "nature", "perception", "stealth", "survival"}}),
			Spellcasting: optJSON(map[string]string{"ability": "wis", "slot_progression": "half"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Favored Enemy", "description": "You have significant experience studying, tracking, hunting, and even talking to a certain type of enemy.", "mechanical_effect": "advantage_survival_tracking,advantage_int_checks_about_favored_enemy"},
					{"name": "Natural Explorer", "description": "You are particularly familiar with one type of natural environment and are adept at traveling and surviving in such regions.", "mechanical_effect": "difficult_terrain_no_penalty,advantage_initiative,advantage_first_turn_attacks"},
				},
				"2": []map[string]string{
					{"name": "Fighting Style", "description": "You adopt a particular style of fighting as your specialty.", "mechanical_effect": "choose_fighting_style"},
					{"name": "Spellcasting", "description": "You have learned to use the magical essence of nature to cast spells.", "mechanical_effect": "spellcasting_wis"},
				},
				"3": []map[string]string{
					{"name": "Ranger Archetype", "description": "You choose an archetype that you strive to emulate.", "mechanical_effect": "subclass_choice"},
					{"name": "Primeval Awareness", "description": "You can use your action and expend one ranger spell slot to focus your awareness on the region around you.", "mechanical_effect": "detect_aberration_celestial_dragon_elemental_fey_fiend_undead_1mi"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1, "5": 2}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"hunter": map[string]any{
					"name": "Hunter",
					"features_by_level": map[string]any{
						"3": []map[string]string{{"name": "Hunter's Prey", "description": "You gain one of the following features of your choice: Colossus Slayer, Giant Killer, or Horde Breaker.", "mechanical_effect": "choose_colossus_slayer_or_giant_killer_or_horde_breaker"}},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"dex": 13, "wis": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light", "medium", "shields"}, "weapons": []string{"simple", "martial"}, "skills": map[string]any{"choose": 1, "from": []string{"animal-handling", "athletics", "insight", "investigation", "nature", "perception", "stealth", "survival"}}}),
		},
		{
			ID: "rogue", Name: "Rogue", HitDie: "d8", PrimaryAbility: "dex",
			SaveProficiencies:  []string{"dex", "int"},
			ArmorProficiencies: []string{"light"},
			WeaponProficiencies: []string{"simple", "hand-crossbow", "longsword", "rapier", "shortsword"},
			SkillChoices: optJSON(map[string]any{"choose": 4, "from": []string{"acrobatics", "athletics", "deception", "insight", "intimidation", "investigation", "perception", "performance", "persuasion", "sleight-of-hand", "stealth"}}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Expertise", "description": "Choose two of your skill proficiencies. Your proficiency bonus is doubled for any ability check you make that uses either of the chosen proficiencies.", "mechanical_effect": "double_proficiency_2_skills"},
					{"name": "Sneak Attack", "description": "You know how to strike subtly and exploit a foe's distraction. Once per turn, you can deal an extra 1d6 damage to one creature you hit with an attack if you have advantage on the attack roll.", "mechanical_effect": "sneak_attack_1d6"},
					{"name": "Thieves' Cant", "description": "During your rogue training you learned thieves' cant, a secret mix of dialect, jargon, and code.", "mechanical_effect": "language_thieves_cant"},
				},
				"2": []map[string]string{
					{"name": "Cunning Action", "description": "Your quick thinking and agility allow you to move and act quickly. You can take a bonus action on each of your turns in combat to take the Dash, Disengage, or Hide action.", "mechanical_effect": "bonus_action_dash_disengage_hide"},
				},
				"3": []map[string]string{
					{"name": "Roguish Archetype", "description": "You choose an archetype that you emulate in the exercise of your rogue abilities.", "mechanical_effect": "subclass_choice"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 3,
			Subclasses: mustJSON(map[string]any{
				"thief": map[string]any{
					"name": "Thief",
					"features_by_level": map[string]any{
						"3": []map[string]string{
							{"name": "Fast Hands", "description": "You can use the bonus action granted by your Cunning Action to make a Dexterity (Sleight of Hand) check, use your thieves' tools to disarm a trap or open a lock, or take the Use an Object action.", "mechanical_effect": "cunning_action_sleight_of_hand_thieves_tools_use_object"},
							{"name": "Second-Story Work", "description": "You gain the ability to climb faster than normal; climbing no longer costs you extra movement.", "mechanical_effect": "climbing_no_extra_cost,jump_distance_plus_dex_mod"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"dex": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light"}, "weapons": []string{"simple", "hand-crossbow", "longsword", "rapier", "shortsword"}, "skills": map[string]any{"choose": 1}}),
		},
		{
			ID: "sorcerer", Name: "Sorcerer", HitDie: "d6", PrimaryAbility: "cha",
			SaveProficiencies:  []string{"con", "cha"},
			ArmorProficiencies: []string{},
			WeaponProficiencies: []string{"dagger", "dart", "sling", "quarterstaff", "light-crossbow"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"arcana", "deception", "insight", "intimidation", "persuasion", "religion"}}),
			Spellcasting: optJSON(map[string]string{"ability": "cha", "slot_progression": "full"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Spellcasting", "description": "An event in your past, or in the life of a parent or ancestor, left an indelible mark on you, infusing you with arcane magic.", "mechanical_effect": "spellcasting_cha"},
					{"name": "Sorcerous Origin", "description": "Choose a sorcerous origin, which describes the source of your innate magical power.", "mechanical_effect": "subclass_choice"},
				},
				"2": []map[string]string{
					{"name": "Font of Magic", "description": "You tap into a deep wellspring of magic within yourself. This wellspring is represented by sorcery points.", "mechanical_effect": "sorcery_points_equal_sorcerer_level"},
				},
				"3": []map[string]string{
					{"name": "Metamagic", "description": "You gain the ability to twist your spells to suit your needs. You gain two Metamagic options of your choice.", "mechanical_effect": "choose_2_metamagic_options"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 1,
			Subclasses: mustJSON(map[string]any{
				"draconic": map[string]any{
					"name": "Draconic Bloodline",
					"features_by_level": map[string]any{
						"1": []map[string]string{
							{"name": "Dragon Ancestor", "description": "You choose one type of dragon as your ancestor.", "mechanical_effect": "choose_dragon_ancestor_type"},
							{"name": "Draconic Resilience", "description": "As magic flows through your body, it causes physical traits of your dragon ancestors to emerge. Your hit point maximum increases by 1 for each sorcerer level, and when you aren't wearing armor, your AC equals 13 + your Dexterity modifier.", "mechanical_effect": "hp_plus_1_per_level,ac_13_plus_dex_unarmored"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"cha": 13}),
			MulticlassProficiencies: optJSON(map[string]any{}),
		},
		{
			ID: "warlock", Name: "Warlock", HitDie: "d8", PrimaryAbility: "cha",
			SaveProficiencies:  []string{"wis", "cha"},
			ArmorProficiencies: []string{"light"},
			WeaponProficiencies: []string{"simple"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"arcana", "deception", "history", "intimidation", "investigation", "nature", "religion"}}),
			Spellcasting: optJSON(map[string]string{"ability": "cha", "slot_progression": "pact"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Otherworldly Patron", "description": "You have struck a bargain with an otherworldly being of your choice.", "mechanical_effect": "subclass_choice"},
					{"name": "Pact Magic", "description": "Your arcane research and the magic bestowed on you by your patron have given you facility with spells.", "mechanical_effect": "pact_magic_cha"},
				},
				"2": []map[string]string{
					{"name": "Eldritch Invocations", "description": "In your study of occult lore, you have unearthed eldritch invocations, fragments of forbidden knowledge that imbue you with an abiding magical ability.", "mechanical_effect": "choose_2_eldritch_invocations"},
				},
				"3": []map[string]string{
					{"name": "Pact Boon", "description": "Your otherworldly patron bestows a gift upon you for your loyal service.", "mechanical_effect": "choose_pact_boon"},
				},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 1,
			Subclasses: mustJSON(map[string]any{
				"fiend": map[string]any{
					"name": "The Fiend",
					"features_by_level": map[string]any{
						"1": []map[string]string{{"name": "Dark One's Blessing", "description": "When you reduce a hostile creature to 0 hit points, you gain temporary hit points equal to your Charisma modifier + your warlock level.", "mechanical_effect": "temp_hp_cha_mod_plus_warlock_level_on_kill"}},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"cha": 13}),
			MulticlassProficiencies: optJSON(map[string]any{"armor": []string{"light"}, "weapons": []string{"simple"}}),
		},
		{
			ID: "wizard", Name: "Wizard", HitDie: "d6", PrimaryAbility: "int",
			SaveProficiencies:  []string{"int", "wis"},
			ArmorProficiencies: []string{},
			WeaponProficiencies: []string{"dagger", "dart", "sling", "quarterstaff", "light-crossbow"},
			SkillChoices: optJSON(map[string]any{"choose": 2, "from": []string{"arcana", "history", "insight", "investigation", "medicine", "religion"}}),
			Spellcasting: optJSON(map[string]string{"ability": "int", "slot_progression": "full"}),
			FeaturesByLevel: mustJSON(map[string]any{
				"1": []map[string]string{
					{"name": "Spellcasting", "description": "As a student of arcane magic, you have a spellbook containing spells that show the first glimmerings of your true power.", "mechanical_effect": "spellcasting_int"},
					{"name": "Arcane Recovery", "description": "You have learned to regain some of your magical energy by studying your spellbook. Once per day when you finish a short rest, you can choose expended spell slots to recover.", "mechanical_effect": "recover_spell_slots_short_rest_half_wizard_level"},
				},
				"2": []map[string]string{
					{"name": "Arcane Tradition", "description": "You choose an arcane tradition, shaping your practice of magic.", "mechanical_effect": "subclass_choice"},
				},
				"3": []map[string]string{},
			}),
			AttacksPerAction: mustJSON(map[string]int{"1": 1}),
			SubclassLevel: 2,
			Subclasses: mustJSON(map[string]any{
				"evocation": map[string]any{
					"name": "School of Evocation",
					"features_by_level": map[string]any{
						"2": []map[string]string{
							{"name": "Evocation Savant", "description": "The gold and time you must spend to copy an evocation spell into your spellbook is halved.", "mechanical_effect": "half_cost_copy_evocation_spells"},
							{"name": "Sculpt Spells", "description": "You can create pockets of relative safety within the effects of your evocation spells.", "mechanical_effect": "choose_creatures_auto_save_evocation_no_damage"},
						},
					},
				},
			}),
			MulticlassPrereqs:       optJSON(map[string]int{"int": 13}),
			MulticlassProficiencies: optJSON(map[string]any{}),
		},
	}

	return seedEntities(ctx, classes, q.UpsertClass, "class")
}
