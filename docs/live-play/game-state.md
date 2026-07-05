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
- **✔ ROAD POSTED + PARTY AT THE WAYHOUSE (07-04).** The road Ashfall → Palewatch was built + posted (≈3 days
  moor→sea-cliffs; planted a **wayhouse with a forgotten guest** = the Renegade Name-Keeper's wake, un-named). The
  party took the hook: they entered the wayhouse and are working the mystery.
- **➤ Live scene — the wayhouse (THE ANTAGONIST IS NAMED; spotlight on the players):** the "blank" was a **hole in
  the keeper's memory** — a traveler passed through 3 nights back and erased both the guest and his own face.
  Resolved across two #the-story beats: (6:00 PM) **Forge Investigation 10** (set table obsessively kept, guest's
  belongings cleared clean, one fresh *deliberate* tool-cut mark unread) + **Windreth Perception 5** (keeper reads as
  just tired; eye slides off the empty chair); then (**6:04 PM**) **Vale Arcana 22** on the god's token → **the big
  reveal** (see [`campaign-arc.md`](campaign-arc.md) — this is the **Renegade Name-Keeper** surfacing on-screen):
  the traveler = **grey-cloaked, unhurried, wearing a defaced Order-of-the-Pale-Watch disc** (ex-warden), who *took*
  the guest (erased them), left **Forge's tally-cut** as a bookkeeper's ledger-tick, asked the road **up to the
  Palewatch**, and is **3 nights ahead on the party's own road** = the clock. Face still scraped (token can't
  rebuild it). Kicker: **among the string of names he murmured while working was WINDRETH'S** — fusing Windreth's
  stolen-name thread to the arc antagonist. **➤ NOW AWAIT the party's reaction** — esp. **Windreth** (react IC /
  `/check insight`|`/check history`), + Forge/Vale/anyone (press the keeper, or press on toward the Palewatch to
  chase the renegade).
- **➤ Live scene — supper + the long rest (WINDRETH REACTED; spotlight open):** the party decided to **rest at the
  wayhouse overnight** (Vale 6:32 PM "let's have a rest here"; Jonathan/Forge agreed + asked OOC "any meal that buffs
  the morning after rest?"). Over supper Vale asked Windreth *"does this traveller mean anything to you?"* →
  **Windreth answered IC (7:54 PM):** *"No. I don't know him. But he knows me... years chasing where one stolen name
  went — now a faceless man walks to Palewatch with mine in his mouth. Your mystery and mine just became the same
  road."* Then **Windreth `/check insight` → 17** (queue `5e28c724`, resolved 204). **Narrated 8:18 PM** (#the-story,
  DM author `1089351036650668143`, verified rendered — OOC-first/box-last): his ex-false-name-dealer's gut reads
  **the keeper** = harmless victim, erasure still ongoing, safe to rest but "a thin place"; and **the renegade** = a
  *collector, not a killer* — took the name clean like a clerk closing a ledger line, so Windreth's name was
  **inventory, not a personal vendetta**, and the man walked his 3-night lead *unhurried, sure of a welcome ahead* →
  **delivering, not fleeing** (= campaign-arc **rung 3 delivered: Windreth recognizes the method**). **Rules ruling
  (OOC coda):** no by-the-book morning-meal buff at their level (only *Heroes' Feast*, 5th-lvl, out of reach); a long
  rest already full-heals + returns half Hit Dice, **but resting at the wayhouse's real hearth = ALL spent Hit Dice
  back, not half** (on-theme DM grant; players run their own `/rest long`). **➤ NOW AWAIT the party's moves** —
  Windreth (`/check history` on the defaced Pale-Watch disc / speak), Vale (press the keeper again / plan dawn), Forge
  (`/check investigation` re: the scrap / take watch), anyone (`/rest long` + who keeps watch). **DM-secret still held**
  (do NOT reveal): the renegade's face/identity + ex-warden link, the Order's right-to-refuse, Vale's patron = rival
  collector, the reassemble/scatter/hand-over choice.
- **➤ Live scene — dawn at the wayhouse, rested (TWO CHECKS DONE; spotlight open, dawn departure pending):** all
  three took the long rest at the hearth (roll-history: Forge Long Rest 8:22 PM, Windreth 8:31 PM, Vale 9:45 AM) →
  **full HP + ALL spent Hit Dice back** (the grant applied). Two checks resolved into one dawn beat (**narrated
  07-05 ~11:53 AM local / 04:53Z**, #the-story, DM author `1089351036650668143`, verified rendered OOC-first/box-last;
  queue `2244ada0` + `b251493d` both **resolved 204**, Console `pending:0`): **Windreth `/check history` → 8**
  (raw d20+0, no proficiency) on the defaced Pale-Watch disc — *thin recall*: he **places** the disc (surface rep:
  a forgetting-order up the sea-cliffs that hoards on purpose; the scratch is a deliberate unmaking) but the **deep of
  the order stays past reach** — *why* a warden defaces his own sign / whether they cast out or hunt their own **won't
  come** (kept **rung 4** locked; dangled as an open question, not a reveal). **Forge `/check investigation` → 18**
  (strong) on his unreadable scrap — *craft read*: his scrap + last night's tally-cut are **same blade-angle, same
  hand, one maker** → the pieces the party carries and the pieces the renegade walks to Palewatch are the **same stock**
  (and a collector would want theirs too — stakes up); **but the letters still won't resolve** — decipherment is the
  **deliberately-withheld Palewatch payoff** (per campaign-arc "Forge scrap stays un-deciphered until a place that can
  read it"). **➤ NOW AWAIT the party's moves** — set out up the sea-cliff road (≈3 days on foot) / more wayhouse RP /
  Windreth speak on what he placed / Vale set marching order + the plan for the wardens. Same **DM-secrets still held**.

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

1. **★ WAYHOUSE REVEAL DONE — RENEGADE NAME-KEEPER NAMED + HOOKED TO WINDRETH → WINDRETH REACTED (INSIGHT 17) → PARTY LONG-RESTED + DAWN CHECKS DONE (Windreth HISTORY 8 thin / Forge INVESTIGATION 18 = same-stock, letters withheld); AWAIT DAWN DEPARTURE up the road to the Palewatch (07-05). Spine → [`campaign-arc.md`](campaign-arc.md).** The road Ashfall→Palewatch was posted 3:32 PM (planted the **forgotten-guest wayhouse** = Renegade Name-Keeper's wake + orphan seed, the barn-sized half-sunk rib). Party took the hook; three checks resolved across two #the-story beats (Narrate, DM author `1089351036650668143`; queue items `6f8bd515…`/`7a197cea…`/`f98e359b…` all resolved 204, DM Console `pending:[]`): **(6:00 PM)** Forge **Investigation 10** (set table obsessively kept, guest's belongings cleared clean, one fresh *deliberate* tool-cut mark unread) + Windreth **Perception 5** (keeper just tired; eye slides off the empty chair); **(6:04 PM)** Vale **Arcana 22** on the god's cold-iron token = the payoff. **What the memory revealed (now table-public canon):** the traveler = the **Renegade Name-Keeper** — grey cloak, unhurried, a **defaced Pale-Watch warden's disc** at his throat (ex-warden); he *took* the forgotten guest (erased them), left **Forge's tally-cut** as a ledger-tick, asked the road **up to the Palewatch**, and is **3 nights ahead on the party's own road** (= the clock). His **face stays scraped** (token can't rebuild it). Kicker: **Windreth's name was in the string he murmured** → Windreth's stolen-name quest now = the arc antagonist. **➤ NOW AWAIT the party's reaction** — esp. **Windreth** (react IC / `/check insight`|`/check history`), + Forge/Vale/anyone (press the keeper, or set out to chase the renegade up to the Palewatch). **DM-secret still held** (do NOT reveal): the Order-of-the-Pale-Watch's *right to refuse* / keep-it-scattered stance, Vale's patron = rival collector using her, and the reassemble-vs-scatter-vs-hand-it-over choice. Scale per [`big-party.md`](big-party.md) — 3-PC party, more expected. **Never roll/act/decide for the PCs.** _(Superseded below: the closed-Ashfall AWAIT-a-destination text, kept for history.)_
   - _★ (SUPERSEDED 07-04 — destination now picked) ARC CLOSED — THE FACELESS GOD IS PAID; PARTY LEAVES ASHFALL TOGETHER NEXT. AWAIT the players to pick a destination + level up (07-03 02:57Z)._ The pivot resolved on Vale's die: **Vale** (dewa) ran `/check performance` → **21** (top tier vs the secret ≈16-ish DC for a god; queue item `31259813` RESOLVED via the direct `/resolve` POST, empty outcome — ISSUE-059 button still broken). A 21 = she told the god its story **TRUE** — the very *"legend of the nameless god"* she first gave it before the fight (per the user: *"vale did tell a story in the beginning already"*), now **backed by the two recovered name-scraps** (her ashen shard-stroke from Forge's Investigation-19 + the twin Windreth's Perception-20 turned up in the heap), so a **truth-taster tasted no lie**. The geas is **discharged**, the god **UN-FORGOTTEN + released**: the cold lifts, the hundred bowed dead settle for good (a few blank faces are faces again for a moment, then still), no combat. **REWARD given in the beat:** the god's **cold-iron token** (its mark, *"a key that likes to open forgotten things"* — a real item, effect deliberately open; Vale can `/check arcana` to sense it) + **MILESTONE LEVEL 4** offered to all three (**Vale→Warlock 4, Forge→Barbarian 4, Windreth→Rogue 4** — each runs the level-up flow, **DM approves** on the dashboard; this is the promised milestone, confirmed to Forge 02:36Z). **TRAVEL HOOK planted (do NOT resolve — it's the next arc's engine):** the name is still **scattered across the realm**, the patron-pull now turns **OUTWARD** away from Ashfall, and the party leaves **together**. Three named destinations seeded — see [`world.md`](world.md) "Scattered-name destinations": **The Mask Market of Sesh** (caravan-city on the ash-waste; a name-scrap hangs on a stall), **Morran's Reach** (drowned bell-town downriver; a fragment sank with its temple), **The Palewatch** (cliff-top monastery of a forgetting-order that hoards one piece on purpose). Finale narrated → #the-story (OOC coda + read-aloud, msgs `1522436160599621728` + `1522436170968203405`, verified 02:57:49Z). **AWAIT the players:** (a) run their **level-ups** (approve each on the dashboard), (b) talk it over in **#in-character** — **which destination** + **how to travel** (moor-road on foot / hire a caravan / the river), (c) any last actions in the gallery before it's sealed (Vale `/check arcana` on the token; Forge `/check investigation` the heap; Windreth `/check perception`). **When they pick a destination, that's the NEW arc → build the road/location live as fresh unprepped territory** (record durable lore in world.md, scale per big-party.md — 3-PC party, more friends still expected toward 5-6). **Never roll/act/decide for the PCs.**
   _(Superseded parley/combat beats + the ✓ shrine/journal/cold-door history that were inlined here are recorded in [`sessions/session-01.md`](sessions/session-01.md) — trimmed 07-04. Still-live durable hooks: Vale carries the god's cold-iron token + ashen shard + 2 name-scraps + Potion of Healing + Cold Iron Key (all on her sheet); the scattered name across the realm is the next arc's engine — see [`world.md`](world.md).)_
   - **↳ Supper + long rest at the wayhouse — LIVE (07-04, narrated 8:18 PM, post by DM `1089351036650668143`):** the
     party chose to bed down overnight (Vale proposed; Jonathan/Forge agreed + asked the meal-buff Q). **Windreth
     `/check insight` → 17** read: keeper = harmless victim / "thin place" (safe to rest); renegade = *collector, not
     killer* — Windreth's name = **inventory not vendetta**, and his unhurried 3-night lead = **delivering, not fleeing**
     (= arc **rung 3: Windreth recognizes the method**). Narrated + queue `5e28c724` **resolved 204**, Console
     `pending:0`. **Rest grant ruled:** resting at the wayhouse hearth returns **ALL** spent Hit Dice (not half) — no
     by-the-book meal buff at their level (*Heroes' Feast* is 5th-lvl). Players run their own `/rest long`. **Await**
     Windreth (`/check history` on the Pale-Watch disc / speak), Vale (press keeper / plan dawn), Forge
     (`/check investigation` / watch), + who keeps watch. Full beat in "Current scene". **Never roll/act/decide for PCs.**
   - **↳ Dawn beat — two checks, party rested — DONE + narrated (07-05 ~11:53 AM local / 04:53Z, post by DM `1089351036650668143`):** all three long-rested at the hearth → **full HP + ALL Hit Dice back** (grant applied). **Windreth History 8** (raw d20+0) = thin recall, places the Pale-Watch disc's surface rep only, **rung 4 held** (ex-warden / right-to-refuse won't come). **Forge Investigation 18** = strong craft read — his scrap + the tally-cut are one maker/one tool, so the party carries the **same name-stock** the renegade collects (stakes up), **but the letters stay unreadable** (decipherment withheld for the Palewatch). Both queue items (`2244ada0` + `b251493d`) **resolved 204**, Console `pending:0`. Full beat in "Current scene". **➤ Await dawn departure** (up the sea-cliff road, ≈3 days) / more wayhouse RP. **Never roll/act/decide for PCs.**
   - **↳ Pending player action — Forge's short-rest heal (07-04) — RESOLVED (MOOT):** the party **took the long rest** (roll-history: all three), which full-heals + returns Hit Dice, so the botched short rest is fully superseded — no re-run needed, no DM action. _(Background: he'd `/undo`'d a Skip-tapped short rest → 0 dice spent, lost nothing; new DM-grant `POST /api/campaigns/{id}/spend-hit-dice` shipped but deliberately NOT used — it rolls server-side; players roll their own. See [`sessions/session-01.md`](sessions/session-01.md).)_

2. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row + sheet →
   fold into the fiction). 3-4 more PCs expected. See [`runbook.md`](runbook.md) "Onboarding
   players" + [`big-party.md`](big-party.md).
3. **After every beat:** narrate to #the-story (read-aloud) + append the narrative
   [`sessions/`](sessions/) log. Do **not** transcribe HP/positions back into this file.
