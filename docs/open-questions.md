# Open Questions — Player Perspective Spec Review

Gaps, ambiguities, and missing features identified by reviewing `dnd-async-discord-spec.md` from a player's point of view.

---

## Onboarding & Registration

1. **No player-facing character creation.** Characters are created by the DM or imported from D&D Beyond. There is no workflow for a player to create or submit their own character. How does a new player know what to do after joining the Discord server?
2. **Registration approval is opaque.** After `/register Thorn`, the player waits for DM approval with no feedback — no "pending" status, no way to check, no notification when approved or rejected.
3. **No `/unregister` or character swap.** What if a player wants to switch characters mid-campaign? The spec says "DM can override" but there is no player-facing flow.
4. **No onboarding message.** When a player joins the server, is there an automated welcome explaining channels, commands, and how to get started?
5. **Mistyped character name on `/register`.** What error does the player get? Is it fuzzy-matched against existing characters?

## Turn Flow & Action Economy

6. **No explicit stand/prone toggle.** Standing from prone auto-deducts movement on `/move`, but can a player choose to stay prone? How do they intentionally drop prone?
7. **No crawl mechanic.** Crawling while prone is mentioned (double cost) but there is no command to crawl vs. stand-then-move.
~~8. **Disengage is missing from the command table.** Referenced under Opportunity Attacks and Cunning Action but never formally listed. Players would not discover it.~~ — Resolved: Added `/action disengage` to command table and Standard Actions section. Added `/bonus cunning-action disengage` example to bonus action row for Rogue discoverability.
9. **`/done` has no safety net.** If a player has unused attacks or bonus actions and types `/done`, is there a confirmation prompt, or does the turn just end?
10. **Movement cost is invisible.** When a player types `/move D4`, can they see the path cost and remaining movement before committing? Can they query "how far is D4?" without committing?
11. **No `/map` command.** The map posts to `#combat-map` on state change. Can a player request a fresh map view on demand?
12. **No `/initiative` query command.** Players can see `#initiative-tracker`, but can they query turn order with a command?

## Combat Mechanics

~~13. **Grapple escape has no command.** "Target uses action to repeat the contested check" — but what command does the grappled player type? `/action escape`?~~ — Resolved: Added `/action escape` as an auto-resolved standard action. Runs contested Athletics/Acrobatics vs grappler's Athletics. Auto-selects higher modifier; `--athletics` or `--acrobatics` to override.
~~14. **No unarmed strike.** The spec covers weapons extensively but never mentions unarmed strikes (1 + STR mod bludgeoning).~~ — Resolved: Added unarmed strike as a built-in pseudo-weapon in the weapons table (damage: "0", 1 + STR mod bludgeoning). Defaults when no weapon equipped. Monk Martial Arts overrides damage die via Feature Effect System.
15. **Improvised weapons.** Not mentioned. What if a player wants to hit someone with a chair?
16. **Ranged spell attacks in melee.** Disadvantage for ranged attacks while hostile within 5ft — does this apply to ranged spell attacks too? Crossbow Expert exception not mentioned.
17. **Natural armor / unarmored defense.** AC is stored as a single integer. How are Monk (10+DEX+WIS) or Barbarian (10+DEX+CON) unarmored defense recalculated when stats change?
18. **Shield equip/unequip.** Shields are in the armor table, but how does a player equip one? Does donning/doffing take an action?
~~19. **Dual wielding.** `equipped_weapon` is a single field. How does the system track two weapons in two hands?~~ — Resolved: Replaced `equipped_weapon` with `equipped_main_hand` and `equipped_off_hand`. Off-hand holds second light weapon or shield; null = free hand. Two-handed/versatile requires off-hand to be empty.

## Spellcasting

20. **Material components with gold cost.** The spec tracks `{v, s, m}` but never addresses consumed components or costly material components (e.g., Revivify's 300gp diamond). Is a focus/component pouch assumed?
21. **No `/prepare` command.** How does a Cleric/Druid/Paladin change prepared spells after a long rest? Dashboard-only?
22. **Counterspell timing in async.** Does the player see what spell is being cast before deciding to Counterspell? Do they know the level?
23. **Spell targeting syntax.** For single-target beneficial spells (Healing Word, Bless), can a player target allies by name or short ID?
24. **Warlock spell slots.** Warlocks have unique slots (fewer, all same level, recharge on short rest). The `spell_slots` JSONB and rest mechanics do not explicitly address this.
~~25. **Bonus action spell restriction — reverse direction.** The spec enforces "bonus action spell = only cantrip with action." But per 5e, casting a leveled action spell also prevents bonus action spells. Is this enforced?~~ — Resolved: Both directions enforced. Added `action_spell_cast` boolean to turns table. Casting a leveled action spell blocks subsequent bonus action spells, and vice versa.
26. **Casting in Silence.** The spec mentions Silence breaking concentration but not blocking new V/S spells from being cast inside the zone.

## Movement & Positioning

~~27. **Moving through occupied tiles.** 5e allows moving through allied spaces but not hostile spaces (unless 2+ sizes different). The spec says "tile occupancy" is validated but does not explain the rules.~~ — Resolved: Added full 5e occupied tile rules. Allies: pass through but can't end turn there. Hostiles: blocked unless 2+ sizes different. Can never end turn in another creature's space.
28. **Map readability.** On a 30x20 map at 32-48px/tile, coordinate labels may be too small. Mobile viewing could be very difficult.
29. **Multiple floors / z-levels.** The altitude system handles flying. What about multi-story buildings, stairs, or dungeons with different levels?
30. **Teleportation spells.** Misty Step, Dimension Door, Thunder Step — the movement system is path-based. Does teleportation bypass path validation?

## Reactions

31. **No way to view active declarations.** Declarations persist until used, cancelled, or encounter ends. Can a player list their current active declarations?
32. **Multiple simultaneous declarations.** Can a player have Shield AND Counterspell declared? If both trigger in the same round (only one reaction allowed), which fires?
33. **Freeform declaration ambiguity.** `/reaction OA if goblin moves away` — which goblin? Declarations are freeform text; there is no structured targeting.
~~34. **OA prompts stall combat.** The system prompts players for opportunity attacks when an enemy moves away. Does combat pause until the player responds? What is the timeout? This conflicts with the "zero stalling" design goal.~~ — Resolved: Queue-and-continue model. Movement is not paused; hostile has until end-of-round to take the OA. If OA drops target to 0 HP, system notifies DM to retroactively correct position. No auto-invalidation.
35. **Readied action reminders.** A readied action could persist for days of real time in async play. Is the player reminded it is still held?

## Death & Healing

~~36. **Death save timeout has no default rule.** "If the player doesn't send `/deathsave` before timeout, the turn is skipped — DM decides outcome." This is life-or-death with pure DM fiat. Should there be a default (e.g., auto-roll)?~~ — Resolved: System auto-rolls the death save on timeout using standard d20 rules. Result posted to #combat-log as "(auto-rolled — player timed out)".
37. **Targeting unconscious allies.** How does another player target an unconscious ally with a healing spell? Is there a visual indicator on the map distinguishing dying from dead?
38. **Medicine check to stabilize.** Listed as an option but `/check` has no target parameter. How do you target a specific dying creature with a skill check?
39. **Spare the Dying.** Mentioned as a stabilization option but has no specific implementation detail. Is adjacency validated for touch range?
40. **Massive damage / instant death.** What does the player see when their character is instantly killed? Is there a distinct message vs. dropping to 0 HP?

## Inventory & Equipment

41. **Inventory management is "future phases" but MVP depends on it.** Ammunition tracking, thrown weapons, and equipment are all in MVP scope. What is the actual MVP inventory scope?
42. **No `/inventory` command.** Players cannot view their inventory in Discord.
43. **No looting workflow.** After combat, how do players pick up items from defeated enemies?
44. **No consumables system.** How does a player drink a healing potion? There is no `/use` command.
45. **No gold or currency tracking.** Not in the data model at all.
46. **Equipping armor timing.** Donning/doffing armor takes 1-10 minutes in 5e. Is this enforced or is `/equip` instant?

## Out-of-Combat Gameplay

47. **No structured exploration mode.** No marching order, trap detection, or environmental interaction system.
48. **Secret skill checks.** All rolls post to `#roll-history`. What about Insight, Perception, Stealth checks that should sometimes be hidden from other players?
49. **No downtime activities.** Crafting, training, research not mentioned.
50. **Rest interruption is vague.** "Partial benefits at DM discretion via manual override" — the player has no idea what they will or won't get.
51. **Short rest hit dice command.** The player is prompted to `/spend-hd 2` but this command is not in any command table. Is it real?

## Communication

52. **No in-character channel for players.** `#the-story` is DM-write-only. `#player-chat` is OOC. Where do players write in-character?
53. **No private DM whisper.** How does a player secretly communicate with the DM (e.g., "I want to steal from the party")?
54. **No private player-to-player communication.** Characters cannot whisper to each other without the whole party seeing.
55. **DM notification workflow.** After skill checks and freeform actions, "DM narrates outcome in `#the-story`" — but how is the DM notified? A queue item?

## Information Asymmetry & Visibility

~~56. **Enemy AC exposure contradiction.** The spec says "AC is hidden" but the miss example shows "11 (6 + 5) vs AC 13 — MISS" revealing the AC. Which is correct?~~ — Resolved: Enemy AC is hidden. Attack results show only Hit/Miss without revealing the target's AC. Fixed the miss example to omit AC.
57. **Character card contents undefined.** `#character-cards` is mentioned but its contents are never specified. What does it show?
58. **Allied HP visibility.** Enemy HP is hidden (health tiers). Can players see exact HP for their allies?
59. **Creature identification.** Do players see creature names ("Goblin") or just token labels ("G1")?
60. **Hidden enemy detection.** Enemies with `is_visible = false` are hidden. Beyond passive Perception, how does a player actively search for hidden enemies?

## Error Recovery

61. **No player undo.** If a player targets the wrong enemy or moves to the wrong tile, the spec says no undo in MVP. How does the player request a correction from the DM?
62. **Disconnection handling.** If a player's Discord client crashes mid-turn, the turn timer continues. Any reconnection awareness?
63. **Invalid command feedback.** Is there a consistent error format for bad arguments, missing targets, etc.?
64. **Cancelling queued freeform actions.** `/action flip the table` goes to `#dm-queue`. Can the player edit or cancel it before the DM resolves it?

## Notifications & Awareness

65. **No backup notification system.** Discord mobile notifications can be unreliable. Any fallback (email, webhook)?
66. **Turn reminder content.** Do the 50%/75% timeout reminders re-state available actions, conditions, and battlefield context, or just say "hurry up"?
67. **Between-turn awareness.** When it is not your turn, how do you stay informed? Is there a summary of what happened since your last turn?
68. **Ping fatigue.** In 8+ combatant fights, `#your-turn` could get very noisy with OA prompts, save prompts, reaction prompts. Any batching or filtering?

## Bonus Actions & Free Actions

69. **Bonus action spell casting syntax.** Is `/cast healing-word AR` auto-detected as bonus action? Or must the player use `/bonus cast healing-word AR`?
70. **Monk ki abilities.** Flurry of Blows needs unarmed strikes, but there is no unarmed strike mechanic. Patient Defense and Step of the Wind — only Step of the Wind is mentioned.
71. **Free action speaking.** Can players speak in-character during combat without using any action resource?

## Class-Specific Features

72. **Rage activation and tracking.** How does a Barbarian enter rage (`/bonus rage`?)? How do they end it? Rage ends if they don't attack or take damage — is this auto-tracked?
73. **Wild Shape.** Not mentioned anywhere. Replaces stats, HP, and available actions. Massive mechanical change with no workflow.
74. **Metamagic.** Sorcerer's Twinned Spell, Quickened Spell, Subtle Spell, etc. — not mentioned. These fundamentally alter how spells work.
75. **Action Surge.** Fighter gets a whole additional action. How is this expressed? Does the system grant another full action's worth of attacks?
76. **Bardic Inspiration.** A Bard grants a die to an ally. How is this targeted and applied?
77. **Divine Smite on hit.** The Feature Effect System has `resource_on_hit`, but does the system prompt "Apply Divine Smite?" after hitting? Or must the player pre-declare it?
78. **Evasion.** Rogue/Monk Evasion (DEX save success = no damage, fail = half) — is this implemented?
79. **Cunning Action: Hide.** Is hiding as a bonus action supported (`/bonus cunning-action hide`)? Hide is not listed as a Cunning Action option.
80. **Channel Divinity.** Mentioned as a short-rest recharge example but specific options (Turn Undead, class-specific) have no implementation guidance.

## Conditions & Status Effects

81. **No `/status` command.** How does a player see what conditions currently affect their character?
~~82. **Condition application notification.** When a condition is applied, where does the player see it? Only in `#combat-log`?~~ — Resolved: Added Combat Log Output Reference section with explicit formats for condition application ("⚠️ Aria is now Grappled") and removal ("✅ Grappled removed from Aria"). All posted to #combat-log.
83. **Frightened source indicator.** The frightened condition tracks `source_combatant_id` but the player has no indication of which creature they are frightened of or which direction they cannot move.
84. **Grapple dragging.** The grappler can drag at half speed, but there is no drag command. Does `/move` automatically drag the grappled creature?
85. **Invisible condition.** Referenced in advantage/disadvantage rules but not listed as a trackable condition.
~~86. **Prone attack penalty.** Being prone gives disadvantage on the prone creature's own attack rolls. This is not explicitly stated.~~ — Resolved: Prone attack disadvantage is now covered in the Combat Log Output Reference section's auto-detected advantage/disadvantage examples, and implicitly via the existing condition effects tables.

## Level-Up & Progression

87. **No player notification of level-up.** DM edits level in dashboard, system recalculates, but the player is never told.
88. **Multiclassing.** The `class` field is a single TEXT column. How would Fighter 5 / Rogue 3 work?
89. **Feat selection.** Some ASI levels allow feats. How is this handled? Dashboard-only?
90. **Subclass not in data model.** No subclass column on characters or classes table.
91. **XP vs milestone.** Experience points are not mentioned anywhere. Is it milestone-only?

## Map & Visual Feedback

92. **No map legend.** Is there a legend showing what colors/patterns mean (difficult terrain, walls, water, etc.)?
93. **No distance indicator.** Can a player see range rings or distance to a target without counting squares on a PNG?
94. **Token overlap at same tile.** If flying creatures share a tile at different altitudes, how are they visually distinguished on a 2D map?
95. **Spell effect visualization.** "Active effects tracked on map" but no description of how they are rendered (circles, overlays, labels?).
96. **Duplicate initials.** Player tokens show "initials." What if two players share initials?
97. **Color-blind accessibility.** Health tiers use "color shift." Are there non-color indicators for color-blind players?

## Miscellaneous

98. **Saving throw modifiers.** When prompted to `/save dex`, are all bonuses (proficiency, Paladin aura, magic items) automatically included?
99. **Magic items.** Not mentioned at all — no attunement, no magic weapon bonuses, no tracking.
100. **Carrying capacity.** Not mentioned. Can a player carry 500 arrows?
101. **Languages.** In the data model but no mechanic for understanding or not understanding speech.
102. **Darkvision and dim light.** Darkvision lets you see in darkness as dim light, but dim light imposes disadvantage on Perception. Is this auto-applied?
103. **Initiative ties.** How are ties resolved? DEX score? DM choice? Roll-off?
104. **Summoned creatures and companions.** No mechanism for familiars, animal companions, summoned creatures, or Spiritual Weapon. Who controls them?
105. **Combat recap.** In async play, a single combat can span days. Is there a recap feature for a player returning after 3 days?
106. **Simultaneous encounters.** Can two encounters run at once (party split)? The data model supports it but Discord channels (`#combat-map`, `#your-turn`) do not.
107. **Campaign pause UX.** Campaigns can be paused (`status: 'paused'`). What does the player see? Are commands disabled?
