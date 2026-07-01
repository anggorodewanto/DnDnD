# Vale — character sheet

> Per-PC detail. The at-a-glance line lives in [`roster.md`](roster.md); only
> load this file when you need Vale's full kit (a cast, an equip ruling, a save).
> **DURABLE reference only** (build / kit / spells / traits). Live HP / slots /
> conditions / position come from the **DM Console** (`GET /api/dm/situation`) or
> `/character` — **not** this file.

- **Player:** the user (DM's primary).
- **Character id:** `b6ca7f49-c173-4290-8c80-6fb785fbe733`
- **Tiefling Warlock 3** (patron: **the Fiend**), entertainer background.
- **APPROVED** 2026-06-25 (first portal character; built clean after the
  ISSUE-001 / 008 fixes went live — see [`../issues.md`](../issues.md)).

## Core stats

| | |
| --- | --- |
| HP | **24/24** |
| AC | **10** (DEX +0, no armor equipped — leather sits in the pack; equip → **AC 11**) |
| Speed | 30 |
| Prof bonus | +2 |
| Abilities | STR 10 · DEX 10 · CON 15 (+2) · INT 14 · **CHA 16 (+3)** · WIS 10 |
| Saves | WIS, CHA |
| Skills | acrobatics, performance, deception, history |

## Spellcasting — Pact Magic

- **2 slots @ slot level 2** (`pact_magic_slots {current:2, max:2, slot_level:2}`).
- Spell save **DC 13**, spell atk **+5** (CHA).
- **Cantrips:** chill touch, mage hand (+ thaumaturgy from Infernal Legacy).
- **Known spells:** hellish rebuke (L1) · hold person, shatter, misty step (L2).
- Infernal Legacy also grants **1/day hellish rebuke @ L2** (free, CHA).

## Racial / kit

- Tiefling: **resistance to fire** (Hellish Resistance), darkvision.
- **Kit:** dagger, light crossbow + bolts, arcane focus, dungeoneer's pack,
  entertainer's pack (instrument, costume), leather armor (equip → AC 11).
- Languages empty (ISSUE-009 cosmetic gap — fixed for new builds, not backfilled).

## Live notes

- Equip leather armor for AC 11: `/equip item:leather armor:true` (item already in
  inventory). Dagger / light-crossbow likewise equippable.

## Story-durable kit (narrative, not on the character-sheet inventory)

Clue-relics + bindings Vale carries for the faceless-god arc — narrative state the
DB can't derive; see [`../game-state.md`](../game-state.md) + [`../sessions/`](../sessions/):

- **Patron-conduit disc** — a broken round disc of exotic metal ("looks like stone,
  but it isn't"), worn at her neck. Both *"one of the clues"* AND *"the tether, a
  conduit, between me and my patron."* Shown to the faceless god as auto-proof (a god
  that trades only in kind tastes its own).
- **Ashen face-shard relic** *(taken 07-02)* — a smooth blank ashen face the size of
  her palm, humming like a struck bell; *"a story with the name still folded inside."*
  A story-vessel, almost certainly another of the god's scattered clues. Lifted with
  Mage Hand from the god's own offering.
- **GEAS — owe the god the telling** *(struck 07-02)* — the price of taking the shard:
  carry the god's tale, tell it TRUE, un-forget it — *"a debt no god forgives."*
  **Struck on a Deception (her 21)** — a truth-taster paid in a lie; the geas holds
  only while she tells the tale true / the bluff never surfaces. **Latent hook, not
  resolved.**
