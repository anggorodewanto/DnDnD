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
Goblin #1  (G1)  HP: 12/12  [B4]
Goblin #2  (G2)  HP:  8/12  [C6]
Orc Shaman (OS)  HP: 28/28  [D5]
```

**Rules:**
- IDs are stable for the entire encounter — if G1 dies, G2 stays G2
- Token labels on the map image display these IDs so players can target by sight
- The combat log confirms the reference in every response

### Grid Movement — Chess Notation

The map uses a standard chess-style coordinate grid: **columns as letters (A–Z), rows as numbers**.

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
```

The backend validates every movement command:
- Is the destination within the character's remaining movement speed?
- Is the tile occupied?
- Is there difficult terrain or an obstacle?

If invalid, the bot replies with a specific reason in `#combat-log` before DM intervention is needed.

**Diagonal movement:** Follows 5e standard rules — diagonals cost 5ft, same as cardinal movement. This is enforced by the validator.

### Structured Commands

| Command | Example | Description |
|---|---|---|
| `/move` | `/move D4` | Move token to grid coordinate (repeatable for split movement) |
| `/attack` | `/attack G2` | Attack a target by ID (repeatable for Extra Attack) |
| `/cast` | `/cast fireball D5` | Cast a spell, with target coordinate or enemy ID |
| `/bonus` | `/bonus cunning-action dash` | Bonus action |
| `/shove` | `/shove OS` | Shove a target by ID |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare a reaction intent, posted to `#dm-queue` for DM to resolve |
| `/interact` | `/interact draw longsword` | Free object interaction, routed to DM |
| `/action` | `/action flip the table at B3 for cover` | Freeform action, routed to DM |
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

Player inputs:
```
/move D4
/attack G1
```

Bot output in `#combat-log`:
```
🗺️  Aria moves from C3 → D4  (25ft used of 30ft)
⚔️  Aria attacks Goblin #1
    → Roll to hit: 19 (14 + 5) vs AC 13 — HIT
    → Damage: 9 slashing
    → Goblin #1: 12 HP → 3 HP  ⚠️ Bloodied
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

---

## DM Dashboard

The DM manages everything through a web app — they never type raw commands into Discord.

**Features:**
- **Combat Manager** — drag tokens on a grid, click to move, auto-calculates distance and range
- **HP & Condition Tracker** — click to apply damage, healing, and status conditions
- **Turn Queue** — shows current initiative order; "End Turn" auto-advances and pings the next player
- **Action Resolver** — view `#dm-queue` items, apply outcomes with a click
- **Stat Block Library** — preloaded monster stat blocks, reusable across encounters
- **Asset Library** — maps, token images, custom monsters
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
| `/attack` | `/attack G1` | Attack a target. Repeatable for Extra Attack (backend tracks attacks remaining by class) |
| `/cast` | `/cast fireball D5` | Cast a spell (uses action or bonus action depending on spell) |
| `/bonus` | `/bonus cunning-action dash` | Bonus action |
| `/interact` | `/interact draw longsword` | Free object interaction — routed to `#dm-queue` for DM resolution |
| `/reaction` | `/reaction Shield if I get hit` | Pre-declare reaction intent — persists until used, cancelled, or encounter ends. Routed to `#dm-queue` |
| `/action` | `/action flip the table` | Freeform action — routed to `#dm-queue` |
| `/deathsave` | `/deathsave` | Roll a death saving throw (only available while dying at 0 HP) |
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

**8. `/attack` Weapon & Option Selection**
- Multiple weapons — `/attack G2` doesn't specify *with what*
- Extra Attack — fighters get 2–4 attacks per action
- Attack modifiers — Great Weapon Master (-5/+10), Sharpshooter, Reckless Attack
- Advantage/disadvantage — does the system auto-detect conditions (prone, flanking, invisible)?

**9. Concurrency & Race Conditions**
- Player submits two commands before the first resolves
- DM applies dashboard changes while a player is mid-turn
- WebSockets for sync but no conflict resolution strategy
- Needs: command queuing, optimistic locking, or turn-state locking

**10. Discord API Constraints**
- ~~Message edit rate limits (~5 edits/5s/channel) — rapid state changes could bottleneck~~ **Mitigated** — map images are appended as new messages in `#combat-map` instead of editing, avoiding edit rate limits
- Slash command registration limits (100 global) and propagation delays
- Embed character limits (6000 chars) — large parties could overflow initiative tracker or character cards
- Required bot permissions and server setup not documented

**11. Map & Grid Limitations**
- Grid size: A–Z = 26 columns max; is this enough for large outdoor maps?
- ~~Fog of war: can players see the entire map? Hidden enemies?~~ **Resolved** — dynamic fog of war using shared party vision (union of all player tokens' shadowcast results). Enemies in fogged cells are hidden. See "Dynamic Fog of War" section.
- Vertical dimension: flying, multi-level terrain, elevation
- Map creation/import workflow for the DM

**12. Rollback / Undo**
No mistake recovery. Can the DM undo the last action, revert a turn, or correct a misapplied rule? Since Discord is read-only output, corrections need a system-level mechanism.

### Minor — Good to Decide

**13. Diagonal Movement Rule**
Spec uses the simplified rule (all diagonals = 5ft). The PHB variant (5ft/10ft alternating) is more commonly used. Should this be configurable or is the simplification intentional?

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
