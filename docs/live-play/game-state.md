# Game State — live save file

> **The save file: where we are *right now*.** Keep it slim and current — this is
> not a history (that's [`sessions/`](sessions/)) and not the character sheets
> (those are [`party/`](party/)). Update as play advances. Real-world dates are
> absolute; in-fiction time is loose.
>
> **Before acting, pull the live picture** — the DM Console (`GET /api/dm/situation`
> / the `#dm-console` tab) is the *generated* source of truth and this file drifts.
> See [`README.md`](README.md) "DM Console."

_Last updated: 2026-06-27 — **COMBAT LIVE: "The Cellar," ROUND 2, VALE'S TURN.** Round 1
fully resolved + Round 2 opened. **R1:** lead ghoul bit Vale (5 → 19/24, bloodied); Vale
point-blank crossbow on the lead ghoul (hit, 2 → 20/22) then **Misty Step** (bonus, 1 pact
slot → **1/2 left**) E1→**K2**; Forge greataxe **missed** the lead ghoul; 2nd ghoul (init 9)
closed C8→**D2** and **bit Forge** (18 vs AC 14 → **12** → Forge **20/32**). **R2 so far:**
lead ghoul (init 19) **bit at Forge and MISSED** (4 vs AC 14). Turn now **Vale (init 15)**.
Every beat narrated to #the-story (read-aloud). **Enemy-turn executor (first live runs,
ISSUE-018 fix): WORKS — no crash, damage + action_log written** — but it resolves the
**attack only** (no auto-move into reach, no auto-advance); DM drags the token + clicks End
Turn each enemy turn (→ ISSUE-021). **Next beat = Vale's turn** (player-driven), then Forge
(12), then 2nd Ghoul (9). 3-4 more players still joining._

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

## Party

See [`party/roster.md`](party/roster.md) for the at-a-glance table (HP / AC /
resources / position / status) and per-PC sheets. Current: **Vale** (Tiefling
Warlock 3) + **Forge** (Hill-Dwarf Barbarian 3), both full HP, out of combat.
**3-4 more PCs joining** — onboard per [`runbook.md`](runbook.md) "Onboarding
players," then add roster rows.

## Active encounter / combat

- **LIVE — "The Cellar"** (internal "Cellar — the brood"), combat/encounter id
  `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map *Ashfall Waystation — cellar*. **Round 2**,
  4 combatants, no surprise (re-ruled off both sides).
  - **Positions / HP:** Vale **K2** (**19/24**, bloodied — Misty Stepped clear; CURRENT) +
    Forge **E1** (**20/32**, bitten) at the stairs landing; **lead Ghoul E2** (20/22, *init
    19*, adjacent to Forge) + **2nd Ghoul D2** (22/22, init 9, adjacent to Forge).
  - **Initiative / turn order:** Ghoul 19 → **Vale 15 (CURRENT)** → Forge 12 → Ghoul 9.
  - **Done — R1:** lead ghoul bit Vale (5); Vale crossbow (hit, 2) + Misty Step E1→K2; Forge
    greataxe MISS; 2nd ghoul closed C8→D2 + bit Forge (12). **R2:** lead ghoul bit at Forge →
    **MISS** (4 vs AC 14).
  - **Enemy-turn executor (first live runs, ISSUE-018 fix): WORKS — no crash, damage +
    action_log written.** Residual (→ ISSUE-021): resolves the **attack only** — no NPC move
    into reach, no turn advance. DM drags the token (did C8→D2 for the 2nd ghoul) + clicks
    **End Turn** each enemy turn.
  - **Sheets show live combat HP** (ISSUE-020): portal sheet, `/character`, dashboard Party
    Overview overlay the combatant HP (Vale 19/24, Forge 20/32).
  - **Vale pact slots: 1/2** (spent 1 on Misty Step). Pact-slot write-back gap (combat spend
    not written to the `characters` row, à la ISSUE-020 HP) **fixed by another agent** this
    session (→ ISSUE-022).
  - Prior fight — "Waystation — the cellar wretch" (id
    `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) — ended 2026-06-26 in victory. Full
    chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene

**Down in the cellar, the brood closes on Forge.** First blood went both ways: the lead
ghoul bit Vale (bloodied, 19/24); she shot it point-blank and **Misty Stepped** across the
room to K2, Forge's greataxe whiffed, the **second ghoul lunged from the dark and bit Forge**
(20/32), and the lead ghoul's follow-up bite **missed**. Both pale things now crowd Forge at
the foot of the stairs while Vale stands clear and untouched across the cellar. It's
**Vale's turn** (Round 2). World / lore: [`world.md`](world.md).

## Next action

1. **Vale's turn (CURRENT, init 15, R2).** The player acts — she types her own
   `/move`/`/attack`/`/cast` in Discord and **rolls her own dice**; never roll for her.
   Adjudicate vs her reported numbers; keep enemy HP/AC secret (describe state, don't quote
   it). She's bloodied (19/24), at **K2** — clear of both ghouls (they're on Forge at E1),
   with 1 pact slot + her light crossbow. After her beat, **narrate** + **update docs**.
2. **Then Forge (12),** player-driven — bitten (20/32), pinned by two ghouls (D2/E2).
3. **Then the 2nd Ghoul (init 9, D2) — DM enemy turn.** Run via **"⚔ Run Enemy Turn"** →
   Review → Confirm & Post, then **manually** drag-into-reach (if needed) + **End Turn**
   (executor is attack-only — ISSUE-021). It's already adjacent to Forge, so likely no move.
4. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row
   + sheet → fold in). See [`runbook.md`](runbook.md) + [`big-party.md`](big-party.md).
5. **Bookkeeping:** Vale's leather armor **equipped (AC 11)**; pact slots **1/2**. Forge's
   equipped weapon is a **greataxe** (roster previously said "dual handaxes" — corrected).
