# Windreth — character sheet

> Per-PC detail. The at-a-glance line lives in [`roster.md`](roster.md); only
> load this file when you need Windreth's full kit. **DURABLE reference only**
> (build / kit / traits). Live HP / conditions / position come from the **DM
> Console** (`GET /api/dm/situation`) or `/character` — **not** this file.

- **Player:** the DM's friend — **new 3rd player** (joined 2026-07-02), built via
  the portal. (Remote vs local TBD; reached the builder + OAuth normally.)
  **Discord handle: `Winfroz`** (confirmed 07-02 — Party Overview shows exactly 3 PCs
  and no pending approvals, so the new author "Winfroz" acting in-character maps to
  Windreth by elimination). Watch for his actions under `Winfroz` in #in-character.
- **Character id:** `b2c436da-6762-458f-8016-3fe8f18e35e6`
- **High-Elf Rogue 4** (Roguish Archetype: **Thief**), **urchin** background. _(Approved at
  L3 07-02; **milestone L4 applied 07-03** — Defensive Duelist, below.)_
- **APPROVED** 2026-07-02 via the dashboard approvals page
  (`POST /dashboard/api/approvals/…/approve → 200`; reviewed first — clean, legal
  L3 build). Character card auto-posted to #character-cards on approval; bot
  auto-DMed the welcome.

## Core stats

| | |
| --- | --- |
| HP | **24/24** |
| AC | **15** (leather + DEX +4) |
| Speed | 30 |
| Prof bonus | +2 |
| Abilities | STR 8 (−1) · **DEX 18 (+4)** · CON 14 (+2) · INT 11 · WIS 14 (+2) · CHA 10 |
| Saves | DEX, INT |
| Skills | acrobatics, athletics, insight, perception, sleight-of-hand, **investigation, stealth** |
| Expertise | **investigation, stealth** (double proficiency) |

## Racial / kit

- **High-elf:** Fey Ancestry (adv vs charm, no magic sleep), Keen Senses
  (Perception prof), **Trance** (4 h "sleep", no long-rest unconsciousness),
  darkvision 60 ft.
- **Rogue (Thief):** **Sneak Attack** (+2d6 on advantage / an ally-adjacent
  target), **Cunning Action** (Dash/Disengage/Hide as a bonus action), Thieves'
  Cant, Thief archetype (fast hands / second-story work — engine-tagged).
- **Weapon masteries:** dagger, shortsword.
- **Feat (L4): Defensive Duelist** — 2024 half-feat: **+1 DEX → 18** (so AC 14→15);
  **reaction:** add proficiency bonus to AC vs a melee hit while wielding a finesse
  weapon. Applied 07-03.
- **Kit:** shortbow + arrows, shortsword, dagger, small knife, leather armor,
  **thieves' tools**, burglar's pack, common clothes; urchin trinkets — **map of a
  city**, **pet mouse**, **token of remembrance**.
- **Languages:** Common, Elvish. *(Cosmetic gap, same class as Vale's ISSUE-009:
  a 2024 high-elf should also get a wizard cantrip + a 3rd language — not wired,
  non-blocking, left as-is rather than stall a waiting player.)*

## Live notes

- **Player-controlled** — narrate his *arrival / the world's reaction*, never his
  choices or dialogue. (Standing rule, see [`../dm-rules.md`](../dm-rules.md).)
- **Role:** scout / skirmisher / the party's lock-trap-and-lore reader — Expertise
  in **investigation + stealth** makes him the natural examine/scout lead
  (Vale + Forge already read the shard; Windreth is built to out-read both).
  Sneak Attack + Cunning Action make him the mobile burst-damage PC in a fight.
- **Joined the fiction** as a traveler who followed the cold down through Ashfall (per
  [`../world.md`](../world.md) fold-in) — **no backstory imposed, motive left to the
  player.** Play-by-play beats now live in [`../sessions/session-01.md`](../sessions/session-01.md).

## Story-durable kit (arc-critical — on his `/character` sheet)

- **⚠ THE SEAL — _The Kept Name-Scrap (warded)_** (`identified:false`, on **Windreth's** sheet, not Vale's —
  verified 07-14). The Order's kept prize taken at the Palewatch vault; **likely Windreth's own stolen name**,
  strongly implied, never confirmed. He keeps it **buried and cold** (07-13 tail RP). It opens **only on a proper
  reading** — a route runs through Sesh's **Sabinnet** (the hostile Reader-under-glass), NOT at the scene and never
  on a failed roll. See [`../game-state.md`](../game-state.md) "THE SEAL."
- **Token of Remembrance** — minor keepsake on his sheet.
