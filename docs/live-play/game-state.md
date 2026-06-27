# Game State — live save file

> **The save file: where we are *right now*.** Keep it slim and current — this is
> not a history (that's [`sessions/`](sessions/)) and not the character sheets
> (those are [`party/`](party/)). Update as play advances. Real-world dates are
> absolute; in-fiction time is loose.
>
> **Before acting, pull the live picture** — the DM Console (`GET /api/dm/situation`
> / the `#dm-console` tab) is the *generated* source of truth and this file drifts.
> See [`README.md`](README.md) "DM Console."

_Last updated: 2026-06-27 — **COMBAT LIVE: "The Cellar," ROUND 3, VALE'S TURN.** **R2 + top of
R3 resolved.** **R2:** Vale (15) held; **Forge (12) RAGED** (resistance to B/P/S, 10 rounds
left); 2nd ghoul (init 9, D2) **bit Forge** — 8 raw → **4 after Rage resist** → Forge 20→16.
**R3:** lead ghoul (init 19, E2) **bit Forge** — 8 raw → **4 resisted** → Forge 16→**12/32**.
Both ghoul turns run via the dashboard Turn Builder (Confirm & Post + manual End Turn).
**Two fixes verified LIVE this session:** (1) enemy-turn combat log now names the actor —
posts `**Ghoul's Turn**`, not blank `**'s Turn**` (ISSUE-021 tail); (2) enemy-turn log now
shows **post-resistance** damage (`4 piercing damage (resisted — halved from 8)`) instead of
the raw roll (ISSUE-023). NB: the two R2/R3 logs already posted to #combat-log predate fix #2
so they still read "8 piercing" — actual dealt was 4 each (Rage). **Enemy-turn executor still
attack-only** (no auto-move/advance; DM ends the turn — ISSUE-021). **Next beat = Vale's turn**
(player-driven, init 15, R3), then Forge (12), then 2nd Ghoul (9). 3-4 more players still joining._

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
  `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map *Ashfall Waystation — cellar*. **Round 3**,
  4 combatants, no surprise (re-ruled off both sides).
  - **Positions / HP:** Vale **K2** (**19/24**, bloodied — Misty Stepped clear; **CURRENT**) +
    Forge **E1** (**12/32**, bitten, **RAGING** — rage_rounds ≈10, B/P/S resist) at the stairs
    landing; **lead Ghoul E2** (20/22, *init 19*, adjacent to Forge) + **2nd Ghoul D2** (22/22,
    init 9, adjacent to Forge).
  - **Initiative / turn order:** Ghoul 19 → **Vale 15 (CURRENT)** → Forge 12 → Ghoul 9.
  - **Done — R1:** lead ghoul bit Vale (5); Vale crossbow (hit, 2) + Misty Step E1→K2; Forge
    greataxe MISS; 2nd ghoul closed C8→D2 + bit Forge (12). **R2:** lead ghoul bit Forge →
    **MISS**; Vale (15) **held**; Forge (12) **RAGED**; 2nd ghoul (9) **bit Forge** 8 raw → **4
    resisted** (20→16). **R3:** lead ghoul (19) **bit Forge** 8 raw → **4 resisted** (16→**12**).
  - **Enemy-turn executor (ISSUE-018 fix): WORKS.** Combat-log now (a) names the actor
    (`**Ghoul's Turn**`) and (b) shows post-resistance damage (`4 piercing (resisted — halved
    from 8)`) — both fixes verified live this session. Residual (→ ISSUE-021): resolves the
    **attack only** — no NPC move into reach, no turn advance; DM clicks **End Turn** each
    enemy turn (both ghouls already adjacent to Forge, so no move needed).
  - **Stale queue:** two `enemy_turn_ready` items (init-9 + lead ghoul) linger in the DM
    Console pending (executor doesn't auto-resolve them — ISSUE-021); resolve/ignore.
  - **Sheets show live combat HP** (ISSUE-020): portal sheet, `/character`, dashboard Party
    Overview overlay the combatant HP (Vale 19/24, Forge 20/32).
  - **Vale pact slots: 1/2** (spent 1 on Misty Step). Pact-slot write-back gap (combat spend
    not written to the `characters` row, à la ISSUE-020 HP) **fixed by another agent** this
    session (→ ISSUE-022).
  - Prior fight — "Waystation — the cellar wretch" (id
    `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) — ended 2026-06-26 in victory. Full
    chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene

**Down in the cellar, the brood swarms the raging dwarf.** Forge has gone berserk — both
ghouls sank their teeth into him across R2/R3, but his **Rage** halves every bite (8→4), so
he's **bloodied but standing at 12/32**, axe up, pinned at the foot of the stairs by both pale
things (D2/E2). Vale is clear and untouched at **K2** across the cellar, both ghouls' backs
turned to her, one spent bolt on the stones. It's **Vale's turn** (Round 3). World / lore:
[`world.md`](world.md).

## Next action

1. **Vale's turn (CURRENT, init 15, R3).** The player acts — she types her own
   `/move`/`/attack`/`/cast` in Discord and **rolls her own dice**; never roll for her.
   Adjudicate vs her reported numbers; keep enemy HP/AC secret (describe state, don't quote
   it). She's bloodied (19/24), at **K2** — clear of both ghouls (they're on Forge at E1),
   with 1 pact slot + her light crossbow; both ghouls' backs are to her. After her beat,
   **narrate** + **update docs**.
2. **Then Forge (12),** player-driven — **raging**, bloodied (12/32), pinned by two ghouls
   (D2/E2). Rage halves incoming B/P/S; his next hit also gets Rage damage + (if Reckless)
   advantage.
3. **Then the 2nd Ghoul (init 9, D2) — DM enemy turn.** Run via **"⚔ Run Enemy Turn"** →
   Review → Confirm & Post, then **manually End Turn** (executor is attack-only — ISSUE-021).
   Already adjacent to Forge (no move). Log now names the actor + shows resisted damage.
4. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row
   + sheet → fold in). See [`runbook.md`](runbook.md) + [`big-party.md`](big-party.md).
5. **Bookkeeping:** Vale's leather armor **equipped (AC 11)**; pact slots **1/2**. Forge's
   equipped weapon is a **greataxe**; **Rage active** (rage_rounds ≈10, was "unused"). Two
   stale `enemy_turn_ready` queue items linger (ISSUE-021) — resolve/ignore.
