# Spec Review: Async D&D via Discord

Review of `dnd-async-discord-spec.md` — identifying gaps, ambiguities, and risks.

---

## Critical Gaps (Address Before Building)

### 1. No Turn Timeout or AFK Handling

The spec says players act "on their own schedule" but never addresses what happens when a player doesn't take their turn. Missing:
- Timeout duration (e.g., 24h) before auto-skipping or reminding
- Escalating pings / reminder cadence
- DM override to skip or delay a player's turn
- What happens to initiative order if a player is absent for days

This is the **single biggest operational risk** for an async system.

### 2. Multi-Step Turns Are Underspecified

A D&D 5e turn can include: movement, action, bonus action, reaction, free object interaction. The spec only models `/move` + one action.

- **Split movement** — Can a player move, attack, then move again? The command model implies a single `/move` then `/attack`.
- **Bonus actions** — No `/bonus` command. Rogue's Cunning Action, cleric's Spiritual Weapon, monk's Flurry of Blows — all need this.
- **Free object interactions** — Draw a weapon, open a door, pick up an item — no command.
- **Command batching** — Should a player submit all parts of their turn at once, or sequentially? If sequentially, when is the turn "done"?

### 3. Reactions Are Missing Entirely

Reactions are the hardest async problem in D&D and the spec ignores them.

- **Opportunity Attacks** — When an enemy moves away from a fighter, does the system pause and ping the fighter? That could stall the game for hours.
- **Counterspell / Shield** — These interrupt *during* another creature's action. How?
- **Readied Actions** — A player says "I ready an attack for when the goblin moves." How is this tracked and triggered?

**Design options to consider:**
- Auto-skip reactions with a short window (e.g., 10 minutes)
- Pre-declare reactions at the start of your turn ("If anyone moves away from me, I take an OA")
- DM resolves reactions manually
- Remove reactions from the system entirely (significant 5e simplification)

### 4. Enemy/NPC Turns Are a Black Box

The spec details player turns but says nothing about enemy turns:
- Does the DM manually execute every enemy action in the dashboard?
- Does the bot auto-resolve enemy attacks (roll to hit, damage)?
- How are enemy actions communicated to `#combat-log`?
- If there are 8 goblins, does the DM click through each one individually? That's extremely tedious.
- Can the DM batch/group identical enemy actions?

### 5. Death Saves and Unconsciousness

A character hitting 0 HP is unaddressed. The spec shows "bloodied" and "dead" token states but no "unconscious/dying" state:
- Death saving throws on the downed player's turn
- Tracking successes/failures
- Stabilization
- Healing from 0 HP
- Instant death (massive damage)
- What happens to the downed player's position on the turn order?

### 6. No Authentication or Authorization Model

- How does the system know which Discord user maps to which character?
- Can any player type `/attack` or only the player whose turn it is?
- What prevents a player from submitting commands out of turn?
- How does the DM authenticate to the web dashboard?
- Multi-campaign: can one bot instance serve multiple Discord servers?

---

## Significant Gaps (Will Hit During Development)

### ~~7. `/cast` Command is Too Vague~~ Resolved

- **AoE targeting** — target a coordinate, backend calculates affected creatures by shape/radius from spell data
- **Spell saves** — auto-rolled for all affected creatures (enemies and allies)
- **Concentration** — fully tracked: one spell at a time, auto-drop on new cast, auto-roll CON save on damage
- **Spell slots** — tracked and enforced in MVP (not deferred to future phase)
- **Spell range** — enforced by backend using spell range data

### 8. `/attack` Lacks Weapon/Option Selection ✅ **Resolved**

- **Weapon selection** — default equipped weapon + optional override: `/attack G2` or `/attack G2 handaxe`. Primary weapon set via `/equip`.
- **Extra Attack** — one `/attack` per swing. Backend tracks attacks remaining by class/level. Player sees result of each attack before choosing the next target/weapon. Unused attacks forfeited on `/done`.
- **Attack modifiers** — opt-in flags per swing: `--gwm`, `--sharpshooter`, `--reckless`. Backend validates eligibility (weapon type, class).
- **Advantage/disadvantage** — auto-detected from tracked conditions (prone, restrained, stunned, paralyzed, unconscious, blinded, poisoned, invisible, Reckless Attack). No auto-flanking in MVP. DM override via dashboard for edge cases. Multiple sources of adv/disadv cancel per 5e rules.

### 9. Concurrency and Race Conditions ✅ **Resolved**

- **Per-turn pessimistic locking** — PostgreSQL advisory lock keyed on `turn_id` serializes all combat state mutations. Rapid player commands block (wait) rather than fail; second command processes against updated state.
- **DM + player concurrency** — DM dashboard mutations go through the same lock. No special conflict resolution needed; contention is inherently low (one player per turn, short lock duration).
- **WebSocket sync** — server-authoritative, push-only. Dashboard renders server state; no client-side game state to conflict. Active DM form inputs are not clobbered by incoming updates (optimistic UI with "value changed" indicator).

### 10. Map/Grid Design Limitations ✅ **Resolved**

- **Grid size** — Extended to spreadsheet-style lettering (A–Z, AA, AB, …). No practical column limit.
- **Fog of war** — Previously resolved (dynamic shadowcasting, shared party vision).
- **Vertical dimension** — Tokens carry altitude as integer feet, displayed as label suffix (`AR↑30`). 3D Euclidean distance for range checks. Ascending/descending costs movement 1:1.
- **Map creation** — Full authoring workflow defined: blank grid → terrain/wall tools → image import (Phase 1). Tileset painting + Tiled desktop import (Phase 2). Maps stored as Tiled-compatible JSON (`.tmj` format), parsed via `go-tiled` or `encoding/json`.

### 11. Discord API Constraints ✅ **Resolved**

- **Message edit rate limits** — Mitigated: map images appended as new messages, not edited.
- **Slash command limits** — Not a risk: ~12 commands using arguments, registered per-guild for instant propagation.
- **Embed character limits** — Resolved: bot uses plain text messages for most output; uploads text file attachments for very large output instead of embeds.
- **Bot permissions** — Resolved: required permissions documented (`Send Messages`, `Attach Files`, `Manage Messages`, `Use Application Commands`, `Mention Everyone`). `/setup` command auto-creates channel structure with permission overrides.

### 12. Rollback / Undo

No mention of mistake recovery. Can the DM:
- Undo the last action?
- Revert a full turn?
- Correct a misapplied rule?

Since "Discord is read-only output," you can't just delete a message and pretend it didn't happen. The system needs a correction mechanism.

---

## Minor Issues

### 13. Diagonal Movement Rule

The spec states diagonals cost 5ft, "same as cardinal movement." This is the *optional simplified rule*. The PHB variant (5ft/10ft alternating) is more commonly used at tables. Should either acknowledge this is a deliberate simplification or make it configurable.

### 14. Data Model Is Implied but Not Defined

No schema or ERD. Key relationships are ambiguous:
- Campaign → Encounter → Combat State → Turns
- Character ↔ Player (Discord user)
- How are spell lists, class features, and abilities stored/modeled?
- What's the character creation workflow?

### 15. Non-Combat Gameplay

Acknowledged as out of MVP scope, but even the future phases list is thin. No mention of:
- Skill checks / ability checks
- Social encounters / NPC dialogue
- Exploration / travel
- Short and long rest mechanics
- Leveling up

### 16. Tech Stack Indecision

"Node/Express **or** FastAPI" for the backend. This should be a firm decision before coding — it affects the map rendering pipeline (Node Canvas vs Python Pillow), the ORM choice, and the deployment model. Mixing Node bot + Python backend adds deployment complexity.

---

## Recommendations

**Before writing code, resolve:**
1. Turn timeout and AFK handling (#1)
2. Full turn action model — movement splits, bonus actions, command batching (#2)
3. Reaction strategy — pick one of the design options and commit (#3)
4. Enemy turn workflow for the DM (#4)
5. 0 HP / death save mechanics (#5)
6. Auth model — Discord user ↔ character mapping and turn enforcement (#6)

**These will fundamentally shape the database schema and bot command architecture.** Getting them wrong means significant rework later.
