# Async D&D via Discord — System Specification

## Overview

A system for running D&D 5e asynchronously through Discord. Players and the DM take turns through chat on their own schedule — no shared session required. Game state is managed through a backend, surfaced into Discord via a bot, and controlled by the DM through a dedicated web dashboard.

---

## Core Architecture

### Mental Model

Discord is the **display and input terminal**, not the source of truth. The system has three distinct layers:

- **Game State Layer** — a backend database that owns all truth: HP, positions, initiative, inventory, spell slots, conditions
- **DM Tool Layer** — a web dashboard where the DM manages combat and assets
- **Discord Layer** — where players read output and submit commands

### Guiding Rule
> Game state lives in the database. Discord is read-only output. Any state change goes through the bot or DM dashboard — never manually through Discord.

### Authentication & Authorization

**Discord user → Character mapping:**
- Player runs `/register <character_name>` in the Discord server
- Bot creates a `discord_user_id → character_id` mapping in the database
- DM confirms/approves the registration via the dashboard
- One player = one character per campaign (DM can override in dashboard if needed)

**Out-of-turn prevention:**
- On every combat command (`/move`, `/attack`, `/cast`, `/bonus`, `/interact`, `/action`, `/done`, `/deathsave`), the backend validates that the requesting Discord user ID matches the active turn's character owner
- If not their turn, bot replies: "It's not your turn. Current turn: **[Character]** (@player)"
- Exception: `/reaction` can be submitted at any time — it's a declaration, not a turn action
- Exception: `/check`, `/save` are usable any time (in or out of combat, no turn required — DM may call for checks mid-combat)
- Exception: `/rest` operates outside active combat only

**DM dashboard authentication:**
- Discord OAuth2 — DM logs in with their Discord account
- System verifies the authenticated Discord user ID matches the campaign's designated DM
- No separate passwords or accounts to manage

**Multi-campaign support:**
- One bot instance serves multiple Discord servers (multi-tenant)
- All database queries are scoped by `guild_id` / `campaign_id`
- One campaign per Discord server (keeps channel structure clean and unambiguous)

### Concurrency Model

All combat state mutations are serialized through a **per-turn pessimistic lock** using PostgreSQL advisory locks keyed on `turn_id`.

**Rapid player commands** (e.g., two `/attack` commands sent before the first resolves):
- Each command acquires the turn lock before processing
- If the lock is held, the second command **blocks** (waits) rather than failing
- Once the first command completes and releases the lock, the second processes against updated state
- Player experiences at most a few hundred milliseconds of delay — no "conflict, retry" errors

**DM + player concurrent actions:**
- DM dashboard mutations go through the same backend and acquire the same per-turn lock
- If the DM ends a player's turn, any in-flight player command either completes first (if it holds the lock) or fails with "It's no longer your turn"

**WebSocket state sync** — server-authoritative, push-only:
- The Go backend + PostgreSQL is the single source of truth
- The DM dashboard renders state received via WebSocket pushes; it does not maintain its own game state copy
- Flow: command → backend acquires lock → resolves → releases lock → pushes state over WebSocket → dashboard re-renders

**Dashboard optimistic UI** — when a WebSocket push arrives while the DM has a form open:
- Read-only display areas (initiative tracker, HP bars) update immediately
- Active form inputs are **not clobbered** — the DM's draft is preserved with a subtle indicator: "HP updated to 3 by player action"

---

## Discord Server Structure

```
📋 SYSTEM
  #initiative-tracker   ← bot posts/edits current turn order
  #combat-log           ← mechanical results ("Goblin #1 rolled 14 to hit Aria — HIT")
  #roll-history         ← all dice rolls, timestamped

🎭 NARRATION
  #the-story            ← DM narration only, clean prose
  #player-chat          ← out-of-character chatter

⚔️ COMBAT
  #combat-map           ← bot posts the grid image each turn
  #your-turn            ← bot pings the active player; players submit commands here

🎒 REFERENCE
  #character-cards      ← auto-updated character info per player
  #dm-queue             ← freeform player actions awaiting DM resolution
```

### Bot Permissions

- `Send Messages` — post to all bot-managed channels
- `Attach Files` — upload map PNGs and text file attachments
- `Manage Messages` — pin/edit the initiative tracker message
- `Use Application Commands` — register and respond to slash commands
- `Mention Everyone` — ping players on turn start (or a configurable `@combat` role)

### Server Setup

The `/setup` slash command auto-creates the channel structure with appropriate permission overrides (e.g., `#the-story` is DM-write-only, `#combat-map` is bot-write-only). DM runs `/setup` once after inviting the bot. Channels that already exist are skipped.

The bot uses **plain text messages** for most output. For very large output (20+ combatant initiative orders, detailed character cards), the bot uploads a **text file attachment** instead. No embeds required.

---

## Player Input Model

### Enemy Targeting

Enemies are assigned short stable IDs at the start of combat. Unique enemies get an abbreviated code; duplicates are numbered.

```
Goblin #1  (G1)  [B4]  Uninjured
Goblin #2  (G2)  [C6]  Bloodied
Orc Shaman (OS)  [D5]  Uninjured
```

**Enemy HP is hidden from players.** The bot never reveals exact HP, AC, or stat block details in player-visible channels. Instead, enemies show a **descriptive health tier:**
- **Uninjured** — 100% HP
- **Scratched** — 75–99% HP
- **Bloodied** — 25–74% HP (standard 5e convention: below half)
- **Critical** — 1–24% HP
- **Dying / Dead** — 0 HP

The DM sees exact numbers in the dashboard. IDs are stable for the entire encounter — if G1 dies, G2 stays G2. Token labels on the map display these IDs so players can target by sight.

### Grid Movement

The map uses a standard chess-style coordinate grid: **columns as letters, rows as numbers**. Columns use spreadsheet-style lettering: A–Z for the first 26, then AA, AB, … AZ, BA, BB, … for larger maps.

```
  A   B   C   D   E   F
1 [ ] [ ] [ ] [ ] [ ] [ ]
2 [ ] [G1] [ ] [ ] [ ] [ ]
3 [ ] [ ] [AR] [ ] [G2] [ ]
4 [ ] [ ] [ ] [OS] [ ] [ ]
```

Movement is expressed as a destination coordinate:

```
/move D4
/move E6
/move AA12     ← valid on maps wider than 26 columns
```

The backend validates every movement command: remaining speed, tile occupancy, difficult terrain, and obstacles. Invalid moves return a specific reason. Movement can be split across actions: `/move D4` → `/attack G1` → `/move E5` — each `/move` deducts from remaining speed.

**Diagonal movement:** diagonals cost 5ft, same as cardinal movement. Deliberate simplification for async play — the PHB alternating 5/10 variant is not supported.

### Altitude & Elevation

Tokens carry an **altitude** value (integer feet, default 0) representing height above the ground plane.

```
/fly 30        ← rise to 30ft altitude (costs 30ft of movement)
/fly 0         ← descend to ground level
/move D4       ← horizontal movement while maintaining current altitude
```

- Ascending and descending costs movement **1:1**
- Altitude displayed as a **label suffix**: `AR↑30` means Aria at 30ft
- **Distance calculation** uses 3D Euclidean distance (rounded to nearest 5ft) for range checks
- Tokens at different altitudes **do not block** each other's ground tile
- Falling: if a flying creature is knocked prone or loses fly speed, fall damage is 1d6 per 10ft (standard 5e), applied automatically

### Structured Commands

Players submit slash commands in `#your-turn` (where they receive their turn ping), but commands work from any channel in the server — the bot validates and routes output to the correct channels regardless.

**Combat commands** (only usable on your turn, except `/reaction`):

| Command | Example | Description |
|---|---|---|
| `/move` | `/move D4` | Move to coordinate. Repeatable for split movement as long as total ≤ speed |
| `/fly` | `/fly 30` | Set altitude in feet. Costs movement 1:1 |
| `/attack` | `/attack G2` or `/attack G2 handaxe --gwm` or `/attack G2 --twohanded` | Attack a target. One `/attack` per swing; backend tracks attacks remaining. `--twohanded` for versatile weapons |
| `/cast` | `/cast fireball D5` or `/cast fireball D5 --slot 5` or `/cast detect-magic --ritual` | Cast a spell at a target coordinate or enemy ID. `--slot N` to upcast; `--ritual` for ritual casting |
| `/bonus` | `/bonus cunning-action dash` or `/bonus cunning-action disengage` | Bonus action |
| `/shove` | `/shove OS` | Shove a target (push or knock prone) |
| `/interact` | `/interact draw longsword` | Object interaction (first per turn is free; see Free Object Interaction) |
| `/action disengage` | `/action disengage` | Disengage — move without provoking opportunity attacks (auto-resolved) |
| `/action dash` | `/action dash` | Dash — double movement this turn (auto-resolved) |
| `/action dodge` | `/action dodge` | Dodge — attacks against you have disadvantage until next turn (auto-resolved) |
| `/action help` | `/action help Thorn G1` | Help — grant ally advantage on next attack/check (auto-resolved) |
| `/action ready` | `/action ready I attack when the goblin moves past me` | Ready — hold action for a trigger (see Reactions) |
| `/action hide` | `/action hide` | Hide — Stealth vs passive Perception (auto-resolved) |
| `/action escape` | `/action escape` or `/action escape --acrobatics` | Escape a grapple — contested check (auto-resolved) |
| `/action` | `/action flip the table` | Freeform action — routed to `#dm-queue` |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare reaction intent (usable any time) — routed to `#dm-queue` |
| `/deathsave` | `/deathsave` | Roll a death saving throw (only at 0 HP) |
| `/done` | `/done` | End turn, advance initiative |

**Anytime commands** (usable in or out of combat, no turn required):

| Command | Example | Description |
|---|---|---|
| `/check` | `/check perception` or `/check athletics --adv` | Skill/ability check (DM-prompted or player-initiated) |
| `/save` | `/save dex` | Saving throw (DM-prompted) |
| `/rest` | `/rest short` or `/rest long` | Initiate a rest (DM must approve, not during combat) |

Note: saving throws triggered by spells and attacks (e.g., Fireball's DEX save) prompt affected players to roll via `/save` — the bot pings them in `#your-turn`. Enemy saves are rolled by the DM from the dashboard.

**Utility commands** (usable any time):

| Command | Example | Description |
|---|---|---|
| `/equip` | `/equip longsword` | Set primary weapon (persists between turns) |
| `/register` | `/register Thorn` | Link Discord account to a character (DM approves) |
| `/setup` | `/setup` | Auto-create channel structure (DM only, run once) |
| `/help` | `/help` or `/help attack` | Show command list, or detailed usage for a specific command |

**Command discoverability:** Discord's built-in slash command UI is the primary discovery mechanism — typing `/` shows all registered commands with parameter hints. The `/help` command supplements this with usage examples, available flags (e.g., `--gwm`, `--adv`), and context-specific tips (e.g., remaining attacks, available spell slots).

### Attack Mechanics

**Weapon selection:** each character has an equipped weapon (set via `/equip`). `/attack G2` uses it; `/attack G2 handaxe` overrides for that swing. Backend validates the character has the weapon.

**Extra Attack:** resolved one swing at a time. Each `/attack` resolves a single attack roll. Backend tracks attacks remaining by class/level (Fighter 5 = 2, Fighter 11 = 3, Fighter 20 = 4). After each swing, the bot reports remaining attacks. Players retarget freely between swings. Unused attacks forfeited on `/done`.

**Two-Weapon Fighting:** when a character attacks with a light melee weapon in their main hand (`equipped_main_hand`), they can use their bonus action to attack with a different light melee weapon held in the off-hand (`equipped_off_hand`). Invoked via `/bonus offhand`. The off-hand attack does not add the ability modifier to damage unless the character has the Two-Weapon Fighting fighting style. System validates both `equipped_main_hand` and `equipped_off_hand` have the "light" property.

**Finesse weapons:** weapons with the "finesse" property (rapier, dagger, shortsword, etc.) allow the attacker to use either STR or DEX for attack and damage rolls. The system auto-selects the higher of the two modifiers — no player input required.

**Loading property:** weapons with the "loading" property (crossbows) can only fire once per action, bonus action, or reaction regardless of Extra Attack, unless the character has the Crossbow Expert feat. System limits attacks to 1 when a loading weapon is used.

**Versatile weapons:** weapons with the "versatile" property can be used one-handed or two-handed for increased damage. Use `/attack [target] --twohanded` to roll the `versatile_damage` die instead of the base damage die. The `--twohanded` flag is rejected if `equipped_off_hand` is not null (off-hand must be free to grip with both hands).

**Reach weapons:** weapons with the "reach" property (glaive, halberd, pike) extend melee range to 10ft instead of the standard 5ft. The system validates attack distance against the weapon's reach when processing `/attack`. If the target is beyond reach, the command is rejected: "Target is out of melee range (10ft reach)."

**Heavy weapons:** weapons with the "heavy" property (greataxe, greatsword, heavy crossbow, etc.) are unwieldy for small creatures. Small or Tiny creatures have disadvantage on attack rolls with heavy weapons. Auto-detected from the creature/character's `size` (via `races.size` or `creatures.size`) and the weapon's `properties` array.

**Ammunition:** weapons with the "ammunition" property (longbow, crossbow, etc.) consume one piece of ammunition per attack. The system auto-deducts from the character's `inventory` on each `/attack` with an ammunition weapon. If no ammunition remains, the command is rejected: "No arrows remaining." After combat ends, half of expended ammunition can be recovered — the DM triggers recovery from the dashboard, and the system restores half (rounded down) to inventory.

**Thrown weapons:** weapons with the "thrown" property (handaxe, javelin, dagger, etc.) can be thrown for a ranged attack using `range_normal_ft` / `range_long_ft`. Beyond normal range: disadvantage (per standard ranged rules). Beyond long range: attack auto-rejected. After a thrown attack, the weapon is removed from the character's hand. The character must draw another weapon (free object interaction) or retrieve the thrown weapon (requires movement to the target's square + free object interaction).

**Attack modifier flags** (opt-in per swing):
- `--gwm` — Great Weapon Master: -5 to hit, +10 damage. Requires a heavy melee weapon.
- `--sharpshooter` — Sharpshooter: -5 to hit, +10 damage. Requires a ranged weapon.
- `--reckless` — Reckless Attack (Barbarian): advantage on melee STR attacks this turn, enemies get advantage against you until next turn. First attack only. Requires Barbarian class.
- Invalid flags return an error explaining why.

**Advantage/disadvantage** — auto-detected from game state:
- **From conditions:** applied automatically per the Condition Effects tables (e.g., blinded attacker → disadv, stunned target → adv, prone target within 5ft → adv / beyond 5ft → disadv). See Conditions & Combat Mechanics for full details.
- **From combat context:** Reckless Attack (adv), invisible attacker/target (adv/disadv as appropriate), ranged attack while hostile within 5ft (disadv), ranged attack beyond normal range (disadv), Small/Tiny creature using a Heavy weapon (disadv)
- **Not auto-detected in MVP:** flanking (optional rule — may add as campaign toggle later)
- **DM override:** DM can force advantage or disadvantage from the dashboard. Posts to `#combat-log`.
- **Stacking:** when both apply, they cancel out per 5e rules — rolled normally regardless of source count.

**Critical hits:** a natural 20 on the attack roll is always a hit regardless of AC, and all damage dice are doubled (roll twice as many dice, then add modifiers once). The system auto-detects nat 20s and doubles the dice in the damage formula.

**Auto-crit:** melee attacks within 5ft against paralyzed or unconscious targets are automatic critical hits (per 5e rules) — the attack auto-hits and damage dice are doubled, same as a nat 20.

### Spell Casting Details

**AoE targeting:** `/cast fireball D5` targets a coordinate. Backend calculates affected creatures by shape/radius from spell data (`{ shape: "sphere", radius_ft: 20 }`, `{ shape: "cone", length_ft: 15 }`, `{ shape: "line", length_ft: 60, width_ft: 5 }`). Cones originate from the caster toward the target. All affected creatures (including allies) listed in `#combat-log`.

**Spell saves:** when a spell requires saves, the bot pings each affected player in `#your-turn` to roll `/save <ability>`. Enemy saves are rolled by the DM from the dashboard. Spell damage/effects are applied once all saves are resolved.

**Concentration:** fully tracked by backend:
- One concentration spell at a time; new cast auto-drops the previous
- Taking damage triggers a concentration check — bot pings the caster to roll `/save con` (DC = max(10, half damage)); failure breaks concentration
- Being incapacitated (stunned, paralyzed, unconscious, petrified) auto-breaks concentration immediately — no save prompted
- Entering a Silence zone (or similar effect preventing verbal/somatic components) breaks concentration on spells requiring those components — auto-detected when a concentrating caster's position overlaps a Silence zone
- Active effects (Fog Cloud zone, Spirit Guardians aura) tracked on the map

**Bonus action spell restriction:** If a player casts a spell with a casting time of "1 bonus action" (e.g., Healing Word, Misty Step), the only other spell they can cast that turn is a cantrip with a casting time of 1 action. If they attempt `/cast` with a non-cantrip after using a bonus action spell, the command is rejected: "You already cast a bonus action spell this turn — you can only cast a cantrip with your action."

**Spell save DC:** calculated as `8 + proficiency_bonus + spellcasting_ability_modifier`. The spellcasting ability varies by class (referenced from `classes.spellcasting.ability`): INT for Wizards, WIS for Clerics/Druids/Rangers, CHA for Bards/Paladins/Sorcerers/Warlocks. Creature stat blocks store the DC directly in their abilities data.

**Spell attack rolls:** some spells require a spell attack roll instead of a saving throw. The attack roll is `d20 + proficiency_bonus + spellcasting_ability_modifier` vs the target's AC. Spells are categorized by `attack_type` on the `spells` table: `'melee'` (e.g., Shocking Grasp), `'ranged'` (e.g., Fire Bolt), or `NULL` (save-based or auto-hit). Melee spell attacks within 5ft of a prone target get advantage; ranged spell attacks against a prone target get disadvantage (same as weapon attacks).

**Spell slots:** tracked and enforced. Backend knows slots per level, deducts on cast, rejects `/cast` if no slots remaining.

**Upcasting:** spells can be cast at higher levels for increased effect by specifying `/cast fireball D5 --slot 5` to use a 5th-level slot. The system parses the spell's `higher_levels` field (e.g., "1d6 per slot level above 3rd") and auto-calculates the scaled damage or healing. If `--slot` is omitted, the system defaults to the lowest available slot of sufficient level. The `--slot` value must be ≥ the spell's base level and the character must have a slot available at that level.

**Ritual casting:** spells with `ritual: true` can be cast without expending a spell slot by using `/cast detect-magic --ritual`. Ritual casting adds 10 minutes to the casting time and is only available outside active combat (`encounter.status != 'active'`). Only classes with the Ritual Casting feature (Wizard, Cleric, Druid, Bard with Ritual Casting) can use this option. The system checks both the spell's `ritual` flag and the character's class features.

**Spell range:** enforced by backend. Touch spells require adjacency (5ft), self spells need no target.

**Cantrip damage scaling:** cantrip damage dice scale automatically based on character level — 2 dice at level 5, 3 dice at level 11, 4 dice at level 17. The system auto-calculates the correct number of dice from the caster's character level using the spell's `damage` JSONB field (`cantrip_scaling: true` flag). No player input required. Example: Fire Bolt deals 1d10 at levels 1–4, 2d10 at 5–10, 3d10 at 11–16, 4d10 at 17+.

### Reactions

Players pre-declare reaction intent using `/reaction`. The DM resolves all reactions manually.

**Declaration:**
- `/reaction Shield if I get hit`
- `/reaction OA if goblin moves away`
- `/reaction Counterspell if enemy casts`
- Declarations persist until used, cancelled (`/reaction cancel`), or the encounter ends
- One reaction per round per player (per 5e rules) — tracked by `turns.reaction_used`. Reaction resets at the start of the creature's turn (not the start of the round), matching 5e rules and ensuring correct sequencing in async play

**DM workflow:**
1. DM sees declarations in `#dm-queue` or the dashboard
2. When the trigger occurs during enemy/NPC turns, DM decides whether it fires
3. DM resolves in the dashboard (rolls, applies effects) and posts the result
4. System marks the player's reaction as spent for the round

**Readied Actions:** a player can use their action to ready a response to a trigger via `/action ready [description]` (e.g., `/action ready I attack when the goblin moves past me`). This costs the action for the turn. When the trigger occurs, the readied action fires using the creature's reaction (`reaction_used = true`). If the trigger never occurs before the creature's next turn, the readied action is lost. For readied spells: the spell slot is expended when readying (not when releasing), and the caster must hold concentration on the readied spell until the trigger fires — if concentration is broken, the spell is lost along with the slot. Readied actions follow the same DM-resolution flow as other `/reaction` declarations.

**System-generated reaction triggers:** opportunity attacks (see Opportunity Attacks section) bypass the `/reaction` declaration flow — the system auto-detects and prompts directly.

**Why this works for async:** zero stalling — combat never pauses for a reaction response. Players declare intent on their own time. DM has full control over timing and adjudication.

### Freeform Actions

For anything that can't be expressed through structured commands, `/action` routes to the DM:

```
/action I want to flip the table at B3 for half cover and duck behind it
/action I grab the chandelier and swing across to F2
```

These post to `#dm-queue`. The DM resolves them in the dashboard, applies state changes, and the bot posts the result. Structured commands handle ~90% of combat; `/action` is the escape hatch for creative play.

### Standard Actions (Auto-Resolved)

The following standard actions are recognized by `/action` and resolved automatically without routing to `#dm-queue`:

**Disengage:** `/action disengage` prevents the character from provoking opportunity attacks for the rest of the turn. Costs the action (`action_used = true`). Tracked via `has_disengaged = true` on the `turns` table. Rogues can Disengage as a bonus action via `/bonus cunning-action disengage` (costs bonus action instead). Monks can Disengage as a bonus action by spending 1 ki point via `/bonus step-of-the-wind`.

**Dash:** `/action dash` adds the character's speed to their remaining movement for the turn. Costs the action (`action_used = true`). Rogues can Dash as a bonus action via `/bonus cunning-action dash` (costs bonus action instead). Monks can Dash as a bonus action by spending 1 ki point via `/bonus step-of-the-wind`. The extra movement is subject to difficult terrain, prone costs, and all other movement modifiers.

**Dodge:** `/action dodge` grants two benefits until the start of the character's next turn: attacks against the character have disadvantage, and the character has advantage on DEX saving throws. Tracked via a "dodge" condition with 1-round duration. Already referenced in Turn Timeout (auto-skip applies Dodge).

**Help:** `/action help [ally] [target]` grants an ally advantage on their next attack roll against the specified target, or advantage on their next ability check. Costs the action. For attack help, the helper must be within 5ft of the target. The advantage applies to the next qualifying roll only, then expires. Tracked as a temporary effect: `{condition: "helped", source_combatant_id: [helper], target_combatant_id: [enemy], duration: "next_roll"}`.

**Escape:** `/action escape` allows a grappled (or creature-restrained) character to break free. Costs the action (`action_used = true`). System runs a contested check: the escaping character's Athletics (STR) or Acrobatics (DEX) vs the grappler's Athletics (STR). By default the system uses whichever of the character's two modifiers is higher; use `--athletics` or `--acrobatics` to override. On success, the grappled condition is removed and speed is restored. On failure, the character remains grappled and the action is spent. Rejected if the character is not currently grappled or restrained by a creature.

**Hide:** `/action hide` is described in the Stealth & Hiding section.

### Free Object Interaction

Each creature gets one free object interaction per turn — drawing or sheathing a weapon, opening a door, picking up a dropped item, etc. Tracked via `turns.free_interact_used`.

**Enforcement:**
- First `/interact` per turn: free (sets `free_interact_used = true`, does not cost the action)
- Second `/interact` per turn: costs the action (`action_used = true`). If the action is already spent, the command is rejected: "Free interaction already used and action is spent."
- Simple interactions that the DM has pre-flagged as auto-resolvable (draw/sheathe weapon, open unlocked door) are resolved immediately. All others route to `#dm-queue` for DM adjudication.

---

## Conditions & Combat Mechanics

Conditions and combat modifiers are auto-applied by the backend whenever a relevant command is processed. All auto-applied effects are logged to `#combat-log`.

### Condition Effects

**Effects on saving throws:**

| Condition | Effect |
|-----------|--------|
| Paralyzed | Auto-fail STR and DEX saves |
| Stunned | Auto-fail STR and DEX saves |
| Unconscious | Auto-fail STR and DEX saves |
| Petrified | Auto-fail STR and DEX saves |
| Restrained | Disadvantage on DEX saves |
| Dodge action | Advantage on DEX saves |

When a save is triggered (spell, concentration check, etc.) and the target has one of these conditions, the system auto-resolves the save as failed (or applies disadvantage/advantage) without prompting the player to roll. Dodge action grants advantage on DEX saves, making the auto-skip Dodge action (for AFK/timeout) mechanically meaningful.

**Effects on ability checks:**

| Condition | Effect |
|-----------|--------|
| Frightened | Disadvantage on ability checks while source of fear is within line of sight |
| Poisoned | Disadvantage on ability checks |
| Blinded | Auto-fail ability checks requiring sight |
| Deafened | Auto-fail ability checks requiring hearing |

Auto-applied when `/check` is used. For blinded creatures, checks that require sight (e.g., Perception checks to spot something visual) are auto-failed. For deafened creatures, checks that require hearing (e.g., Perception checks to hear something) are auto-failed. The DM flags which checks require sight or hearing via the dashboard.

**Effects on speed:**

| Condition | Effect |
|-----------|--------|
| Grappled | Speed becomes 0 |
| Restrained | Speed becomes 0 |
| Prone | Standing costs half movement speed |
| Frightened | Can't move closer to source of fear |

When a grappled or restrained condition is applied, the combatant's effective speed is set to 0. `/move` commands are rejected with an explanation (e.g., "You can't move — you are grappled"). Speed restores when the condition is removed. For frightened creatures, `/move` commands that would decrease distance to the fear source are rejected. The fear source is tracked as metadata on the frightened condition (`{source_combatant_id}`).

Standing from prone: when a prone combatant issues `/move`, the system deducts half their speed before calculating remaining movement. If insufficient movement remains, standing is still allowed but no further movement is possible.

**Action blocking:**

| Condition | Effect |
|-----------|--------|
| Incapacitated | Can't take actions or reactions |
| Stunned | Can't take actions or reactions (includes incapacitated) |
| Paralyzed | Can't take actions or reactions (includes incapacitated) |
| Unconscious | Can't take actions or reactions (includes incapacitated) |
| Petrified | Can't take actions or reactions (includes incapacitated) |

When a combatant with one of these conditions attempts `/attack`, `/cast`, `/action`, or `/reaction`, the command is rejected (e.g., "You can't act — you are stunned"). The combatant's turn is auto-skipped with a `#combat-log` message. This generalizes the unconscious-at-0-HP behavior to all incapacitating conditions.

**Charmed attack restriction:**

Charmed creatures can't attack the charmer or target them with harmful abilities. `/attack` and `/cast` (harmful) commands targeting the charm source are rejected. The charm source is tracked as metadata on the charmed condition (`{source_combatant_id}`). Non-harmful interactions with the charmer are still allowed.

### Duration Tracking & Auto-Expiration

Conditions and spell effects with limited durations are tracked via `duration_rounds` and `started_round` on each condition entry. At specific trigger points, the system checks for expired effects:

**Expiration check timing:**
- **Start of source creature's turn:** compare `encounters.round_number` against `started_round + duration_rounds`. If expired, auto-remove the condition and post to `#combat-log`: "⏱️ [Effect] on [Target] has expired (placed by [Source])."
- **End of source creature's turn:** same check, for effects with `expires_on: "end_of_turn"`.

**Duration tracking fields** (on each condition in `combatants.conditions` JSONB):
- `duration_rounds` — NULL for indefinite (e.g., grappled until escape); integer for timed effects
- `started_round` — the `encounters.round_number` when applied
- `source_combatant_id` — who applied it (determines whose turn triggers expiration)
- `expires_on` — `"start_of_turn"` (default) or `"end_of_turn"`

**Turn-start sequence** (updated):
1. Check for expired conditions/spell effects on this combatant → auto-remove and log
2. Apply start-of-turn effects (e.g., ongoing damage zones)
3. Ping player with available resources

**Indefinite conditions** (grappled, prone, charmed without duration) have `duration_rounds: NULL` and are only removed by specific actions (escape, standing up, spell ending).

### Damage Processing

**Resistance, immunity, and vulnerability:**

When damage is dealt, the system checks the target's `damage_resistances`, `damage_immunities`, and `damage_vulnerabilities` arrays:
- **Resistance**: damage of that type is halved (rounded down)
- **Immunity**: damage of that type is reduced to 0
- **Vulnerability**: damage of that type is doubled
- **Resistance + Vulnerability:** if a creature has both resistance and vulnerability to the same damage type, they cancel out — normal damage is dealt
- **Immunity precedence:** immunity always takes priority over resistance and vulnerability — damage is reduced to 0 regardless of other modifiers

Petrified creatures have resistance to all damage types, auto-applied in addition to existing resistances (no double-stacking).

**Temporary hit points:**

Temp HP acts as a damage buffer and follows these rules:
- Damage is absorbed by `temp_hp` before `hp_current` — system deducts temp HP first, then applies remainder to real HP
- Temp HP **does not stack** — if a new source grants temp HP, the creature keeps the higher value (current remaining or new grant)
- Temp HP **cannot be healed** — healing spells and effects only restore `hp_current`, never `temp_hp`
- Temp HP expires at the end of the duration specified by the granting effect (tracked via the condition/effect system), or on long rest if no duration specified

**Exhaustion (progressive condition):**

| Level | Effect |
|-------|--------|
| 1 | Disadvantage on ability checks |
| 2 | Speed halved |
| 3 | Disadvantage on attack rolls and saving throws |
| 4 | Hit point maximum halved |
| 5 | Speed reduced to 0 |
| 6 | Death |

Effects are cumulative. Exhaustion level is tracked as an integer (0–6) on the combatant. Each level's effects auto-apply:
- Levels 1, 3: system applies disadvantage to relevant rolls
- Level 2, 5: system modifies effective speed
- Level 4: system reduces max HP (and current HP if above new max)
- Level 6: character dies immediately

Exhaustion increases from forced march, starvation, environmental effects — applied by DM from dashboard. Decreases by 1 level per long rest (with food/water).

**Condition immunity:**

When a condition is being applied to a creature, the system checks `condition_immunities`. If immune, the condition is not applied and a message is posted to `#combat-log`.

### Cover

Cover is computed dynamically from map geometry — walls, obstacles, and other creatures in `tiled_json` between attacker and target. Cover is not stored on combatants; it is calculated at the moment of each attack or spell.

| Cover Type | AC Bonus | DEX Save Bonus | Determination |
|------------|----------|----------------|---------------|
| Half cover | +2 | +2 | Target is behind an obstacle that covers at least half their body (low wall, furniture, another creature) |
| Three-quarters cover | +5 | +5 | Target is behind an obstacle covering at least three-quarters of their body (portcullis, arrow slit) |
| Full cover | — | — | Target is completely concealed — cannot be targeted by attacks or spells requiring line of sight |

**Calculation (DMG grid variant):** the system picks a corner of the attacker's square and draws lines to each corner of the target's square. If 1–2 lines are blocked by obstacles, half cover applies. If 3 lines are blocked, three-quarters cover. If all 4 are blocked, full cover. The system tests from the corner that gives the attacker the best (least) cover result. Creature-granted cover (another creature between attacker and target) counts as half cover.

**Integration with attacks:** cover AC bonus is added to the target's effective AC during attack resolution. If full cover applies, `/attack` and targeted `/cast` commands are rejected: "Target has full cover — no line of sight."

**Integration with saves:** cover DEX save bonus applies to area-of-effect spells (e.g., Fireball). The system checks cover between the spell's point of origin and each affected creature.

### Difficult Terrain

Difficult terrain (rubble, undergrowth, mud, shallow water, etc.) doubles movement cost — each foot of movement in difficult terrain costs 2 feet of speed. Defined per tile in the map's `tiled_json` data using a terrain property (`difficult: true`).

**Enforcement:** on each `/move`, the system calculates the path cost including difficult terrain multipliers. If the character lacks sufficient remaining movement, the command is rejected with the shortfall: "Not enough movement — path requires 40ft (includes difficult terrain), 25ft remaining."

**Interactions:**
- Stacks with prone stand-up cost (half speed deducted first, then difficult terrain doubles remaining path cost)
- Dash action grants extra movement that is also subject to difficult terrain costs
- Difficult terrain does not affect flying creatures above ground level (altitude > 0) unless the difficult terrain is airborne (e.g., Wind Wall)
- Crawling (moving while prone without standing) also costs double, stacking with difficult terrain for 3x total cost

### Opportunity Attacks

When a creature moves out of a hostile creature's melee reach without taking the Disengage action, the system auto-detects the trigger and prompts the hostile for an opportunity attack.

**Detection logic:**
1. On each `/move` command, the system checks whether the moving creature leaves any hostile creature's threatened area
2. Threatened area = `reach_ft` from the hostile's position (default 5ft; 10ft for reach weapons)
3. If the moving creature used `/action disengage` (costs their action) or `/bonus cunning-action disengage` (Rogue Cunning Action), no opportunity attacks are triggered — the system tracks `has_disengaged` on the `turns` table

**Resolution:**
- For **player-controlled hostiles:** the bot pings the player in `#your-turn` with a reaction prompt: "⚔️ [Enemy] is moving out of your reach — use your reaction for an opportunity attack? `/reaction oa [target]`"
- For **DM-controlled hostiles:** the dashboard surfaces the opportunity attack option; DM confirms or declines
- Opportunity attacks use the hostile's reaction for the round (`reaction_used = true`)
- Movement is paused at the trigger point until the opportunity attack is resolved (hit or declined), then continues

**Interaction with reactions:** opportunity attacks are system-generated reaction triggers — they consume the creature's reaction and follow the same one-per-round limit.

### Grapple & Shove

Grapple and shove are contested ability check actions available via `/action grapple [target]` and `/shove [target]`.

**Grapple:**
- Requires a free hand (system checks equipped items — rejected if both hands occupied)
- Target must be no more than one size category larger than the grappler
- Contested check: attacker Athletics (STR) vs target's choice of Athletics (STR) or Acrobatics (DEX)
- Success: target gains the "grappled" condition (`{condition: "grappled", source_combatant_id: [grappler]}`)
- Grappled creature's speed becomes 0; grappler can drag the target at half speed
- Escape: target uses `/action escape` (costs their action) to repeat the contested check; success removes the grappled condition. See Standard Actions below

**Shove:**
- Target must be no more than one size category larger
- Contested check: attacker Athletics (STR) vs target's choice of Athletics (STR) or Acrobatics (DEX)
- Attacker chooses mode via flag: `/shove OS --prone` or `/shove OS --push`
  - `--prone`: target is knocked prone
  - `--push`: target is pushed 5ft away from the attacker (system validates destination is unoccupied)

### Stealth & Hiding

The Hide action allows a creature to become unseen during combat.

**Hiding:**
- Player uses their action: `/action hide`
- System rolls Stealth (DEX) check for the hider vs **passive Perception** of each hostile creature with line of sight
- If the Stealth result beats all hostiles' passive Perception: `is_visible` is set to `false` on the combatant; token is hidden from the player-facing map
- If any hostile's passive Perception meets or exceeds the roll: hide fails, `is_visible` remains `true`

**While hidden:**
- Attacks against the hidden creature have disadvantage (attacker can't see target)
- The hidden creature's first attack has advantage (unseen attacker)
- After attacking (hit or miss), the creature is automatically revealed: `is_visible = true`
- Making noise (casting a spell with verbal components, etc.) may reveal the creature — DM adjudicates via dashboard

**Passive Perception:** calculated as `10 + Perception modifier` (including proficiency if proficient). Stored on the combatant for quick lookup.

### Equipment Enforcement

The system enforces armor-related penalties automatically during combat.

**Heavy armor strength requirement:**
- If a character's STR score is below their equipped armor's `strength_req`, their effective speed is reduced by 10ft
- Checked when combat begins and when equipment changes
- Example: Plate armor requires STR 15; a character with STR 13 moves at `speed_ft - 10`

**Stealth disadvantage:**
- Armor with `stealth_disadv: true` (e.g., chain mail, plate) automatically applies disadvantage to Stealth checks
- Applied when the character uses `/action hide` or rolls `/check stealth`
- Stacks with other sources of disadvantage per standard 5e rules (multiple disadvantage sources still result in a single disadvantage)

---

## Feature Effect System

Class features, racial traits, creature abilities, and spell effects share a common data-driven resolution system. Rather than hardcoding each feature individually, features declare their mechanical effects using a vocabulary of **effect types** and **trigger points**. The combat engine processes these declarations at the appropriate moments.

### Effect Types

Each feature declares one or more effects from this vocabulary:

| Effect Type | Description | Example |
|-------------|-------------|---------|
| `modify_attack_roll` | Add/subtract from attack rolls | Archery fighting style (+2 ranged) |
| `modify_damage_roll` | Add/subtract flat damage | Rage damage bonus (+2/+3/+4) |
| `extra_damage_dice` | Add extra dice to damage | Sneak Attack (1d6–10d6), Divine Smite (2d8) |
| `modify_ac` | Add/subtract from AC | Shield of Faith (+2), Unarmored Defense |
| `modify_save` | Add/subtract from saving throws | Aura of Protection (CHA mod to saves) |
| `modify_check` | Add/subtract from ability checks | Guidance (+1d4), Jack of All Trades |
| `modify_speed` | Add/subtract from speed | Barbarian Fast Movement (+10ft) |
| `grant_resistance` | Grant damage type resistance | Rage (resistance to B/P/S), Tiefling (fire) |
| `grant_immunity` | Grant condition or damage immunity | Paladin Aura of Courage (immunity to frightened) |
| `extra_attack` | Grant additional attacks per action | Extra Attack, Thirsting Blade |
| `modify_hp` | Modify HP maximum or temporary HP | Tough feat (+2 per level), Heroism (temp HP) |
| `conditional_advantage` | Grant advantage/disadvantage on rolls | Reckless Attack (advantage on STR melee) |
| `resource_on_hit` | Trigger resource use on hit/damage | Battlemaster maneuvers, Divine Smite on hit |
| `reaction_trigger` | Define automatic reaction prompts | Shield spell, Uncanny Dodge, Sentinel |
| `aura` | Apply effects to creatures within radius | Paladin auras, Spirit Guardians |

### Trigger Points

Effects are evaluated at specific moments during combat resolution:

| Trigger | When Evaluated | Examples |
|---------|---------------|----------|
| `on_attack_roll` | After rolling d20, before comparing to AC | Archery, conditional advantage |
| `on_damage_roll` | After hit confirmed, calculating damage | Rage bonus, Sneak Attack |
| `on_take_damage` | When receiving damage, before applying | Resistance, Uncanny Dodge |
| `on_save` | When making a saving throw | Aura of Protection, Evasion |
| `on_check` | When making an ability check | Jack of All Trades, Expertise |
| `on_turn_start` | At the start of the creature's turn | Regeneration, ongoing damage |
| `on_turn_end` | At the end of the creature's turn | End-of-turn saves, aura damage |
| `on_rest` | When completing a short or long rest | Feature use recharge |

### Condition Filters

Each effect can specify conditions that must be true for it to apply:

```json
{
  "effect_type": "modify_damage_roll",
  "modifier": 2,
  "conditions": {
    "when_raging": true,
    "attack_type": "melee",
    "ability_used": "str"
  }
}
```

Common condition filters:
- `when_raging`, `when_concentrating` — active state checks
- `weapon_property: "heavy"` / `"finesse"` / `"ranged"` — weapon type constraints
- `attack_type: "melee"` / `"ranged"` — attack category (includes both weapon and spell attacks of that type)
- `ability_used: "str"` / `"dex"` — ability score used for the roll
- `target_condition: "frightened"` — target must have a specific condition
- `ally_within: 5` — an ally within N feet of the target (for Sneak Attack, Pack Tactics)
- `uses_remaining: true` — feature has remaining uses (checked against `feature_uses`)

### Resolution Priority

When multiple effects apply to the same roll, they are resolved in this order:

1. **Immunities** — condition immunities prevent effects from applying
2. **Resistances / Vulnerabilities** — damage type modifications (see Damage Processing for cancel/immunity rules)
3. **Flat modifiers** — summed (+2 from Archery, +CHA from Aura of Protection)
4. **Dice modifiers** — additional dice added (Sneak Attack, Divine Smite)
5. **Advantage / Disadvantage** — cancel out if both apply (see Attack Mechanics for details)

### Example Declarations

**Rage (Barbarian):**
```json
{
  "feature": "Rage",
  "effects": [
    {"type": "modify_damage_roll", "modifier": 2, "trigger": "on_damage_roll",
     "conditions": {"when_raging": true, "attack_type": "melee", "ability_used": "str"}},
    {"type": "grant_resistance", "damage_types": ["bludgeoning", "piercing", "slashing"],
     "trigger": "on_take_damage", "conditions": {"when_raging": true}},
    {"type": "conditional_advantage", "on": "str_check", "trigger": "on_check",
     "conditions": {"when_raging": true, "ability_used": "str"}}
  ]
}
```

**Sneak Attack (Rogue):**
```json
{
  "feature": "Sneak Attack",
  "effects": [
    {"type": "extra_damage_dice", "dice": "level_scaled", "trigger": "on_damage_roll",
     "conditions": {"weapon_property": "finesse_or_ranged", "once_per_turn": true,
                    "any_of": [{"has_advantage": true}, {"ally_within": 5}]}}
  ]
}
```

**Aura of Protection (Paladin 6+):**
```json
{
  "feature": "Aura of Protection",
  "effects": [
    {"type": "modify_save", "modifier": "cha_mod", "trigger": "on_save",
     "conditions": {"aura_radius": 10, "target": "self_and_allies"}}
  ]
}
```

**Bless (Spell):**
```json
{
  "feature": "Bless",
  "effects": [
    {"type": "modify_attack_roll", "modifier": "1d4", "trigger": "on_attack_roll"},
    {"type": "modify_save", "modifier": "1d4", "trigger": "on_save"}
  ]
}
```

### Integration with Existing Systems

- **`conditions_ref.mechanical_effects`** uses the same effect type vocabulary — condition effects (e.g., blinded → `disadvantage_on_attack`) are processed alongside feature effects
- **`classes.features_by_level`** and **`characters.features`** store effect declarations in the `mechanical_effect` field using this format
- **Auto-Detected Class & Creature Features** (Sneak Attack, Rage, Evasion, Pack Tactics) are specific instances of this general system — their logic is driven by their effect declarations, not hardcoded
- The combat engine processes all applicable effects at each trigger point using a **single-pass processor**: collect all active effects for the trigger → filter by conditions → apply in priority order → return modified result

---

## Combat Turn Flow

### Overview

```
DM clicks "Start Combat" in dashboard
  → backend initializes combat state, rolls initiative
  → bot posts initiative order to #initiative-tracker
  → bot posts map image to #combat-map
  → bot pings first player/enemy in #your-turn

Player logs in, sees ping, submits commands
  → bot validates and resolves mechanical actions
  → results posted to #combat-log, rolls to #roll-history
  → freeform /action posts to #dm-queue if needed

DM reviews, resolves any queued actions in dashboard
  → clicks "Apply", "Next Turn"
  → bot regenerates map image in #combat-map
  → bot pings next player in #your-turn
```

### Surprise

When combat begins from an ambush or when some combatants are unaware, the DM marks specific combatants as **surprised** during encounter setup in the dashboard.

**Implementation:**
- Surprise is applied as a condition: `{condition: "surprised", duration_rounds: 1, started_round: 0}`
- Surprised combatants cannot move, take actions, use bonus actions, or use reactions during round 1
- The system auto-skips surprised combatants' turns in round 1 (no Dodge action — they are fully inert)
- A surprised creature can use reactions after its turn ends in round 1 (the "surprised" condition is removed at the end of their skipped turn)

**Combat log output:**
```
⏭️ Goblin #2 is surprised — turn skipped
```

**Interaction with initiative:** surprised creatures still roll initiative normally. Their position in the initiative order matters because it determines when during round 1 they stop being surprised (and can start using reactions).

### Player Turns

Turns are **sequential** — players send commands one at a time and see results before deciding their next action.

**Turn resources tracked by the backend:**
- Movement (feet remaining out of speed)
- Action (used / not used)
- Bonus action (used / not used)
- Free object interaction (used / not used)
- Attacks remaining (by class/level)
- Reaction (used / not used, per round)

Each command validates against remaining resources. If a player tries to use something already spent, the bot replies with a specific error.

**Turn status prompt:** the bot shows available resources at two points:

1. **Turn start** — included in the ping message in `#your-turn`:
```
🔔 @Aria — it's your turn!
📋 Available: 🏃 30ft move | ⚔️ 2 attacks | 🎁 Bonus action | 🤚 Free interact | 🛡️ Reaction
```

2. **After every command** — appended to the command's response in `#combat-log`:
```
⚔️  Aria attacks Goblin #1 with Longsword (attack 1 of 2)
    → Roll to hit: 19 (14 + 5) — HIT
    → Damage: 9 slashing
    → Goblin #1 is now Bloodied

📋 Remaining: 🏃 5ft move | ⚔️ 1 attack | 🎁 Bonus action | 🤚 Free interact
```

Spent resources are omitted from the list. When nothing remains, the prompt shows: "📋 All actions spent — type `/done` to end your turn."

**Ending a turn:**
- **Explicit:** player sends `/done`
- **DM override:** DM can end any turn from the dashboard
- **Timeout:** remaining unused actions are forfeited (see Turn Timeout below)

### Enemy / NPC Turns

Each enemy takes its own turn in initiative order. The DM resolves enemy turns through the dashboard with **smart defaults** — the system pre-fills suggestions, DM confirms or overrides.

**Dashboard flow:**
1. Dashboard highlights the active enemy and pre-fills:
   - **Suggested move:** shortest path toward nearest hostile (reuses pathfinding)
   - **Suggested attack:** nearest target in range; defaults to creature's primary attack from stat block
   - **Suggested ability:** if the creature has a special ability (e.g. Breath Weapon), suggest it when conditions are met (multiple targets in cone/line)
2. DM clicks **Confirm** to accept defaults, or overrides any field
3. DM rolls to-hit and damage from the dashboard; system applies HP changes
4. DM sees results and can adjust before posting (e.g. fudge a crit that would one-shot a level 2 player)
5. On confirm, results post to `#combat-log` and map updates

**`#combat-log` output** for enemy turns (same format as player actions):
```
🏃 Goblin 1 moves to D5
⚔️ Goblin 1 attacks Thorn — 🎲 14 vs AC 18 — Miss!
🏃 Goblin 2 moves to E4
⚔️ Goblin 2 attacks Kael — 🎲 19 vs AC 15 — Hit! 7 slashing damage
```

**Pending reactions:** if a player has a pre-declared `/reaction` that triggers during the enemy turn, the dashboard surfaces it to the DM before confirming that step. DM resolves the reaction inline.

### Turn Timeout & AFK Handling

Turn timeout: **24 hours**, DM-configurable per campaign (1h–72h range).

**Escalation:**
- Reminder ping at 50% of timeout (e.g., 12h) in `#your-turn`
- Final warning at 75% (e.g., 18h) — "your turn will be skipped in 6 hours"
- Auto-skip at 100% — player takes the **Dodge action with no movement**

**DM manual overrides (via dashboard):**
- **Skip now** — immediately advance past a player
- **Extend timer** — grant more time without changing the campaign default
- **Pause combat** — freeze all timers

**Prolonged absence:**
- After 3 consecutive auto-skips, the character is flagged as "absent" in the dashboard
- DM decides: auto-pilot the character, narrate a retreat, or remove from initiative
- Initiative slot stays reserved so the player can return seamlessly

### Death Saves & Unconsciousness

When a character drops to 0 HP, they fall **unconscious** and begin making death saving throws.

**Dropping to 0 HP:**
- Character status becomes **Dying** (unconscious, prone)
- Concentration is broken automatically
- All commands except `/deathsave` are blocked on their turn
- Token state changes to "dying" on the map (distinct visual)

**Instant death check:**
- If damage remaining after reaching 0 HP ≥ character's max HP → instant death
- Token state → "dead", no death saves

**Death saving throws:**
- When initiative reaches a dying player, bot pings them to send `/deathsave`
- System rolls d20 (no modifiers unless granted by a feature like Diamond Soul)
- ≥10 = success, <10 = failure
- **Nat 20** → regain 1 HP, conscious, still prone. Tallies reset
- **Nat 1** → counts as 2 failures
- 3 successes → **stabilized** (unconscious at 0 HP, no more death saves)
- 3 failures → **dead**
- Rolls posted publicly in `#combat-log`
- If the player doesn't send `/deathsave` before timeout, the system auto-rolls the death save on their behalf — same d20 rules apply (nat 1 = 2 failures, nat 20 = regain 1 HP). Result posts to `#combat-log` marked as "(auto-rolled — player timed out)"

**Taking damage while at 0 HP:**
- Each hit = 1 automatic death save failure
- Critical hit (attacker within 5ft) = 2 failures

**Stabilization:**
- 3 death save successes, Medicine check (DC 10), or Spare the Dying cantrip
- Stable characters remain unconscious at 0 HP, no further death saves
- Regain 1 HP after 1d4 hours (post-combat only)

**Healing from 0 HP:**
- Any healing sets HP to 0 + healing amount
- Status → conscious, still **prone** (costs half movement to stand)
- Death save tallies reset to zero

**Token states:** normal / bloodied / dying / stable / dead

### Example: A Full Player Turn

Player inputs (level 5 Fighter with longsword equipped, 2 attacks per action):
```
/move D4
/attack G1
```

Bot output in `#combat-log`:
```
🗺️  Aria moves from C3 → D4  (25ft used of 30ft)
⚔️  Aria attacks Goblin #1 with Longsword (attack 1 of 2)
    → Roll to hit: 19 (14 + 5) — HIT
    → Damage: 9 slashing
    → Goblin #1 is now Bloodied
    ℹ️  1 attack remaining
```

Player sees the result and decides to finish off G1 or switch targets:
```
/attack G2
```

```
⚔️  Aria attacks Goblin #2 with Longsword (attack 2 of 2)
    → Roll to hit: 11 (6 + 5) — MISS
    ℹ️  No attacks remaining
```

Then:
- Map image regenerates with Aria at D4
- G1's token gets a visual indicator (color shift or damage overlay)
- Bot pings next player in `#your-turn`

---

## Map System

### Map Rendering

- Grid images generated **server-side** as PNGs on every state change
- Bot **appends a new message** in `#combat-map` — creates a visual log players can scroll through
- Token labels display enemy IDs (G1, OS) and player initials
- Token visual states: normal / bloodied / dying / stable / dead
- Tile size: 32–48px per square to stay within Discord's 8MB file limit
- Obstacles and difficult terrain drawn as part of the base map layer

### Dynamic Fog of War

Fog of war is computed automatically based on **shared party vision** — the union of all player tokens' visible cells. One map image is rendered per update (no per-player maps).

**How it works:**
1. Each token carries vision properties: `base_vision_ft`, `darkvision_ft`, `blindsight_ft`, `truesight_ft`
2. Server runs **shadowcasting** from each player token's position against walls and obstacles
3. Visible cells for all party tokens are unioned → the "party known" area
4. Previously seen but currently out-of-range cells rendered as **dim/greyed out** (explored but not active)
5. Never-seen cells are **fully fogged** (black)
6. Enemy tokens in fogged cells are **hidden**; enemies in dim cells are visible but greyed

**Vision sources & modifiers:**
- **Darkvision** (60/120/300ft by race/feat) — darkness → dim, dim → normal
- **Light sources** — torches (20ft bright + 20ft dim), Light cantrip, Daylight spell — point lights on the grid
- **Blindsight / Tremorsense / Truesight** — ignore fog/obstacles within range
- **Devil's Sight** — sees through magical darkness

**Obscurement zones (DM-placed on grid):**
- `Darkness` spell → blocks all vision including darkvision (except Devil's Sight)
- `Fog Cloud` → heavily obscured, blocks line of sight
- `Wall of Fire / Stone` → blocks line of sight through the wall
- Heavy foliage / smoke → light or heavy obscurement

**Rendering layers (bottom to top):**
1. Base map (terrain, walls, obstacles)
2. Fog overlay (black for unknown, semi-transparent grey for explored-but-not-visible)
3. Tokens (only drawn if their cell is visible or explored)
4. Grid lines and labels

### Map Creation & Authoring

The DM creates and edits maps through the **Svelte-based map editor** in the web dashboard.

**Creating a new map:**
1. DM specifies grid dimensions (width × height in squares, e.g., 30×20)
2. Editor opens a blank grid with default terrain (open ground)
3. DM paints terrain, walls, and obstacles
4. DM optionally imports a background image as a visual underlay
5. Map is saved and available for encounters

**Map-making tools:**
- **Terrain brush** — paint per tile: open ground, difficult terrain, water, lava, pit, etc.
- **Wall tool** — draw walls along tile edges (block movement and line of sight)
- **Object placement** — doors (open/closed/locked), traps, furniture, interactables with custom properties
- **Elevation painting** — set ground elevation per tile (cliffs, platforms, stairs)
- **Spawn zones** — mark player and enemy token placement areas at encounter start

**Image import:**
- Upload a pre-made battle map image (PNG/JPG) as a **background layer** beneath the grid
- Grid overlaid with adjustable opacity
- Walls and terrain still need tool definitions (the image is purely visual; server needs structured data for pathfinding and fog of war)

**Storage format — Tiled-compatible JSON:**

Maps are stored using a format based on the **Tiled map editor's JSON specification** (`.tmj`):
- **Tile layers** — terrain types as tile GIDs referencing a tileset
- **Object layers** — walls, doors, spawn points, traps as typed objects with custom properties
- **Tileset references** — external `.tsj` files defining tile images and properties
- **Custom properties** — arbitrary key-value data on any element

Benefits:
- DMs can **import maps from the Tiled desktop editor** (free, widely used in TTRPG/game dev)
- Future support for **tileset-based map painting**
- Go backend parses via [`go-tiled`](https://github.com/lafriks/go-tiled) or `encoding/json`
- Maps export for backup, sharing, or community map packs

**Phase 1 (MVP):** blank grid + terrain/wall tools + image import. Maps stored as Tiled-compatible JSON.
**Phase 2:** tileset support — load `.tsj` tilesets, paint with tile brushes, import full `.tmj` maps from Tiled.

---

## Character Creation & Import

Characters are fully 5e-compatible — all SRD races, classes, backgrounds, and features. Characters can be created manually via the DM dashboard or imported from D&D Beyond.

### Internal Character Format

Characters are stored using a schema based on [BrianWendt/dnd5e_json_schema](https://github.com/BrianWendt/dnd5e_json_schema). The schema covers ability scores, class features, spells, equipment, and all mechanical fields needed for combat resolution.

The internal format is not exposed directly to users — it's the canonical storage representation that the dashboard UI and D&D Beyond importer both write to.

### Manual Character Creation (DM Dashboard)

The DM creates characters through a guided workflow:

1. **Basics** — name, race, class, level, background
2. **Ability scores** — manual entry (rolled or point-buy, DM's choice — system doesn't enforce a generation method)
3. **Derived stats** — HP, AC, proficiency bonus, saving throws, skill proficiencies auto-calculated from race + class + ability scores + level using SRD rules
4. **Equipment** — select from SRD weapons/armor/items; set equipped weapon and worn armor
5. **Spells** — for caster classes, select known/prepared spells from class spell list (filtered by level). Spell slots auto-calculated
6. **Features** — racial traits and class features auto-populated from SRD data based on race + class + level

After creation, the player links to the character via `/register <character_name>` in Discord (DM approves).

**Class/subclass/feat interactions:** the system implements SRD class features mechanically (Extra Attack, Sneak Attack damage, Rage bonus, etc.) and auto-applies them during combat. Non-SRD content can be added manually by the DM as custom features with mechanical effects.

### D&D Beyond Import

1. Player provides their D&D Beyond character URL (e.g., `https://www.dndbeyond.com/characters/12345678`)
2. System fetches from DDB's undocumented character API (`character-service.dndbeyond.com/character/v5/character/{id}`)
3. Parser converts DDB JSON into internal format — mapping ability scores, features, equipment, spells, HP, AC
4. DM reviews and approves in the dashboard

**Implementation reference:** [MrPrimate/ddb-importer](https://github.com/MrPrimate/ddb-importer) (Foundry VTT's DDB import module) — the most mature open-source DDB character parser.

**Caveats:**
- No official public API — endpoint is undocumented and may change
- Character must be set to **public** sharing on D&D Beyond
- Rate limiting/CAPTCHAs possible; importer includes exponential backoff
- Non-SRD content imports as names/descriptions but may need DM manual setup for mechanics

**Re-sync:** players can re-import to pull updates (level-ups, new equipment). System diffs and shows DM what changed before applying.

### Character Leveling

- DM edits character level in the dashboard
- System auto-recalculates HP, proficiency bonus, spell slots, attacks per action
- DM selects new class features / spells if applicable
- For DDB-imported characters, player levels up in D&D Beyond and re-imports

---

## Reference Data Sources

### SRD Content (Seeded at Startup)

The system ships with the full **D&D 5e SRD** content, seeded into PostgreSQL on first run. Data sourced from [5e-bits/5e-database](https://github.com/5e-bits/5e-database) — a comprehensive, MIT-licensed JSON dataset.

**Included:**
- **Monsters** — ~325 creature stat blocks (name, HP formula, AC, speed, attacks, abilities, CR)
- **Spells** — ~320 spells (name, level, school, range, components, duration, area, damage, save type, concentration)
- **Weapons** — all SRD weapons (damage, damage type, properties: finesse, heavy, ranged, thrown, etc.)
- **Armor** — all SRD armor (AC formula, type, stealth disadvantage, strength requirement)
- **Equipment** — adventuring gear, tools, packs
- **Classes** — all 12 SRD classes with features by level, hit dice, proficiencies, spell lists
- **Races** — all SRD races with traits, ability score bonuses, speed, darkvision
- **Conditions** — all 15 standard conditions with mechanical effects
- **Skills** — all 18 skills mapped to ability scores

**Licensing:** SRD 5.1 content used under **CC-BY-4.0** (attribution only, included in about/credits page).

### Extended Content (Open5e)

For content beyond the core SRD, the system can optionally pull from the [Open5e API](https://api.open5e.com/):
- ~3,200 monsters (vs ~325 in SRD alone)
- ~1,400 spells
- Content from Tome of Beasts, Creature Codex, Deep Magic, and other OGL publishers

Fetched on-demand and cached locally. DM enables/disables third-party sources per campaign.

### Homebrew Content

DMs create custom entries for any reference data type via the dashboard:
- Custom monsters (full stat block editor)
- Custom spells, weapons, items
- Custom races and class features (name + mechanical effect)

Homebrew entries are scoped to the campaign and stored alongside SRD data with a `homebrew: true` flag.

---

## Non-Combat Gameplay

### Skill & Ability Checks

**Player-initiated:**
```
/check perception           ← d20 + WIS mod + proficiency (if proficient)
/check athletics --adv      ← roll with advantage
/check stealth --disadv     ← roll with disadvantage
/check dexterity            ← raw ability check (no skill proficiency)
```

**DM-prompted:** DM requests a check from the dashboard, pinging the player in `#your-turn`.

**Mechanics:**
- Roll formula: `d20 + ability_modifier + proficiency_bonus` (if proficient)
- Expertise (Rogue, Bard): doubles proficiency bonus — tracked as `{skill: "expertise"}`
- Jack of All Trades (Bard): adds half proficiency to non-proficient checks
- Passive checks: `10 + modifier`, displayed on character card. DM uses passive Perception for hidden checks
- All rolls post to `#roll-history`; DM narrates outcome in `#the-story`

**Group checks:** DM triggers (e.g., "group Stealth"). All players pinged simultaneously. System waits for responses (subject to timeout), reports results. Group succeeds if at least half succeed (per 5e).

**Contested checks:** DM triggers from dashboard (e.g., grapple: Athletics vs Athletics/Acrobatics). Both roll; system compares and reports.

### Short & Long Rests

**Short Rest** (`/rest short`):
1. Player types `/rest short` → posts to `#dm-queue`
2. DM approves from dashboard
3. System prompts the player to spend hit dice: `/spend-hd 2` (spend 2 hit dice)
   - Each hit die heals `1dX + CON modifier` (X = class hit die size)
   - System rolls and applies healing automatically, capped at `hp_max`
   - Player can spend 0 to `hit_dice_remaining` dice
4. System resets all features with `recharge: "short"` (e.g., Action Surge, Channel Divinity, Second Wind)
5. Results posted to `#combat-log`

**Long Rest** (`/rest long`):
1. Player types `/rest long` → posts to `#dm-queue`
2. DM approves from dashboard
3. System applies:
   - HP restored to `hp_max`
   - All spell slots restored
   - All features with `recharge: "short"` or `"long"` reset
   - Hit dice restored: regain up to half character level (minimum 1), capped at max
   - Death save tallies reset to 0/0
4. Results posted to `#combat-log`

**Constraints:**
- Only one long rest per 24 in-game hours (DM tracks narrative time; system does not enforce calendar)
- Rests cannot be initiated during active combat (system checks `encounter.status != 'active'`)
- If interrupted (DM cancels mid-rest from dashboard), partial benefits at DM discretion via manual override

### Exploration, Social & Travel

Narrative-driven — no dedicated mechanical systems needed in MVP:

- **Exploration:** DM narrates in `#the-story`, players describe actions in `#player-chat` or `/action`. DM calls for checks as needed. If combat breaks out, DM starts an encounter from the dashboard.
- **Social:** Players roleplay in `#the-story` or `#player-chat`. DM calls for Charisma checks when uncertain. Discord's text format is ideal for RP.
- **Travel:** DM narrates distance and terrain. Random encounters DM-triggered. Forced march / exhaustion checks via `/check constitution`.

`#dm-queue` serves as the universal escape hatch for anything that doesn't map to a command.

---

## DM Dashboard

The DM manages everything through a web app — they never type raw commands into Discord.

**Features:**
- **Combat Manager** — drag tokens on a grid, click to move, auto-calculate distance and range
- **HP & Condition Tracker** — click to apply damage, healing, and status conditions
- **Turn Queue** — shows initiative order; "End Turn" auto-advances and pings next player
- **Action Resolver** — view `#dm-queue` items, apply outcomes with a click
- **Stat Block Library** — preloaded monster stat blocks, reusable across encounters
- **Asset Library** — maps, token images, tilesets, custom monsters
- **Map Editor** — create and edit battle maps (see Map System)
- **Character Overview** — read-only view of all player character sheets

### Undo & Corrections

- **Undo Last Action** — reverts the most recent mutation by restoring its `before_state` from the action log. Repeatable to walk back multiple steps within a turn. DM-only.
- **Manual State Override** — directly edit any value at any time: HP, position, conditions, spell slots, initiative order. Overrides go through the per-turn lock.
- **Discord Corrections** — every undo or override posts a correction to `#combat-log`: "⚠️ **DM Correction:** Goblin #1 HP adjusted (resistance to fire was missed)". Original messages are never edited or deleted.

**Not in MVP:** full turn rewind (reverting an entire multi-action turn) or player-initiated undo. DM uses manual overrides instead.

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| Discord Bot | [discordgo](https://github.com/bwmarrin/discordgo) | Bot logic, slash commands, message editing |
| Backend API | Go stdlib `net/http` + Chi router | Game state management, command processing |
| Database | PostgreSQL + [sqlc](https://sqlc.dev) | Type-safe Go from raw SQL |
| DM Web App | Go templates for admin pages, [Svelte](https://svelte.dev) SPA for map editor | Svelte compiles to static JS/CSS, embedded in Go binary via `embed.FS` |
| Map Rendering | Go `image/draw` stdlib + [gg](https://github.com/fogleman/gg) | Server-side PNG generation |
| Real-time Sync | [nhooyr/websocket](https://github.com/nhooyr/websocket) | Live dashboard ↔ backend updates |
| Deployment | Single Go binary (dashboard embedded via `embed.FS`) | One artifact to deploy |

---

## Data Model

The database schema below defines the core entities and their relationships. All tables use UUID primary keys and include `created_at` / `updated_at` timestamps (omitted for brevity).

### Campaign & Player Tables

```sql
campaigns
  id              UUID PK
  guild_id        TEXT NOT NULL UNIQUE   -- Discord server ID (one campaign per server)
  dm_user_id      TEXT NOT NULL          -- Discord user ID of the DM
  name            TEXT NOT NULL
  settings        JSONB                  -- turn_timeout_hours, diagonal_rule, open5e_sources[], etc.
  status          TEXT NOT NULL          -- 'active', 'paused', 'archived'

characters
  id              UUID PK
  campaign_id     UUID FK → campaigns
  name            TEXT NOT NULL
  race            TEXT NOT NULL          -- FK → reference race or homebrew
  class           TEXT NOT NULL          -- FK → reference class or homebrew
  level           INTEGER NOT NULL DEFAULT 1
  ability_scores  JSONB NOT NULL         -- { str, dex, con, int, wis, cha }
  hp_max          INTEGER NOT NULL
  hp_current      INTEGER NOT NULL
  temp_hp         INTEGER NOT NULL DEFAULT 0
  ac              INTEGER NOT NULL
  speed_ft        INTEGER NOT NULL DEFAULT 30
  proficiency_bonus INTEGER NOT NULL
  equipped_main_hand TEXT                -- FK → weapons (primary weapon)
  equipped_off_hand  TEXT                -- FK → weapons or armor (second weapon or shield; null = free hand)
  equipped_armor  TEXT                   -- FK → armor (body armor only, not shield)
  spell_slots     JSONB                  -- { "1": {current: 2, max: 4}, "2": {current: 3, max: 3}, ... }
  hit_dice_remaining INTEGER NOT NULL     -- current pool; max = character level. Hit die size from class.hit_die
  feature_uses    JSONB                  -- { "action-surge": {current: 1, max: 1, recharge: "short"}, ... }
  features        JSONB                  -- [{name, source, level, description, mechanical_effect}]
  proficiencies   JSONB                  -- {saves: [str, con], skills: [athletics, perception], weapons: [...], armor: [...]}
  inventory       JSONB                  -- [{item_id, quantity, equipped}]
  character_data  JSONB                  -- full dnd5e_json_schema blob for import/export fidelity
  ddb_url         TEXT                   -- D&D Beyond URL if imported
  homebrew        BOOLEAN DEFAULT false

player_characters
  id              UUID PK
  campaign_id     UUID FK → campaigns
  character_id    UUID FK → characters   UNIQUE per campaign
  discord_user_id TEXT NOT NULL
  approved        BOOLEAN DEFAULT false  -- DM must approve /register
  UNIQUE(campaign_id, discord_user_id)   -- one character per player per campaign
```

### Encounter & Combat Tables

```sql
encounters
  id              UUID PK
  campaign_id     UUID FK → campaigns
  map_id          UUID FK → maps
  name            TEXT
  status          TEXT NOT NULL          -- 'preparing', 'active', 'completed'
  round_number    INTEGER DEFAULT 0
  current_turn_id UUID FK → turns        -- nullable; set when combat is active

combatants
  id              UUID PK
  encounter_id    UUID FK → encounters
  character_id    UUID FK → characters   -- NULL for creatures
  creature_ref_id TEXT                   -- FK → creatures reference data; NULL for PCs
  short_id        TEXT NOT NULL          -- "G1", "OS", "AR" — stable for entire encounter
  display_name    TEXT NOT NULL          -- "Goblin #1", "Orc Shaman", "Aria"
  initiative_roll INTEGER NOT NULL
  initiative_order INTEGER NOT NULL      -- tiebreaker ordering
  position_col    TEXT NOT NULL          -- grid column: "A", "AA", etc.
  position_row    INTEGER NOT NULL       -- grid row: 1, 2, ...
  altitude_ft     INTEGER DEFAULT 0
  hp_max          INTEGER NOT NULL       -- instance HP (creatures may vary from template)
  hp_current      INTEGER NOT NULL
  temp_hp         INTEGER DEFAULT 0
  ac              INTEGER NOT NULL
  conditions      JSONB DEFAULT '[]'     -- [{condition, source_combatant_id, duration_rounds, started_round, expires_on}]
  exhaustion_level INTEGER DEFAULT 0    -- 0-6; effects are cumulative (see Exhaustion)
  death_saves     JSONB                  -- {successes: 0, failures: 0} — PCs only
  is_visible      BOOLEAN DEFAULT true   -- hidden enemies (ambush, stealth)
  is_alive        BOOLEAN DEFAULT true
  is_npc          BOOLEAN DEFAULT false  -- DM-controlled ally vs enemy

turns
  id              UUID PK
  encounter_id    UUID FK → encounters
  combatant_id    UUID FK → combatants
  round_number    INTEGER NOT NULL
  status          TEXT NOT NULL          -- 'active', 'completed', 'skipped'
  movement_remaining_ft INTEGER NOT NULL
  action_used     BOOLEAN DEFAULT false
  bonus_action_used BOOLEAN DEFAULT false
  bonus_action_spell_cast BOOLEAN DEFAULT false  -- tracks bonus action spell restriction
  reaction_used   BOOLEAN DEFAULT false  -- per-round, not per-turn
  free_interact_used BOOLEAN DEFAULT false
  attacks_remaining INTEGER NOT NULL DEFAULT 1
  has_disengaged  BOOLEAN DEFAULT false   -- tracks Disengage action for opportunity attack suppression
  started_at      TIMESTAMPTZ
  timeout_at      TIMESTAMPTZ           -- started_at + campaign timeout setting
  completed_at    TIMESTAMPTZ

action_log
  id              UUID PK
  turn_id         UUID FK → turns
  encounter_id    UUID FK → encounters   -- denormalized for fast queries
  action_type     TEXT NOT NULL          -- 'move', 'attack', 'cast', 'damage', 'heal', 'condition_add', 'condition_remove', 'death_save', 'dm_override', etc.
  actor_id        UUID FK → combatants
  target_id       UUID FK → combatants   -- nullable
  description     TEXT                   -- human-readable summary
  before_state    JSONB NOT NULL         -- snapshot of affected fields before mutation
  after_state     JSONB NOT NULL         -- snapshot after mutation
  dice_rolls      JSONB                  -- [{die, count, results[], modifier, total, purpose}]
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
```

### Reaction Declarations

```sql
reaction_declarations
  id              UUID PK
  encounter_id    UUID FK → encounters
  combatant_id    UUID FK → combatants
  description     TEXT NOT NULL          -- "Shield if I get hit", "OA if goblin moves away"
  status          TEXT NOT NULL          -- 'active', 'used', 'cancelled'
  created_at      TIMESTAMPTZ NOT NULL
  resolved_at     TIMESTAMPTZ
  resolution_note TEXT                   -- DM's note on how it resolved
```

### Maps

```sql
maps
  id              UUID PK
  campaign_id     UUID FK → campaigns
  name            TEXT NOT NULL
  width_squares   INTEGER NOT NULL
  height_squares  INTEGER NOT NULL
  tiled_json      JSONB NOT NULL         -- full Tiled-compatible .tmj data
  background_image_url TEXT              -- optional uploaded battle map image
  tileset_refs    JSONB                  -- [{name, source_url, first_gid}]
```

### Reference Data (SRD + Homebrew)

```sql
creatures
  id              TEXT PK               -- slug: "goblin", "adult-red-dragon"
  campaign_id     UUID FK → campaigns   -- NULL for SRD entries
  name            TEXT NOT NULL
  size            TEXT NOT NULL
  type            TEXT NOT NULL          -- beast, humanoid, undead, etc.
  alignment       TEXT
  ac              INTEGER NOT NULL
  ac_type         TEXT                   -- "natural armor", "chain mail", etc.
  hp_formula      TEXT NOT NULL          -- "2d6+2"
  hp_average      INTEGER NOT NULL
  speed           JSONB NOT NULL         -- {walk: 30, fly: 60, swim: 30}
  ability_scores  JSONB NOT NULL         -- {str, dex, con, int, wis, cha}
  saving_throws   JSONB                  -- {dex: 4, wis: 2}
  skills          JSONB                  -- {perception: 4, stealth: 6}
  damage_resistances   TEXT[]
  damage_immunities    TEXT[]
  damage_vulnerabilities TEXT[]
  condition_immunities TEXT[]
  senses          JSONB                  -- {darkvision: 60, passive_perception: 14}
  languages       TEXT[]
  cr              TEXT NOT NULL          -- "1/4", "1", "17"
  attacks         JSONB NOT NULL         -- [{name, to_hit, damage, damage_type, reach_ft, range_ft, description}]
  abilities       JSONB                  -- [{name, description, recharge}] — special abilities, legendary actions
  homebrew        BOOLEAN DEFAULT false
  source          TEXT DEFAULT 'srd'     -- 'srd', 'open5e:tome-of-beasts', 'homebrew'

spells
  id              TEXT PK               -- slug: "fireball", "cure-wounds"
  campaign_id     UUID FK → campaigns   -- NULL for SRD entries
  name            TEXT NOT NULL
  level           INTEGER NOT NULL       -- 0 for cantrips
  school          TEXT NOT NULL          -- evocation, abjuration, etc.
  casting_time    TEXT NOT NULL          -- "1 action", "1 bonus action", "1 reaction"
  range_ft        INTEGER                -- NULL for "self", "touch" = 5
  range_type      TEXT NOT NULL          -- 'ranged', 'touch', 'self'
  components      JSONB NOT NULL         -- {v: true, s: true, m: "a tiny ball of bat guano"}
  duration        TEXT NOT NULL          -- "instantaneous", "1 minute", "concentration, up to 1 hour"
  concentration   BOOLEAN DEFAULT false
  ritual          BOOLEAN DEFAULT false
  area            JSONB                  -- {shape: "sphere", radius_ft: 20} or {shape: "cone", length_ft: 15}
  attack_type     TEXT                   -- 'melee', 'ranged', or NULL (save-based/auto-hit)
  save_type       TEXT                   -- "dex", "wis", etc. NULL if no save
  damage          JSONB                  -- {dice: "8d6", type: "fire", higher_levels: "1d6 per slot above 3rd", cantrip_scaling: true}
  healing         JSONB                  -- {dice: "1d8+mod", higher_levels: "1d8 per slot above 1st"}
  effects         TEXT                   -- description of non-damage effects
  classes         TEXT[] NOT NULL        -- ["wizard", "sorcerer"]
  homebrew        BOOLEAN DEFAULT false
  source          TEXT DEFAULT 'srd'

weapons
  id              TEXT PK               -- slug: "longsword", "longbow"
  name            TEXT NOT NULL
  damage          TEXT NOT NULL          -- "1d8"
  damage_type     TEXT NOT NULL          -- slashing, piercing, bludgeoning
  weight_lb       REAL
  properties      TEXT[] NOT NULL        -- ["versatile", "finesse", "heavy", "two-handed", "ranged", "thrown", "light", "ammunition", "reach", "loading"]
  range_normal_ft INTEGER               -- 80 for longbow, NULL for melee
  range_long_ft   INTEGER               -- 320 for longbow
  versatile_damage TEXT                  -- "1d10" for longsword
  weapon_type     TEXT NOT NULL          -- 'simple_melee', 'simple_ranged', 'martial_melee', 'martial_ranged'

armor
  id              TEXT PK
  name            TEXT NOT NULL
  ac_base         INTEGER NOT NULL       -- 11 for leather, 18 for plate
  ac_dex_bonus    BOOLEAN DEFAULT true   -- false for heavy armor
  ac_dex_max      INTEGER                -- 2 for medium armor, NULL for light
  strength_req    INTEGER                -- 15 for plate
  stealth_disadv  BOOLEAN DEFAULT false
  armor_type      TEXT NOT NULL          -- 'light', 'medium', 'heavy', 'shield'
  weight_lb       REAL

classes
  id              TEXT PK               -- "fighter", "wizard"
  name            TEXT NOT NULL
  hit_die         TEXT NOT NULL          -- "d10"
  primary_ability TEXT NOT NULL          -- "str" or "dex"
  save_proficiencies TEXT[] NOT NULL     -- ["str", "con"]
  armor_proficiencies TEXT[]
  weapon_proficiencies TEXT[]
  skill_choices   JSONB                  -- {choose: 2, from: ["athletics", "perception", ...]}
  spellcasting    JSONB                  -- {ability: "int", slot_progression: "full"} or NULL
  features_by_level JSONB NOT NULL       -- {"1": [{name, description, mechanical_effect}], "2": [...], ...}
  attacks_per_action JSONB NOT NULL      -- {"1": 1, "5": 2, "11": 3, "20": 4} (Fighter example)

races
  id              TEXT PK               -- "elf", "dwarf", "human"
  name            TEXT NOT NULL
  speed_ft        INTEGER NOT NULL
  size            TEXT NOT NULL          -- "Medium", "Small"
  ability_bonuses JSONB NOT NULL         -- {dex: 2} or {all: 1}
  darkvision_ft   INTEGER DEFAULT 0
  traits          JSONB NOT NULL         -- [{name, description, mechanical_effect}]
  languages       TEXT[]
  subraces        JSONB                  -- [{id, name, ability_bonuses, traits}]

conditions_ref
  id              TEXT PK               -- "blinded", "prone", "stunned"
  name            TEXT NOT NULL
  description     TEXT NOT NULL
  mechanical_effects JSONB NOT NULL      -- [{effect_type, ...}] — uses Feature Effect System vocabulary
                                         -- see Condition Effects section for full mechanics per condition
```

### Key Design Decisions

1. **Combatants as instances** — the `combatants` table joins encounters to characters/creatures with instance-level state (position, current HP, conditions). A single creature template spawns multiple combatants (G1, G2, G3) each tracking independent HP and conditions.

2. **JSONB for flexible data** — ability scores, features, spell slots, and inventory use JSONB columns rather than normalized tables. This matches the nested structure of 5e character data and avoids dozens of join tables for rarely-queried data. Combat-critical fields (HP, AC, position) remain as typed columns for indexed queries.

3. **Reference data with campaign scope** — SRD entries have `campaign_id = NULL` (global). Homebrew entries are scoped to a campaign. Queries use `WHERE campaign_id = $1 OR campaign_id IS NULL` to merge both.

4. **Action log for undo** — every mutation records `before_state` / `after_state` as JSONB snapshots. The DM's undo operation restores `before_state`. The log also serves as the audit trail for `#combat-log` and `#roll-history` Discord channels.

5. **Character data blob** — the `character_data` JSONB column stores the full dnd5e_json_schema representation. This preserves import fidelity (D&D Beyond data that doesn't map to typed columns) and enables future export. The typed columns are the source of truth for gameplay; `character_data` is the source of truth for display and re-export.

6. **No separate combat state table** — the encounter tracks `status` and `current_turn_id`. Current combat state is derived from the encounter's combatants + the active turn row.

7. **Single-pass effect processor** — all combat modifiers (conditions, class features, spell effects, racial traits) are resolved through one unified pipeline using the Feature Effect System's effect type vocabulary. At each trigger point, the processor collects all applicable effects, filters by conditions, and applies them in priority order. This avoids scattered hardcoded checks and makes new features data-driven.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| State drift from manual Discord edits | Enforce read-only Discord policy; all state changes via bot or dashboard only |
| Map images exceeding 8MB | Cap tile size at 48px; compress PNGs; limit grid size |
| Enemy ID ambiguity | IDs are stable, map-labeled, and confirmed in every combat log response |
| Complex player actions breaking automation | `/action` routes to DM; no attempt to auto-parse freeform intent |
| D&D Beyond API instability | Undocumented endpoint may change; importer includes fallback to manual creation |
| Turn stalls from AFK players | 24h configurable timeout with escalating pings and auto-skip to Dodge |
| Reaction timing in async | Pre-declaration model — no combat pauses; DM resolves manually |
| Concurrent commands corrupting state | Per-turn PostgreSQL advisory locks serialize all mutations |

---

## MVP Scope

**Included:**
- Discord bot with full channel structure (`/setup` auto-creates)
- All slash commands: `/move`, `/attack`, `/cast`, `/bonus`, `/shove`, `/fly`, `/interact`, `/action`, `/reaction`, `/deathsave`, `/done`, `/equip`, `/register`, `/check`, `/save`, `/rest`
- Combat state: HP, position, turn order, conditions, death saves, spell slots, concentration
- Turn timeout with escalating pings and auto-skip
- Enemy/NPC turns with smart-default suggestions for DM
- Server-side map PNG generation with dynamic fog of war
- DM web dashboard: grid view, token drag-drop, HP management, turn advancement, undo, manual overrides
- Character creation (full 5e SRD: race, class, abilities, equipment, spells)
- D&D Beyond character import
- SRD reference data seeded at startup (monsters, spells, weapons, armor, classes, races, conditions)
- Skill/ability checks and rest mechanics with feature-use recharge
- Dice rolls auto-logged to `#roll-history`
- Map editor: blank grid + terrain/wall tools + image import (Tiled-compatible JSON)
- Data-driven Feature Effect System for class features, racial traits, and spell effects
- Core combat rules: opportunity attacks, cover, difficult terrain, surprise, critical hits, temp HP, two-weapon fighting, grapple/shove, stealth/hiding
- Spell mechanics: spell save DC, spell attack rolls, upcasting, ritual casting, cantrip damage scaling
- Weapon mechanics: finesse auto-select, loading, versatile, reach validation, heavy weapon size restriction, ammunition tracking, thrown weapon range
- Condition/spell duration auto-expiration
- Equipment enforcement (armor STR requirements, stealth disadvantage, free object interaction limits)
- Standard actions: Dash, Disengage, Dodge, Escape, Help, Hide, Ready (auto-resolved where deterministic)

**Future phases:**
- Tileset-based map painting + Tiled desktop import
- Full asset library
- Open5e third-party content integration
- Inventory management
- Campaign/session management
