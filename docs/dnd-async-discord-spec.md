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
  #combat-map           ← bot posts/edits the grid image each turn
  #your-turn            ← bot pings the active player

🎒 REFERENCE
  #character-cards      ← auto-updated embeds per player
  #dm-queue             ← freeform player actions awaiting DM resolution
```

---

## Player Input Model

### Enemy Targeting

Enemies are assigned short stable IDs at the start of combat. Unique enemies get an abbreviated code; duplicates are numbered.

```
Goblin #1  (G1)  [B4]  Uninjured
Goblin #2  (G2)  [C6]  Bloodied
Orc Shaman (OS)  [D5]  Uninjured
```

**Enemy HP is hidden from players.** The bot never reveals exact HP numbers, AC values, or stat block details for enemies in any player-visible channel. Instead, enemies show a **descriptive health tier:**
- **Uninjured** — 100% HP
- **Scratched** — 75–99% HP
- **Bloodied** — 25–74% HP (standard 5e convention: below half)
- **Critical** — 1–24% HP
- **Dying / Dead** — 0 HP

The DM sees exact numbers in the dashboard. This preserves tension and prevents metagaming around kill thresholds.

**Rules:**
- IDs are stable for the entire encounter — if G1 dies, G2 stays G2
- Token labels on the map image display these IDs so players can target by sight
- The combat log confirms the reference in every response

### Grid Movement — Chess Notation

The map uses a standard chess-style coordinate grid: **columns as letters, rows as numbers**.

Columns use spreadsheet-style lettering: A–Z for the first 26 columns, then AA, AB, … AZ, BA, BB, … for larger maps. This allows grids of any practical size while keeping coordinates human-readable.

```
  A   B   C   D   E   F
1 [ ] [ ] [ ] [ ] [ ] [ ]
2 [ ] [G1] [ ] [ ] [ ] [ ]
3 [ ] [ ] [AR] [ ] [G2] [ ]
4 [ ] [ ] [ ] [OS] [ ] [ ]
```

Movement is expressed as a single destination coordinate:

```
/move D4
/move E6
/move AA12     ← valid on maps wider than 26 columns
```

The backend validates every movement command:
- Is the destination within the character's remaining movement speed?
- Is the tile occupied?
- Is there difficult terrain or an obstacle?

If invalid, the bot replies with a specific reason in `#combat-log` before DM intervention is needed.

**Diagonal movement:** Follows 5e standard rules — diagonals cost 5ft, same as cardinal movement. This is enforced by the validator.

### Altitude & Elevation

Tokens carry an **altitude** value (integer, in feet, default 0) representing height above the ground plane.

```
/fly 30        ← rise to 30ft altitude (costs 30ft of movement)
/fly 0         ← descend to ground level
/move D4       ← horizontal movement while maintaining current altitude
```

- Ascending and descending costs movement **1:1** (flying 30ft up costs 30ft of movement speed)
- A token's altitude is displayed as a **suffix on the map label**: `AR↑30` means Aria at 30ft altitude
- **Distance calculation** uses 3D Euclidean distance (rounded to nearest 5ft) for range checks — a creature 20ft away horizontally at 30ft altitude is ~36ft away (rounded to 35ft)
- Tokens at different altitudes **do not block** each other's ground tile — multiple tokens can occupy the same column at different heights
- Falling: if a flying creature is knocked prone or loses its fly speed, it falls. Fall damage is 1d6 per 10ft fallen (standard 5e rules), applied automatically

### Structured Commands

| Command | Example | Description |
|---|---|---|
| `/move` | `/move D4` | Move token to grid coordinate (repeatable for split movement) |
| `/attack` | `/attack G2` or `/attack G2 handaxe` | Attack a target by ID, optionally specifying a weapon (defaults to equipped weapon). Repeatable for Extra Attack — one `/attack` per swing |
| `/cast` | `/cast fireball D5` | Cast a spell, with target coordinate or enemy ID |
| `/bonus` | `/bonus cunning-action dash` | Bonus action |
| `/shove` | `/shove OS` | Shove a target by ID |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare a reaction intent, posted to `#dm-queue` for DM to resolve |
| `/interact` | `/interact draw longsword` | Free object interaction, routed to DM |
| `/action` | `/action flip the table at B3 for cover` | Freeform action, routed to DM |
| `/equip` | `/equip longsword` | Set primary weapon (persists between turns) |
| `/done` | `/done` | End turn, advance initiative |
| `/check` | `/check perception` or `/check athletics --adv` | Roll a skill/ability check (out of combat) |
| `/save` | `/save dex` | Roll a saving throw (out of combat, DM-prompted) |
| `/rest` | `/rest short` or `/rest long` | Initiate a short or long rest (DM must approve) |

### Freeform Actions — `/action`

For anything that can't be expressed through coordinates and IDs, `/action` routes to the DM rather than the bot:

```
/action I want to flip the table at B3 for half cover and duck behind it
/action I grab the chandelier and swing across to F2
```

These post to `#dm-queue`. The DM resolves them in the dashboard, applies any state changes, and the bot posts the result. Structured commands handle ~90% of combat; `/action` is the escape hatch for creative play.

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

```
DM clicks "Start Combat" in dashboard
  → backend initializes combat state, rolls initiative
  → bot posts initiative order to #initiative-tracker
  → bot posts map image to #combat-map
  → bot pings first player in #your-turn

Player logs in, sees ping, submits commands
  → bot validates and resolves mechanical actions
  → results posted to #combat-log, rolls to #roll-history
  → freeform /action posts to #dm-queue if needed

DM reviews, resolves any queued actions in dashboard
  → clicks "Apply", "Next Turn"
  → bot regenerates and edits map image in #combat-map
  → bot pings next player in #your-turn
```

---

## Map Rendering

- Grid images are generated **server-side** as PNGs on every state change
- Bot **appends a new message** in `#combat-map` (dedicated channel) — creates a visual combat log that players can scroll through to see how the fight unfolded
- Token labels display enemy IDs (G1, OS, etc.) and player initials
- Token visual states: normal / bloodied / dying / stable / dead
- Tile size: 32–48px per square to stay within Discord's 8MB file limit
- Obstacles and difficult terrain are drawn as part of the base map layer

### Dynamic Fog of War

Fog of war is computed automatically based on **shared party vision** — the union of all player tokens' visible cells. One map image is rendered per update (no per-player maps).

**How it works:**
1. Each token carries vision properties: `base_vision_ft`, `darkvision_ft`, `blindsight_ft`, `truesight_ft`
2. Server runs **shadowcasting** from each player token's position against walls and obstacles
3. Visible cells for all party tokens are unioned → the "party known" area
4. Previously seen but currently out-of-range cells are rendered as **dim/greyed out** (explored but not active)
5. Never-seen cells are **fully fogged** (black)
6. Enemy tokens in fogged cells are **hidden**; enemies in dim cells are visible but greyed

**Vision sources & modifiers:**
- **Darkvision** (60/120/300ft by race/feat) — darkness → dim, dim → normal
- **Light sources** — torches (20ft bright + 20ft dim), Light cantrip, Daylight spell — added as point lights on the grid
- **Blindsight / Tremorsense / Truesight** — ignore fog/obstacles within range
- **Devil's Sight** — sees through magical darkness

**Obscurement zones (DM-placed on grid):**
- `Darkness` spell → blocks all vision including darkvision (except Devil's Sight)
- `Fog Cloud` → heavily obscured area, blocks line of sight
- `Wall of Fire / Stone` → blocks line of sight through the wall
- Heavy foliage / smoke → light or heavy obscurement

**Rendering layers (bottom to top):**
1. Base map (terrain, walls, obstacles)
2. Fog overlay (black for unknown, semi-transparent grey for explored-but-not-visible)
3. Tokens (only drawn if their cell is in the party's current visible set or explored set)
4. Grid lines and labels

### Map Creation & Authoring

The DM creates and edits maps through the **Svelte-based map editor** in the web dashboard.

**Creating a new map:**
1. DM specifies grid dimensions (width × height in squares, e.g., 30×20)
2. Editor opens a blank grid with the default terrain (open ground)
3. DM paints terrain, walls, and obstacles using the map-making tools (see below)
4. DM optionally imports a background image to use as a visual underlay
5. Map is saved and available for use in encounters

**Map-making tools (in the Svelte editor):**
- **Terrain brush** — paint terrain types per tile: open ground, difficult terrain, water, lava, pit, etc.
- **Wall tool** — draw walls along tile edges (not tile centers). Walls block movement and line of sight for fog of war
- **Object placement** — place doors (open/closed/locked), traps, furniture, and other interactables. Objects carry custom properties (e.g., a door's DC to pick the lock)
- **Elevation painting** — set ground elevation per tile (for cliffs, raised platforms, stairs)
- **Spawn zones** — mark areas where player tokens and enemy tokens are placed at encounter start

**Image import:**
- DM can upload a pre-made battle map image (PNG/JPG) as a **background layer** beneath the grid
- The grid is overlaid on top with adjustable opacity so hand-drawn or purchased maps can be used
- Walls and terrain still need to be defined with the tools (the image is purely visual; the server needs structured data for pathfinding and fog of war)

**Storage format — Tiled-compatible JSON:**

Maps are stored internally using a format based on the **Tiled map editor's JSON specification** (`.tmj`). This provides:
- **Tile layers** — terrain types stored as tile GIDs referencing a tileset
- **Object layers** — walls, doors, spawn points, traps stored as typed objects with custom properties
- **Tileset references** — external `.tsj` files defining tile images and their properties (terrain type, blocks-movement, blocks-sight, etc.)
- **Custom properties** — arbitrary key-value data on any element (tile, object, layer)

Adopting the Tiled JSON format means:
- DMs can **import maps created in the Tiled desktop editor** (a free, open-source tool widely used in the TTRPG and game dev communities)
- Future support for **tileset-based map painting** — DMs load a tileset (dungeon tiles, forest tiles, cave tiles) and paint maps tile-by-tile, like in game development
- The Go backend parses maps using [`github.com/lafriks/go-tiled`](https://github.com/lafriks/go-tiled) for TMX or standard `encoding/json` for the JSON variant
- Maps export cleanly for backup, sharing between campaigns, or community map packs

**Phase 1 (MVP):** blank grid + terrain/wall tools + image import. Maps stored as Tiled-compatible JSON.
**Phase 2:** tileset support — load `.tsj` tilesets, paint with tile brushes, import full `.tmj` maps from Tiled desktop.

---

## Character Creation & Import

Characters are fully 5e-compatible — all SRD races, classes, backgrounds, and features are supported. Characters can be created manually via the DM dashboard or imported from D&D Beyond.

### Internal Character Format

Characters are stored using a schema based on [BrianWendt/dnd5e_json_schema](https://github.com/BrianWendt/dnd5e_json_schema) — the closest thing to a community-standard JSON schema for 5e character data. The schema covers ability scores, class features, spells, equipment, and all mechanical fields needed for combat resolution.

The internal format is not exposed directly to users — it's the canonical storage representation that the dashboard UI and D&D Beyond importer both write to.

### Manual Character Creation (DM Dashboard)

The DM creates characters through a guided workflow in the web dashboard:

1. **Basics** — name, race, class, level, background
2. **Ability scores** — manual entry (rolled or point-buy, DM's choice — the system doesn't enforce a generation method)
3. **Derived stats** — HP, AC, proficiency bonus, saving throws, skill proficiencies are auto-calculated from race + class + ability scores + level using SRD rules
4. **Equipment** — select from SRD weapons/armor/items; set equipped weapon and worn armor
5. **Spells** — for caster classes, select known/prepared spells from the class spell list (filtered by level). Spell slots auto-calculated by class + level
6. **Features** — racial traits and class features are auto-populated from SRD data based on race + class + level selections

After creation, the character appears in the campaign and the player links to it via `/register <character_name>` in Discord (DM approves).

**Class/subclass/feat interactions:** the system implements SRD class features mechanically (Extra Attack, Sneak Attack damage, Rage bonus, etc.) and auto-applies them during combat. Non-SRD subclasses, feats, and multiclass combinations can be added manually by the DM as custom features with mechanical effects (bonus to hit, extra damage dice, etc.).

### D&D Beyond Import

Players with D&D Beyond character sheets can import directly:

1. Player provides their D&D Beyond character URL (e.g., `https://www.dndbeyond.com/characters/12345678`)
2. The system fetches character data from D&D Beyond's undocumented character API (`https://character-service.dndbeyond.com/character/v5/character/{id}`)
3. A parser converts the DDB JSON into the internal character format — mapping ability scores, class features, equipment, spells, HP, AC, and all combat-relevant fields
4. The DM reviews and approves the imported character in the dashboard before it enters play

**Implementation reference:** [MrPrimate/ddb-importer](https://github.com/MrPrimate/ddb-importer) (Foundry VTT's DDB import module) is the most mature open-source implementation of DDB character parsing. Its source code is the primary reference for handling DDB's data quirks (derived stats that need client-side calculation, nested feature structures, equipment attunement).

**Caveats:**
- D&D Beyond has no official public API — the character endpoint is undocumented and may change without notice
- Character must be set to **public** sharing on D&D Beyond for the fetch to work
- Rate limiting and CAPTCHAs may apply; the importer includes exponential backoff
- Non-SRD content (paid sourcebook subclasses, feats, spells) imports as names/descriptions but may need DM manual setup for mechanical effects not in our SRD data

**Re-sync:** players can re-import at any time to pull updates (level-ups, new equipment purchased in DDB). The system diffs against the existing character and shows the DM what changed before applying.

### Character Leveling

Level-ups are handled through the same creation workflow:
- DM edits the character's level in the dashboard
- System auto-recalculates HP, proficiency bonus, spell slots, attacks per action
- DM selects new class features / spells if applicable
- For DDB-imported characters, the player levels up in D&D Beyond and re-imports

---

## Reference Data Sources

### SRD Content (Seeded at Startup)

The system ships with the full **D&D 5e SRD** (Systems Reference Document) content, seeded into PostgreSQL on first run. Data is sourced from [5e-bits/5e-database](https://github.com/5e-bits/5e-database) — a comprehensive, MIT-licensed JSON dataset of all SRD content.

**Included SRD data:**
- **Monsters** — ~325 creature stat blocks (name, HP formula, AC, speed, attacks, abilities, CR)
- **Spells** — ~320 spells (name, level, school, range, components, duration, area, damage, save type, concentration)
- **Weapons** — all SRD weapons (damage, damage type, properties: finesse, heavy, ranged, thrown, etc.)
- **Armor** — all SRD armor (AC formula, type, stealth disadvantage, strength requirement)
- **Equipment** — adventuring gear, tools, packs
- **Classes** — all 12 SRD classes with features by level, hit dice, proficiencies, spell lists
- **Races** — all SRD races with traits, ability score bonuses, speed, darkvision
- **Conditions** — all 15 standard conditions with mechanical effects
- **Skills** — all 18 skills mapped to ability scores

**Licensing:** SRD 5.1 content is dual-licensed under OGL 1.0a and **CC-BY-4.0**. The system uses CC-BY-4.0, which requires attribution only (included in the app's about/credits page).

### Extended Content (Open5e)

For content beyond the core SRD (third-party OGL publisher content), the system can optionally pull from the [Open5e API](https://api.open5e.com/):
- ~3,200 monsters (vs ~325 in SRD alone)
- ~1,400 spells
- Content from Tome of Beasts, Creature Codex, Deep Magic, and other OGL publishers

Open5e data is fetched on-demand and cached locally. DM enables/disables third-party sources per campaign in settings.

### Homebrew Content

DMs can create custom entries for any reference data type via the dashboard:
- Custom monsters (full stat block editor)
- Custom spells, weapons, items
- Custom races and class features (name + mechanical effect)

Homebrew entries are scoped to the campaign and stored alongside SRD data with a `homebrew: true` flag.

---

## Non-Combat Gameplay

### Skill & Ability Checks

Skill checks are the backbone of non-combat D&D. The system supports them through a simple command + DM resolution flow.

**Player-initiated checks:**
```
/check perception           ← rolls d20 + WIS mod + proficiency (if proficient)
/check athletics --adv      ← roll with advantage
/check stealth --disadv     ← roll with disadvantage
/check dexterity            ← raw ability check (no skill proficiency)
```

**DM-prompted checks:**
The DM can request a check from the dashboard, which pings the player in `#your-turn`:
- "Kael, roll a Perception check" → player uses `/check perception`
- "Everyone roll Dexterity saves" → DM triggers a group save; each player is pinged and uses `/save dex`

**Mechanics:**
- Roll formula: `d20 + ability_modifier + proficiency_bonus` (if proficient in the skill)
- Expertise (Rogue, Bard): doubles proficiency bonus — tracked in `characters.proficiencies` as `{skill: "expertise"}`
- Jack of All Trades (Bard): adds half proficiency to non-proficient checks — detected from class features
- Passive checks: calculated as `10 + modifier` and displayed on the character card. DM uses passive Perception for hidden checks without alerting the player
- All rolls post to `#roll-history`; the DM sees the result in the dashboard and narrates the outcome in `#the-story`

**Group checks:** When the DM triggers a group check (e.g., "group Stealth"), all players are pinged simultaneously. The system waits for all responses (subject to the campaign's turn timeout), then reports results to the DM. Per 5e rules, the group succeeds if at least half the individuals succeed.

**Contested checks:** The DM triggers these from the dashboard (e.g., grapple: player Athletics vs target Athletics/Acrobatics). Both parties roll; the system compares and reports the winner.

### Short & Long Rests

Rests reset character resources. A player initiates a rest; the DM approves it from the dashboard (to prevent resting in unsafe situations).

**Short Rest** (`/rest short`):
1. Player types `/rest short` → request posts to `#dm-queue`
2. DM approves from dashboard
3. System prompts the player to spend hit dice: `/spend-hd 2` (spend 2 hit dice)
   - Each hit die heals `1dX + CON modifier` (X = class hit die size)
   - System rolls and applies healing automatically, capped at `hp_max`
   - Player can spend 0 to `hit_dice_remaining` dice
4. System resets all features with `recharge: "short"` in `feature_uses` (e.g., Action Surge, Channel Divinity, Second Wind)
5. Results posted to `#combat-log`: "Short rest: Kael spends 2 hit dice, heals 14 HP. Action Surge recharged."

**Long Rest** (`/rest long`):
1. Player types `/rest long` → request posts to `#dm-queue`
2. DM approves from dashboard
3. System automatically applies:
   - HP restored to `hp_max`
   - All spell slots restored to max
   - All features with `recharge: "short"` or `recharge: "long"` reset
   - Hit dice restored: regain up to half character level (minimum 1), capped at max (= character level)
   - Death save tallies reset to 0/0
4. Results posted to `#combat-log`: "Long rest: Aria fully healed. Spell slots restored. 3 hit dice recovered."

**Constraints:**
- Only one long rest per 24 in-game hours (DM tracks narrative time; system does not enforce calendar)
- Rests cannot be initiated during active combat (system checks `encounter.status != 'active'`)
- If interrupted (DM cancels mid-rest from dashboard), partial benefits may apply at DM discretion via manual override

### Exploration, Social & Travel

These modes are narrative-driven and don't need dedicated mechanical systems in MVP. The existing Discord channel structure handles them naturally:

- **Exploration:** DM narrates in `#the-story`, players describe actions in `#player-chat` or `/action`. DM calls for checks as needed (Perception, Investigation, Survival). If combat breaks out, DM starts an encounter from the dashboard.
- **Social encounters:** Players roleplay in `#the-story` or `#player-chat`. DM calls for Charisma checks (Persuasion, Deception, Intimidation) when the outcome is uncertain. No special NPC dialogue system needed — Discord's text format is ideal for RP.
- **Travel:** DM narrates distance and terrain. Random encounters are DM-triggered. Forced march / exhaustion checks can use `/check constitution` if the DM calls for them.

The `#dm-queue` channel serves as the universal escape hatch — any player action that doesn't map to a command goes there for DM resolution.

---

## DM Dashboard

The DM manages everything through a web app — they never type raw commands into Discord.

**Features:**
- **Combat Manager** — drag tokens on a grid, click to move, auto-calculates distance and range
- **HP & Condition Tracker** — click to apply damage, healing, and status conditions
- **Turn Queue** — shows current initiative order; "End Turn" auto-advances and pings the next player
- **Action Resolver** — view `#dm-queue` items, apply outcomes with a click
- **Stat Block Library** — preloaded monster stat blocks, reusable across encounters
- **Asset Library** — maps, token images, tilesets, custom monsters
- **Map Editor** — create and edit battle maps (see "Map Creation & Authoring" above)
- **Character Overview** — read-only view of all player character sheets

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| Discord Bot | [discordgo](https://github.com/bwmarrin/discordgo) | Bot logic, slash commands, message editing |
| Backend API | Go stdlib `net/http` + Chi router | Game state management, command processing |
| Database | PostgreSQL + [sqlc](https://sqlc.dev) | Type-safe Go from raw SQL — characters, campaigns, combat state, maps |
| DM Web App | Go templates for admin pages, [Svelte](https://svelte.dev) SPA for map editor | Svelte compiles to static JS/CSS — embedded in Go binary via `embed.FS`, no separate deployment |
| Map Rendering | Go `image/draw` stdlib + [gg](https://github.com/fogleman/gg) | Server-side PNG generation for grid maps |
| Real-time Sync | [nhooyr/websocket](https://github.com/nhooyr/websocket) | Live DM dashboard ↔ backend updates |
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

5. **Character data blob** — the `character_data` JSONB column stores the full dnd5e_json_schema representation. This preserves import fidelity (D&D Beyond data that doesn't map to our typed columns) and enables future export. The typed columns (`hp_max`, `ac`, `ability_scores`, etc.) are the source of truth for gameplay; `character_data` is the source of truth for display and re-export.

6. **No separate combat state table** — the encounter itself tracks `status` and `current_turn_id`. The current combat state is derived from the encounter's combatants + the active turn row. Simpler than a separate state machine.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| State drift from manual Discord edits | Enforce read-only Discord policy; all state changes via bot or dashboard only |
| Map images exceeding 8MB | Cap tile size at 48px; compress PNGs; limit grid size |
| Enemy ID ambiguity | IDs are stable, map-labeled, and confirmed in every combat log response |
| Complex player actions breaking automation | `/action` command routes to DM; no attempt to auto-parse freeform intent |
| Scope creep | Build MVP (combat only), then layer in inventory/spells/character sheets |

---

## MVP Scope

A first playable version includes:

- Discord bot posting to the correct channels
- Combat state in the database (HP, position, turn order, conditions)
- Server-side map PNG generation and Discord message editing
- Validated `/move`, `/attack`, `/cast`, `/action` slash commands
- All dice rolls auto-logged to `#roll-history`
- Turn notification pings
- Minimal DM web UI: grid view, token drag-drop, HP management, turn advancement
- Character creation in dashboard (full 5e SRD: race, class, abilities, equipment, spells)
- D&D Beyond character import (paste URL → parse → DM approves)
- SRD reference data seeded at startup (monsters, spells, weapons, armor, classes, races)

- Skill/ability checks (`/check`, `/save`) and short/long rest mechanics (`/rest`)
- Feature use tracking with short/long rest recharge

Future phases: full asset library, Open5e third-party content integration, inventory management, campaign/session management. (Note: spell slot tracking is included in MVP — see resolved issue #7. Basic non-combat gameplay — skill checks and rests — is included in MVP; see resolved issue #15.)

---

## Open Questions & Gaps

The following items need resolution before or during implementation. Refer to them by number.

### Critical — Resolve Before Building

**1. Turn Timeout / AFK Handling** ✅

Turn timeout: **24 hours**, DM-configurable per campaign (1h–72h range).

**Escalation:**
- Reminder ping at 50% of timeout (e.g., 12h) in `#your-turn`
- Final warning at 75% (e.g., 18h) — "your turn will be skipped in 6 hours"
- Auto-skip at 100% — player takes the **Dodge action with no movement**

**DM manual overrides (via dashboard):**
- **Skip now** — immediately advance past a player
- **Extend timer** — grant more time without changing the campaign default
- **Pause combat** — freeze all timers (real life happens)

**Prolonged absence:**
- After 3 consecutive auto-skips, the character is flagged as "absent" in the dashboard
- DM decides: auto-pilot the character, narrate a retreat, or remove from initiative
- Initiative slot stays reserved so the player can return seamlessly

**2. Full Turn Action Model** ✅

Turns are **sequential** — players send commands one at a time and see results before deciding their next action.

**Turn resources tracked by the backend:**
- Movement (feet remaining out of speed)
- Action (used / not used)
- Bonus action (used / not used)
- Free object interaction (used / not used)

Each command validates against remaining resources. If a player tries to use something they've already spent, the bot replies with a specific error (e.g., "You've already used your action this turn").

**Commands (updated):**

| Command | Example | Description |
|---|---|---|
| `/move` | `/move D4` | Move to coordinate. Can be used multiple times (split movement) as long as total ≤ speed |
| `/attack` | `/attack G1` or `/attack G1 handaxe --gwm` | Attack a target with equipped weapon (or specify weapon). One `/attack` per swing; backend tracks attacks remaining by class. Supports flags: `--gwm`, `--sharpshooter`, `--reckless` |
| `/cast` | `/cast fireball D5` | Cast a spell (uses action or bonus action depending on spell) |
| `/bonus` | `/bonus cunning-action dash` | Bonus action |
| `/interact` | `/interact draw longsword` | Free object interaction — routed to `#dm-queue` for DM resolution |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare reaction intent — persists until used, cancelled, or encounter ends. Routed to `#dm-queue` |
| `/action` | `/action flip the table` | Freeform action — routed to `#dm-queue` |
| `/deathsave` | `/deathsave` | Roll a death saving throw (only available while dying at 0 HP) |
| `/equip` | `/equip longsword` | Set primary weapon (persists between turns, usable any time) |
| `/done` | `/done` | Explicitly end turn, advance to next in initiative |
| `/register` | `/register Thorn` | Link your Discord account to a character (DM must approve) |

**Ending a turn:**
- **Explicit:** player sends `/done`
- **DM override:** DM can end any player's turn from the dashboard at any time
- **Timeout:** if the player goes silent mid-turn, the #1 timeout system applies — remaining unused actions are forfeited

**Split movement** works naturally: `/move D4` → `/attack G1` → `/move E5` — each `/move` deducts from remaining movement.

**3. Reactions** ✅

DM resolves all reactions manually. Players can pre-declare reaction intent at any time using `/reaction`, which posts to `#dm-queue` for the DM to act on when the trigger occurs.

**`/reaction <description>`** — declare a reaction intent
- `/reaction Shield if I get hit` → posts to `#dm-queue`: "🛡️ **Thorn** wants to react: *Shield if I get hit*"
- `/reaction OA if goblin moves away` → posts to `#dm-queue`: "⚔️ **Kael** wants to react: *OA if goblin moves away*"
- `/reaction Counterspell if enemy casts` → posts to `#dm-queue`: "✋ **Lyra** wants to react: *Counterspell if enemy casts*"
- Reaction declarations persist until used, cancelled (`/reaction cancel`), or the encounter ends
- One reaction per round per player (per D&D rules) — system tracks whether it's been spent

**DM workflow:**
1. DM sees reaction declarations in `#dm-queue` or the dashboard
2. When the trigger condition occurs during enemy/NPC turns, DM decides whether it fires
3. DM resolves the reaction in the dashboard (rolls, applies effects) and posts the result
4. System marks the player's reaction as spent for the round

**Why this works for async:**
- Zero stalling — combat never pauses waiting for a reaction response
- Players declare intent on their own time, even between turns
- DM has full control over timing and adjudication
- Readied Actions use the same flow: `/reaction I attack when the goblin moves past me`

**4. Enemy / NPC Turn Workflow** ✅

Each enemy takes its own turn in initiative order. The DM resolves enemy turns through the dashboard with **smart defaults** — the system pre-fills suggestions, DM confirms or overrides.

**Dashboard flow when it's an enemy's turn:**
1. Dashboard highlights the active enemy and pre-fills:
   - **Suggested move:** shortest path toward nearest hostile (reuses existing pathfinding)
   - **Suggested attack:** nearest target in range; defaults to creature's primary attack from stat block
   - **Suggested ability:** if the creature has a special ability (e.g. Breath Weapon), suggest it when conditions are met (multiple targets in cone/line)
2. DM clicks **Confirm** to accept defaults, or overrides any field (different target, different ability, hold position, etc.)
3. System auto-rolls to-hit vs target AC, rolls damage, applies HP changes
4. DM sees the roll results and can adjust before posting (e.g. fudge a crit that would one-shot a level 2 player)
5. On confirm, results are posted to `#combat-log` and map is updated

**`#combat-log` output** — same format as player actions:
```
🏃 Goblin 1 moves to D5
⚔️ Goblin 1 attacks Thorn — 🎲 14 vs AC 18 — Miss!
🏃 Goblin 2 moves to E4
⚔️ Goblin 2 attacks Kael — 🎲 19 vs AC 15 — Hit! 7 slashing damage
```

**Pending reactions:** if a player has a pre-declared `/reaction` that triggers during the enemy turn (e.g. Opportunity Attack on enemy movement, Shield on being hit), the dashboard surfaces it to the DM before confirming that step. DM resolves the reaction inline.

**Complexity:** low — suggested move reuses pathfinding already needed for `/move` validation, suggested target is nearest-by-distance sort, suggested action is the first entry in the creature's stat block. It's pre-filling form fields, not AI.

**5. Death Saves & Unconsciousness (0 HP)** ✅

When a character drops to 0 HP, they fall **unconscious** and begin making death saving throws. The system fully tracks this state.

**Dropping to 0 HP:**
- Character status becomes **Dying** (unconscious, prone)
- Concentration is broken automatically
- All commands except `/deathsave` are blocked on their turn
- Token state changes to "dying" on the map (distinct visual from bloodied/dead)

**Instant death check:**
- On every damage application, the system checks: if damage remaining after reaching 0 HP ≥ character's max HP, the character dies instantly
- Token state → "dead", no death saves occur

**Death saving throws:**
- When initiative reaches a dying player, the bot pings them in `#your-turn` to send `/deathsave`
- Player sends `/deathsave` → system rolls d20 (no modifiers unless granted by a feature like Diamond Soul)
- ≥10 = success, <10 = failure
- **Nat 20** → regain 1 HP, conscious, still prone. Death save tallies reset
- **Nat 1** → counts as 2 failures
- 3 successes → **stabilized** (unconscious at 0 HP, no more death saves; token state → "stable")
- 3 failures → **dead** (token state → "dead")
- Rolls and results are posted **publicly** in `#combat-log`
- If the player doesn't send `/deathsave` before the AFK timeout, the system auto-rolls for them

**Taking damage while at 0 HP:**
- Each hit = 1 automatic death save failure
- Critical hit (attacker within 5ft of unconscious target) = 2 failures
- System auto-applies these when damage targets a dying character

**Stabilization:**
- 3 death save successes, or Medicine check (DC 10), or Spare the Dying cantrip
- Stable characters remain unconscious at 0 HP, make no further death saves
- Regain 1 HP after 1d4 hours (post-combat only)

**Healing from 0 HP:**
- Any healing (Healing Word, potion, Lay on Hands, etc.) sets HP to 0 + healing amount
- Status → conscious, still **prone** (costs half movement to stand)
- All death save successes and failures reset to zero

**Token states (updated):** normal / bloodied / **dying** / **stable** / dead

**6. Authentication & Authorization** ✅

**Discord user → Character mapping:**
- Player runs `/register <character_name>` in the Discord server
- Bot creates a `discord_user_id → character_id` mapping in the database
- DM confirms/approves the registration via the dashboard
- One player = one character per campaign (DM can override in dashboard if needed)

**Out-of-turn prevention:**
- On every command (`/move`, `/attack`, `/cast`, `/bonus`, `/interact`, `/action`, `/done`, `/deathsave`), the backend validates that the requesting player's Discord user ID matches the active turn's character owner
- If not their turn, bot replies: "It's not your turn. Current turn: **[Character]** (@player)"
- Exception: `/reaction` can be submitted at any time — it's a declaration, not a turn action

**DM dashboard authentication:**
- Discord OAuth2 — DM logs in with their Discord account
- System verifies the authenticated Discord user ID matches the campaign's designated DM
- No separate passwords or accounts to manage

**Multi-campaign support:**
- One bot instance serves multiple Discord servers (multi-tenant)
- All database queries are scoped by `guild_id` / `campaign_id`
- One campaign per Discord server (keeps channel structure clean and unambiguous)
- Multiple campaigns in a single server is not MVP scope

### Significant — Will Hit During Development

~~**7. `/cast` Spell Handling**~~ **Resolved**

- **AoE targeting** — `/cast fireball D5` targets a coordinate; backend calculates affected creatures by shape/radius from spell data. Spell data includes area definitions: `{ shape: "sphere", radius_ft: 20 }`, `{ shape: "cone", length_ft: 15 }`, `{ shape: "line", length_ft: 60, width_ft: 5 }`. Cones originate from the caster and fan toward the target coordinate. All affected creatures (including allies) are listed in `#combat-log` — no confirmation prompt.
- **Spell saves** — auto-rolled for all affected creatures (enemies and allies). Saves are mechanical (d20 + modifier vs DC) with no decision-making, so auto-rolling keeps async moving. Results posted to `#combat-log`.
- **Concentration** — fully tracked by the backend:
  - Only one concentration spell active per character at a time
  - Casting a new concentration spell auto-drops the previous one (notified in `#combat-log`)
  - Taking damage while concentrating triggers an auto-rolled CON save (DC = max(10, half damage taken)); failure breaks concentration
  - Active concentration effects (e.g., Fog Cloud zone, Spirit Guardians aura) are tracked on the map
- **Spell slots** — tracked and enforced in MVP. Backend knows each character's slots per level, deducts on cast, rejects `/cast` if no slots remaining. `/cast` without slot management would break caster balance.
- **Spell range** — enforced by backend. Spell data includes range; backend validates target is within range. Touch spells require adjacency (5ft), self spells need no target.

**8. `/attack` Weapon & Option Selection** ✅

**Command syntax:** `/attack <target> [weapon] [--flags]`

Examples:
- `/attack G2` — attack with equipped weapon
- `/attack G2 handaxe` — attack with a specific weapon
- `/attack G2 --gwm` — attack with Great Weapon Master (-5 to hit, +10 damage)
- `/attack G2 longbow --sharpshooter` — attack with longbow using Sharpshooter (-5/+10)
- `/attack G1 --reckless` — Reckless Attack (advantage, enemies get advantage on you until next turn)

**Weapon selection** — each character has an equipped/primary weapon (set during character setup or via `/equip <weapon>`). `/attack G2` uses the equipped weapon; `/attack G2 handaxe` overrides for that swing. Backend validates the character actually has the specified weapon.

**Extra Attack** — resolved one swing at a time. Each `/attack` command resolves a single attack roll. The backend tracks attacks remaining per turn based on class/level (e.g., Fighter 5 = 2, Fighter 11 = 3, Fighter 20 = 4). After each `/attack`, the bot reports remaining attacks. The player can retarget freely between swings — see the result of attack 1 before choosing where to aim attack 2. If the player sends `/done` with unused attacks, those attacks are forfeited.

**Attack modifier flags** — opt-in per swing:
- `--gwm` — Great Weapon Master: -5 to hit, +10 damage. Backend validates a heavy melee weapon is equipped.
- `--sharpshooter` — Sharpshooter: -5 to hit, +10 damage. Backend validates a ranged weapon is being used.
- `--reckless` — Reckless Attack (Barbarian): grants advantage on all melee STR attacks this turn, but enemies get advantage on attacks against you until your next turn. Only valid on the first attack of the turn. Backend validates Barbarian class.
- Invalid flags return an error explaining why (e.g., "GWM requires a heavy weapon; Longsword is not heavy").

**Advantage/disadvantage** — auto-detected from tracked conditions:
- **Auto-detected (system knows):** target is prone (melee adv / ranged disadv), attacker is prone (disadv), target is restrained/stunned/paralyzed/unconscious (adv), attacker is restrained/blinded/poisoned (disadv), Reckless Attack flag (adv), attacker/target invisible (adv/disadv as appropriate)
- **Not auto-detected in MVP:** flanking (optional rule, not all tables use it — may add as campaign setting toggle later)
- **DM override:** DM can force advantage or disadvantage via the dashboard for situations the system can't detect (creative tactics, terrain, narrative). Override posts to `#combat-log`: "DM grants advantage to Kael's attack (high ground)."
- **Stacking:** when both advantage and disadvantage apply from any combination of sources, they cancel out per 5e rules — system rolls normally regardless of how many sources of each exist.

**9. Concurrency & Race Conditions** ✅

All combat state mutations are serialized through a **per-turn pessimistic lock** using PostgreSQL advisory locks keyed on `turn_id`.

**Rapid player commands** (e.g., two `/attack` commands sent before the first resolves):
- Each command acquires the turn lock before processing
- If the lock is held, the second command **blocks** (waits) rather than failing
- Once the first command completes and releases the lock, the second command processes against the updated state (correct attacks-remaining count, movement left, etc.)
- Player experiences at most a few hundred milliseconds of delay — no "conflict, retry" errors

**DM + player concurrent actions** (e.g., DM edits creature HP while player attacks that creature):
- DM dashboard mutations go through the same backend and acquire the same per-turn lock
- If a player's `/attack` is processing, the DM's edit blocks until the attack resolves, then applies against post-attack HP
- If the DM ends a player's turn, any in-flight player command either completes first (if it holds the lock) or fails with "It's no longer your turn" (if the DM's end-turn acquired the lock first)

**WebSocket state sync** — server-authoritative, push-only:
- The Go backend + PostgreSQL is the single source of truth for all game state
- The DM dashboard renders state received via WebSocket pushes; it does not maintain its own game state copy
- All mutations (from Discord commands or the dashboard) go through the backend API
- Flow: command → backend acquires lock → resolves → releases lock → pushes updated state over WebSocket → dashboard re-renders

**Dashboard optimistic UI** — when a WebSocket push arrives while the DM has a form open:
- Read-only display areas (initiative tracker, HP bars) update immediately
- Active form inputs are **not clobbered** — the DM's draft is preserved with a subtle indicator: "HP updated to 3 by player action"
- On form submit, the DM's intended value is sent to the backend and applied through the lock

**Why pessimistic locking over alternatives:**
- Optimistic locking (version numbers, retry-on-conflict) creates bad UX — players get conflict errors for normal rapid commands
- Command queues (Redis, channels) add infrastructure; the database lock achieves the same serial execution within the single-binary deployment
- Contention is inherently low: only one player acts per turn, DM interventions during a player's command are rare, and lock duration is a single fast DB transaction

**10. Discord API Constraints** ✅

- ~~Message edit rate limits (~5 edits/5s/channel) — rapid state changes could bottleneck~~ **Mitigated** — map images are appended as new messages in `#combat-map` instead of editing, avoiding edit rate limits
- ~~Slash command registration limits (100 global) and propagation delays~~ **Not a risk** — the command set is ~12 commands using arguments for variation (e.g., `/cast fireball D5`), well under the 100-command cap. Commands are registered per-guild (instant propagation) rather than globally.
- ~~Embed character limits (6000 chars) — large parties could overflow initiative tracker or character cards~~ **Resolved** — embeds are not required. The bot uses **plain text messages** for most output (combat log, initiative order, turn pings). For very large output (full initiative order with 20+ combatants, detailed character cards), the bot uploads a **text file attachment** instead. This avoids the 6000-char embed limit entirely while keeping output readable.
- ~~Required bot permissions and server setup not documented~~ **Resolved** — see below.

**Required bot permissions:**
- `Send Messages` — post to all bot-managed channels
- `Attach Files` — upload map PNGs and text file attachments
- `Manage Messages` — pin/edit the initiative tracker message
- `Use Application Commands` — register and respond to slash commands
- `Mention Everyone` — ping players on turn start (or a configurable `@combat` role)

**Server setup:** a `/setup` slash command auto-creates the channel structure (`#initiative-tracker`, `#combat-log`, `#roll-history`, `#the-story`, `#player-chat`, `#combat-map`, `#your-turn`, `#character-cards`, `#dm-queue`) with appropriate permission overrides (e.g., `#the-story` is DM-write-only, `#combat-map` is bot-write-only). DM runs `/setup` once after inviting the bot. Channels that already exist are skipped.

**11. Map & Grid Limitations** ✅
- ~~Grid size: A–Z = 26 columns max~~ **Resolved** — extended to spreadsheet-style lettering (A–Z, AA, AB, …) for arbitrarily large maps
- ~~Fog of war: can players see the entire map? Hidden enemies?~~ **Resolved** — dynamic fog of war using shared party vision (union of all player tokens' shadowcast results). Enemies in fogged cells are hidden. See "Dynamic Fog of War" section.
- ~~Vertical dimension: flying, multi-level terrain, elevation~~ **Resolved** — tokens carry altitude (integer feet), displayed as label suffix (e.g., `AR↑30`). 3D distance for range checks. See "Altitude & Elevation" section.
- ~~Map creation/import workflow for the DM~~ **Resolved** — blank grid + terrain/wall tools + image import (Phase 1), tileset painting + Tiled import (Phase 2). Maps stored as Tiled-compatible JSON. See "Map Creation & Authoring" section.

**12. Rollback / Undo** ✅

The DM dashboard is the correction mechanism. Discord messages are an append-only log — corrections are posted as new messages, not edits or deletions.

**Action log:** every state mutation (HP change, position move, condition applied/removed, spell slot spent, etc.) is recorded in an append-only `action_log` table (`action_id`, `turn_id`, `timestamp`, `action_type`, `before_state`, `after_state`). This enables single-step undo and provides an audit trail.

**DM correction tools (dashboard):**
- **Undo last action** — reverts the most recent mutation by restoring its `before_state`. Only the DM can undo. Can be applied repeatedly to walk back multiple steps within the current turn.
- **Manual state override** — DM can directly edit any value at any time: set HP, move a token, add/remove conditions, adjust spell slots, change initiative order. Overrides go through the same per-turn lock as normal commands.
- **Discord correction message** — every undo or manual override posts a correction to `#combat-log`: "⚠️ **DM Correction:** Goblin #1 HP adjusted (resistance to fire was missed)". Original messages are never edited or deleted.

**Not in MVP:**
- Full turn rewind (reverting an entire multi-action turn across multiple affected creatures is complex and error-prone — DM uses manual overrides instead)
- Player-initiated undo (all corrections are DM-only)

### Minor — Good to Decide

**13. Diagonal Movement Rule** ✅

Diagonals cost 5ft — same as cardinal movement. This is a deliberate simplification for async play, where easy mental math matters more than geometric precision. The PHB alternating 5/10 variant is not supported.

**14. Data Model / Schema** ✅

Full database schema defined — see "Data Model" section. Key decisions:
- Campaign → Encounter → Combatants → Turns hierarchy with instance-level combat state
- Character ↔ Player mapping via `player_characters` table with DM approval
- SRD reference data (creatures, spells, weapons, armor, classes, races) seeded from [5e-bits/5e-database](https://github.com/5e-bits/5e-database), extensible with homebrew per campaign
- Full 5e character creation in dashboard + D&D Beyond import via undocumented character API
- Internal character format based on [BrianWendt/dnd5e_json_schema](https://github.com/BrianWendt/dnd5e_json_schema)
- JSONB for flexible nested data (features, inventory, spell slots); typed columns for combat-critical fields (HP, AC, position)
- Action log with before/after state snapshots for undo and audit trail

**15. Non-Combat Gameplay** ✅

Skill/ability checks and short/long rest mechanics are now in MVP scope — see "Non-Combat Gameplay" section. `/check`, `/save`, and `/rest` commands added. Hit dice tracking and feature-use recharge (`feature_uses` column) added to the data model. Exploration, social, and travel are handled narratively through existing Discord channels and `#dm-queue`. Leveling was already covered (see "Character Leveling" section).

**16. Tech Stack Decision** ✅

Go for everything. discordgo for the bot, stdlib `net/http` + Chi for the API, sqlc for type-safe database access, Go `image` stdlib for map rendering. DM dashboard is split: Go `html/template` for admin/combat management pages, Svelte SPA for the interactive map editor (drag-and-drop tokens, fog of war). Svelte compiles to static JS/CSS at build time — embedded into the Go binary via `embed.FS` for single-artifact deployment. Node is a dev-only build dependency, not a runtime dependency.
