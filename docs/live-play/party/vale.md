# Vale — character sheet

> Per-PC detail. The at-a-glance line lives in [`roster.md`](roster.md); only
> load this file when you need Vale's full kit (a cast, an equip ruling, a save).
> **DURABLE reference only** (build / kit / spells / traits). Live HP / slots /
> conditions / position come from the **DM Console** (`GET /api/dm/situation`) or
> `/character` — **not** this file.

- **Player:** the user (DM's primary).
- **Character id:** `b6ca7f49-c173-4290-8c80-6fb785fbe733`
- **Tiefling Warlock 4** (patron: **the Fiend**), entertainer background. _L4 ASI: **+2 CHA → 18**._
- **APPROVED** 2026-06-25 (first portal character; built clean after the
  ISSUE-001 / 008 fixes went live — see [`../issues.md`](../issues.md)).

## Core stats

| | |
| --- | --- |
| HP | **24/24** |
| AC | **10** (DEX +0, no armor equipped — leather sits in the pack; equip → **AC 11**) |
| Speed | 30 |
| Prof bonus | +2 |
| Abilities | STR 10 · DEX 10 · CON 15 (+2) · INT 14 · **CHA 18 (+4)** · WIS 10 |
| Saves | WIS, CHA |
| Skills | acrobatics, performance, deception, history |

## Spellcasting — Pact Magic

- **2 slots @ slot level 2** (`pact_magic_slots {current:2, max:2, slot_level:2}`).
- Spell save **DC 14**, spell atk **+6** (CHA).
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
- **GEAS — DISCHARGED / arc resolved 07-03** — the debt to carry the god's tale and
  tell it TRUE was **paid**: Vale told the god its true story (**Performance 21**, backed
  by two recovered name-scraps), a truth-taster tasted no lie, and the god was
  **un-forgotten and released** (see [`../world.md`](../world.md) "The Faceless God — arc
  RESOLVED"). She still carries the god's items out of the gallery — the **cold-iron token**
  (its mark, *"a key that likes to open forgotten things"*), the **ashen face-shard**, and
  the **name-scraps** of the un-rememberable name (the campaign's long thread, now pointing
  OUTWARD). Live inventory: her `/character` sheet / the DM Console.
