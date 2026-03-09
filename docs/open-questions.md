# Open Questions — Player Perspective Spec Review

Gaps, ambiguities, and missing features identified by reviewing `dnd-async-discord-spec.md` from a player's point of view.

---

## Onboarding & Registration

- [x] 1. **No player-facing character creation.** — Resolved: Hybrid approach. Web-based player portal for character creation (`/create-character` links to builder) and viewing (`/character` shows sheet). `/import <ddb-url>` for self-service D&D Beyond import. All paths require DM approval. Welcome DM sent on server join with getting-started instructions.
- [x] 2. **Registration approval is opaque.** — Resolved: Immediate ephemeral confirmation on submit. Bot DMs the player when DM approves, requests changes, or rejects. Pre-approval game commands show current registration status with relative timestamp and any DM feedback, rather than a generic error.
- [x] 3. **No `/unregister` or character swap.** — Resolved: Added `/retire [reason]` command. Posts retirement request to `#dm-queue` for DM approval. On approval, character status set to `retired`, player unlinked, can then `/create-character`, `/import`, or `/register` a new character. Blocked during combat. Retired characters remain in DB; DM can re-activate from dashboard.
- [x] 4. **No onboarding message.** — Resolved: Already addressed by #1. Welcome DM sent on server join with getting-started instructions (character creation paths, DM approval flow, channel overview).
- [x] 5. **Mistyped character name on `/register`.** — Resolved: Case-insensitive exact match first; on failure, fuzzy match suggests up to 3 close names. No auto-selection — player must re-run `/register` with the correct name.

## Turn Flow & Action Economy

- [x] 6. **No explicit stand/prone toggle.** — Resolved: Added `/action stand` (costs half movement, no action) and `/action drop-prone` (no cost) as auto-resolved commands. Staying prone is the default; standing can be explicit or implicit via `/move`.
- [x] 7. **No crawl mechanic.** — Resolved: `/move` while prone prompts with Discord buttons: [Stand & Move] or [Crawl]. Crawling costs ×2 movement and keeps the character prone. Stacks with difficult terrain for ×3.
- [x] 8. **Disengage is missing from the command table.** — Resolved: Added `/action disengage` to command table and Standard Actions section. Added `/bonus cunning-action disengage` example to bonus action row for Rogue discoverability.
- [x] 9. **`/done` has no safety net.** — Resolved: `/done` shows an ephemeral confirmation prompt listing unused resources (action, bonus action, remaining attacks) before ending the turn. If all resources are spent, `/done` ends immediately with no prompt.
- [x] 10. **Movement cost is invisible.** — Resolved: `/move` shows an ephemeral confirmation prompt with path cost and remaining movement before committing. Player clicks Confirm or Cancel. Invalid moves are rejected immediately with no confirmation step.
- [x] 11. **No `/map` command.** — Resolved: Already addressed by design. `#combat-map` appends a new message on every state change, so the latest message is always the current map. No additional command needed.
- [x] 12. **No `/initiative` query command.** — Resolved: Already addressed by design, same as #11. `#initiative-tracker` is a persistent bot-managed channel showing current turn order. The latest message is always current. No additional command needed.

## Combat Mechanics

- [x] 13. **Grapple escape has no command.** — Resolved: Added `/action escape` as an auto-resolved standard action. Runs contested Athletics/Acrobatics vs grappler's Athletics. Auto-selects higher modifier; `--athletics` or `--acrobatics` to override.
- [x] 14. **No unarmed strike.** — Resolved: Added unarmed strike as a built-in pseudo-weapon in the weapons table (damage: "0", 1 + STR mod bludgeoning). Defaults when no weapon equipped. Monk Martial Arts overrides damage die via Feature Effect System.
- [x] 15. **Improvised weapons.** — Resolved: `/attack [target] improvised` auto-resolves with 1d4 + STR bludgeoning, no proficiency bonus. Thrown via `--thrown` (range 20/60). DM can retroactively adjust damage type/amount from dashboard. Tavern Brawler grants proficiency. No inventory tracking needed.
- [x] 16. **Ranged spell attacks in melee.** — Resolved: Yes, disadvantage applies to both ranged weapon attacks and ranged spell attacks per 5e RAW. Crossbow Expert removes the penalty for ranged weapon attacks only, not spell attacks.
- [x] 17. **Natural armor / unarmored defense.** — Resolved: Added `ac_formula TEXT` to characters table. When non-null (e.g., "10 + DEX + WIS"), system recalculates `ac` on ability score changes. Standard armor-based characters keep `ac_formula = NULL`. Unarmored Defense only applies when no armor is worn; system takes higher of formula vs armor AC.
- [x] 18. **Shield equip/unequip.** — Resolved: `/equip shield` and `/equip none --offhand`. Donning/doffing costs an action in combat (5e RAW), instant out of combat. `/equip` expanded with `--offhand` flag for off-hand weapon/shield management.
- [x] 19. **Dual wielding.** — Resolved: Replaced `equipped_weapon` with `equipped_main_hand` and `equipped_off_hand`. Off-hand holds second light weapon or shield; null = free hand. Two-handed/versatile requires off-hand to be empty.

## Spellcasting

- [x] 20. **Material components with gold cost.** — Resolved: Focus/component pouch assumed for non-costly components. Costly components checked in inventory first; if missing, player can buy with gold via prompt. `material_cost_gp` and `material_consumed` fields added to spells table. DM can stock components via dashboard or in-game merchant shops.
- [x] 21. **No `/prepare` command.** — Resolved: Added `/prepare` command for prepared casters (Cleric, Druid, Paladin). Shows full class spell list via paginated Discord select menus. Available out of combat; system reminds after long rest. Domain/Oath/Circle always-prepared spells shown separately, don't count against limit.
- [x] 22. **Counterspell timing in async.** — Resolved: Spell name is revealed but cast level is hidden. Player picks Counterspell slot via buttons. If slot < cast level, player rolls a spellcasting ability check (DC 10 + spell level). Enemy turn continues async; success retroactively removes spell effects.
- [x] 23. **Spell targeting syntax.** — Resolved: Already addressed in spec. All targeting (attacks, spells, shoves) uses the universal short ID system (e.g., `/cast healing-word AR`). AoE spells target coordinates (e.g., `/cast fireball D5`).
- [x] 24. **Warlock spell slots.** — Resolved: Added separate `pact_magic_slots` JSONB field for Warlocks. Pact slots recharge on short rest, are used by default for `/cast`, and exist independently from regular `spell_slots` for multiclass support. `--spell-slot` flag overrides to draw from regular pool.
- [x] 25. **Bonus action spell restriction — reverse direction.** — Resolved: Both directions enforced. Added `action_spell_cast` boolean to turns table. Casting a leveled action spell blocks subsequent bonus action spells, and vice versa.
- [x] 26. **Casting in Silence.** — Resolved: `/cast` is blocked inside Silence zones for spells with verbal or somatic components. System rejects with an explanatory message. Spells with only material components (no V or S) are unaffected.

## Movement & Positioning

- [x] 27. **Moving through occupied tiles.** — Resolved: Added full 5e occupied tile rules. Allies: pass through but can't end turn there. Hostiles: blocked unless 2+ sizes different. Can never end turn in another creature's space.
- [ ] 28. **Map readability.** On a 30x20 map at 32-48px/tile, coordinate labels may be too small. Mobile viewing could be very difficult.
- [ ] 29. **Multiple floors / z-levels.** The altitude system handles flying. What about multi-story buildings, stairs, or dungeons with different levels?
- [x] 30. **Teleportation spells.** — Resolved: Added `teleport JSONB` field to spells table. Teleportation spells bypass all path validation (no movement cost, no difficult terrain, no OA). System validates only destination occupancy, range, and line of sight (when required). Full SRD teleportation spell data included.

## Reactions

- [x] 31. **No way to view active declarations.** — Resolved: Already addressed. `/status` shows active reaction declarations. `/reaction cancel [description]` and `/reaction cancel-all` manage them. DM sees all declarations in the Active Reactions Panel.
- [x] 32. **Multiple simultaneous declarations.** — Resolved: Multiple active declarations allowed. When multiple trigger from the same event, DM picks which fires via the Active Reactions Panel in the dashboard. After one fires, rest stay dormant until reaction resets next round.
- [x] 33. **Freeform declaration ambiguity.** — Resolved: Declarations stay fully freeform. Players encouraged to use short IDs for clarity; DM resolves ambiguity using battlefield context, pinging the player for clarification if needed.
- [x] 34. **OA prompts stall combat.** — Resolved: Queue-and-continue model. Movement is not paused; hostile has until end-of-round to take the OA. If OA drops target to 0 HP, system notifies DM to retroactively correct position. No auto-invalidation.
- [x] 35. **Readied action reminders.** — Resolved: Automatic expiry notice at turn start if a readied action went unused. For readied spells, confirms concentration ended and slot was lost. No mid-round reminders; `/status` available for on-demand checks.

## Death & Healing

- [x] 36. **Death save timeout has no default rule.** — Resolved: System auto-rolls the death save on timeout using standard d20 rules. Result posted to #combat-log as "(auto-rolled — player timed out)".
- [x] 37. **Targeting unconscious allies.** — Resolved: All combatants (enemies and PCs) get short IDs (e.g., AR for Aria). Target allies the same way as enemies: `/cast healing-word AR`. Dying tokens show dimmed with heartbeat icon; dead tokens greyed out with X. Allied HP is visible as exact values.
- [x] 38. **Medicine check to stabilize.** — Resolved: Added optional target parameter to `/check`. `/check medicine AR` validates adjacency (5ft) and target is dying. Costs action in combat. DC 10; failure has no penalty.
- [x] 39. **Spare the Dying.** — Resolved: `/cast spare-the-dying AR` uses standard `/cast` flow. Validates touch range (adjacent, 5ft). Auto-succeeds, no roll. Grave Domain Clerics can cast at 30ft via Feature Effect System.
- [x] 40. **Massive damage / instant death.** — Resolved: Added full combat log output formats for all death-related events. Instant death shows overflow damage vs max HP. Distinct messages for dropping to 0 HP (dying), instant death, death save results (including nat 1/nat 20), damage at 0 HP, and healing from 0 HP.

## Inventory & Equipment

- [x] 41. **Inventory management is "future phases" but MVP depends on it.** — Resolved: Full inventory system in MVP. Added `/inventory`, `/use`, `/give`, `/loot` commands, gold tracking, consumable auto-resolution, and post-combat loot pool. Moved from Future Phases to Included.
- [x] 42. **No `/inventory` command.** — Resolved: `/inventory` shows ephemeral message with items grouped by type, equipped status, and gold.
- [x] 43. **No looting workflow.** — Resolved: DM populates loot pool from dashboard after encounter ends. Players `/loot` to claim items via Discord buttons. Gold can be split evenly.
- [x] 44. **No consumables system.** — Resolved: `/use healing-potion` consumes item and auto-resolves effects. Costs action in combat (DM can configure potions as bonus action). Items without defined effects route to DM queue.
- [x] 45. **No gold or currency tracking.** — Resolved: Added `gold INTEGER` field to characters table. All currency simplified to gold pieces. DM manages via dashboard; changes logged to combat log.
- [x] 46. **Equipping armor timing.** — Resolved: Blocked in combat ("can't don/doff armor during combat"), instant out of combat with no time tracking. DM can override from dashboard. `/equip [armor] --armor` and `/equip none --armor` added.

## Out-of-Combat Gameplay

- [ ] 47. **No structured exploration mode.** No marching order, trap detection, or environmental interaction system.
- [x] 48. **Secret skill checks.** — Resolved: All rolls remain public in `#roll-history`. Players are trusted not to metagame; the DM controls information flow through narration in `#the-story`. No secret roll mechanism needed.
- [ ] 49. **No downtime activities.** Crafting, training, research not mentioned.
- [x] 50. **Rest interruption is vague.** — Resolved: 5e RAW rules with clear player messaging. Short rest interrupted: no benefits. Long rest interrupted: short rest benefits if 1+ hour elapsed, otherwise nothing. Combat ≤1 hour doesn't break long rest. Bot notifies player with reason and outcome.
- [x] 51. **Short rest hit dice command.** — Resolved: Replaced `/spend-hd` with a Discord button menu prompt during the `/rest short` flow. No separate command needed; player selects how many hit dice to spend via buttons.

## Communication

- [x] 52. **No in-character channel for players.** — Resolved: Added `#in-character` channel for player roleplay, dialogue, and actions outside combat. `#the-story` stays DM-write-only narration. Flow: players act in `#in-character` → DM narrates in `#the-story` → players react. During combat, IC speech remains in `#your-turn`.
- [x] 53. **No private DM whisper.** — Resolved: Added `/whisper` command. Ephemeral to the player, posts structured message to `#dm-queue`. DM replies from dashboard, bot sends reply as Discord DM to the player. Supports back-and-forth via additional `/whisper` messages.
- [ ] 54. **No private player-to-player communication.** Characters cannot whisper to each other without the whole party seeing.
- [x] 55. **DM notification workflow.** — Resolved: `#dm-queue` expanded to be the DM's single notification hub. Bot posts structured messages and pings DM for every event requiring attention (freeform actions, rest requests, skill check narrations, enemy turns, reaction declarations, unresolved consumables). Each notification includes context and a dashboard link. Resolved items are marked ✅.

## Information Asymmetry & Visibility

- [x] 56. **Enemy AC exposure contradiction.** — Resolved: Enemy AC is hidden. Attack results show only Hit/Miss without revealing the target's AC. Fixed the miss example to omit AC.
- [x] 57. **Character card contents undefined.** — Resolved: Defined character card contents in spec. Each card shows: name, short ID, level, race, class, HP, AC, speed, ability scores, equipped weapons, spell slots, active conditions with duration, concentration, temp HP, exhaustion, and gold. Auto-updated on state changes.
- [x] 58. **Allied HP visibility.** — Resolved: Already addressed in spec. Player character HP is visible to all players as exact values in `#initiative-tracker`, character tokens, and `#character-cards`. Allied HP and conditions are public information.
- [x] 59. **Creature identification.** — Resolved: Already addressed in spec. Players see full creature names in `#initiative-tracker` (e.g., "Goblin #1 (G1)") and `#combat-log`. Map tokens show short IDs only (G1, OS) for space. Names and IDs are cross-referenced everywhere.
- [x] 60. **Hidden enemy detection.** — Resolved: Already addressed by existing mechanics. Players use `/action search for hidden enemies` (freeform action → DM queue) or `/check perception` to actively search. DM resolves via dashboard using active Perception vs Stealth. Passive Perception handles automatic detection.

## Error Recovery

- [x] 61. **No player undo.** — Resolved: `/undo` command posts a structured correction request to `#dm-queue` with last action details and optional reason. DM reviews and applies undo from dashboard. No automatic reversal.
- [ ] 62. **Disconnection handling.** If a player's Discord client crashes mid-turn, the turn timer continues. Any reconnection awareness?
- [x] 63. **Invalid command feedback.** — Resolved: Already addressed throughout the spec. All error messages use a consistent `❌ [reason]` format as ephemeral replies (e.g., "❌ Not enough movement — path requires 40ft", "❌ You can't move — you are grappled"). Dozens of examples across all command sections.
- [x] 64. **Cancelling queued freeform actions.** — Resolved: Added `/action cancel` to withdraw pending freeform actions before DM resolves them. DM queue message marked as cancelled. Rejected if no pending action or already resolved. Full `/help action` output added.

## Notifications & Awareness

- [ ] 65. **No backup notification system.** Discord mobile notifications can be unreliable. Any fallback (email, webhook)?
- [x] 66. **Turn reminder content.** — Resolved: Two-tier approach. 50% reminder is a light nudge with `/recap` hint. 75% final warning includes full tactical summary (HP, conditions, remaining resources, adjacent enemies) so the player can act immediately.
- [x] 67. **Between-turn awareness.** — Resolved: Turn-start prompt includes a personal impact summary (damage taken, conditions applied/removed, enemies entering/leaving reach, saves pending) for events since the player's last turn. Omitted if nothing affected them. Full round details available via `/recap`.
- [ ] 68. **Ping fatigue.** In 8+ combatant fights, `#your-turn` could get very noisy with OA prompts, save prompts, reaction prompts. Any batching or filtering?

## Bonus Actions & Free Actions

- [x] 69. **Bonus action spell casting syntax.** — Resolved: Auto-detected. `/cast` checks `spells_ref.casting_time`; bonus action spells deduct the bonus action automatically. No `/bonus cast` syntax exists.
- [x] 70. **Monk ki abilities.** — Resolved: Full ki system with ki points tracked in `feature_uses["ki"]` (recharges on short rest). `/bonus flurry-of-blows` (2 unarmed strikes, 1 ki), `/bonus patient-defense` (Dodge, 1 ki), `/bonus step-of-the-wind` (Dash/Disengage, 1 ki). Stunning Strike as post-hit prompt (1 ki, CON save or Stunned). Martial Arts bonus attack (`/bonus martial-arts`, free). Full `/help ki` output defined.
- [x] 71. **Free action speaking.** — Resolved: Players type plain messages (non-commands) in `#your-turn` for in-character speech during combat — free, no action cost. Out-of-character goes in `#player-chat`. Bot ignores non-command messages. No new command needed.

## Class-Specific Features

- [x] 72. **Rage activation and tracking.** — Resolved: `/bonus rage` activates rage (costs bonus action). `is_raging` tracked on combatants table with round countdown and per-round attack/damage flags. Auto-ends if no attack or damage taken, on unconscious, or after 10 rounds. `/bonus end-rage` to end early. Blocks spellcasting and drops concentration.
- [x] 73. **Wild Shape.** — Resolved: `/bonus wild-shape [beast]` auto-resolves with CR/level validation, stat swap from creatures table, dual HP pool with overflow damage on revert. `/bonus revert` to end early. Spellcasting blocked (except Beast Spells at 18). Edge cases (equipment in form, speaking) route to DM queue.
- [x] 74. **Metamagic.** — Resolved: All 8 SRD Metamagic options auto-resolved via flags on `/cast` (e.g., `--quickened`, `--twinned`, `--subtle`). Sorcery points tracked in `feature_uses`. Font of Magic (`/bonus font-of-magic`) for slot ↔ point conversion. Full `/help metamagic` output defined. Quickened Spell still subject to bonus action spell restriction.
- [x] 75. **Action Surge.** — Resolved: Added `/action surge` command for Fighters. Resets action and attacks remaining, tracked via `action_surged` on turns table. 1 use per short rest (2 at level 17). Action Surge availability shown in turn status prompt.
- [x] 76. **Bardic Inspiration.** — Resolved: `/bonus bardic-inspiration [target]` grants die to ally (CHA mod uses, long rest recharge, short rest at 5+). Die scales d6→d8→d10→d12 by level. Auto-prompt on recipient's attack rolls, checks, and saves. Shown in turn status prompt so players never forget it. 10-minute expiration with DM override for async.
- [x] 77. **Divine Smite on hit.** — Resolved: Post-hit ephemeral prompt with Discord buttons showing available slot levels. Paladin picks slot or declines. Smite dice doubled on crits, +1d8 vs undead/fiend. Uses `resource_on_hit` effect type pattern for future on-hit features.
- [x] 78. **Evasion.** — Resolved: Already addressed via the Feature Effect System. Evasion is explicitly listed as an `on_save` trigger effect and named as an auto-detected class feature driven by effect declarations.
- [x] 79. **Cunning Action: Hide.** — Resolved: Added `/bonus cunning-action hide` as a bonus action Hide for Rogues (level 2+). Uses same Stealth vs passive Perception flow as `/action hide`. Added to command table, Stealth & Hiding section, and full `/help rogue` output covering all Rogue abilities.
- [x] 80. **Channel Divinity.** — Resolved: `/action channel-divinity [option]` with uses tracked in `feature_uses["channel-divinity"]` (short rest recharge). Turn Undead auto-resolved (WIS save, Turned condition, Destroy Undead by CR at 5+). Subclass options auto-resolved via Feature Effect System when mechanical, DM-resolved when narrative. Full `/help cleric` and `/help paladin` outputs defined.

## Conditions & Status Effects

- [x] 81. **No `/status` command.** — Resolved: Added `/status` command (ephemeral summary of active conditions, concentration, temp HP, exhaustion, reaction declarations). Also defined `#character-cards` as auto-updated persistent character state including conditions. Both work together: cards for passive awareness, `/status` for on-demand query.
- [x] 82. **Condition application notification.** — Resolved: Added Combat Log Output Reference section with explicit formats for condition application ("⚠️ Aria is now Grappled") and removal ("✅ Grappled removed from Aria"). All posted to #combat-log.
- [x] 83. **Frightened source indicator.** — Resolved: Already addressed in spec. Combat log shows source on application ("⚠️ Aria is now Frightened (source: Orc Shaman)"), movement rejections name the source, and condition metadata tracks `source_combatant_id`.
- [x] 84. **Grapple dragging.** — Resolved: `/move` detects grappled targets and prompts with Discord buttons to Drag (×2 movement cost, targets follow) or Release & Move (breaks grapple, normal speed).
- [x] 85. **Invisible condition.** — Resolved: Added Invisible as a full trackable condition in `conditions_ref`, distinct from `is_visible` (stealth). Documented mechanical effects, interaction with hiding, and auto-removal for non-Greater Invisibility on attack/cast.
- [x] 86. **Prone attack penalty.** — Resolved: Prone attack disadvantage is now covered in the Combat Log Output Reference section's auto-detected advantage/disadvantage examples, and implicitly via the existing condition effects tables.

## Level-Up & Progression

- [x] 87. **No player notification of level-up.** — Resolved: Public announcement in `#the-story` ("Aria has reached Level 6!") plus private detail ping in `#your-turn` listing all mechanical changes and pending choices (ASI/feat, new spells, subclass).
- [x] 88. **Multiclassing.** — Resolved: Replaced `class TEXT` with `classes JSONB` array of `{class, subclass, level}` entries. `level` kept as cached total. `hit_dice_remaining` changed to JSONB keyed by die size. Spell slots use 5e multiclass spellcasting table. Extra Attack doesn't stack. Spellcasting ability resolved per-spell from class list. Multiclass prereqs and proficiency subsets added to classes table.
- [x] 89. **Feat selection.** — Resolved: Interactive button prompt on level-up in `#your-turn`. Player picks ASI (+2/+1+1) or feat from paginated select menu (prereqs auto-checked). Choice posted to `#dm-queue` for DM approval. Feats stored in `feats` reference table with prerequisites and Feature Effect System declarations. Pending choices don't block gameplay.
- [x] 90. **Subclass not in data model.** — Resolved: Subclass stored per class entry in `characters.classes` JSONB (e.g., `{class: "fighter", subclass: "champion", level: 5}`). Classes table gains `subclasses JSONB` with features_by_level per subclass, and `subclass_level` indicating when subclass is chosen. Subclass features merged into character's features array.
- [x] 91. **XP vs milestone.** — Resolved: Milestone only. No XP tracking, no XP fields in the data model. DM decides when characters level up based on story progression.

## Map & Visual Feedback

- [ ] 92. **No map legend.** Is there a legend showing what colors/patterns mean (difficult terrain, walls, water, etc.)?
- [ ] 93. **No distance indicator.** Can a player see range rings or distance to a target without counting squares on a PNG?
- [ ] 94. **Token overlap at same tile.** If flying creatures share a tile at different altitudes, how are they visually distinguished on a 2D map?
- [ ] 95. **Spell effect visualization.** "Active effects tracked on map" but no description of how they are rendered (circles, overlays, labels?).
- [x] 96. **Duplicate initials.** — Resolved: Already addressed in spec. Short IDs are derived from character name initials; duplicates are disambiguated by appending a number (e.g., AR and AR1). Defined in the Combatant Targeting section.
- [ ] 97. **Color-blind accessibility.** Health tiers use "color shift." Are there non-color indicators for color-blind players?

## Miscellaneous

- [x] 98. **Saving throw modifiers.** — Resolved: Yes, all modifiers auto-included (proficiency, ability mod, feature bonuses, spell effects, conditions, magic items). Combat log shows full breakdown so players can verify the math.
- [x] 99. **Magic items.** — Resolved: Full magic item system. `magic_items` reference table with rarity, attunement, passive effects (Feature Effect System), active abilities (charges with recharge-at-dawn), and `magic_bonus` for +N weapons/armor/shields. `/attune` and `/unattune` commands with 3-slot limit. Inventory display shows rarity, attunement status, and charges. Unidentified items supported via DM toggle.
- [ ] 100. **Carrying capacity.** Not mentioned. Can a player carry 500 arrows?
- [ ] 101. **Languages.** In the data model but no mechanic for understanding or not understanding speech.
- [x] 102. **Darkvision and dim light.** — Resolved: DM-placed lighting/obscurement zones with auto-applied combat modifiers. Dim light → Perception disadvantage; darkness → Blinded effects. Darkvision downgrades zones (darkness→dim, dim→bright). Integrated into advantage/disadvantage auto-detection pipeline. Combat log shows lighting modifiers.
- [x] 103. **Initiative ties.** — Resolved: DEX modifier tiebreaker, then alphabetical by name. Fully automatic, no DM input needed.
- [x] 104. **Summoned creatures and companions.** — Resolved: All summoned creatures are player-controlled via `/command [creature-id] [action]`. Creatures get their own combatant entries with short IDs, initiative placement per spell rules, and standard turn resource tracking. `summoner_id` field links creatures to their summoner.
- [x] 105. **Combat recap.** — Resolved: `/recap` command shows `#combat-log` entries since the player's last turn (or `/recap N` for last N rounds) as an ephemeral message. Direct replay of combat log grouped by round/turn — no summarization. Works during or after combat from any channel.
- [x] 106. **Simultaneous encounters.** — Resolved: Shared channels with encounter-labeled messages. All bot messages to combat channels are prefixed with the encounter name and round number. Multiple encounters interleave in the same `#combat-map`, `#your-turn`, `#combat-log`, and `#initiative-tracker` channels. Players read the labels to identify relevant messages. Per-turn locks scoped per encounter so commands don't block across encounters.
- [ ] 107. **Campaign pause UX.** Campaigns can be paused (`status: 'paused'`). What does the player see? Are commands disabled?
