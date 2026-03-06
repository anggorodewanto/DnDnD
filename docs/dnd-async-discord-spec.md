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
- Bot **edits the existing message** in `#combat-map` (no new message spam)
- Token labels display enemy IDs (G1, OS, etc.) and player initials
- Token visual states: normal / bloodied / dead
- Tile size: 32–48px per square to stay within Discord's 8MB file limit
- Obstacles and difficult terrain are drawn as part of the base map layer

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

Future phases: full asset library, character sheet integration, spell slot tracking, inventory management, campaign/session management.

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
| `/action` | `/action flip the table` | Freeform action — routed to `#dm-queue` |
| `/done` | `/done` | Explicitly end turn, advance to next in initiative |

**Ending a turn:**
- **Explicit:** player sends `/done`
- **DM override:** DM can end any player's turn from the dashboard at any time
- **Timeout:** if the player goes silent mid-turn, the #1 timeout system applies — remaining unused actions are forfeited

**Split movement** works naturally: `/move D4` → `/attack G1` → `/move E5` — each `/move` deducts from remaining movement.

**3. Reactions**
Reactions interrupt other creatures' turns and are the hardest async D&D problem. Unaddressed cases:
- Opportunity Attacks — enemy moves away from a fighter; does the system pause and ping?
- Counterspell / Shield — these interrupt *during* another creature's action
- Readied Actions — "I attack when the goblin moves"
- Possible approaches: auto-skip with a short window, pre-declare reactions at start of turn, DM resolves manually, or remove reactions entirely.

**4. Enemy / NPC Turn Workflow**
How does the DM execute enemy turns?
- Manual click-by-click per enemy in the dashboard?
- Batch actions for groups of identical enemies?
- Does the bot auto-resolve enemy attacks (roll to hit, damage)?
- How are enemy actions posted to `#combat-log`?

**5. Death Saves & Unconsciousness (0 HP)**
No mechanic for when a character drops to 0 HP:
- Death saving throws on the downed player's turn
- Tracking successes/failures
- Stabilization and healing from 0 HP
- Instant death from massive damage
- Token state: "bloodied" and "dead" exist, but "dying/unconscious" does not

**6. Authentication & Authorization**
- How does the system map a Discord user to a character?
- How is out-of-turn command submission prevented?
- How does the DM authenticate to the web dashboard?
- Can one bot instance serve multiple campaigns / Discord servers?

### Significant — Will Hit During Development

**7. `/cast` Spell Handling**
- AoE spells target a *point*, not creatures — `/cast fireball D5` not `/cast fireball G1 G2`; backend calculates who's in the radius
- Who rolls spell saves? Auto-rolled for enemies?
- Concentration tracking — what happens when casting a new concentration spell while concentrating?
- Spell slot validation — `/cast` is MVP but slot tracking is "future phase"; does MVP ignore limits?
- Spell range validation

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
- Message edit rate limits (~5 edits/5s/channel) — rapid state changes could bottleneck
- Slash command registration limits (100 global) and propagation delays
- Embed character limits (6000 chars) — large parties could overflow initiative tracker or character cards
- Required bot permissions and server setup not documented

**11. Map & Grid Limitations**
- Grid size: A–Z = 26 columns max; is this enough for large outdoor maps?
- Fog of war: can players see the entire map? Hidden enemies?
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
