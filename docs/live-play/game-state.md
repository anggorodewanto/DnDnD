# Game State — live save file

> **The save file: where we are *right now*.** Keep it slim and current — this is
> not a history (that's [`sessions/`](sessions/)) and not the character sheets
> (those are [`party/`](party/)). Update as play advances. Real-world dates are
> absolute; in-fiction time is loose.
>
> **Before acting, pull the live picture** — the DM Console (`GET /api/dm/situation`
> / the `#dm-console` tab) is the *generated* source of truth and this file drifts.
> See [`README.md`](README.md) "DM Console."

_Last updated: 2026-06-27 — **COMBAT LIVE: "The Cellar," Round 1, VALE'S TURN.** Lead
ghoul (init 19) closed J8→**E2** and **bit Vale for 5** (engine To Hit 15 vs AC 10) →
Vale **19/24**, bloodied, no paralysis (Bite, not Claws). Bite narrated to #the-story
(`narration_posts` 5:30:18 PM). Turn advanced to **Vale (init 15)** — her move now.
**All three live-play fixes are now DEPLOYED:** ISSUE-018 (`before_state` enemy-turn
crash), ISSUE-019 (Turn-Builder "⚔ Run Enemy Turn" button), and ISSUE-020 (character
sheets showed stale full HP mid-combat — sheets now overlay the live combatant HP on
the portal sheet, `/character`, and the dashboard Party Overview). The **Turn Builder is
now safe to use** for enemy turns. **Next beat = Vale's turn**, then Forge (12), then
2nd Ghoul (9, at C8) — run it via the new button (first live test of the fixed executor).
3-4 more players still joining._

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
  `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map *Ashfall Waystation — cellar*. **Round 1**,
  4 combatants, no surprise (re-ruled off both sides).
  - **Positions / HP:** Vale **E1** (**19/24**, bloodied) + Forge **E1** (32/32) — at the
    stairs landing; **lead Ghoul** now **E2** (22/22, *init 19*, adjacent to the party,
    just bit Vale) + Ghoul **C8** (22/22, init 9, still at the back).
  - **Initiative / turn order:** Ghoul 19 → **Vale 15 (CURRENT)** → Forge 12 → Ghoul 9.
  - **Done:** lead ghoul moved J8→E2 + Bite vs Vale (To Hit 15 vs AC 10 → hit, 5 piercing).
  - **Turn Builder fixed + safe** (ISSUE-018 deployed). The next enemy turn (2nd ghoul) is
    the first live run of the fixed executor — reach it via the new **"⚔ Run Enemy Turn"**
    button (ISSUE-019) or right-click → Plan Turn.
  - **Sheets now show live combat HP** (ISSUE-020): the portal sheet, `/character`, and the
    dashboard Party Overview overlay the combatant's HP during a fight (Vale reads 19/24,
    not the stale 24/24 base-sheet value).
  - Prior fight — "Waystation — the cellar wretch" (id
    `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) — ended 2026-06-26 in victory. Full
    chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene

**Down in the cellar, first blood.** Vale trance-walked down, Forge a step behind; two
ghouls peeled from the dark and battle joined — **no surprise** (the brood heard them).
The lead ghoul rushed Vale and **bit her** (bloodied, 19/24); a second shape still
unfolds from the black. It's **Vale's turn** — the cellar holds its breath. World /
lore: [`world.md`](world.md).

## Next action

1. **Vale's turn (CURRENT, init 15).** The player acts — she types her own
   `/move`/`/attack`/`/cast` in Discord and **rolls her own dice**; never roll for her.
   Adjudicate vs her reported numbers; keep enemy HP/AC secret (describe state, don't
   quote it). She's bloodied (19/24) and a ghoul is adjacent at E2. After her beat,
   **narrate** + **update docs** in lockstep.
2. **Then Forge (12),** same player-driven flow.
3. **Then the 2nd Ghoul (init 9, C8) — DM enemy turn.** Run it via the new **"⚔ Run Enemy
   Turn"** button (combat right panel, shown when an NPC is current) or right-click → **Plan
   Turn**. This is the **first live run of the fixed enemy-turn executor** (ISSUE-018) — watch
   that damage applies, the action logs, and the turn advances cleanly.
4. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row
   + sheet → fold in). See [`runbook.md`](runbook.md) + [`big-party.md`](big-party.md).
5. **Bookkeeping:** Vale's leather armor is now **equipped (AC 11)** — confirmed on the
   Party Overview. (The bite landed while she was still exposed at AC 10.)
