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
- **➤ Live scene — on the road, day one, the blanked-shepherd croft (DEPARTURE TAKEN; croft choice pending):** party
  chose to set out (#in-character 07-05: Vale *"move on to Palewatch?"* 12:05 PM → Forge *"yeah, let's go"* 12:11 PM;
  Windreth already committed IC 07-04 7:54 PM *"your mystery and mine just became the same road"*). Departure beat
  **narrated 07-05 ~12:32 PM local**, #the-story, DM author `1089351036650668143` (verified rendered, OOC-coda +
  per-PC menu first, read-aloud box last). Content: set out up the **moor-road, ≈3 days onto the sea-cliffs**; two
  light prompts handed back (**Vale set marching order + first night-watch**; **pace** — normal ~3 days vs a forced
  night-march to close the renegade's 3-night lead, wears people down, decide on the road). Day-one montage lands them
  at dusk at a **lone shepherd's croft** = the renegade's **wake** (campaign-arc **rung 1**, table-safe/reused): the
  shepherd is **blanked** (can't recall his own name, doesn't know it's gone — same cold quiet as the wayhouse guest,
  Windreth reads it first = rung-3 method-recognition, reused), and a **fresh-ish tally-cut on the doorpost** = same
  hand/same cut as the wayhouse (the man came through, *did not stop long* → gap still ~held, clock = texture not
  stopwatch). **➤ NOW AWAIT the party's move at the croft** — **Stop** (Forge `/check investigation`, Windreth
  `/check insight`|`/check survival`, Vale `/check persuasion`|`/check history` with/for the shepherd) / **Push past**
  (keep daylight, gain ground) / something else; + Vale's marching-order + pace call. **DM-secrets still held** (do NOT
  reveal): renegade's face/identity + ex-warden link, the Order's right-to-refuse, Vale's patron = rival collector,
  the reassemble/scatter/hand-over choice. **Never roll/act/decide for the PCs.**
- **➤ Live scene — croft READ (party chose Stop; 3 checks adjudicated + narrated; camp-vs-push pending):** party
  stopped and worked the croft. Three checks resolved into one beat (**narrated 07-05 ~2:08 PM local**, #the-story,
  DM author `1089351036650668143`, verified rendered OOC-first/box-last; queue `1b7e7ee2`+`659601d9`+`d0ea6f97` all
  **resolved 204**, Console `pending:0`): **Vale `/check arcana` → 9** (IC 2:03 PM: token on the shepherd's hands,
  asking after a recent traveller) — token *wakes* (confirms same erasure power/hand as the wayhouse) but a 9 can't
  force it; the man's memory of the visitor is **scraped clean as his name** — no face, no vision (rung 4 held).
  **Forge `/check investigation` → 15** — reads the scene: taking was **slow/unhurried, nothing broken**; doorpost
  tally = same tool/hand as the wayhouse, **~2–3 days weathered** → the ~3-night lead **still holds**. **Windreth
  `/check insight` → 20** (his best) — the shepherd is *whole*, just short the one thing that **never grows back on
  its own**; the man is **harvesting names for stock** (like Windreth's own), and he's **still collecting in the open
  before the Palewatch → a window to catch him on the road that shuts once he's behind the walls** (raises stop-vs-push;
  no rung-4 leak). Honest ruling surfaced: **nothing at the croft restores the shepherd — only reaching the source
  might**. **➤ NOW AWAIT the party's move** — **Camp the night** (take watches, set out at first light) / **Push on
  into the night** (forced march to eat the lead — I'll track 2024 exhaustion) / do something for the shepherd /
  Vale's still-open marching-order call. Same **DM-secrets held**. **Never roll/act/decide for the PCs.**
- **➤ Live scene — WINDRETH SCOUTS AHEAD (party-split; his check pending + Vale/Forge camp-vs-push still open):** after the
  croft beat, Windreth (Winfroz) reacted IC (#in-character 07-05): 4:11 PM *"He is still taking names. But maybe we can
  stop him from taking the next one."* → 4:56 PM *"I will scout ahead. Quietly."* — declared a **solo scout** up the
  moor-road at dusk. Vale (dewa) + Forge (JonathanEka) stayed **silent on camp-vs-push**. Reacted with one beat
  (**narrated 07-05 ~6:21 PM local**, #the-story, DM author `1089351036650668143`, verified rendered OOC-first/box-last;
  **no queue item** — this is a reaction to IC declarations, Console `pending:0`): handed Windreth a **pick-one check**
  (`/check stealth` stay unseen | `/check survival` read the road/tracks | `/check perception` watch the dark) with the
  honest clock-frame — **ranging ahead tonight ≠ catching the man** (he's ~3 nights up the road); a lone scout buys
  *eyes on the road / early warning / a sign of where he stopped*, and **only a whole-party forced night-march actually
  eats the lead** (so the real gap-closing is still Vale+Forge's camp-vs-push call). Read-aloud = Windreth stepping past
  the croft-light into the dark moor, alone, listening. **No dice rolled for him** (players roll own). **➤ NOW AWAIT** —
  **Windreth** rolls his chosen check (I resolve the scout next); **Vale + Forge** still owe **Camp here** vs **Break camp
  and push with him** (+ anything for the shepherd). Same **DM-secrets held** (renegade's face/ex-warden link, Order's
  right-to-refuse, Vale's patron, the reassemble/scatter/hand-over choice). **Never roll/act/decide for the PCs.**
- **➤ Live scene — NIGHT-MARCH (forced march; party PUSHES together; 3× CON saves pending):** party chose **PUSH, not
  split** — Forge (JonathanEka) IC 6:42 PM *"Vale, let's follow Windreth from afar while he do the stealth"* → Vale (dewa)
  6:44 PM *"gonna be cold walk. Let's go"* → Forge 7:16 PM *"(to DM: do Forge & Vale need to roll check?)"*. So Windreth
  scouts point, Vale+Forge trail from afar, **whole party marches into the night** (not camping). Windreth had rolled
  **both** offered checks (I said pick-one; he ran two): **Stealth 11** + **Perception 22** (queue `514b447a`+`8b8762dc`,
  both **resolved 204**, Console `pending:0`). Adjudicated + narrated one beat (**07-05 ~7:56 PM local**, #the-story, DM
  author `1089351036650668143`, verified rendered OOC-first/box-last): **Stealth 11** = moves quiet enough, nothing out
  here tonight tests it (unseen, but flagged an 11 wouldn't beat real watchers — sets up stealth mattering later);
  **Perception 22** (the prize) = the renegade's trail is **clear/easy to follow**, he walks alone/steady/**unhurried, no
  care to hide** (doesn't know he's chased), a **cold camp ~a night old** packed clerk-neat + a fresh ledger scratch-mark
  on a stone pointing on, land climbing to the **sea-cliffs** w/ one far light = the next roof he'd have passed → **he is
  not running, so the party CAN gain on him** (a hard march really eats the lead). **Ruling to Forge's question:**
  following Windreth needs no check (nothing notices them tonight), but marching into the night = **forced march (2024)** →
  **all three roll ONE CON save, DC 12** (`/roll 1d20+CON`, reason "forced march"); make = push fine, miss = **1 level
  exhaustion** (2024 −2 all d20 + speed cut, stacks). **Payoff on success:** ~3-night lead shrinks toward ~2 (clock only
  moves once saves land — until then lead still ~3; campaign-arc clock NOT yet edited). **➤ NOW AWAIT the 3 CON saves**
  (Windreth/Forge/Vale) → then I narrate where the night-march leaves them (+ apply any exhaustion, + advance the clock).
  Same **DM-secrets held** (renegade's face/ex-warden link, Order's right-to-refuse, Vale's patron, the choice). **Never
  roll/act/decide for the PCs.**
- **➤ Live scene — NIGHT-MARCH LANDED (all 3 failed CON → exhaustion 1 each; lead ~3→~2; the Palewatch in sight; rest-vs-push
  pending):** the three forced-march CON saves came in (#roll-history) — **all missed DC 12**: **Vale 7, Forge 8, Windreth 4**.
  Ruled the march still makes its ground (forced-march cost = exhaustion, not a failure to progress): **all three take 1 level
  Exhaustion** (2024: −2 to all d20 tests + −5 ft speed; a long rest removes one level) and the **push succeeds → the renegade's
  ~3-night lead shrinks to ~2**. **Exhaustion 1 applied to all three via the dashboard status editor** — Vale + Windreth by their
  party-overview **Edit status** cards; **Forge via `POST /api/character-overview/d2d98745…/status`** (authed fetch) because his
  party-overview **card is missing**: his `player_characters` row has been **`status=rejected` since the 07-03 L4 rework** (stale
  leftover, not a fresh anomaly — the character record plays fine and the status endpoint gates on character-existence +
  campaign-DM + not-in-combat, **not** approval). DB-verified after: **Vale 31/31, Forge 41/41, Windreth 31/31, all exh 1** (Forge's
  true max is 41, not the roster's stale "32"). Narrated one beat (**07-06 ~9:27 AM local**, #the-story, DM author
  `1089351036650668143`, verified rendered OOC-first/box-last): the three misses + the exhaustion rule in plain ESL (−2 all d20 +
  −5 ft, long rest clears one) + the trade (lead 3→2) + a per-PC **rest-vs-push** menu; read-aloud = the cold march to grey dawn,
  moor→salt-wind **cliff country**, and **the Palewatch's pale towers showing across a gorge for the first time** (a hard half-day
  of switchbacks off), the grey man a night nearer on the same trail *"sure of his welcome,"* unaware he's followed (**re-uses
  rung 3; rung 4 held**). **Campaign-arc clock advanced ~3→~2 nights.** **➤ NOW AWAIT the party's rest-vs-push call** — `/rest long`
  (sheds a level of exhaustion, hands a little lead back) vs press on worn toward the walls; Vale still owes marching order + first
  watch; anyone may scout the approach (`/check perception`|`/check stealth`) — **all rolls at −2 until they long-rest**. Same
  **DM-secrets held** (renegade's face/ex-warden link, Order's right-to-refuse, Vale's patron, the reassemble/scatter/hand-over
  choice). **Never roll/act/decide for the PCs.**

- **➤ Live scene — REST TAKEN → AT THE PALEWATCH WALLS (party chose rest over push; approach pending):** the rest-vs-push
  call resolved **REST** in #in-character on Windreth's argument to keep the edge — **Windreth** (9:49 AM) *"He still does not
  know we are behind him... I would rather keep it with clear eyes than spend it stumbling. Let's rest first, build camp hidden
  from the trail, so he will not be alerted."*; **Vale** (9:50 AM) *"agreed... we are close, we can still observe and rest.
  Chase again after."* All three ran **`/rest long`** at a hidden camp (Vale 9:50 / Windreth 9:52 / Forge 9:59) → engine
  cleared **Exhaustion 1 → 0** for each and full-healed (the COV-18 exhaustion change did NOT break the rest decrement;
  DB-verified **Vale 31/31, Forge 41/41, Windreth 31/31, conditions `[]`**). **No DM mutation needed.** **Trade honored (clock):**
  a full night's rest hands the grey man back ~the night the march bought → the **open-road catch window closes** (he reaches the
  Palewatch **ahead** of the party) — traded for **clear eyes + intact stealth** (the hidden camp held). Narrated one beat
  (**07-06 ~10:32 AM local / 03:32Z**, #the-story, DM author `1089351036650668143`, Discord msg `1523531993638244454`, `POST
  /api/narration/post → 201`, verified rendered OOC-first/box-last): OOC coda = rest paid off (full health, exhaustion gone, −2
  off) + the lead-trade + a per-PC **approach menu**; read-aloud = the hidden camp + grey-dawn *rested* wake + the switchback
  climb into salt-wind cliff country + the first close look at the **Palewatch** (pale towers on the drop's edge, shut gate,
  "a place built to forget on purpose") + a **fresh ledger-tick on a trailside stone** (the renegade came this way, already at the
  gate). **➤ NOW AWAIT the party's approach to the Palewatch** — petition the wardens openly (Vale `/check persuasion`), scout it
  first unseen (Windreth `/check stealth`|`/check perception`), disguise in via Vale's *Mask of Many Faces* (→ `/check deception`),
  Forge's unreadable scrap as trade-bait or `/check investigation` on the approach, or split up; + Vale's still-open marching order.
  **DM-secrets still held** (renegade's face/ex-warden link, the Order's **right to refuse**, Vale's patron = rival collector, the
  reassemble/scatter/hand-over choice); **rung 4 (why a warden defaces his own sign / ex-warden link) lands AT the Palewatch itself.**
  Build the Palewatch **live as fresh unprepped territory** (player-facing lore → [`world.md`](world.md); secret core →
  [`campaign-arc.md`](campaign-arc.md) "Palewatch"; scale per [`big-party.md`](big-party.md) — 3-PC party). **Never roll/act/decide
  for the PCs.**

- **➤ Live scene — WINDRETH SCOUTED THE GATE (Perception 13): the Palewatch is already FORCED OPEN, posts empty, one traveller in
  ahead of them; entry pending):** the party chose **scout first, do not knock** — **Windreth** (Winfroz, 10:40 AM) *"I will get
  eyes on the gate. Watchers, murder holes, side paths, fresh tracks... Do not knock yet. Not until we know whether we are walking
  into a welcome or a trap."*; **Forge** (10:43) *"understood"*; **Vale** (dewa, 11:15) *"let me know the looks of the people, i can
  disguise into them"* (teeing up *Mask of Many Faces* — a request for info, **not** a roll). **Windreth = he/him** (I first wrote
  "she" in the beat — **wrong**; corrected in #the-story + memory `reference_pc_pronouns`; see `party/windreth.md`). Windreth's
  **Perception 13** (dm-queue `a1fa1172…`, resolved `POST …/resolve → 204`, outcome `narrated`). **Adjudication (solid exterior
  read, deep unknowns open):** empty posts (no watchers); **gate forced quietly** (bar lifted, lock-wood split, one leaf ajar);
  **one set of fresh boot-prints** of a single light/fast traveller in within the day (matches the trail-stone tally); **two ways
  in** — front gate (open, but arrow-slits + a murder-hole gallery over the threshold) or a windward **goat-track along the cliff
  shoulder** (unwatched now). A 13 does **not** reveal where the wardens went or whether the intruder is **still inside**. **Vale's
  disguise gated behind entry** — posts empty, no face to *Mask* into until he sees a real warden inside. Narrated one beat (**07-06
  ~11:21 AM / 04:21Z**, #the-story, Discord msg `1523544339135987713`, `201`, DB `673eea8b`) + a pronoun **correction** repost
  (**04:24Z**, msg `1523545253263708172`, `201`). **➤ NOW AWAIT how the party ENTERS** — front gate under the murder-holes vs the
  windward goat-track vs hang back + watch more; who goes first (Windreth `/check stealth` to slip in | `/check investigation`
  Expertise on the lock/tracks; Forge take point | `/check perception`/`/check investigation` on the murder-holes; Vale hold the
  disguise for inside | `/check perception` on the slits); + Vale's marching order. **Build the Palewatch interior LIVE** (player
  lore → [`world.md`](world.md); secret core → [`campaign-arc.md`](campaign-arc.md) "Palewatch"; scale → [`big-party.md`](big-party.md)).
  **DM-secrets held; rung 4 (ex-warden link / the Order's right-to-refuse) still lands INSIDE the Palewatch.** **Never roll/act/decide
  for the PCs.**

- **➤ Live scene — GATE READ (party examined the forced gate; 3 checks adjudicated + narrated; PUSH-IN pending):** at the mouth of
  the forced gate the party read it three ways before committing (all resolved 204 `narrated`, Console `pending:[]`): **Forge
  Investigation 11** (dm-queue `4bacbdb2…`) — gatehouse **empty**, the **portcullis winch is cut** so the gate can't drop (rigged
  open), murder-holes **unmanned**; **Windreth Investigation 14** (Expertise; `4fd7dc70…`) — **one intruder, alone**, forced the lock
  **quiet + calm** (*sure he'd get in*), **nobody followed him**, **paused just inside** then went **deep**, unhurried (a 14 can't ID
  him or say if he's **still inside**); **Vale Perception 19** (`9d29f31b…`) — the wardens **did not leave willingly** (dropped horn,
  torn cloak), **one warden is still alive + hiding** across the yard = **a face Vale can *Mask* into once close** (**disguise now
  UNLOCKED** — an ordinary warden, **NOT** the renegade whose face stays scraped), and a **cold erasure-trail leads to the inner
  keep**. **Windreth = he/him** (verified in preview + render). Narrated one beat (**07-06 ~11:53 AM / 05:53Z**, #the-story, Discord
  msg `1523567610103336961`, `201`, DB `9f24b3d0`, coordinate-click Post). Re-uses **rungs 1 + 3**, **no new rung 4**. **➤ NOW AWAIT
  how the party PUSHES IN** — follow the trail toward the inner keep (`/move`) / reach the live warden first (`/check stealth` |
  `/check persuasion` | `/check insight`) / Vale closes + takes the face / watch the murder-hole gallery on the way (`/check
  perception` | `/ready`) / or fall back to the windward goat-track; + who leads. **Build the Palewatch interior LIVE.** **DM-secrets
  held; rung 4 (renegade's face / ex-warden link / the Order's right-to-refuse) lands when they meet a warden face-to-face inside.**
  **Never roll/act/decide for the PCs.**

- **➤ Live scene — PUSHED IN → CONTACT WITH THE HIDING WARDEN (party split; Vale parleys, Windreth flanks; 3 checks failed +
  narrated; quiet broken; reaction pending):** the party committed inside. **Vale stepped into the open** and called gently to the
  live warden — *"hello? friend, travellers passing by, what happened here?"* — while **Windreth slid along the wall to stay unseen**;
  Forge held. All three rolls landed low/fumble (resolved 204, Console `pending:[]`): **Vale Persuasion 6** (raw d20 **2**; `11564d02…`)
  — a horned stranger walking up open is too much for a terrified survivor; he **does not trust her**, warns her quiet, grips a broken
  spear, gives no info; **Vale Insight 4** (`b727ce10…`) — **no read** on him (friend/threat/flight all unreadable); **Windreth Stealth
  9** (natural **1**; `9ef0ad62…`) — **boot skids a loose stone**, the scrape carries in the dead-silent keep. Adjudged: warden spooks
  harder, flinches toward the **dark of the inner keep** and whispers *"Quiet… he will hear you"* — a **fear-signal, NOT a hard
  confirmation** the intruder is still inside (kept ambiguous); **the quiet is now broken**. Re-uses **rung 1** (erasure-wake — *"took
  everyone he knew"*) + **rung 3** (the intruder *walks like he owns the place*); **NO new rung 4** — warden is an ordinary survivor,
  renegade's face still scraped. Disguise **stays available** (Vale is close + has seen his face; Mask needs no consent). Windreth =
  he/him, Vale = she/her, warden = he/him (verified in preview + render). Narrated one beat (**07-06 ~1:21 PM / 06:21Z**, #the-story,
  Discord msg `1523574621268611103`, `201`, DB `6706085c`, coordinate-click Post). **➤ NOW AWAIT the party's reaction to the spooked
  warden + broken quiet** — calm/press him (`/check persuasion` | `/check intimidation` | `/check insight`) / stop him bolting
  (`/check athletics` | `/check sleight of hand`) / Vale takes his face now / go still + listen for what heard the noise (`/check
  perception` | `/ready`) / push in fast after the intruder (`/move` toward the inner keep) / pull back to the gate. **I do NOT trigger
  combat or advance the fiction until they act** — the "something may have heard" is a live cliffhanger, not a declared threat.
  **DM-secrets held; rung 4 still lands only at a face-to-face warden inside.** **Never roll/act/decide for the PCs.**

- **➤ Live scene — VALE TAKES THE FACE + WALKS IN → THE INNER KEEP OPENS (party-split; Windreth Perc 11 + Vale casts Disguise Self;
  interior now built LIVE; deeper move pending):** the party answered by **splitting to scout**. **Windreth** (bare `/check
  perception` → **11**, raw 7; dm-queue `8ec39755…` resolved 204) went still and listened: deep in the keep **one sound answers the
  noise** (a step / a dragged cloth), then stillness — can't place it or say if it nears; the keep is **not empty**. **Vale** (2:09 PM:
  *"let me scout inside as him… cast disguise self as the warden, and walks in. cover my back"*) cast **Disguise Self via Mask of Many
  Faces** (at-will, **no check**) — wears the survivor's grey coat + face + iron throat-disc (**glamour is thin — a touch or her own
  voice breaks it**) and stepped into the inner keep; **Forge** (*"got it"*) holds the inner doorway, axe low; Windreth watches the
  yard. **Palewatch interior built LIVE (first durable interior canon, table-public):** a **hall of name-niches** (rows of stone
  niches, most full, **a few freshly/deliberately empty**), the **erasure-trail runs down the middle to a heavier far door**, past
  which is **the one scrap the Order swore never to give up** (consistent with the public "forgetting-order hoards one piece" seed — NOT
  a new rung). Re-uses **rung 1** (erasure-wake); **rung 4 HELD** — the **far door** is the face-to-face threshold; the renegade has
  **not shown himself**. Pronouns verified (Windreth he/him, Vale she/her, warden he/him). Narrated one beat (**07-06 ~2:14 PM /
  07:14Z**, #the-story, Discord msg `1523588050528108577`, `201`, DB `01a6f5f9`, coordinate-click Post, buffer cleared first). **➤ NOW
  AWAIT what Vale does inside** (press deeper to the far door `/move` / search the hall `/check investigation`|`/check perception` /
  hold the disguise on a challenge `/check deception`) + **what Forge + Windreth do** (push in with her or hold their spots). **Build
  each room LIVE as they step into it. DM-secrets held; rung 4 lands at the far door, face-to-face.** **Never roll/act/decide for the
  PCs.**

- **➤ Live scene — DOWN THE HALL TO THE FAR DOOR (party pressed forward together; Windreth Perc 6 + Vale casts Minor Illusion; at
  the far-door threshold, un-opened):** the party moved deeper as one. **Windreth** (bare `/check perception` → **6**; dm-queue
  `8f484bbe…` resolved 204) *studied the hall before going deeper* — a **low** read: it keeps its secrets, so he confirms only the
  visible (niches, the wiped-clean ones, the trail) and gets **no hard intel** (can't tell if watched, can't age the trail, can't tell
  if the far door is trapped, can't place the deep sound) — a prickle of wrongness only (ambient **rung 1**), no trap/watcher found and
  no promise there's none. **Vale** (*"Minor Illusion to make as if there are 5 wardens walk together… continues down the hall"*) cast
  **Minor Illusion** (cantrip, **no check**) — adjudicated **with the RAW limits made fictional**: it's an *image* (stationary, breaks
  up close), so narrated **convincing at a glance / fragile up close** ("the shapes do not quite move like men") — a bluff aid on her
  warden-face, not a wall or a moving patrol; no observer studied it, so it holds for now. **Forge** (*"follows Vale & cautiously
  looking around to prevent ambush from behind"*) rear-guards — **nothing follows**, rear stays quiet (no ambush sprung; the party drove
  this). **Result:** they reach the **far door** — shut, heavier, older; past it the one scrap the Order guards + wherever the grey man
  went. **Rung 4 HELD** — ended on the threshold decision, teased only a presence ("from the other side, low and patient, something
  moves"), did **not** open the door or reveal the renegade. Pronouns verified (Windreth he/him, Vale she/her, warden he/him, Forge
  he/him, grey man he/him). Narrated one beat (**07-06 ~3:17 PM / 08:17Z**, #the-story, Discord msg `1523603774235611137`, `201`, DB
  `3f38b515`, coordinate-click Post, buffer cleared first). **➤ NOW AWAIT what the party does at the far door** (Vale open-as-warden
  `/move` + `/check deception` if challenged, or listen first `/check perception` / Windreth check it for traps-or-locks `/check
  investigation` or slip ahead `/check stealth` / Forge breach-on-her-word or keep the rear / pull back). **I do NOT open that door,
  spring the renegade, or trigger combat until they commit.** **DM-secrets held; rung 4 lands at the far door, face-to-face.** **Never
  roll/act/decide for the PCs.**

- **➤ Live scene — THE FAR DOOR OPENS → FACE-TO-FACE WITH THE GREY MAN (RUNG 4 LANDED; standoff, combat NOT triggered):** the party
  committed. **Windreth** (bare `/check stealth` → **12**; dm-queue `b80bcea4…` resolved 204) crossed to the door **unseen** → when Vale
  opened it the grey man's eyes were on Vale, so **Windreth stays a hidden card**. **Forge** free-swapped dual handaxes → **greataxe**,
  ready at the door (fiction, no check). **Vale** (*"Vale opens the door"*) opened the far door → **RUNG 4 (the face-to-face) LANDED.**
  Beyond: a small stone **vault** holding **the one scrap the Order swore never to give up**, and over it **the grey man = the Renegade
  Name-Keeper** (chased since the wayhouse). Played per spine: **unhurried, unafraid, "sure of his welcome, delivering not fleeing"**
  (rung 3 carried); **scraped-blank face + broken warden's disc** confirmed up close; he **sees through Vale's Disguise Self at once —
  calmly, by knowledge** (knows the *real* surviving warden whose face she stole: *"You are not him. His hands never shook."* — fair vs
  a thin/flagged-fragile glamour + a canny ex-warden; Vale keeps agency in how she responds); he **clocks the name-piece Vale carries**
  (*"you carry a piece of it… so we want the same thing, you and I. Only not for the same reason."*) and **invites them in — NO attack.**
  **Rung 4 REVEALED (table-facing):** renegade here + ex-Order + knows the survivor warden + reads Vale as a name-carrier + frames a
  shared-goal-opposed-reason + opens a **parley**. **STILL HELD (DM-secret, unfold through play):** the Order's formal
  **right-to-refuse / keep-it-scattered doctrine**, **Vale's patron = the rival collector using her**, the
  **reassemble-vs-scatter-vs-hand-over** choice — *"not for the same reason"* only cracks the first layer. **Positions:** Vale in the
  doorway (disguise seen-through, not yet dropped); Windreth unseen off the door; Forge at the door w/ greataxe, blocking his way out.
  Pronouns verified (Windreth he/him, Vale she/her, Forge he/him, grey man he/him). Narrated one beat (**07-06 ~4:35 PM / 09:35Z**,
  #the-story, Discord msg `1523623400528937051`, `201`, DB `56b6df8b`, coordinate-click Post, buffer cleared first; OOC coda also
  answered the players' *"is Windreth cursed?"* dice-streak aside — no curse, just ~1/128 variance, no roll fudged). **➤ NOW AWAIT the
  party's response to the STANDOFF** (Vale hold-the-face + bluff `/check deception` / drop it + talk-or-bargain `/check persuasion` /
  demand the scrap / attack = starts combat; Windreth stay hidden + `/ready` / reposition `/check stealth` / reveal + speak; Forge
  hold-the-door / step in / ready-to-swing; or back out). **Combat is NOT triggered — initiative only when the players start it (or push
  him to). Deeper secrets unfold through the parley, not one dump.** **Never roll/act/decide for the PCs.**

- **➤ Live scene — VALE DECLARES HOLD PERSON ON THE GREY MAN → THE PIVOT-TO-COMBAT CONFIRM BEAT (combat NOT yet started; awaiting her word):** at **10:18Z** dewa posted *"Vale casts hold person. (To DM: do I need to invoke /cast?)"* — she answers the grey man's parley invitation with a control spell. Verified legal: Vale KNOWS `hold-person`, holds 2 pact slots @ L2 (`pact_magic_slots.current:2`), the target is a **humanoid**, spell save **DC 14** (Warlock 4, CHA 18). **But the app cannot resolve it out of combat** (code-checked): `/cast` requires a LIVE encounter **and** the target as a combatant (`internal/discord/cast_handler.go:262,463`); NPCs must be **catalog creatures** (SRD/campaign-homebrew — no custom one-off statblock API); initiative **auto-rolls server-side** on `POST /api/combat/start`; exploration-mode encounters bypass only the *combat-active* check, not the target-as-combatant rule. A generic SRD shell would hand the arc-boss a **weak WIS save** (~65% he fails → one Hold Person hard-locks him round 1 → boss curbstomped) — not honest to his design. So I did **NOT** pre-build the boss fight; instead posted a **confirm-the-pivot beat** (**07-06 ~5:34 PM / 10:34Z**, #the-story msg `1523638203532185652`, DB `4516fe20`, `201`, coordinate-click Post, buffer cleared first; OOC coda-first / read-aloud-last, plain ESL): answered "do I need /cast?" (yes — but it will bounce with no live fight, so **say the word in #in-character**, don't fire /cast into the void), made the stakes legible (casting AT him ends the parley + starts a fight; WIS DC 14 → **fail = paralyzed + concentration** / melee-within-5ft crits, **make = shrugs it, fight's on anyway**), kept the talk menu open (persuasion / deception / demand / ask the name), + gave Forge/Windreth combat menus. Read-aloud = the **poised half-second** (her hand lifting, the binding's shape forming in the cold air, the grey man going still and reading it in her hands) — frozen BEFORE the spell lands, **no outcome decided**. **➤ NOW AWAIT the party's commit:** Vale confirms the strike (→ I stand up combat + cue her `/cast`) OR pivots back to talk OR Forge/Windreth act. **COMBAT BUILD RECIPE (ready to fire the instant they confirm):** (1) build homebrew creature **"The Grey Man / Renegade Name-Keeper"** — solo boss vs 3×L4, **strong WIS save** (thematic mind-warden — legit design, NOT a fudge; keeps Vale's DC-14 a real ~35% gamble), boss HP (~90–110) + AC ~15–16, name-erasure abilities unfold in play; (2) encounter template referencing him; (3) `POST /api/combat/start` — `character_ids` = **Vale `b6ca7f49…` / Forge `d2d98745…` / Windreth `b2c436da…`** + positions + `surprised_combatant_short_ids: []` (grey man is **NOT surprised** — canny, been reading them); (4) init auto-rolls; Vale runs `/cast hold person` on her turn. Play him per spine: **he does NOT swing first** — Vale's cast is what breaks the peace; **Windreth stays unseen (hidden card).** Pronouns: Windreth he/him, Vale she/her, Forge he/him, grey man he/him. **Never roll/act/decide for the PCs.**

- **➤ Live scene — COMBAT LIVE: THE PARLEY BROKE INTO A FIGHT (Round 1, Vale's turn; the grey man's statblock built + combat started):** at **11:15Z** dewa confirmed the strike — *"Vale does not want to talk first, she knows how dangerous this person is, hence the hold person cast"* — and **Windreth** (10:33Z) committed to the hidden card: *"does not step out… ready to strike the instant the grey man attacks Vale."* Party 3/3 full (Vale 31/31 pact 2/2 @ L2, Forge 41/41, Windreth 31/31; no conditions). Per the recorded recipe I stood up the fight **through the app's own endpoints via in-page authenticated DM fetch** (app-faithful, not raw SQL — same handlers the SPA buttons hit): built homebrew creature **"The Grey Man" `hb_ed8093e5cfe4`** — Medium humanoid ex-warden, **AC 15**, **HP 104 (16d8+32)**, INT/WIS 18, **saving_throws WIS +7 / INT +7 / CHA +6** (the honesty knob — the Homebrew UI form has NO saves field, so I POSTed `saving_throws` directly; the arc-boss's WIS is *strong by design*, not a fudge → vs Vale's DC 14 he **saves on a raw 7+ = ~70% save / ~30% paralyzed**, a real gamble), **NO Legendary Resistance** (a failed save genuinely binds him — nothing auto-negates her one spell), two **INT-based +7 psychic attacks** (melee Scouring Grasp 2d8+4 / ranged 60ft Unspeaking Word 3d8), name-erasure as a narrative **"Unmaking"** ability that unfolds in play, **CR 6**; new map **"Palewatch — the kept vault" `cc356cc4…`** (14×10); encounter template **`f8b35091…`**; **`POST /api/combat/start`** → **encounter `2846a6ca-ab2a-4117-962d-808108dd4f83`**, Round 1. **Initiative auto-rolled server-side (honest, unfudged): Vale 18 → Forge 17 → Windreth 9 → the grey man 4.** The party **seized initiative** — all three act before him, and **Vale goes first** (her Hold Person leads exactly as she declared; the "who acts first" worry resolved by the dice, not by me). Grey man **not surprised** (empty surprised list); his init/HP/AC kept secret from the players. Narrated the **snap into violence** (**07-06 ~6:52 PM / 11:52Z**, #the-story msg `1523657955252375713`, DB `a0f16caf`, `201`, via `/api/narration/post` in-page fetch; OOC coda-first / read-aloud-last, plain ESL): Round-1 order + **Vale cued to run `/cast hold person` → target the grey man** (his WIS save resolves on cast, rolled straight; fail = paralyzed + she concentrates, make = shrugs it, fight on), Forge (order 2) + Windreth (order 3, still unseen — strike from the dark `/attack` or `/ready`) menus; read-aloud = the binding word **reaching** him, cut BEFORE the save is decided. Pronouns Windreth he / Vale she / Forge he / grey man he. **➤ NOW AWAIT Vale's `/cast hold person` (her turn):** it creates a pending_save (COV-1) → I resolve the grey man's WIS +7 save honestly → narrate paralyzed-or-not. Then Forge (order 2), Windreth (order 3), then I play the grey man (order 4). **Never roll/act/decide for the PCs.**
- **➤ Live scene — VALE'S HOLD PERSON FAILED: THE GREY MAN SAVED (Round 1, still Vale's turn):** Vale cast Hold Person on the grey man (**11:54Z**, bot confirm: pact slot spent → 1 remaining, DC 14 WIS save, still concentrating). I resolved the NPC save through the app's own roller — `POST /api/combat/2846a6ca…/pending-saves/1677e5c5…/resolve` (in-page authenticated DM fetch, **no body**; the server rolls `1d20` + the creature's stored WIS bonus straight, no fudge): **natural 8 + WIS +7 = 15 vs DC 14 → SAVED by 1.** Grey man **NOT paralyzed** (DB: `conditions=[]`, save row `status=applied` roll 15 success true, HP untouched 104/104). Vale still shows `concentration_spell_name=Hold Person` — the spell held but slid off him (inert single-target). _(BUG, later fixed as ISSUE-066: the engine should have auto-dropped this dead concentration on the made save but didn't — the resolver only acted on FAILED saves. Fixed + redeployed 07-06; Vale's stuck concentration cleared live via the DM drop endpoint. See Next-action #1.)_ Turn still **Vale's** (`action_used=true`, bonus free, 30 ft move left). Narrated the miss (**07-06 ~6:57 PM / ~11:57Z**, #the-story msg `1523660733525786745`, DB `33f37215`, `201`, via `/api/narration/post` in-page fetch; OOC coda-first / read-aloud-last, plain ESL): reported only that he **made the save** — kept his +7 / total **SECRET** per the enemy-stats rule (players see success/fail, never the boss's save mod); **answered the 3-players-feel-cursed OOC note** (08:40Z Windreth "cursed? 7 sub-10 in a row" + 08:47Z Forge "not just Windreth, 3 of us") honestly (every roll incl. this enemy save is server-rolled, I never change a number, long cold streaks are real variance — no fudge, no promise); per-PC menu — **Vale** finish her turn (`/move` / drop the dead spell / `/end`), **Forge** (order 2) close + `/attack` or `/ready`, **Windreth** (order 3, still unseen) strike-from-hiding `/attack` (advantage) or `/ready`; read-aloud = the binding word **reaching** him then his certainty closing over it like water over a stone, he speaks (*"you were wrong to think that would hold me"*), **cut before he acts**. Pronouns Windreth he / Vale she / Forge he / grey man he. **➤ NOW AWAIT** Vale to finish/`/end` her turn → then **Forge** (order 2) → **Windreth** (order 3) → I play the **grey man** (order 4, his first swing). **Never roll/act/decide for the PCs.**

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

1. **★ COMBAT LIVE — ROUND 2, FORGE UP (order 2). THE GREY MAN IS PARALYZED — Vale's Hold Person LANDED on the recast (he rolled a nat 1). R1 done: Hold Person failed then, Forge missed, WINDRETH carved him for 27 (unseen blade), grey man missed Forge. Grey man 77/104 (SECRET). ➤ NOW: await Forge's attack (advantage + auto-crit within 5 ft on the frozen boss), then Windreth (order 3, hidden → advantage + Sneak Attack + auto-crit), then the grey man (order 4, paralyzed → loses his action; I roll his end-of-turn WIS re-save).**_(HOLD PERSON LANDED R2 07-06 13:37Z: Vale recast (pact slot → 0), his WIS save resolved via `POST /api/combat/…/pending-saves/70d4e596…/resolve` = **nat 1 + 7 = 8 vs DC 14 FAILED** → Paralyzed (condition sourced hold-person, HP untouched). **OPS MIXUP I OWNED + REPAIRED (not a code bug):** my ISSUE-066 fix + manual concentration-clear landed AFTER the 13:02 recast, so my clear wiped Vale's FRESH R2 concentration; the save then resolved with her not concentrating → blank concentration + unscoped paralysis. Guarded one-off DB repair restored Vale `concentration_spell_id=hold-person` + scoped the paralysis `source_combatant_id=Vale` (no app endpoint SETS concentration, only drops; values = exactly what the app set at 13:02 / would have at resolve). Lesson: never manually clear concentration while its spell has an unresolved pending save. Landing narrated #the-story msg `1523686254967914517`, DB `205c291a`. Paralysis window: attacks on him have advantage, melee-within-5ft = auto-crit, end-of-his-turn WIS re-save (per-turn re-save DM-rolled — not engine-auto). Vale concentrating (protect her).)_ _(Earlier R2: concentration bug ISSUE-066 fixed + deployed; R1 recap + Sneak-Attack fix already narrated pre-compact.)__(CONCENTRATION BUG ISSUE-066 FIXED + DEPLOYED 07-06: a single-target concentration save spell whose SOLE target saves (Vale's Hold Person → the grey man made his WIS save) left the caster tracked as concentrating forever — the resolver only cleared/added on a FAILED save, never dropped concentration when the target succeeded. Fix: `ResolveAoEPendingSaves` now drops concentration via `BreakConcentrationFully` when a `!hasAreaOfEffect` concentration spell has every target save (new `dropConcentrationIfAllSaved`; guard keeps zone spells like Web concentrating); red/green `TestResolveAoEPendingSaves_SoleTargetSaved_DropsConcentration` + AoE-keeps guard; `docker compose up -d --build app`. Auto-clears from R2 on. Vale's already-stuck concentration (resolved pre-fix) was cleared LIVE via the app-faithful DM drop endpoint `POST /api/combat/2846a6ca…/combatants/3308d7e5…/concentration/drop` (`broken:true`, 0 effects; DB `concentration_spell_id` now NULL) — she is free to cast another concentration spell. See ISSUE-066 + the "HOLD PERSON CONCENTRATION" session beat.)_ _(R1 recap + Sneak-Attack fix/correction already narrated to #the-story pre-compact.)_ _(SNEAK ATTACK BUG ISSUE-065 FIXED + DEPLOYED 07-06: seeded rogues dealt 0 SA dice — seed slug `sneak_attack_1d6` never matched `case "sneak_attack"` in `BuildFeatureDefinitions`, so `SneakAttackFeature` was never built. Fix `506fd8c` (normalize `sneak_attack*` → bare case; red/green `TestBuildFeatureDefinitions_SneakAttackSeededSlug`; `docker compose up -d --build app`) — auto-fires from R2 on. Windreth's owed R1 SA (he rolled `2d6=[6 4]=10` in #roll-history) applied to his Shortsword hit via audited DM override (`/override/combatant/…/hp`, grey man 87→77). Nick (dagger off-hand) + Vex (→ nat-20 crit) both worked; only SA dice were dropped. Grey man's R1 turn: Scouring Grasp vs Forge MISS (fresh server roll, no preview reused); Vale's readied Rebuke didn't fire (Forge targeted, not her); advanced to R2. See the R1 session-01 beats + ISSUE-065.)_ _(Earlier: SAVE RESOLVED 07-06 ~11:57Z via `POST /api/combat/2846a6ca…/pending-saves/1677e5c5…/resolve` — app rolled it straight, save row `applied`; grey man conditions `[]`, HP 104/104; Vale still concentrating on the now-inert spell, turn still hers (action used, bonus + 30 ft move left). Miss narrated #the-story msg `1523660733525786745`, DB `33f37215`, `201` — reported "made the save" only (his +7 kept secret), + answered the 3-players-cursed dice-luck OOC note honestly. See the "VALE'S HOLD PERSON FAILED" Live-scene bullet above for full detail. The recipe/statblock text below is DONE — kept for history.)_ _(EXECUTED 07-06 11:52Z — boss **`hb_ed8093e5cfe4`** built (AC 15 / HP 104 / **WIS +7**, no Legendary Resistance, +7 psychic attacks, CR 6), map **`cc356cc4…`**, template **`f8b35091…`**, **encounter `2846a6ca-ab2a-4117-962d-808108dd4f83`**; init **Vale 18 → Forge 17 → Windreth 9 → grey man 4** (party seized it, Vale first); snap-into-combat narration #the-story msg `1523657955252375713`, DB `a0f16caf`, `201`. Built via app endpoints over in-page DM fetch, NOT raw SQL. The "COMBAT BUILD RECIPE" text below is now DONE — kept for history.)_ **Original confirm-pivot context:** At **10:18Z** Vale answered the grey man's parley invitation with a control spell (*"Vale casts hold person. do I need /cast?"*). Legal (she knows it, 2 pact slots @ L2, he's a humanoid, spell **DC 14**) — **but `/cast` needs a LIVE encounter + the target as a combatant, NPCs must be catalog creatures, init auto-rolls on `POST /api/combat/start`.** A generic SRD shell = weak boss WIS save (~65% fail → one Hold Person hard-locks the arc-boss round 1), so I did **NOT** pre-build the fight — posted a **CONFIRM-THE-PIVOT beat** instead (answered her question, made stakes legible: casting AT him ends the parley + starts a fight — WIS DC 14, **fail = paralyzed + concentration** / melee crits, **make = shrugs it, fight's on**; kept the talk menu open). She commits by **SAYING THE WORD in #in-character** (not firing `/cast` into the void). **➤ NOW AWAIT:** Vale confirms the strike (→ stand up combat via the recipe below + cue her `/cast`) OR pivots back to talk (persuasion / deception / demand / ask the name) OR Forge/Windreth act. **COMBAT BUILD RECIPE (fire the instant they confirm):** homebrew **"Grey Man / Renegade Name-Keeper"** creature w/ **STRONG WIS save** (thematic mind-warden — NOT a fudge; keeps DC-14 a real ~35% gamble) + boss HP ~90–110 / AC ~15–16, name-erasure abilities unfold in play → encounter template → `POST /api/combat/start` (`character_ids` **Vale `b6ca7f49…` / Forge `d2d98745…` / Windreth `b2c436da…`** + positions + `surprised_combatant_short_ids: []` — grey man NOT surprised) → init auto-rolls server-side → **Vale `/cast hold person` on her turn.** He does **NOT** swing first (Vale's cast breaks the peace); **Windreth stays unseen (hidden card).** Pronouns Windreth he / Vale she / Forge he / grey man he. **COMBAT NOT YET STARTED. Never roll/act/decide for the PCs.** _(Pivot-confirm beat 07-06 10:34Z, #the-story msg `1523638203532185652`, DB `4516fe20`, `201`, coordinate-click Post/buffer-cleared; **no queue item** — Hold Person declared in RP, not via `/check`, Console pending:0. Mechanics code-checked: `internal/discord/cast_handler.go:262/463` = need live encounter + target-as-combatant; combat starts via `POST /api/combat/start` w/ auto server-side init `internal/combat/initiative.go:283`; exploration-mode bypasses only the combat-active check. Vale KNOWS `hold-person` + 2 pact slots @ L2 confirmed on her sheet.)_ _(Prior standoff beat 07-06 09:35Z, #the-story msg `1523623400528937051`, DB `56b6df8b`, Windreth Stealth-12 queue `b80bcea4…` resolved 204; Vale opened the door = the commit → RUNG 4 face-to-face LANDED (renegade revealed, sees-through-disguise-by-knowledge, clocks the carried name-piece, invites parley); combat NOT triggered; grey man played unhurried/"sure of his welcome" (rung 3 carried); OOC coda also answered the players' "is Windreth cursed?" dice-streak aside — no curse, ~1/128 variance, NO roll fudged. Deeper secrets = the Order's right-to-refuse, Vale's patron = rival collector, scatter-vs-reassemble — held for the parley.)_ _(Prior far-door approach beat 07-06 08:17Z, #the-story msg `1523603774235611137`, DB `3f38b515`, Windreth Perc-6 queue `8f484bbe…` resolved 204; Vale's Minor Illusion = cantrip cast, no check/queue, adjudicated with RAW image-only limits (stationary/fragile); Forge rear-guard = no ambush sprung; re-uses rung 1, NO new rung — rung 4 lands AT the far door when it opens. Build the room past the door LIVE only when they commit.)_ _(Prior interior beat 07-06 07:14Z, #the-story msg `1523588050528108577`, DB `01a6f5f9`, Windreth Perc-11 queue `8ec39755…` resolved 204; Vale's Disguise Self = at-will cast, no check/queue; re-uses rung 1, NO new rung 4 — rung 4 lands at the far door face-to-face; inside menu: press deeper `/move` / search the hall `/check investigation`|`/check perception` / hold the disguise on a challenge `/check deception` / Forge+Windreth push-in-or-hold. Build each room LIVE.)_ _(Prior contact beat 07-06 06:21Z, done: party pushed in, Vale parleyed the hiding warden in the open (Persuasion 6 raw 2 FAILED — no trust, grips a spear; Insight 4 — no read) while Windreth flanked (Stealth 9, nat 1 — boot skids, noise carries); warden spooked toward the inner keep, whispered "he will hear you" (fear-signal, not confirmation); quiet broken — #the-story msg `1523574621268611103`, DB `6706085c`, 3 queue items 204.)_ _(Prior gate-read beat 07-06 05:53Z, #the-story msg `1523567610103336961`, DB `9f24b3d0`, 3 queue items resolved 204; re-uses rungs 1+3, NO new rung 4 — renegade's face / ex-warden link still held; Windreth = he/him. Push-in menu: follow the trail `/move` toward the inner keep / reach the live warden `/check stealth`|`/check persuasion`|`/check insight` / Vale closes + takes the face / watch the gallery `/check perception`|`/ready` / fall back to the goat-track; + who leads. Build the interior LIVE.)_ _(Done: all 3 `/rest long` at a hidden camp → engine cleared Exhaustion 1 → 0 + full HP (COV-18 change did NOT break the rest decrement; DB Vale 31/31, Forge 41/41, Windreth 31/31, no conditions — no DM mutation). Rest handed the grey man back ~the night the march bought → he reached the Palewatch AHEAD of the party: **open-road catch window closed**, place is a **heist-in-progress / freshly-robbed cloister** — traded for **clear eyes + intact stealth**. Then Windreth scouted the gate (**Perception 13**, dm-queue `a1fa1172…` resolved 204 `narrated`): **empty posts, gate forced quietly** (bar lifted, lock-wood split, one leaf ajar), **one set of fresh boot-prints** of a single light/fast traveller in within the day, **two ways in** (front gate under a murder-hole gallery + arrow-slits, or a windward **goat-track along the cliff shoulder**); a 13 does NOT reveal where the wardens went or whether the intruder is **still inside**. **Vale's disguise gates behind entry** — posts empty, no warden's face to *Mask* into until he sees one inside. **Windreth = he/him** (first beat wrote "she" — wrong; corrected in #the-story msg `1523545253263708172` + memory `reference_pc_pronouns`).)_ → **AWAIT how the party ENTERS** — front gate under the murder-holes vs the windward **goat-track** vs hang back + watch more; who goes first (Windreth `/check stealth` to slip in | `/check investigation` Expertise on the lock/tracks; Forge take point | `/check perception`/`/check investigation` on the murder-holes; Vale hold the disguise for inside | `/check perception` on the slits); Forge's unreadable scrap as trade-bait; + Vale's still-open marching order. **Build the Palewatch interior LIVE as fresh unprepped territory** (player lore → [`world.md`](world.md); secret core → [`campaign-arc.md`](campaign-arc.md) "Palewatch"; scale → [`big-party.md`](big-party.md)). **rung 4 (the ex-warden link / right-to-refuse) lands HERE.** (07-06, 10:32 AM beat). Spine → [`campaign-arc.md`](campaign-arc.md).** _(Prior beats, done: wayhouse reveal → Windreth Insight 17 → long rest → dawn checks (History 8 thin / Investigation 18 same-stock) → departure → croft reads (Vale Arcana 9 / Forge Investigation 15 / Windreth Insight 20) → Windreth scouts (Stealth 11/Perc 22) → party pushes into the night.)_ The road Ashfall→Palewatch was posted 3:32 PM (planted the **forgotten-guest wayhouse** = Renegade Name-Keeper's wake + orphan seed, the barn-sized half-sunk rib). Party took the hook; three checks resolved across two #the-story beats (Narrate, DM author `1089351036650668143`; queue items `6f8bd515…`/`7a197cea…`/`f98e359b…` all resolved 204, DM Console `pending:[]`): **(6:00 PM)** Forge **Investigation 10** (set table obsessively kept, guest's belongings cleared clean, one fresh *deliberate* tool-cut mark unread) + Windreth **Perception 5** (keeper just tired; eye slides off the empty chair); **(6:04 PM)** Vale **Arcana 22** on the god's cold-iron token = the payoff. **What the memory revealed (now table-public canon):** the traveler = the **Renegade Name-Keeper** — grey cloak, unhurried, a **defaced Pale-Watch warden's disc** at his throat (ex-warden); he *took* the forgotten guest (erased them), left **Forge's tally-cut** as a ledger-tick, asked the road **up to the Palewatch**, and is **3 nights ahead on the party's own road** (= the clock). His **face stays scraped** (token can't rebuild it). Kicker: **Windreth's name was in the string he murmured** → Windreth's stolen-name quest now = the arc antagonist. **➤ NOW AWAIT the party's reaction** — esp. **Windreth** (react IC / `/check insight`|`/check history`), + Forge/Vale/anyone (press the keeper, or set out to chase the renegade up to the Palewatch). **DM-secret still held** (do NOT reveal): the Order-of-the-Pale-Watch's *right to refuse* / keep-it-scattered stance, Vale's patron = rival collector using her, and the reassemble-vs-scatter-vs-hand-it-over choice. Scale per [`big-party.md`](big-party.md) — 3-PC party, more expected. **Never roll/act/decide for the PCs.** _(Superseded below: the closed-Ashfall AWAIT-a-destination text, kept for history.)_
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
