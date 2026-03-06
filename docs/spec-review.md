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

### 7. `/cast` Command is Too Vague

- **AoE targeting** — `/cast fireball G1 G2` targets creatures, but Fireball targets a *point* and hits everyone in a 20ft radius. Should be `/cast fireball D5` (a coordinate), with the backend calculating who's in the blast.
- **Spell save DCs** — Who rolls the save? Is it auto-rolled for enemies?
- **Concentration** — No mention. What happens when a player casts a concentration spell while already concentrating on another?
- **Spell slots** — `/cast` is in MVP but spell slot tracking is listed as a "future phase." Does MVP ignore slot limits?
- **Spell range** — Does the backend validate range to target?

### 8. `/attack` Lacks Weapon/Option Selection

- Multiple weapons — `/attack G2` doesn't specify *with what*.
- **Extra Attack** — Fighters, monks, etc. get multiple attacks per action. How?
- Attack modifiers — Great Weapon Master, Sharpshooter, Reckless Attack.
- **Advantage/disadvantage** — Does the system auto-detect conditions (prone, flanking, invisible)?

### 9. Concurrency and Race Conditions

- Player submits two `/attack` commands before the first resolves
- DM applies changes in the dashboard while a player is mid-turn
- WebSockets mentioned for "live sync" but no conflict resolution strategy
- Need command queuing or optimistic locking

### 10. Map/Grid Design Limitations

- **Grid size** — Chess notation A–Z gives 26 columns max. Sufficient for most encounters, but large outdoor maps?
- **Fog of war** — No mention. Can players see the entire map?
- **Vertical dimension** — Flying, multi-level terrain, elevation not addressed.
- **Map creation** — How does the DM author/import maps? "Asset Library" is mentioned but the workflow isn't defined.

### 11. Discord API Constraints

- **Message edit rate limits** — Discord rate-limits to ~5 edits per 5 seconds per channel. Rapid state changes could hit this.
- **Slash command limits** — Discord caps at 100 global commands with propagation delays.
- **Embed character limits** — Initiative tracker, character cards could exceed Discord's 6000-character embed limit with large parties.
- **Bot permissions** — Required Discord permissions and server setup not documented.

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
