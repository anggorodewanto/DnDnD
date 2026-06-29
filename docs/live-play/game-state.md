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

_Last updated: 2026-06-29 — **"The Cellar" WON and CLOSED; party out of combat and rested up.** The party retreated upstairs and took a **long rest** in the cleared common room: **both PCs back to full — Vale 24/24 (Pact L2 2/2), Forge 32/32, no conditions** (Forge woke + stood; all applied via Party → Edit status / Edit slots). The three finds are now **on Vale's sheet** (Manage inventory): an unused **Potion of Healing**, a **Cold Iron Key**, and a now-readable **Water-Rotted Journal**. A 4-paragraph read-aloud closed the rest in #the-story (1:13 PM) and seeded two hooks — the **journal has dried enough to read** and **something below the barred cellar door is "not entirely silent."** **No active encounter.** ISSUE-038 fixed: End Combat now AUTO-carries PC HP/conditions to the sheets (the manual carry-out footgun is gone).
Out of combat there is no live board to pull — the durable post-combat state is in "Current scene"; non-derivable intent is under "Next action."_

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

- **CLOSED — "The Cellar"** (internal "Cellar — the brood"), encounter id
  `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map *Ashfall Waystation — cellar*. 4 combatants
  (Vale + Forge vs two ghouls). **Ended in victory 2026-06-28 at R11** — both ghouls dead, Forge
  stabilized. Combat ended; **no active encounter**. Post-combat PC state was carried out by hand
  (no combat-end HP write-back) — see "Current scene". Chronology: [`sessions/session-01.md`](sessions/session-01.md).
- Prior fight — "Waystation — the cellar wretch" (id `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) —
  ended 2026-06-26 in victory. Full chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene (narrative framing — non-derivable)

**The cellar fight is over and won — the party is out of combat, catching its breath over a downed-but-living Forge.**
Forge cleaved the lead ghoul apart in R4; he and Vale ground down the smaller flank ghoul across R5–R8,
but in R6 it tore out Forge's throat and **he fell** (0 HP, unconscious + prone, into death saves). In
**R8 Vale finished the last ghoul** with a thrown dagger. Vale then rushed to Forge and tried first aid
(**Medicine 6 vs DC 10 — failed**), but Forge held on by his own toughness — his death saves climbed to
**✓3 and he stabilized** (still unconscious at 0 HP, not dying). With no hostiles left, **combat was
ended** (R11). Out of combat now: **Forge 0/32, unconscious + prone (stabilized, needs tending); Vale
7/24**.

**06-29 — Vale searches the nest, then the party rests.** With Forge down and no healing on hand, Vale searched
the brood's nest (Investigation **10**) and found, in a split traveler's pack: **one healing draught (a common
Potion of Healing), whole**; a **cold iron key** on a leather thong; and a **water-rotted journal**. Rather than
drink the draught or press on, the party **retreated upstairs** — Vale hauled the unconscious Forge up the
splintered stairs, **barred the cellar door**, relit the dead hearth in the common room, and the party took a
**long rest**. Both came back whole: **Vale 24/24 (Pact L2 2/2), Forge 32/32, no conditions** — Forge woke and
stands. The three finds are now **on Vale's character sheet** (Manage inventory): the **Potion of Healing**
(2d4+2, unused), the **Cold Iron Key**, and the **Water-Rotted Journal** — the journal has now **dried enough
to read** (a clue, if a patient eye works at it). The brood lie dead behind the barred door, but the deeper
cellar past it is **unexplored** — and in the quiet, the dark down there was **"not entirely silent."**
World / lore: [`world.md`](world.md).

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** first for `next_step` + the live board, then apply this intent.

1. **Out of combat, rested up — the choice is with the players (player-driven lull).** No active encounter,
   no initiative, no hostiles. The party **long-rested in the cleared common room**; **both PCs are at full —
   Vale 24/24 (Pact L2 2/2), Forge 32/32, no conditions** (Forge awake + standing; all applied via the
   dashboard). The draught is **unused**, the iron key pocketed, and the **journal is now readable**. A
   read-aloud closed the rest in #the-story (1:13 PM). **All three finds are now on Vale's sheet** (Manage
   inventory: Potion of Healing, Cold Iron Key, Water-Rotted Journal). **Awaiting the players' next move** —
   let them decide; don't narrate the choice. The seeded options:
   - **Read the journal** (a clue beat — world.md keeper/brood threads). If a player has Vale (or whoever)
     work at it, adjudicate as a read/Investigation/Intelligence beat and narrate what the dried pages give
     up; this is the cleanest hook toward *why* the cellar went wrong and what's deeper.
   - **The iron key** — now on the sheet; it opens *something* (a locked door/chest below, DM's call per
     world.md). It surfaces when they actually try it on a lock.
   - **Descend into the deeper cellar** — past the barred door the dark was **"not entirely silent."** The
     brood-descent fight is **pre-built**: [`encounters/cellar-brood.md`](encounters/cellar-brood.md). If they
     press on, prep + run it through the combat tools; keep enemy HP/AC secret ([`dm-rules.md`](dm-rules.md));
     players roll their own dice. (Living-wretch ruling still applies — *hold person* etc. are valid; see
     [`world.md`](world.md).)
   - **The healing draught (on the sheet, unused):** if a PC drinks it later it restores **2d4+2** — **the
     players roll it** ([`dm-rules.md`](dm-rules.md)). Apply via **Party → Edit status** (add to current HP,
     capped at max) and decrement the potion in Manage inventory. At full HP now it's a saved resource for the
     next fight.
   - **Drop-to-0 logging gap (follow-up, non-blocking):** the feature shipped 06-28 (`dfefd8e`) did **not**
     log G1's defeat — the player `/attack` damage path doesn't funnel through `Service.ApplyDamage` where
     `notifyDroppedToZero` is gated. Forward-fix candidate; logged in session log + issues, table not blocked.
2. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row + sheet →
   fold into the fiction). 3-4 more PCs expected. See [`runbook.md`](runbook.md) "Onboarding
   players" + [`big-party.md`](big-party.md).
3. **After every beat:** narrate to #the-story (read-aloud) + append the narrative
   [`sessions/`](sessions/) log. Do **not** transcribe HP/positions back into this file.
