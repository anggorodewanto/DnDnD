# Game State — save file (durable IDs + DM intent)

> **This file holds only what the DB can't derive:** durable IDs, the ops snapshot,
> the current *scene* framing, and the **Next action** (DM intent). It deliberately
> does **NOT** track round / turn / HP / AC / positions / conditions / slots / the
> queue — those are *generated* and served live, aggregated, by the **DM Console**
> (`GET /api/dm/situation` / the `#dm-console` tab). Hand-copying mechanical state
> here is what drifts (it bit this folder repeatedly — see [`sessions/`](sessions/)),
> so we stopped. **Pull live state from the Console; record only intent + IDs + scene
> here.** Per-PC durable kit is in [`party/`](party/); play-by-play in
> [`sessions/`](sessions/). See [`dm-rules.md`](dm-rules.md) "Keep the record straight."

_Last updated: 2026-06-28 — mid-combat in **"The Cellar"** (Round 8, Vale's turn — Forge is **down** (0 HP, dying; the engine now prompts his death saves after ISSUE-036), the last ghoul is one hit from dying and now adjacent to a bloodied Vale).
For the live board (whose turn, HP, positions, conditions, pending queue, recent actions) open the
**DM Console** — do not read a hand-copied snapshot from this file. Non-derivable intent is under "Next action."_

## Live mechanical state → DM Console (do not hand-copy here)

Round, turn order, every combatant's HP/AC/position/conditions, the pending queue, and the
recent action timeline are **generated** — read them live, never transcribe them into this file:

- **DM Console:** `GET /api/dm/situation` or the `#dm-console` dashboard tab (`next_step`,
  `pending[]`, `state`, `timeline[]`).
- Source tables (read-only sanity checks): `encounters`, `combatants`, `turns`, `action_log`,
  `dm_queue_items`.

## Ops snapshot

- **Stack:** UP via `make local-up` (docker compose). App `localhost:8080`, DB
  `localhost:5432`. Bot `DnDnD` (id `1507904367301496862`) connected to guild
  `DnDnD`.
- **Last deploy:** `main` (see `git log`); combat state has survived every redeploy.
  Rebuild + redeploy after any code fix: `docker compose up -d --build app`
  (see [`runbook.md`](runbook.md) §1). Redeploy *history* lives in the session logs.
- **Remote-player tunnel:** an **ngrok tunnel on a reserved domain** exposes the
  local app so remote players reach the builder + OAuth. **URL is STABLE**
  (`https://unhustling-cushionless-karan.ngrok-free.dev`), so the OAuth callback
  is registered in Discord **once** and never changes. How-to + `make tunnel-*`
  targets: [`runbook.md`](runbook.md) "Remote players"; one-time setup +
  `NGROK_DOMAIN`/`NGROK_AUTHTOKEN` in `.env`: header of
  [`scripts/tunnel.sh`](../../scripts/tunnel.sh).
  - **OAuth callback (registered, stable):**
    `https://unhustling-cushionless-karan.ngrok-free.dev/portal/auth/callback`.
    No per-restart Discord change. `make tunnel-up` always yields this URL;
    `make tunnel-down` restores `.env` to `localhost` while keeping the ngrok vars.
  - Migrated off the old cloudflared quick tunnel (2026-06-27) — that URL was
    ephemeral and forced a Discord re-register on every restart.

## Campaign

| Field | Value |
| --- | --- |
| Campaign ID | `532b4774-47ff-4f83-b591-632ce3509e40` |
| Name | "Campaign for guild 1507910398886543532" (unrenamed) |
| Guild ID | `1507910398886543532` |
| DM user ID | `1089351036650668143` (the user — already DM) |
| Status | `active` |
| Rules | Diagonal: standard · Sources: `wotc-srd` · Turn timeout: 24h |

### Discord channel IDs (from `campaigns.settings.channel_ids`)

| Channel | ID | Channel | ID |
| --- | --- | --- | --- |
| #the-story | `1507958843769098280` | #combat-map | `1507958850505019462` |
| #in-character | `1507958845547217017` | #initiative-tracker | `1507958836898693310` |
| #player-chat | `1507958847137120267` | #roll-history | `1507958840241684611` |
| #your-turn | `1507958852086399037` | #character-cards | `1507958855185862801` |
| #combat-log | `1507958838442070057` | #dm-queue | `1507958856930557994` |

## Maps

| Map | ID | Notes |
| --- | --- | --- |
| Ashfall Waystation — common room | `1ad14481-f938-462d-be75-25764463ff5b` | 12×10 blank grid; 2×2 **Pit** (SW) = cellar mouth. Features narrated. |
| Ashfall Waystation — cellar | `d2fe03c6-9749-4a24-a6e3-cb9d3a77e3cd` | 12×10 blank stone; PC spawn zone at the top-center stairs landing. For the descent. |

## Active encounter (durable refs — live state via the Console)

- **LIVE — "The Cellar"** (internal "Cellar — the brood"), encounter id
  `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map *Ashfall Waystation — cellar*. 4 combatants
  (Vale + Forge vs two ghouls), no surprise. **Round/turn/HP/positions/conditions: DM Console.**
- Prior fight — "Waystation — the cellar wretch" (id `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) —
  ended 2026-06-26 in victory. Full chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene (narrative framing — non-derivable)

**Down in the cellar one of the brood is dead, the dwarf is down and dying, and the last ghoul has
turned on the only one left standing.** Forge — berserk, bleeding out on Rage alone — cleaved the lead
ghoul apart back in R4 (the one Vale had twice lashed with grave-cold Chill Touch). He and Vale wrecked
the **smaller** flank ghoul across R5–R6 but never quite dropped it; in R6 it sank its jaws into Forge's
throat and **he fell** — 0 HP, unconscious and prone, Rage gone, now in death saves. In R7 the ghoul
ignored the helpless dwarf and **lunged at Vale**, biting her hard (she's bloodied but up). The ghoul
itself is a single solid hit from death, swaying right in Vale's face. The fight rolls into **Round 8,
Vale's turn** — the ghoul is adjacent and one blow from dying; Forge lies dying a few feet away. The
cellar is cramped, stone, lit by whatever the party brought down. World /
lore: [`world.md`](world.md). (Live HP/positions: DM Console.)

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** first for `next_step` + the live board, then apply this intent.

1. **Mid-combat, "The Cellar," Round 8 — Vale's turn (CURRENT, player-driven).** One ghoul left
   (**G1**, the smaller flank one, **1/22, now F2 — adjacent to Vale** — a single hit ends it). R7
   played out: Vale's Chill Touch missed; **G1's bite had dropped Forge to 0** (unconscious + prone,
   Rage gone, dying, E1); then the engine **silently skipped Forge's death save** (ISSUE-036 — now
   FIXED + deployed); per the player's call the ghoul **lunged at Vale** (DM moved G1 D2→F2, then Bite
   vs Vale: to-hit 21 vs AC 11, **12 piercing** → Vale **7/24, G2**). R8 order: **Vale (CURRENT, G2)** →
   **Forge (12 — DOWN, dying; the fixed engine now ACTIVATES his turn and posts a `/deathsave` prompt —
   his remote player rolls, NOT the DM)** → **G1** (NPC, last). Players drive their own turns
   (`/move`/`/attack`/`/cast`, **they roll their own dice** — never roll for them). When **G1's R8 turn**
   comes up, run it via **"⚔ Run Enemy Turn — Ghoul"** → Review → Confirm & Post → **manual End Turn**
   (executor attack-only, no auto-move — ISSUE-021; **G1 is already adjacent to Vale**, no move needed;
   End Turn 409-guards until the enemy turn is executed — ISSUE-030). Keep enemy HP/AC secret — describe
   state, don't quote numbers ([`dm-rules.md`](dm-rules.md)).
   - **Forge's owed R7 death save:** the ISSUE-036 bug skipped it. His remote player should roll
     `/deathsave` **once now (off-turn)** to make it up — it records only and won't advance the turn. From
     R8 on, the fixed engine prompts his save automatically each time his turn activates.
   - **Stakes:** G1 is one solid hit from dying and **adjacent to Vale (7/24)** — she can drop it this
     turn (Chill Touch / crossbow / dagger), but if it survives to act **last** in R8 it bites Vale
     again, or could turn back on the dying Forge. Flag the danger; let Vale decide and roll. Run every
     death save **straight** — don't fudge ([`dm-rules.md`](dm-rules.md)).
2. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row + sheet →
   fold into the fiction). 3-4 more PCs expected. See [`runbook.md`](runbook.md) "Onboarding
   players" + [`big-party.md`](big-party.md).
3. **After every beat:** narrate to #the-story (read-aloud) + append the narrative
   [`sessions/`](sessions/) log. Do **not** transcribe HP/positions back into this file.
