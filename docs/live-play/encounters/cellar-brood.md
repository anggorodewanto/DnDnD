# Encounter — Cellar: the brood (PRE-BUILT, one click to run)

> The next fight, already built in the dashboard. When the party descends the
> cellar, **open this encounter → Start Combat** — no setup needed. Lore in
> [`../world.md`](../world.md); how to drive Start Combat in
> [`../runbook.md`](../runbook.md) §4.

## Identity

| Field | Value |
| --- | --- |
| Encounter (template) id | `0a54efd4-a3a2-47b5-ac7f-0030a9cb22d1` |
| DM name | "Cellar — the brood" |
| Player-facing name | "The Cellar" |
| Map | "Ashfall Waystation — cellar" `d2fe03c6-9749-4a24-a6e3-cb9d3a77e3cd` |

## Map

12×10 blank stone grid (built via Maps → New Map). A **PC spawn zone at the
top-center stairs landing** — the party auto-seats there on Start Combat. Cellar
features (pillars, the deeper shaft, the reek) are **narrated, not painted** —
same convention as the common room.

## Enemies

- **2× Ghoul wretches** (same living-wretch reflavor as upstairs — *hold person*
  etc. valid; see [`../world.md`](../world.md)), placed in the **back corners**,
  away from the PC entry:
  - **G1 at (2,8)**
  - **G2 at (9,8)**
- **Surprise toggles are OFF** — adjudicate surprise *live* (the brood lurking in
  the dark could surprise the party; the players' light / perception decides).
- Per-Ghoul mechanics: AC 12, HP 22, Multiattack (bite + 2 claws). The paralysis
  of a *failed-save* hold-person target makes melee hits within 5 ft **auto-crit**.

## Difficulty & scaling

- As built, **2 Ghouls = a real fight for two L3 PCs**, especially with Vale down
  to **1 pact slot** (no long rest yet — she can't hold-person-lock both). See
  [`vale.md`](../party/vale.md).
- **Lighter:** delete G2 in the builder → 1 wretch.
- **Bigger party (4-6 PCs):** scale up. Rough guide — **~1 wretch per 1.5 PCs**
  (4 PCs → 3 wretches, 6 PCs → 4). Add wretches in the back corners / climbing the
  deeper shaft mid-fight (the reserve mechanic below). Watch action economy: many
  PCs vs few foes trivializes; few foes with high HP drags. See
  [`../big-party.md`](../big-party.md) "Encounter scaling."
- **Reserve mechanic:** a wretch can claw up from the deeper shaft mid-fight if the
  fight is too easy (mirrors the upstairs "2nd wretch from the pit" reserve).

## To run

1. Players decide to descend (await them in #in-character — **don't act for them**).
2. Dashboard → Encounters → open "Cellar — the brood" → **Start Combat**.
3. PCs auto-seat at the stairs spawn zone; G1/G2 lurk in the back. The opening
   board **auto-posts to #combat-map on Start Combat** (this fight starts after the
   `7b6c125` deploy that added the auto-post; it also re-posts on the first `/done`,
   a DM enemy turn, or any player's `/map` — see [`../runbook.md`](../runbook.md) §7).
4. Adjudicate surprise live, then run initiative. Narrate each beat to #the-story
   (read-aloud), keep [`game-state.md`](../game-state.md) + the session log in
   lockstep (see [`../dm-rules.md`](../dm-rules.md)).

## Editing this encounter

The encounter-builder Edit/Save/Delete/Duplicate path is working (ISSUE — the
missing `campaign_id` query param — was fixed 2026-06-26). To add/remove wretches:
Encounters → Edit → place/delete tokens → Save.
