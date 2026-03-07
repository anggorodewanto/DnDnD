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
- Exception: `/check`, `/save`, `/rest` operate outside combat turn order

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
| `/attack` | `/attack G2` or `/attack G2 handaxe --gwm` | Attack a target. One `/attack` per swing; backend tracks attacks remaining |
| `/cast` | `/cast fireball D5` | Cast a spell at a target coordinate or enemy ID |
| `/bonus` | `/bonus cunning-action dash` | Bonus action |
| `/shove` | `/shove OS` | Shove a target (push or knock prone) |
| `/interact` | `/interact draw longsword` | Free object interaction — routed to `#dm-queue` |
| `/action` | `/action flip the table` | Freeform action — routed to `#dm-queue` |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare reaction intent (usable any time) — routed to `#dm-queue` |
| `/deathsave` | `/deathsave` | Roll a death saving throw (only at 0 HP) |
| `/done` | `/done` | End turn, advance initiative |

**Non-combat commands** (usable outside active combat):

| Command | Example | Description |
|---|---|---|
| `/check` | `/check perception` or `/check athletics --adv` | Skill/ability check |
| `/save` | `/save dex` | Saving throw (DM-prompted) |
| `/rest` | `/rest short` or `/rest long` | Initiate a rest (DM must approve) |

**Utility commands** (usable any time):

| Command | Example | Description |
|---|---|---|
| `/equip` | `/equip longsword` | Set primary weapon (persists between turns) |
| `/register` | `/register Thorn` | Link Discord account to a character (DM approves) |
| `/setup` | `/setup` | Auto-create channel structure (DM only, run once) |
| `/help` | `/help` or `/help attack` | Show command list, or detailed usage for a specific command |

**Command discoverability:** Discord's built-in slash command UI is the primary discovery mechanism — typing `/` shows all registered commands with parameter hints. The `/help` command supplements this with usage examples, available flags (e.g., `--gwm`, `--adv`), and context-specific tips (e.g., remaining attacks, available spell slots).

### Attack Details

**Weapon selection:** each character has an equipped weapon (set via `/equip`). `/attack G2` uses it; `/attack G2 handaxe` overrides for that swing. Backend validates the character has the weapon.

**Extra Attack:** resolved one swing at a time. Each `/attack` resolves a single attack roll. Backend tracks attacks remaining by class/level (Fighter 5 = 2, Fighter 11 = 3, Fighter 20 = 4). After each swing, the bot reports remaining attacks. Players retarget freely between swings. Unused attacks forfeited on `/done`.

**Attack modifier flags** (opt-in per swing):
- `--gwm` — Great Weapon Master: -5 to hit, +10 damage. Requires a heavy melee weapon.
- `--sharpshooter` — Sharpshooter: -5 to hit, +10 damage. Requires a ranged weapon.
- `--reckless` — Reckless Attack (Barbarian): advantage on melee STR attacks this turn, enemies get advantage against you until next turn. First attack only. Requires Barbarian class.
- Invalid flags return an error explaining why.

**Advantage/disadvantage** — auto-detected from tracked conditions:
- **Auto-detected:** target prone (melee adv / ranged disadv), attacker prone (disadv), target restrained/stunned/paralyzed/unconscious (adv), attacker restrained/blinded/poisoned (disadv), Reckless Attack (adv), invisible (adv/disadv as appropriate)
- **Not auto-detected in MVP:** flanking (optional rule — may add as campaign toggle later)
- **DM override:** DM can force advantage or disadvantage from the dashboard for edge cases. Posts to `#combat-log`.
- **Stacking:** when both apply, they cancel out per 5e rules — rolled normally regardless of source count.

### Spell Casting Details

**AoE targeting:** `/cast fireball D5` targets a coordinate. Backend calculates affected creatures by shape/radius from spell data (`{ shape: "sphere", radius_ft: 20 }`, `{ shape: "cone", length_ft: 15 }`, `{ shape: "line", length_ft: 60, width_ft: 5 }`). Cones originate from the caster toward the target. All affected creatures (including allies) listed in `#combat-log`.

**Spell saves:** auto-rolled for all affected creatures. Saves are mechanical (d20 + modifier vs DC) with no decision-making, so auto-rolling keeps async play moving.

**Concentration:** fully tracked by backend:
- One concentration spell at a time; new cast auto-drops the previous
- Taking damage triggers auto-rolled CON save (DC = max(10, half damage)); failure breaks concentration
- Active effects (Fog Cloud zone, Spirit Guardians aura) tracked on the map

**Spell slots:** tracked and enforced. Backend knows slots per level, deducts on cast, rejects `/cast` if no slots remaining.

**Spell range:** enforced by backend. Touch spells require adjacency (5ft), self spells need no target.

### Reactions

Players pre-declare reaction intent using `/reaction`. The DM resolves all reactions manually.

**Declaration:**
- `/reaction Shield if I get hit`
- `/reaction OA if goblin moves away`
- `/reaction Counterspell if enemy casts`
- Declarations persist until used, cancelled (`/reaction cancel`), or the encounter ends
- One reaction per round per player (per 5e rules) — tracked by system

**DM workflow:**
1. DM sees declarations in `#dm-queue` or the dashboard
2. When the trigger occurs during enemy/NPC turns, DM decides whether it fires
3. DM resolves in the dashboard (rolls, applies effects) and posts the result
4. System marks the player's reaction as spent for the round

Readied Actions use the same flow: `/reaction I attack when the goblin moves past me`.

**Why this works for async:** zero stalling — combat never pauses for a reaction response. Players declare intent on their own time. DM has full control over timing and adjudication.

### Freeform Actions

For anything that can't be expressed through structured commands, `/action` routes to the DM:

```
/action I want to flip the table at B3 for half cover and duck behind it
/action I grab the chandelier and swing across to F2
```

These post to `#dm-queue`. The DM resolves them in the dashboard, applies state changes, and the bot posts the result. Structured commands handle ~90% of combat; `/action` is the escape hatch for creative play.

---

## A Full Player Turn — Example

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
    → Roll to hit: 11 (6 + 5) vs AC 13 — MISS
    ℹ️  No attacks remaining
```

Then:
- Map image regenerates with Aria at D4
- G1's token gets a visual indicator (color shift or damage overlay)
- Bot pings next player in `#your-turn`

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
3. System auto-rolls to-hit vs target AC, rolls damage, applies HP changes
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
- If the player doesn't send `/deathsave` before timeout, system auto-rolls for them

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
  equipped_weapon TEXT                   -- FK → weapons
  equipped_armor  TEXT                   -- FK → armor
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
  conditions      JSONB DEFAULT '[]'     -- [{condition, source, duration_rounds, started_round}]
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
  reaction_used   BOOLEAN DEFAULT false  -- per-round, not per-turn
  free_interact_used BOOLEAN DEFAULT false
  attacks_remaining INTEGER NOT NULL DEFAULT 1
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
  save_type       TEXT                   -- "dex", "wis", etc. NULL if no save
  damage          JSONB                  -- {dice: "8d6", type: "fire", higher_levels: "1d6 per slot above 3rd"}
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
  mechanical_effects JSONB NOT NULL      -- [{effect_type: "disadvantage_on_attack", ...}, {effect_type: "advantage_against", ...}]
```

### Key Design Decisions

1. **Combatants as instances** — the `combatants` table joins encounters to characters/creatures with instance-level state (position, current HP, conditions). A single creature template spawns multiple combatants (G1, G2, G3) each tracking independent HP and conditions.

2. **JSONB for flexible data** — ability scores, features, spell slots, and inventory use JSONB columns rather than normalized tables. This matches the nested structure of 5e character data and avoids dozens of join tables for rarely-queried data. Combat-critical fields (HP, AC, position) remain as typed columns for indexed queries.

3. **Reference data with campaign scope** — SRD entries have `campaign_id = NULL` (global). Homebrew entries are scoped to a campaign. Queries use `WHERE campaign_id = $1 OR campaign_id IS NULL` to merge both.

4. **Action log for undo** — every mutation records `before_state` / `after_state` as JSONB snapshots. The DM's undo operation restores `before_state`. The log also serves as the audit trail for `#combat-log` and `#roll-history` Discord channels.

5. **Character data blob** — the `character_data` JSONB column stores the full dnd5e_json_schema representation. This preserves import fidelity (D&D Beyond data that doesn't map to typed columns) and enables future export. The typed columns are the source of truth for gameplay; `character_data` is the source of truth for display and re-export.

6. **No separate combat state table** — the encounter tracks `status` and `current_turn_id`. Current combat state is derived from the encounter's combatants + the active turn row.

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

**Future phases:**
- Tileset-based map painting + Tiled desktop import
- Full asset library
- Open5e third-party content integration
- Inventory management
- Campaign/session management
