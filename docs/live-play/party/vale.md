# Vale — character sheet

> Per-PC detail. The at-a-glance line lives in [`roster.md`](roster.md); only
> load this file when you need Vale's full kit (a cast, an equip ruling, a save).
> **Current HP / position / conditions track in [`roster.md`](roster.md)** — keep
> them there in lockstep with the live DB; this file is the durable sheet.

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
- Down to **1 pact slot** after the cellar-wretch fight (no long rest yet) — matters
  for the cellar descent (can't hold-person-lock two foes). See
  [`../encounters/cellar-brood.md`](../encounters/cellar-brood.md).
