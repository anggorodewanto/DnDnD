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

_Last updated: 2026-07-03 — **▶ POST-ARC: ALL 3 L4 LEVEL-UPS RESOLVED + THE GOD'S ITEMS ADDED TO VALE + THE PARTY HAS LEFT ASHFALL — SPOTLIGHT ON THE DESTINATION PICK.** The three milestone level-ups are DONE: Vale +2 CHA→18 (already); **Forge → Great Weapon Master**; **Windreth → Defensive Duelist** (07-03 DM ruling: our 2024 Defensive Duelist is a **half-feat, +1 DEX → 18, so AC 14→15** — the `defensive-duelist` seed had no `AsiBonus`; fixed red/green in `internal/refdata/seed_feats.go` + redeployed so future PCs get it, and Windreth's DEX/AC were corrected by a guarded one-time DB data-repair since he was feated pre-fix. Two product gaps this surfaced — ASI apply never recomputes AC; `ApplyFeat` re-adds duplicate features — filed as **ISSUE-064** OPEN). — feats applied via `POST /api/levelup/feat/apply` from the authed dashboard tab (both 200/"applied", features DB-verified), Forge's pending row said `tough` but his whisper `dad36f26` said "i select great weapon mastery feat" → applied GWM not tough; **both `pending_asi` rows then DELETED** (endpoints don't clear them) so the DM Console no longer shows pending level-ups. Forge's GWM whisper + all **3 held gallery skill-checks** were resolved (`POST /dashboard/queue/<id>/resolve`, empty outcome → 204; ISSUE-059 button still dead). **THE GOD'S ITEMS ARE NOW ON VALE'S SHEET** (per the user's ask; added via `POST /api/inventory/add` from the authed tab, each with a UNIQUE slug id to dodge ISSUE-063): the **Faceless God's Token** (`faceless-god-token`, its cold-iron mark — "a key that likes to open forgotten things," effect open), the **Ashen Face-Shard** (`ashen-face-shard`, the relic/story-vessel that carried scrap #1 of the name — narrated hers since 07-02 but never persisted till now), and a second **Name-Scrap of the Faceless God** (`name-scrap-faceless-god`, the heap-face bearing the twin stroke = scrap #2). (She also still holds the Potion of Healing + Cold Iron Key.) **THE 3 RECENT CHECKS were narrated** in one #the-story beat (OOC coda + read-aloud, `POST /api/narration/post → 201`, post id `59983b0a`, DB author `1089351036650668143`, verified rendered in #the-story — OOC-first/box-last): **Vale Arcana 5** (low — she feels the token opens "forgotten things" + her patron's certainty it matters, but can't parse it down here → matches her IC "we can't decipher that here"; keep it), **Forge Investigation 19** (high — his "quick check of our stuff before we depart": all road-ready, AND his smith-eye reads the two name-scraps as cut by the SAME hand/tool as the shrine wall = two pieces of ONE thing, and more are scattered OUT there — travel hook, name NOT deciphered), **Windreth Perception 9** (low — the bowed dead have settled for good, no cold/no watcher, the way up is clear). Per the players' own IC (Vale 10:08 "let's wrap up / we should get out" + takes the god's items; Forge 10:16 "agree, let's leave" + packs), narrated the party **CLIMBING OUT** — up out of the buried gallery, past the hollow shrine + open cold door, up the clawed stairs, through the dead waystation and OUT under a grey morning; **Ashfall is behind them, the patron-pull now points OUTWARD** across the moor toward the scattered name. **SPOTLIGHT ON THE PLAYERS:** pick a destination (**Sesh / Morran's Reach / Palewatch** — see [`world.md`](world.md)) + how to travel (moor-road on foot / hire a caravan / the river), talk it out in #in-character; Windreth may answer Vale's "why are you here?". When they pick → **build the road/location live as fresh unprepped territory** (record durable lore in world.md; scale per big-party.md — 3-PC party, more friends still expected). **NEW BUG ISSUE-063 (OPEN):** `POST /api/inventory/add` stacks a custom item (empty `item_id`) onto ANY existing empty-`item_id` item because `findItemIndex` matches on ItemID only — worked around with unique slug ids; **Vale's "Cold Iron Key" x2 is likely a prior victim** (do NOT blindly fix). **Never roll/act/decide for the PCs.**_ _(Prior beats — the full superseded history — are in [`sessions/session-01.md`](sessions/session-01.md); trimmed from this save-file 07-04 to keep the resume set lean.)_
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
| Buried Gallery of the Faceless God | `39ecd023-51d8-44bb-bf8e-29e1eff3a231` | 12×12 blank stone (built in-app 07-02). Player spawn bottom-center (the mouth), enemy spawn top-center (the heart). Features narrated. The gallery combat map. Grid: row 1 = heart/top, row 12 = mouth/bottom; ~8 squares = the 40 ft road. |

## Active encounter (durable refs — live state via the Console)

- **CLOSED — "The Buried Gallery of the Faceless God"** (internal **"The Faceless God Wakes — Buried Gallery"**),
  encounter id `9e558982-697a-4cc8-8c25-abe3d34cf201`, map *Buried Gallery of the Faceless God*
  (`39ecd023-…`). Roster: **1 Ghoul (masked sentinel) + 2 Zombies (tensed dead)**, reserve Zombies never needed.
  Started 07-02 ~2:22 PM; **ended in VICTORY 07-03 ~02:19Z (R3)** — all three roused dead destroyed (Windreth
  finished the last zombie), the dormant god itself never lifted a hand. **Combat ended → `status=completed`,
  `has_encounter:false`**; ISSUE-038 auto-carried final HP to sheets (Windreth 19/24, Forge 29/32, Vale 24/24,
  no conditions; rage NOT carried). Party OUT OF COMBAT at the god's heart; the parley reopened. Chronology:
  [`sessions/session-01.md`](sessions/session-01.md).
- **CLOSED — "The Cold Vault"** (internal **"Cold Vault — the keeper"**), encounter id
  `446dce33-e221-4d1f-a88b-4e81534b3399` (template `adc064e7-…`), map *Ashfall Waystation — the cold vault*
  (`2899165e-…`). Started 2026-06-29 ~7:56 PM; **ended in VICTORY 2026-06-30 (R3)** — the lone **Wight** keeper
  destroyed (Vale's two Shatters ground it down to the brink; **Forge finished it** with thrown handaxes, the killing
  blow a NAT 20 vex crit). Surprise off (none — keeper dormant, PCs deliberately opened). Party 2/2, **no casualties**.
  **Combat ended → `status=completed`**; ISSUE-038 auto-carried final HP/conditions to the sheets (Forge 25/32, Vale
  24/24, no conditions). **No active encounter.** Reserve husks were never needed. Chronology:
  [`sessions/session-01.md`](sessions/session-01.md); design: [`encounters/cold-vault.md`](encounters/cold-vault.md).
- **CLOSED — "The Cellar"** (internal "Cellar — the brood"), encounter id
  `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map *Ashfall Waystation — cellar*. 4 combatants
  (Vale + Forge vs two ghouls). **Ended in victory 2026-06-28 at R11** — both ghouls dead, Forge
  stabilized. Combat ended; **no active encounter**. Post-combat PC state was carried out by hand
  (no combat-end HP write-back) — see "Current scene". Chronology: [`sessions/session-01.md`](sessions/session-01.md).
- Prior fight — "Waystation — the cellar wretch" (id `6f317490-c43e-44a0-a1d0-b6ed51e58a3e`) —
  ended 2026-06-26 in victory. Full chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene (narrative framing — non-derivable)

**★ Post-arc — the party has LEFT ASHFALL, out on the grey moor, choosing where to go next.** The whole Ashfall
descent is closed: the cellar brood, the Cold Vault keeper, and the Buried Gallery of the Faceless God are all
cleared, and on 07-03 **the faceless-god arc RESOLVED** — Vale's Performance 21 told the god its own story true
(backed by two recovered name-scraps), so the **geas is discharged** and the god was un-forgotten + released (no
final combat; the hundred bowed dead settled for good). The party climbed out — past the hollow shrine and open
cold door, up through the dead waystation — and stands **under a grey morning with Ashfall behind them**.

- **Party:** all three at **Level 4** (Vale Warlock 4, Forge Barbarian 4 + Great Weapon Master, Windreth Rogue 4 +
  Defensive Duelist) — durable sheet: [`party/roster.md`](party/roster.md); live HP → DM Console.
- **Vale carries** the god's **cold-iron token** ("a key that likes to open forgotten things"), the **ashen
  face-shard**, **two name-scraps**, plus the **Potion of Healing** (2d4+2, unused) and the **Cold Iron Key** —
  all on her sheet.
- **The pull now points OUTWARD:** the god's name is still **scattered across the realm**, so the patron-tether
  turns away from Ashfall toward the scattered pieces — the engine of the next arc.
- **✔ DESTINATION PICKED — PALEWATCH (07-04).** The open spotlight resolved in #in-character: Vale laid out a
  map, focused the patron-pull, and pointed — *"There."* → **The Palewatch** (cliff-top forgetting-order that
  hoards a name-scrap). **Windreth joined** and gave his backstory (the "why are you here?" mystery, now answered):
  he *sold false names, had a real one stolen, now hunts erased things* — his quest fuses with the arc (a
  **name-thief** = the antagonist thread). **Forge is in** (his unreadable scrap needs deciphering). The party
  travels **together**.
- **➤ Immediate spotlight (on the DM):** Vale asked directly — *"how far, what terrain, what's along the way from
  here to Palewatch?"* Build the **road Ashfall → Palewatch** live as a travel situation (skeleton +
  who-wants-what in [`campaign-arc.md`](campaign-arc.md) "The immediate next beat"). The campaign **spine** (secret
  truth, antagonist, clock, per-PC threads) is now written there — load it before narrating the new arc.

Full beat-by-beat history of the closed Ashfall arc: [`sessions/session-01.md`](sessions/session-01.md).
World / lore / destinations: [`world.md`](world.md).

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** first for `next_step` + the live board, then apply this intent.

> **✔ LEVEL-UP RESUME — RESOLVED 07-03 (all 3 L4 level-ups done; story un-held).** _(Was folded in from the now-deleted PLAN-twf-nick-dualwielder.md.)_
> - **Vale** — L4 DONE (+2 CHA → 18; roster mirrored, commits `5916dde`/`56bd79d`).
> - **Forge** — **DONE → Great Weapon Master applied** (his whisper `dad36f26` "i select great weapon mastery feat" confirmed the Tough→GWM swap; applied GWM via `/api/levelup/feat/apply`, NOT tough; `pending_asi` row deleted; whisper resolved). Feature DB-verified on his sheet.
> - **Windreth** — **DONE → Defensive Duelist applied** (he'd submitted feat `defensive-duelist` via the ASI flow → `pending_asi` row; DEX 17 meets the DEX-13 prereq; applied via `/api/levelup/feat/apply`, row deleted). Feature DB-verified. _(Loadout tip still worth relaying: shortsword main / dagger off-hand so main-hand Vex + off-hand Nick both fire — ISSUE-061 note; his equip is currently reversed.)_
> - **Held checks — RESOLVED + narrated** (the 3 gallery skill-checks: **Vale Arcana 5, Forge Investigation 19, Windreth Perception 9**) → all queue items cleared + woven into the #the-story beat above (post `59983b0a`).
> - **Engine fixes shipped this session (both FIXED + TDD + redeployed):** **ISSUE-061** (off-hand two-weapon swings now apply their weapon-mastery on-hit effect) + **ISSUE-062** (off-hand Light-property extra now capped once/turn — no feat → 2 swings; **Dual Wielder + Nick → 3**). See [`issues.md`](issues.md).
> - **Known open bugs:** **ISSUE-059** (DM-Queue Resolve button fires no POST — resolve via `POST /dashboard/queue/<id>/resolve` from the authed tab), **ISSUE-060** (builder never surfaces Warlock pact boon / invocations).
> - **Ops:** app `localhost:8080`; DB `docker exec -i dndnd-db-1 psql -U dndnd -d dndnd` (`-i` required for stdin/heredoc); redeploy detached `docker compose up -d --build app`.

1. **★ NEW ARC OPEN — PALEWATCH; ROAD BEAT POSTED, AWAIT PLAYERS AT THE WAYHOUSE (07-04). Spine → [`campaign-arc.md`](campaign-arc.md).** The players resolved the destination spotlight in #in-character: **Vale pointed the patron-pull at THE PALEWATCH** (07-04 3:18 PM, *"There."*), **Windreth joined + revealed his backstory** (sold false names → had a real one stolen → hunts erased things = the antagonist thread), **Forge is in** (needs his unreadable scrap deciphered). Answered Vale's *"how far / what terrain / what's along the way?"* — **road beat POSTED to #the-story** (Narrate, post 07-04 3:32 PM, DM author `1089351036650668143`, verified rendered): ≈3 days moor→sea-cliffs, and it plants a **wayhouse with a blank guest** (the Renegade Name-Keeper's wake, un-named) + one **orphan future-seed** (a barn-sized half-sunk rib — never explained). The beat pauses there and hands spotlight back. **➤ NOW AWAIT the players' moves at the wayhouse** (investigate the blank guest via `/check perception|investigation|insight|arcana`, or press on / how they travel). **Load [`campaign-arc.md`](campaign-arc.md) before narrating the follow-up** — DM-secret spine (Order of the Pale Watch = wardens; Renegade Name-Keeper = who stole Windreth's name + races them for scraps; Vale's patron = rival collector using her; reassemble-vs-scatter-vs-hand-it-over choice). Scale per [`big-party.md`](big-party.md) — 3-PC party, more expected. **Never roll/act/decide for the PCs.** _(Superseded below: the closed-Ashfall AWAIT-a-destination text, kept for history.)_
   - _★ (SUPERSEDED 07-04 — destination now picked) ARC CLOSED — THE FACELESS GOD IS PAID; PARTY LEAVES ASHFALL TOGETHER NEXT. AWAIT the players to pick a destination + level up (07-03 02:57Z)._ The pivot resolved on Vale's die: **Vale** (dewa) ran `/check performance` → **21** (top tier vs the secret ≈16-ish DC for a god; queue item `31259813` RESOLVED via the direct `/resolve` POST, empty outcome — ISSUE-059 button still broken). A 21 = she told the god its story **TRUE** — the very *"legend of the nameless god"* she first gave it before the fight (per the user: *"vale did tell a story in the beginning already"*), now **backed by the two recovered name-scraps** (her ashen shard-stroke from Forge's Investigation-19 + the twin Windreth's Perception-20 turned up in the heap), so a **truth-taster tasted no lie**. The geas is **discharged**, the god **UN-FORGOTTEN + released**: the cold lifts, the hundred bowed dead settle for good (a few blank faces are faces again for a moment, then still), no combat. **REWARD given in the beat:** the god's **cold-iron token** (its mark, *"a key that likes to open forgotten things"* — a real item, effect deliberately open; Vale can `/check arcana` to sense it) + **MILESTONE LEVEL 4** offered to all three (**Vale→Warlock 4, Forge→Barbarian 4, Windreth→Rogue 4** — each runs the level-up flow, **DM approves** on the dashboard; this is the promised milestone, confirmed to Forge 02:36Z). **TRAVEL HOOK planted (do NOT resolve — it's the next arc's engine):** the name is still **scattered across the realm**, the patron-pull now turns **OUTWARD** away from Ashfall, and the party leaves **together**. Three named destinations seeded — see [`world.md`](world.md) "Scattered-name destinations": **The Mask Market of Sesh** (caravan-city on the ash-waste; a name-scrap hangs on a stall), **Morran's Reach** (drowned bell-town downriver; a fragment sank with its temple), **The Palewatch** (cliff-top monastery of a forgetting-order that hoards one piece on purpose). Finale narrated → #the-story (OOC coda + read-aloud, msgs `1522436160599621728` + `1522436170968203405`, verified 02:57:49Z). **AWAIT the players:** (a) run their **level-ups** (approve each on the dashboard), (b) talk it over in **#in-character** — **which destination** + **how to travel** (moor-road on foot / hire a caravan / the river), (c) any last actions in the gallery before it's sealed (Vale `/check arcana` on the token; Forge `/check investigation` the heap; Windreth `/check perception`). **When they pick a destination, that's the NEW arc → build the road/location live as fresh unprepped territory** (record durable lore in world.md, scale per big-party.md — 3-PC party, more friends still expected toward 5-6). **Never roll/act/decide for the PCs.**
   _(Superseded parley/combat beats + the ✓ shrine/journal/cold-door history that were inlined here are recorded in [`sessions/session-01.md`](sessions/session-01.md) — trimmed 07-04. Still-live durable hooks: Vale carries the god's cold-iron token + ashen shard + 2 name-scraps + Potion of Healing + Cold Iron Key (all on her sheet); the scattered name across the realm is the next arc's engine — see [`world.md`](world.md).)_
   - **↳ Pending player action — Forge's short-rest heal (07-04):** he `/undo`'d a botched short rest (tapped **Skip** → 0 dice spent, so he lost nothing; still 29/41, `{"d12":4}` intact). Ticket resolved + whispered him to **re-run `/rest short` and tap a Hit-Dice number** (he rolls his own; auto-approve ON so it applies instantly). **Await his re-run** — no DM action needed. _(New DM-grant capability `POST /api/campaigns/{id}/spend-hit-dice` shipped this session but deliberately NOT used — it rolls server-side; players roll their own. See [`sessions/session-01.md`](sessions/session-01.md).)_

2. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row + sheet →
   fold into the fiction). 3-4 more PCs expected. See [`runbook.md`](runbook.md) "Onboarding
   players" + [`big-party.md`](big-party.md).
3. **After every beat:** narrate to #the-story (read-aloud) + append the narrative
   [`sessions/`](sessions/) log. Do **not** transcribe HP/positions back into this file.
