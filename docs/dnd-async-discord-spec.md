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
| `/move` | `/move D4` | Move token to grid coordinate |
| `/attack` | `/attack G2` | Attack a target by ID |
| `/cast` | `/cast fireball G1 G2` | Cast a spell, optionally with targets |
| `/shove` | `/shove OS` | Shove a target by ID |
| `/action` | `/action flip the table at B3 for cover` | Freeform action, routed to DM |

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
| Discord Bot | discord.js (Node) | Bot logic, slash commands, message editing |
| Backend API | Node/Express or FastAPI | Game state management, command processing |
| Database | PostgreSQL + Prisma | Characters, campaigns, combat state, maps |
| DM Web App | React (Next.js) | DM dashboard UI |
| Map Rendering | Node Canvas or Python Pillow | Server-side PNG generation |
| Real-time Sync | WebSockets | Live DM dashboard ↔ backend updates |

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

**1. Turn Timeout / AFK Handling**
The system is async, but there's no mechanism for when a player simply doesn't take their turn. Needs: timeout duration (e.g., 24h), reminder pings, DM override to skip/delay, and what happens to initiative order during prolonged absence.

**2. Full Turn Action Model**
A 5e turn can include movement, action, bonus action, reaction, and free object interaction. The spec only models `/move` + one action. Specifically:
- Can a player split movement (move → attack → move again)?
- How are bonus actions handled? (Cunning Action, Spiritual Weapon, Flurry of Blows)
- How are free object interactions handled? (draw weapon, open door)
- Should a player submit all parts of their turn at once, or sequentially? When is a turn "done"?

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

**16. Tech Stack Decision**
"Node/Express **or** FastAPI" — this needs to be a firm choice. It affects map rendering (Node Canvas vs Pillow), ORM, and deployment. Mixing Node bot + Python backend adds operational complexity.
