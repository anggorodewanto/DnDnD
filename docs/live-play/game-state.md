# Game State — live save file

> **The save file: where we are *right now*.** Keep it slim and current — this is
> not a history (that's [`sessions/`](sessions/)) and not the character sheets
> (those are [`party/`](party/)). Update as play advances. Real-world dates are
> absolute; in-fiction time is loose.
>
> **Before acting, pull the live picture** — the DM Console (`GET /api/dm/situation`
> / the `#dm-console` tab) is the *generated* source of truth and this file drifts.
> See [`README.md`](README.md) "DM Console."

_Last updated: 2026-06-27 — post-combat lull. Vale + Forge finished their
introductions in #in-character; Forge asked "is there something interesting in
the cellar?" **DM nudge posted** to #the-story (3:14:31 PM UTC-ish, `narration_posts`)
— answered the question in-fiction, leaned on Vale's patron pull, teed up the
descent decision **without deciding it for them**. **Forge Anvilbearer re-approval
resolved** (backstory + Goblin language added; queue now empty). **Next beat =
the party's descent call**, then the cellar fight. 3-4 more players still joining._

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

- **None active.** Last fight — "Waystation — the cellar wretch" (combat id
  `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) — ended 2026-06-26 in **victory**
  (encounter `completed`; Vale's hold-person concentration auto-cleared; both PCs
  full HP). Full R1-R3 chronology: [`sessions/session-01.md`](sessions/session-01.md).
- **Next fight is PRE-BUILT** and one click to run on descent:
  [`encounters/cellar-brood.md`](encounters/cellar-brood.md) (2× Ghoul wretches;
  scale up for the bigger party).

## Current scene

Post-combat lull in the waystation common room. The upstairs wretch is dead — up
close it was a *person* once (the keeper, maybe), hollowed out. The **cellar mouth
still gapes** in the SW corner (the 2×2 pit), its door clawed to splinters **from
the inside**. The dread points downward. World / lore: [`world.md`](world.md).

## Next action

1. **Await the party's descent call in #in-character.** The cellar nudge is posted;
   Vale's patron is pulling her down, Forge asked what's below. Let *them* decide to
   descend / search / rest — **don't act for them** ([`dm-rules.md`](dm-rules.md)).
   Read #in-character via Chrome to catch their reply (it's Discord-only).
2. **Onboard the new players** as they arrive (`/register` → build → DM-approve →
   roster row + sheet → fold into the fiction). See [`runbook.md`](runbook.md) +
   [`big-party.md`](big-party.md).
3. **On descent → run the cellar fight.** Open "Cellar — the brood" → **Start
   Combat** (PCs auto-seat at the stairs spawn zone). **Scale the wretch count for
   the bigger party** before starting — see
   [`encounters/cellar-brood.md`](encounters/cellar-brood.md).
4. **Optional aftermath/loot:** the keeper's body / common room may hold a clue to
   what's below (key, journal, claw-scored boards). DM's call whether to seed any.
5. **Bookkeeping:** Vale's leather armor still unequipped (AC 10;
   `/equip item:leather armor:true` → AC 11) if she wants it.
