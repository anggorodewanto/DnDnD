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

_Last updated: 2026-06-30 — **COMBAT LIVE — the Cold Vault boss fight is ON (Round 1).** The players turned the key (Forge *"follows to descend below"* 7:17 PM; Vale *"inserts the key… nodded to Forge and turns the key"* 7:54 PM), so the beat ran end-to-end: posted **block B** (the vault read-aloud — the door opening on grave-cold air, the chiselled-out shrine, the keeper rising in its frost-grey clothes; 7:55 PM, Post History confirmed) → opened **"Cold Vault — the keeper"** → **Start Combat** (~7:56 PM). **Live encounter `446dce33-e221-4d1f-a88b-4e81534b3399`.** Surprise adjudicated live = **none** (keeper dormant, PCs deliberately opened — standard initiative). **Round 1 order (from the Console): Forge (14) → Wight keeper (14, tie→Forge first) → Vale (4).** PCs entered bottom-center (the cold door); keeper top-center — ~40 ft gap to close. **R1 so far (06-30):** Forge advanced to F4 and **raged** — but ended his turn without attacking/taking damage, so by RAW the **rage lapsed at end of turn** (`is_raging=f`). The silent drop (no #combat-log / no DM-timeline notice) was **fixed live — ISSUE-041 FIXED**: rage expiry now posts to #combat-log + writes a `rage_expired` action_log row. The **keeper's turn was run from the workspace Turn Builder**: Longsword **hit Forge — 7 slashing** (Forge 25/32), posted to #combat-log + narrated to #the-story (read-aloud). **It is now Vale's turn (PC) — awaiting her slash command; do NOT act/roll for her.** **DM ruling applied (06-30):** Forge's player asked to undo his wasted R1 rage (his F4 move left him 15ft short, rage lapsed for nothing); **granted — rage charge refunded 2→3** via the in-combat Manual Override → Feature Uses (audited `dm_override` + auto #combat-log correction, player-👍'd). He keeps F4 + the hit stands; **DM Queue now empty.** Keep the keeper's HP/AC SECRET. Live board → DM Console. **Escalation in play: the keeper is UNDEAD → Vale's *hold person* will FAIL** (telegraph it the first time she tries). Live round/turn/HP/positions → **DM Console** (`#dm-console`), not this file. ISSUE-038 fixed: End Combat now AUTO-carries PC HP/conditions to the sheets (the manual carry-out footgun is gone). **Latest beat: Vale's Shatter hit the keeper — it failed its CON save (nat 1+3=4 vs DC 13), took 3d8=16 thunder, narrated to #the-story; still Vale's turn.** That cast surfaced + fixed ISSUE-042 (pact-slot cast-log), ISSUE-043 (DM monster-save resolver — new Combat/DM-Console "Resolve save" UI; see [`runbook.md`](runbook.md) §4), and the CRITICAL ISSUE-044 (AoE save-for-half damage never applied in prod — now fixed + idempotent).
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
| Ashfall Waystation — the cold vault | `2899165e-3d1b-46e9-962f-9065e4e3529a` | 12×10 blank stone (built in-app 06-29); PC spawn zone bottom-center = the cold door. Features narrated. For the cold-door boss beat. |

## Active encounter (durable refs — live state via the Console)

- **LIVE — "The Cold Vault"** (internal **"Cold Vault — the keeper"**), **live encounter id
  `446dce33-e221-4d1f-a88b-4e81534b3399`** (template `adc064e7-…`), map *Ashfall Waystation — the cold vault*
  (`2899165e-…`). **Started 2026-06-29 ~7:56 PM** after the players turned the key. **1× Wight** (CR 3)
  reflavored as the frost-rimed **vault-keeper**; surprise off (adjudicated live — none, the keeper was
  dormant and the PCs deliberately opened the door, standard initiative). Party 2/2 (Vale + Forge). **Round /
  turn / HP / positions / conditions → DM Console, NOT here** (don't hand-copy). Design + the
  *hold-person-fails* escalation (the keeper is genuinely UNDEAD): [`encounters/cold-vault.md`](encounters/cold-vault.md).
  Reserve husks (Zombies) added live only if it's too easy. Keep the keeper's HP/AC secret; players roll their
  own dice; run the Wight's turns from the combat workspace when initiative reaches it.
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
(2d4+2, unused), the **Cold Iron Key**, and the **Water-Rotted Journal**. Vale then **read the journal**
(06-29, 1:38 PM): the keeper's account, mostly pulped, the innermost quarter legible. It reveals an **old vault
behind a "cold door" lower than the cellar** that the keeper unlocked; the **wretches came up fleeing whatever
is behind it** (the Harrow couple, buried in autumn, "wearing their own faces") — so the cellar door was clawed
from inside **to escape the cold door, not to reach the keeper**. The **cold iron key locks that cold door**,
and the keeper's last torn line begs *"do not turn it. Whatever else you do, do not turn it."* The brood lie
dead behind the barred door, but the deeper cellar — and the cold door past it — is **unexplored**.
In the morning RP (#in-character, Discord-only) **Vale revealed her patron** — a story-hungry being she found
chasing a forgotten deity, who trades her power for collected tales and is now **steering her toward the cold
door** ("i have no choice"). She told Forge she won't impose; a DM read-aloud (4:18 PM) **turned the spotlight
to Forge** — go down with her, or not? **Awaiting Forge's (Jonathan's) answer.** World / lore: [`world.md`](world.md).

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** first for `next_step` + the live board, then apply this intent.

1. **COMBAT LIVE — the Cold Vault boss fight, Round 1.** Live encounter `446dce33-e221-4d1f-a88b-4e81534b3399`.
   The players turned the key, so the beat ran end-to-end (block B posted 7:55 PM → Start Combat ~7:56 PM).
   **Pull the live board from the DM Console** — round/turn/HP/positions are generated there, not here. R1 order:
   **Forge (14) → Wight keeper (14) → Vale (4)**. **R1 progress (06-30):** Forge moved up + raged (it lapsed —
   no attack); the **keeper's turn was run** (Turn Builder → Longsword **hit Forge, 7 slashing**, 25/32) and
   narrated; then Forge's player undo request was **granted — his wasted rage refunded 2→3** (in-combat Manual
   Override → Feature Uses; audited `dm_override` + #combat-log correction), **DM Queue cleared to empty**.
   **Vale then cast Shatter (L2) at the keeper** — it **failed its CON save in the open (nat 1+3=4 vs DC 13)** and
   took **3d8 = 16 thunder** (HP secret; live total → Console); the blast was **narrated to #the-story** (read-aloud).
   Vale's **action is spent** but her bonus action + move + reaction remain — **still Vale's turn**. *(That cast
   surfaced + fixed three bugs — pact-slot cast-log display **ISSUE-042**, the missing DM monster-save resolver
   **ISSUE-043**, and a CRITICAL pre-existing bug where AoE save-for-half damage never applied **ISSUE-044**; all
   FIXED + redeployed. Monster AoE saves now resolve via the **Combat workspace / DM Console resolver** — see
   [`runbook.md`](runbook.md) §4 "Resolving a monster's saving throw".)*
   **Still Vale's turn (PC) — await her next action; never roll/act for her.** Run the loop: observe each
   player's slash command → adjudicate/narrate to #the-story → when initiative reaches the **Wight**, run its
   turn from the **combat workspace Turn Builder** (`Run Enemy Turn — Wight` → Review → Confirm & Post auto-rolls
   + logs to #combat-log; players roll their own saves). **Keep the keeper's HP/AC SECRET** — describe wounds, never quote numbers
   ([`dm-rules.md`](dm-rules.md)). **Escalation: the keeper is UNDEAD → Vale's *hold person* FAILS** — telegraph
   it the first time she casts (the spell finds nothing living to grip). Her tools that DO bite: *shatter*,
   *hellish rebuke*, *chill touch* (DC 13). Forge's **Rage** resists the keeper's slashing but **NOT** its
   necrotic **Life Drain** (max-HP erosion — watch his HP). **Reserve husks (Zombies) added live only if it's
   too easy** — not pre-placed. Full design: [`encounters/cold-vault.md`](encounters/cold-vault.md). The other
   standing hooks (for after the fight):
   - **✓ Journal read (1:38 PM).** It surfaced the **cold door** — an old vault lower than the cellar that the
     keeper unlocked; the wretches **fled up from it**, so the cellar door was clawed from inside to *escape*
     the cold door, not to reach the keeper. The keeper's last line: *"the cold iron key locks the cold door.
     Do not turn it."* This is now the campaign's central pull downward. (Beat logged in `sessions/session-01.md`.)
   - **The cold iron key** — opens the **cold door** at the bottom of the deeper cellar (per the journal); it's
     the gate into the pre-built Cold Vault below. Surfaces when they reach the door and choose to turn it (the
     keeper warned against it — player's call). What's behind it is now **prepped** (see the descend hook).
   - **▶ Descend to the cold door — the boss beat is PRE-BUILT and ready (06-29).** The brood are dead; the
     deeper cellar leads to the **cold door** Vale's key opens. The fight behind it is built and DB-verified:
     encounter **"Cold Vault — the keeper"** (`adc064e7-…`) — **1× Wight (CR 3)** reflavored as the
     vault-keeper, on the new **cold vault** map, surprise off, party 2/2. **To run** when they descend:
     post the **descent** read-aloud → the **cold door** beat → if they **turn the key**, post the **vault**
     read-aloud → open the encounter → **Start Combat** (adjudicate surprise live). Full design + the two
     staged read-alouds + the key twist: [`encounters/cold-vault.md`](encounters/cold-vault.md). **Escalation
     to remember:** the keeper is genuinely **UNDEAD**, so Vale's **hold person FAILS** here (Humanoid-only) —
     the easy button from the living-wretch fights is gone. Keep enemy HP/AC secret ([`dm-rules.md`](dm-rules.md));
     players roll their own dice; reserve husks (Zombies) added live only if it's too easy.
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
