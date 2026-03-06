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

**Estimated build time (solo developer): 6–10 weeks**

Future phases: full asset library, character sheet integration, inventory management, campaign/session management. (Note: spell slot tracking is included in MVP — see resolved issue #7.)

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

**14. Data Model / Schema**
No ERD or schema. Key relationships are ambiguous:
- Campaign → Encounter → Combat State → Turns
- Character ↔ Player (Discord user)
- Spell lists, class features, ability storage
- Character creation workflow

**15. Non-Combat Gameplay (Future)**
Even for future planning, no mention of: skill/ability checks, social encounters, exploration/travel, short/long rest mechanics, leveling up.

**16. Tech Stack Decision** ✅

Go for everything. discordgo for the bot, stdlib `net/http` + Chi for the API, sqlc for type-safe database access, Go `image` stdlib for map rendering. DM dashboard is split: Go `html/template` for admin/combat management pages, Svelte SPA for the interactive map editor (drag-and-drop tokens, fog of war). Svelte compiles to static JS/CSS at build time — embedded into the Go binary via `embed.FS` for single-artifact deployment. Node is a dev-only build dependency, not a runtime dependency.
