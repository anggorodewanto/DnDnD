package refdata

import (
	"context"

	"github.com/sqlc-dev/pqtype"
)

func seedFeats(ctx context.Context, q *Queries) error {
	feats := []UpsertFeatParams{
		{
			ID: "alert", Name: "Alert",
			Description:      "Always on the lookout for danger, you gain the following benefits: +5 bonus to initiative, you can't be surprised while you are conscious, other creatures don't gain advantage on attack rolls against you as a result of being unseen by you.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "bonus_initiative", "value": "5"}, {"effect_type": "cant_be_surprised"}, {"effect_type": "no_advantage_from_unseen"}}),
		},
		{
			ID: "athlete", Name: "Athlete",
			Description: "You have undergone extensive physical training to gain the following benefits: Increase your Strength or Dexterity by 1. When you are prone, standing up uses only 5 feet of your movement. Climbing doesn't cost you extra movement. You can make a running long jump or high jump after moving only 5 feet on foot.",
			AsiBonus:    optJSON(map[string]any{"choose_ability": 1, "from": []string{"str", "dex"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "stand_up_5ft"}, {"effect_type": "climbing_no_extra_cost"}, {"effect_type": "running_jump_5ft"}}),
		},
		{
			ID: "charger", Name: "Charger",
			Description:      "When you use your action to Dash, you can use a bonus action to make one melee weapon attack or to shove a creature. If you move at least 10 feet in a straight line immediately before taking this bonus action, you either gain a +5 bonus to the attack's damage roll or push the target up to 10 feet away from you.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "bonus_action_attack_after_dash"}, {"effect_type": "bonus_damage_5_or_push_10ft", "condition": "moved_10ft_straight_line"}}),
		},
		{
			ID: "crossbow-expert", Name: "Crossbow Expert",
			Description:      "Thanks to extensive practice with the crossbow, you gain the following benefits: You ignore the loading quality of crossbows. Being within 5 feet of a hostile creature doesn't impose disadvantage on your ranged attack rolls. When you use the Attack action and attack with a one-handed weapon, you can use a bonus action to attack with a hand crossbow you are holding.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "ignore_loading_crossbow"}, {"effect_type": "no_disadvantage_ranged_5ft"}, {"effect_type": "bonus_action_hand_crossbow"}}),
		},
		{
			ID: "defensive-duelist", Name: "Defensive Duelist",
			Description:      "When you are wielding a finesse weapon with which you are proficient and another creature hits you with a melee attack, you can use your reaction to add your proficiency bonus to your AC for that attack, potentially causing the attack to miss you.",
			Prerequisites:    optJSON(map[string]any{"ability": map[string]int{"dex": 13}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "reaction_add_proficiency_to_ac", "condition": "wielding_finesse_weapon"}}),
		},
		{
			ID: "dual-wielder", Name: "Dual Wielder",
			Description:      "You master fighting with two weapons, gaining the following benefits: You gain a +1 bonus to AC while you are wielding a separate melee weapon in each hand. You can use two-weapon fighting even when the one-handed melee weapons you are wielding aren't light. You can draw or stow two one-handed weapons when you would normally be able to draw or stow only one.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "ac_bonus_1_dual_wielding"}, {"effect_type": "two_weapon_fighting_any_one_handed"}, {"effect_type": "draw_stow_two_weapons"}}),
		},
		{
			ID: "dungeon-delver", Name: "Dungeon Delver",
			Description:      "Alert to the hidden traps and secret doors found in many dungeons, you gain the following benefits: Advantage on Perception and Investigation checks to detect secret doors. Advantage on saving throws to avoid or resist traps. Resistance to trap damage. You can search for traps while traveling at a normal pace.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "advantage_detect_secret_doors"}, {"effect_type": "advantage_saves_traps"}, {"effect_type": "resistance_trap_damage"}, {"effect_type": "search_traps_normal_pace"}}),
		},
		{
			ID: "durable", Name: "Durable",
			Description: "Hardy and resilient, you gain the following benefits: Increase your Constitution by 1. When you roll a Hit Die to regain hit points, the minimum number of hit points you regain equals twice your Constitution modifier.",
			AsiBonus:    optJSON(map[string]any{"con": 1}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "min_hit_die_healing_2x_con_mod"}}),
		},
		{
			ID: "elemental-adept", Name: "Elemental Adept",
			Description:      "When you gain this feat, choose one of the following damage types: acid, cold, fire, lightning, or thunder. Spells you cast ignore resistance to damage of the chosen type. When you roll damage for a spell that deals the chosen type, you can treat any 1 on a damage die as a 2.",
			Prerequisites:    optJSON(map[string]any{"spellcasting": true}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "ignore_resistance_chosen_element"}, {"effect_type": "min_damage_die_2_chosen_element"}}),
		},
		{
			ID: "grappler", Name: "Grappler",
			Description:      "You've developed the skills necessary to hold your own in close-quarters grappling. You have advantage on attack rolls against a creature you are grappling. You can use your action to try to pin a creature grappled by you.",
			Prerequisites:    optJSON(map[string]any{"ability": map[string]int{"str": 13}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "advantage_attacks_grappled_target"}, {"effect_type": "action_pin_grappled_creature"}}),
		},
		{
			ID: "great-weapon-master", Name: "Great Weapon Master",
			Description:      "You've learned to put the weight of a weapon to your advantage, letting its momentum empower your strikes. On your turn, when you score a critical hit with a melee weapon or reduce a creature to 0 hit points with one, you can make one melee weapon attack as a bonus action. Before you make a melee attack with a heavy weapon that you are proficient with, you can choose to take a -5 penalty to the attack roll. If the attack hits, you add +10 to the attack's damage.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "bonus_action_attack_on_crit_or_kill"}, {"effect_type": "power_attack_minus_5_plus_10", "condition": "heavy_weapon"}}),
		},
		{
			ID: "healer", Name: "Healer",
			Description:      "You are an able physician, allowing you to mend wounds quickly and get your allies back in the fight. When you use a healer's kit to stabilize a dying creature, that creature also regains 1 hit point. As an action, you can spend one use of a healer's kit to tend to a creature and restore 1d6+4 hit points to it, plus additional hit points equal to the creature's maximum number of Hit Dice.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "healers_kit_stabilize_plus_1hp"}, {"effect_type": "healers_kit_heal_1d6_plus_4_plus_hd"}}),
		},
		{
			ID: "heavily-armored", Name: "Heavily Armored",
			Description:   "You have trained to master the use of heavy armor, gaining the following benefits: Increase your Strength by 1. You gain proficiency with heavy armor.",
			Prerequisites: optJSON(map[string]any{"proficiency": "medium_armor"}),
			AsiBonus:      optJSON(map[string]any{"str": 1}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "proficiency_heavy_armor"}}),
		},
		{
			ID: "heavy-armor-master", Name: "Heavy Armor Master",
			Description:   "You can use your armor to deflect strikes that would kill others. Increase your Strength by 1. While you are wearing heavy armor, bludgeoning, piercing, and slashing damage that you take from nonmagical weapons is reduced by 3.",
			Prerequisites: optJSON(map[string]any{"proficiency": "heavy_armor"}),
			AsiBonus:      optJSON(map[string]any{"str": 1}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "reduce_bps_damage_3_heavy_armor"}}),
		},
		{
			ID: "inspiring-leader", Name: "Inspiring Leader",
			Description:      "You can spend 10 minutes inspiring your companions, shoring up their resolve to fight. When you do so, choose up to six friendly creatures within 30 feet of you. Each creature gains temporary hit points equal to your level + your Charisma modifier.",
			Prerequisites:    optJSON(map[string]any{"ability": map[string]int{"cha": 13}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "grant_temp_hp_level_plus_cha_mod", "target": "up_to_6_allies"}}),
		},
		{
			ID: "keen-mind", Name: "Keen Mind",
			Description: "You have a mind that can track time, direction, and detail with uncanny precision. Increase your Intelligence by 1. You always know which way is north. You always know the number of hours left before the next sunrise or sunset. You can accurately recall anything you have seen or heard within the past month.",
			AsiBonus:    optJSON(map[string]any{"int": 1}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "always_know_north"}, {"effect_type": "always_know_time"}, {"effect_type": "perfect_recall_1_month"}}),
		},
		{
			ID: "lightly-armored", Name: "Lightly Armored",
			Description: "You have trained to master the use of light armor, gaining the following benefits: Increase your Strength or Dexterity by 1. You gain proficiency with light armor.",
			AsiBonus:    optJSON(map[string]any{"choose_ability": 1, "from": []string{"str", "dex"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "proficiency_light_armor"}}),
		},
		{
			ID: "linguist", Name: "Linguist",
			Description: "You have studied languages and codes, gaining the following benefits: Increase your Intelligence by 1. You learn three languages of your choice. You can ably create written ciphers.",
			AsiBonus:    optJSON(map[string]any{"int": 1}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "learn_3_languages"}, {"effect_type": "create_written_ciphers"}}),
		},
		{
			ID: "lucky", Name: "Lucky",
			Description:      "You have inexplicable luck that seems to kick in at just the right moment. You have 3 luck points. Whenever you make an attack roll, an ability check, or a saving throw, you can spend one luck point to roll an additional d20.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "luck_points_3"}, {"effect_type": "spend_luck_point_extra_d20"}}),
		},
		{
			ID: "mage-slayer", Name: "Mage Slayer",
			Description:      "You have practiced techniques useful in melee combat against spellcasters. When a creature within 5 feet of you casts a spell, you can use your reaction to make a melee weapon attack. When you damage a creature concentrating on a spell, that creature has disadvantage on the saving throw to maintain concentration. You have advantage on saving throws against spells cast by creatures within 5 feet of you.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "reaction_attack_on_spell_cast_5ft"}, {"effect_type": "disadvantage_concentration_saves_on_damage"}, {"effect_type": "advantage_saves_spells_5ft"}}),
		},
		{
			ID: "magic-initiate", Name: "Magic Initiate",
			Description:      "Choose a class: bard, cleric, druid, sorcerer, warlock, or wizard. You learn two cantrips of your choice from that class's spell list. In addition, choose one 1st-level spell from that same list. You can cast that spell once at its lowest level, and you must finish a long rest before you can cast it in this way again.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "learn_2_cantrips_from_class"}, {"effect_type": "learn_1_first_level_spell_from_class"}}),
		},
		{
			ID: "martial-adept", Name: "Martial Adept",
			Description:      "You have martial training that allows you to perform special combat maneuvers. You learn two maneuvers of your choice from among those available to the Battle Master archetype. You gain one superiority die, which is a d6.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "learn_2_battle_master_maneuvers"}, {"effect_type": "gain_1_superiority_die_d6"}}),
		},
		{
			ID: "medium-armor-master", Name: "Medium Armor Master",
			Description:   "You have practiced moving in medium armor to gain the following benefits: Wearing medium armor doesn't impose disadvantage on your Dexterity (Stealth) checks. When you wear medium armor, you can add 3, rather than 2, to your AC if you have a Dexterity of 16 or higher.",
			Prerequisites: optJSON(map[string]any{"proficiency": "medium_armor"}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "no_stealth_disadvantage_medium_armor"}, {"effect_type": "medium_armor_max_dex_3"}}),
		},
		{
			ID: "mobile", Name: "Mobile",
			Description:      "You are exceptionally speedy and agile. Your speed increases by 10 feet. When you use the Dash action, difficult terrain doesn't cost you extra movement on that turn. When you make a melee attack against a creature, you don't provoke opportunity attacks from that creature for the rest of the turn, whether you hit or not.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "speed_plus_10"}, {"effect_type": "dash_ignores_difficult_terrain"}, {"effect_type": "no_opportunity_attack_after_melee"}}),
		},
		{
			ID: "moderately-armored", Name: "Moderately Armored",
			Description:   "You have trained to master the use of medium armor and shields, gaining the following benefits: Increase your Strength or Dexterity by 1. You gain proficiency with medium armor and shields.",
			Prerequisites: optJSON(map[string]any{"proficiency": "light_armor"}),
			AsiBonus:      optJSON(map[string]any{"choose_ability": 1, "from": []string{"str", "dex"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "proficiency_medium_armor_shields"}}),
		},
		{
			ID: "mounted-combatant", Name: "Mounted Combatant",
			Description:      "You are a dangerous foe to face while mounted. You have advantage on melee attack rolls against any unmounted creature that is smaller than your mount. You can force an attack targeted at your mount to target you instead. If your mount is subjected to an effect that allows it to make a Dexterity saving throw to take only half damage, it instead takes no damage on a success and half damage on a failure.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "advantage_melee_vs_smaller_unmounted"}, {"effect_type": "redirect_attack_from_mount"}, {"effect_type": "mount_evasion"}}),
		},
		{
			ID: "observant", Name: "Observant",
			Description: "Quick to notice details of your environment, you gain the following benefits: Increase your Intelligence or Wisdom by 1. If you can see a creature's mouth while it is speaking a language you understand, you can interpret what it's saying by reading its lips. You have a +5 bonus to your passive Wisdom (Perception) and passive Intelligence (Investigation) scores.",
			AsiBonus:    optJSON(map[string]any{"choose_ability": 1, "from": []string{"int", "wis"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "lip_reading"}, {"effect_type": "passive_perception_plus_5"}, {"effect_type": "passive_investigation_plus_5"}}),
		},
		{
			ID: "polearm-master", Name: "Polearm Master",
			Description:      "You can keep your enemies at bay with reach weapons. When you take the Attack action and attack with only a glaive, halberd, quarterstaff, or spear, you can use a bonus action to make a melee attack with the opposite end of the weapon. This attack uses the same ability modifier as the primary attack. The weapon's damage die for this attack is a d4. When you are wielding a glaive, halberd, pike, quarterstaff, or spear, other creatures provoke an opportunity attack from you when they enter your reach.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "bonus_action_butt_attack_d4"}, {"effect_type": "opportunity_attack_on_enter_reach"}}),
		},
		{
			ID: "resilient", Name: "Resilient",
			Description: "Choose one ability score. You gain proficiency in saving throws using the chosen ability. Increase the chosen ability score by 1.",
			AsiBonus:    optJSON(map[string]any{"choose_ability": 1, "from": []string{"str", "dex", "con", "int", "wis", "cha"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "proficiency_saving_throw_chosen_ability"}}),
		},
		{
			ID: "ritual-caster", Name: "Ritual Caster",
			Description:      "You have learned a number of spells that you can cast as rituals. You acquire a ritual book holding two 1st-level spells of your choice that have the ritual tag from a class spell list of your choice.",
			Prerequisites:    optJSON(map[string]any{"ability_or": map[string]int{"int": 13, "wis": 13}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "learn_2_ritual_spells"}, {"effect_type": "can_add_ritual_spells_to_book"}}),
		},
		{
			ID: "savage-attacker", Name: "Savage Attacker",
			Description:      "Once per turn when you roll damage for a melee weapon attack, you can reroll the weapon's damage dice and use either total.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "reroll_melee_damage_once_per_turn"}}),
		},
		{
			ID: "sentinel", Name: "Sentinel",
			Description:      "You have mastered techniques to take advantage of every drop in any enemy's guard. When you hit a creature with an opportunity attack, the creature's speed becomes 0 for the rest of the turn. Creatures provoke opportunity attacks from you even if they take the Disengage action before leaving your reach. When a creature within 5 feet of you makes an attack against a target other than you, you can use your reaction to make a melee weapon attack against the attacking creature.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "opportunity_attack_stops_movement"}, {"effect_type": "opportunity_attack_ignores_disengage"}, {"effect_type": "reaction_attack_when_ally_attacked_5ft"}}),
		},
		{
			ID: "sharpshooter", Name: "Sharpshooter",
			Description:      "You have mastered ranged weapons and can make shots that others find impossible. Attacking at long range doesn't impose disadvantage on your ranged weapon attack rolls. Your ranged weapon attacks ignore half cover and three-quarters cover. Before you make an attack with a ranged weapon that you are proficient with, you can choose to take a -5 penalty to the attack roll. If the attack hits, you add +10 to the attack's damage.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "no_disadvantage_long_range"}, {"effect_type": "ignore_half_and_three_quarters_cover"}, {"effect_type": "power_attack_minus_5_plus_10", "condition": "ranged_weapon"}}),
		},
		{
			ID: "shield-master", Name: "Shield Master",
			Description:      "You use shields not just for protection but also for offense. If you take the Attack action on your turn, you can use a bonus action to try to shove a creature within 5 feet of you with your shield. If you aren't incapacitated, you can add your shield's AC bonus to any Dexterity saving throw against a spell or effect that targets only you. If you are subjected to an effect that allows you to make a Dexterity saving throw to take only half damage, you can use your reaction to take no damage if you succeed on the saving throw.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "bonus_action_shield_shove"}, {"effect_type": "add_shield_ac_to_dex_saves"}, {"effect_type": "reaction_evasion_with_shield"}}),
		},
		{
			ID: "skilled", Name: "Skilled",
			Description:      "You gain proficiency in any combination of three skills or tools of your choice.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "gain_3_skill_or_tool_proficiencies"}}),
		},
		{
			ID: "skulker", Name: "Skulker",
			Description:   "You are expert at slinking through shadows. You can try to hide when you are lightly obscured from the creature from which you are hiding. When you are hidden from a creature and miss it with a ranged weapon attack, making the attack doesn't reveal your position. You have darkvision out to a range of 60 feet.",
			Prerequisites: optJSON(map[string]any{"ability": map[string]int{"dex": 13}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "hide_lightly_obscured"}, {"effect_type": "missed_ranged_no_reveal"}, {"effect_type": "darkvision_60"}}),
		},
		{
			ID: "spell-sniper", Name: "Spell Sniper",
			Description:      "You have learned techniques to enhance your attacks with certain kinds of spells. When you cast a spell that requires you to make an attack roll, the spell's range is doubled. Your ranged spell attacks ignore half cover and three-quarters cover. You learn one cantrip that requires an attack roll.",
			Prerequisites:    optJSON(map[string]any{"spellcasting": true}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "double_spell_attack_range"}, {"effect_type": "ignore_half_and_three_quarters_cover_spells"}, {"effect_type": "learn_1_attack_cantrip"}}),
		},
		{
			ID: "tavern-brawler", Name: "Tavern Brawler",
			Description: "Accustomed to rough-and-tumble fighting using whatever is at hand. Increase your Strength or Constitution by 1. You are proficient with improvised weapons. Your unarmed strike uses a d4 for damage. When you hit a creature with an unarmed strike or an improvised weapon on your turn, you can use a bonus action to attempt to grapple the target.",
			AsiBonus:    optJSON(map[string]any{"choose_ability": 1, "from": []string{"str", "con"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "proficiency_improvised_weapons"}, {"effect_type": "unarmed_strike_d4"}, {"effect_type": "bonus_action_grapple_on_hit"}}),
		},
		{
			ID: "tough", Name: "Tough",
			Description:      "Your hit point maximum increases by an amount equal to twice your level when you gain this feat. Whenever you gain a level thereafter, your hit point maximum increases by an additional 2 hit points.",
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "hp_plus_2_per_level"}}),
		},
		{
			ID: "war-caster", Name: "War Caster",
			Description:      "You have practiced casting spells in the midst of combat. You have advantage on Constitution saving throws to maintain concentration on a spell when you take damage. You can perform somatic components of spells even when you have weapons or a shield in one or both hands. When a hostile creature's movement provokes an opportunity attack from you, you can use your reaction to cast a spell at the creature rather than making an opportunity attack.",
			Prerequisites:    optJSON(map[string]any{"spellcasting": true}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "advantage_concentration_saves"}, {"effect_type": "somatic_components_with_hands_full"}, {"effect_type": "opportunity_attack_cast_spell"}}),
		},
		{
			ID: "weapon-master", Name: "Weapon Master",
			Description: "You have practiced extensively with a variety of weapons. Increase your Strength or Dexterity by 1. You gain proficiency with four weapons of your choice. Each one must be a simple or a martial weapon.",
			AsiBonus:    optJSON(map[string]any{"choose_ability": 1, "from": []string{"str", "dex"}}),
			MechanicalEffect: optJSON([]map[string]string{{"effect_type": "proficiency_4_weapons"}}),
		},
	}

	// Null out prerequisites and asi_bonus for feats that don't have them
	for i := range feats {
		if !feats[i].Prerequisites.Valid {
			feats[i].Prerequisites = pqtype.NullRawMessage{}
		}
		if !feats[i].AsiBonus.Valid {
			feats[i].AsiBonus = pqtype.NullRawMessage{}
		}
	}

	return seedEntities(ctx, feats, q.UpsertFeat, "feat")
}
