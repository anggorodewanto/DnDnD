# Game State — save file (durable IDs + DM intent)

> **This file holds only what the DB can't derive:** durable IDs, the ops snapshot,
> the current *scene* framing, and the **Next action** (DM intent). It deliberately
> does **NOT** track round / turn / HP / AC / positions / conditions / slots / the
> queue — those are *generated* and served live, aggregated, by the **DM Console**
> (`GET /api/dm/situation` / the `#dm-console` tab). Hand-copying mechanical state
> here is what drifts (it bit this folder repeatedly — see [`sessions/`](sessions/)),
> so we stopped. **Pull live state from the Console; record only intent + IDs + scene
> here.** Per-PC durable kit is in [`party/`](party/); the full play-by-play is in
> [`sessions/session-01.md`](sessions/session-01.md). See [`dm-rules.md`](dm-rules.md)
> "Keep the record straight."

_Last updated: 2026-07-22 — **⚔ COMBAT LIVE, "The Far-Quarter Front — the breach" (enc
`a46b1472-72c6-49aa-a36c-2343bd613299`), Round 2. **Progress:** Forge raged + killed the Door-Minder (Alarm-Whistle
soft-clock neutralized before it sounded), moved B5→**E5** (10 ft short of the muscle, dodging a pre-turn swing), ended
turn. **ENF2 (Front Muscle) turn done** — moved G6→**F6** (adjacent Forge), Cutter HIT 5 slashing **auto-halved to 2 by
Rage** (Forge **48/50**), 2nd Cutter missed. **Windreth's turn done** — moved A4→**B2** (far wall), **Shortbow → ENF2
(the muscle on Forge), Sneak Attack HIT for 23** (ENF2 32→**9**, reeling = 1 hit from down), ended unseen. **ENF1
(Front Muscle, init 8) turn done** — moved G4→**F4** (now flanks Forge alongside ENF2), **2× Cutter both HIT but Rage
halved each to 4** → Forge 48→**40/50**. **Vale cast Shatter** (pact-slot L3 = 4d8, rolled **16 thunder**; DC 15 CON)
centered to catch **Ledger + ENF2** — **BOTH SAVED → half (8) each**: BOSS 52→**44** (first real damage on the boss),
ENF2 9→**1 HP (barely alive — save + Windreth's shot both landed, still up)**. Resolved via `GET/POST
/api/combat/{enc}/pending-saves[/{id}/resolve]` (last save's resolve applies the whole AoE; rows → status `applied`).
**Vale ended turn** (bonus/move unused). **── ROUND 2 (in progress) ──** Ledger: moved **F5**, Weighted Cane **MISS**
(11), bonus **Bark an Order** → DM-ran ENF1 free Cutter = **nat 1 miss**. **Forge's turn done** — 2× Greataxe at the
reeling ENF2, **BOTH MISSED** (whiffed the free kill; took ZERO from Ledger's round). **ENF2's turn done** — 2× Cutter
at Forge, one miss + one hit **rage-halved to 2** → Forge 40→38. **Windreth's turn done** — Cunning **Hide (Stealth 25)**
→ moved B2→**E6** (into melee) → **Silent Blade → Ledger, advantage, HIT 24, Sneak fired but rolled ROCK-BOTTOM = 11
total** (BOSS 44→**33**, first wound since Shatter) → off-hand **Dagger finished the reeling ENF2** (1 dmg → **ENF2
DEAD**). ⚙ Sneak went on the Silent-Blade/Ledger hit (labeled there, low roll); the dagger was plain 1d4 — engine
correct, no robbery. **ENF1's turn done** — 2× Cutter at Forge: miss + hit **rage-halved to 3** → Forge 38→**35/50**.
**Vale's turn done** — Eldritch Blast at Ledger: beam1 HIT **13 force** (incl +4 Agonizing), beam2 miss → BOSS 33→**20**.
**── ROUND 3 ──** **Ledger's turn done** — rounded on **Windreth**: Weighted Cane HIT **12 bludg**, but Windreth's
standing **Uncanny Dodge halved it → 6** (WI 38→**32**; reaction SPENT this round). ⚠ **Live `/enemy-turn` POST does NOT
auto-apply a defender's standing Uncanny Dodge — engine logged FULL 12; I corrected via HP-override** (COV-16 auto-halve
fires only on the Turn-Builder path, not this raw endpoint). Bonus **Bark an Order** → DM-ran last enforcer (ENF1) free
Cutter at Forge: 16 vs AC16 HIT, 8 slashing **rage-halved to 4** → Forge 35→**31**. **➤ NOW = FORGE'S TURN** (R3, init
20, at **E5**, 31/50 raging; PC — awaiting `/attack`; **Ledger F5 + ENF1 F4 both in reach**, Cleave/GWM live, Ledger
bloodied). **After Forge → ENF1 (8) → Windreth (8) → Vale (3).** Board: **Ledger F5 (20 — bloodied)**, ENF1 **F4** (32),
Forge **E5** (31/50 raging), Windreth **E6** (32), Vale **A6** (36, 1 pact slot). **Enemies left: Ledger + 1 enforcer**
(ENF2 + Door-Minder dead). ⚙ **Bark-an-Order = "(DM-run)"** — engine only LOGS it; roll+rage-halve+HP-override yourself.
⚠ **Uncanny Dodge NOT auto-applied on `/enemy-turn` — halve manually via HP-override.**
Full detail ↓.** (Prior beat: SESH, the unlit rows — "The Collector's Round" (enc `96f9a6bb`) ENDED R1 via
CAPTURE (collector ALIVE, bound + gagged, no PC hurt). INTERROGATION RESOLVED: Forge Intimidation 14 (incl. Vale
Guidance) broke him → gave up the FAR-QUARTER FRONT (chandlery/consignment, market back row, white-taper sign),
tonight's heavy drop, + the walk-in-clean method; Windreth Persuasion NAT 1 → only a STALE countersign (may flag
them). Buyer SEALED (cutout, never saw a face). ➤ NOW = party's move: hit the far-quarter front tonight, work the
captive more (`/check insight` on the countersign), or deal with the prisoner. Seals intact.** "The Unlit Rows" walls+fog fight (encounter `6fffbb99`) is **WON** — all 4 enemies
(3× Blank-Faced Enforcer + Hollow Stalker) dead over 7 rounds, no PC dropped; ended via
`POST /api/combat/{enc}/end` (HP auto-carried to sheets). Party **milestone-leveled L4→L5** and
short-rested (Vale 36/38, Windreth 38/38, pact slots 2/2). THE SEAL already read (Windreth's own name,
07-19); Sabinnet released. Full L5 bookkeeping + engine fixes done + committed this arc — see the
memory notes and [[project_encounter_unlit_rows_staged]].)_
**➤ (historical, now superseded) COMBAT record — "The Collector's Round":** party chose **(a) jump him with surprise** (dewa:
"we jump the person, using surprise because of Windreth's stealth"). Built + started a fresh 3v1 ambush combat
(all via in-page DM fetch): homebrew **The Collector** `hb_edcad54fcfb0` (CR1 courier **cutout** — AC 14, HP 36,
weak Concealed Blade +4/1d4+2, traits *Cutout* [reasoned-with/frightened/interrogated, but knows only his own
link — can't name the buyer] + *Bolt Instinct* [Disengage+Dash to flee] + bonus-action *Signal Whistle 1/combat*
[if it sounds → runners + Pale-Watch converge = soft clock]); template `278649f4`; **live encounter
`96f9a6bb-5395-458a-a75d-7332a1b37900`** on the reused Unlit-Rows map `f2b2f184`. **Round 1 order: Windreth 18 →
Forge 9 → Collector 7 (SURPRISED, skips) → Vale 4.** Positions: Windreth **G5** (adjacent to Collector **H5** = melee
Sneak Attack, no move), Forge **H7**, Vale **G7**. Staged `/initiative` auto-pulled (W18/F9/V4 from
`pending_initiatives`); `surprised_combatant_short_ids:["COL"]` set the Surprised condition (dur 1). Engine
auto-posted #your-turn (Windreth up) + the board. **Windreth attacked (07-22):** Silent Blade (main) to-hit **13 —
MISS**; Dagger (off-hand, **Nick** = free extra attack) **18 — HIT → 1 dmg** (Collector 36→35); then Cunning Action
**Hide → Stealth 27**. ⚠ **ENGINE GAP:** the opener resolved **FLAT — no advantage, no Sneak Attack** — because I
flagged COL *surprised* but never flagged **Windreth Hidden at combat start**, so the engine didn't treat him as an
unseen attacker. Player caught it ("attack should have advantage, can we undo?") + declared **non-lethal intent**.
**RULING posted OOC (`bf7c0e12`):** no clean turn-rewind (a reseat hands a fresh full turn = over-credit), so
**credit the dropped advantage instead** — Dagger hit stands + carries **Sneak Attack (Windreth rolls 3d6)**; Silent
Blade gets its **owed 2nd advantage die (Windreth rolls 1d20, take higher vs orig 13; +1d6+4 if it now lands)**.
**RESOLVED 07-22:** Windreth rolled Sneak **3d6=10** + Silent-Blade owed advantage die **19** (→ total 26, HIT) +
**1d6+4=10**. Applied via HP-override (reason logged): dagger 1 + Sneak 10 + Silent Blade 10 = **21 dmg → Collector
36→15/36** (non-lethal noted — survives). Narrated the strike (read-aloud `8cca4e9a`). **Advanced to Forge (init 9)
— HIS TURN LIVE** (banner fired 6:44; engine features confirmed correct: **35ft** [dwarf 25 + Fast Movement 10] /
**2 attacks** [Extra Attack]). Forge at **H7**, COL staggered at **H5** (10 ft). **FORGE DONE (07-22):** moved H7→**H6**, **grappled** COL
(Athletics 22 vs Acro 10 → *grappled*, speed 0 — can't Bolt-flee), Handaxe **NAT 20 crit 8** + Greataxe (Vex adv)
**5** → **COL 15→2/36, pinned, no Rage spent**. Engine auto-skipped **COL (surprised)**. Narrated (read-aloud
`6ccaac6c`). **COMBAT ENDED 07-22** (`POST /api/combat/{enc}/end` → status *completed*): on dewa's ask ("captured + bloodied —
good enough to end combat?") the party chose capture over the kill. Ruled **YES** — fight over, **collector taken
ALIVE, bound + gagged, whistle cord cut** (this neutralizes the round-2 Signal Whistle soft-clock as the price of
ending decisively while he was pinned); **zero PC damage** (W 38 / F 50 / V 36 carry back clean). Narrated the
capture (read-aloud `36399c8d`). **➤ INTERROGATION RESOLVED 07-22** (both `/check` items resolved via DM Queue → Send Narration; read-aloud + OOC
brief posted to #the-story). **Forge Intimidation 14** (`d20(8) +2 + Vale Guidance 1d4(4)`) = SUCCESS → cutout
broke, gave up his own link: the **far-quarter market front** = a chandlery/consignment house, market back row,
**white taper in the window = the sign**; nightly drops consolidate there; **tonight a heavy crate** ("a season in
one box") lands **late, after the market bell** (= watched); **walk-in-clean** = side alley (not shopfront), empty
hands / no drawn steel, **countersign given + answered** to the door-minder (wrong word → goes loud). **Windreth
Persuasion NAT 1 (total 3)** = BOTCH → the countersign he got is **STALE** (a cycle+ old; may work, may FLAG them —
telegraphed, NOT sprung). **Buyer stays SEALED** (collector genuinely never saw a face; next link up = the front's
**floor boss / handler**, still a cutout). Soft clock foreshadowed (a courier who never checks in tonight = a gap
noticed up-chain; not sprung). **PARTY CHOSE 07-22** (dewa): haul the captive + hit the front tonight, use him as the way in, force it if he
can't. **➤ NOW = AT THE FAR-QUARTER FRONT** (out of combat, side-alley mouth): white taper lit, a **door-minder** on
a stool + a **second watcher** working the street (place watched — heavy drop). Narrated the approach + arrival
(read-aloud) + an OOC door brief. **Frictions surfaced:** (a) collector BALKS at walking them in (snitch = dead;
needs Intimidation to force / Insight to gauge betrayal), (b) countersign STALE, (c) forcing = fight vs
minder + guards + soft clock. **PARTY FORCED IT 07-22** (dewa: "Force it, let's roll") → ⚔ **COMBAT BUILDING — "The Far-Quarter Front — the
breach".** Built via in-page DM fetch: reused open **14×10 map `cc356cc4`** ("Palewatch — kept vault", **zero walls**)
re-skinned as the chandlery back store-room; **2 NEW homebrew creatures** — **Front Lookout** `hb_75fb59bbd9ec`
(CR1/2, AC13/HP16, bonus-action **Alarm Whistle 1/combat = the soft-clock fuse**) + **Ledger, the Floor Boss**
`hb_398dfa50bbee` (CR3, AC15/HP52, Weighted Cane 2d6+2, **Bark-an-Order** BA; traits *Cutout(sealed)* = capture =
one link up but STILL can't name the buyer, + *Clean Exit* = flees at 0 HP unless his exit is cut off); reused
**Blank-Faced Enforcer** `hb_d63836d7fe14` ×2 for the crate-muscle. **Encounter template
`4cf1f0e1-6827-47d1-a755-d4d0b6aa300b`** (4 enemies). Placements (14 wide, A=0..N=13; breach LEFT, boss DEEP RIGHT):
PCs **Forge B5 / Windreth A4 / Vale A6**; enemies **LKT D4** (door-minder), **ENF1 G4**, **ENF2 G6**, **BOSS L5**
(rises from chair). The **second street-watcher is OFF-MAP = the reinforcement fuse** (flees to raise the Pale-Watch
if the whistle sounds). **NO surprise** (loud breach). **⚔ COMBAT LIVE — encounter `a46b1472-72c6-49aa-a36c-2343bd613299`, Round 1.** All 3 PCs staged (Forge 20 /
Windreth 8 / Vale 3); enemies auto-rolled. **Init order R1:** BOSS 21 → Forge 20 → ENF2 17 → **LKT (Door-Minder) 14
[WHISTLE threat — acts before Windreth + Vale]** → Windreth 8 → ENF1 8 → Vale 3. **Combatant UUIDs:** BOSS
`d24b8b18-0e0f-40fd-9802-173f29dbec5d`, FO `5f68f10b-6d0d-4aa4-9ef4-c1247f9140e2`, ENF2
`73901a01-3520-4532-83e4-8b10029f4944`, LKT `dd1a1484-9325-4b18-86a9-812bea9a7cdd`, WI
`c9d996da-2607-449f-8af4-86a99b438cf0`, ENF1 `02f04362-1668-4402-aa69-3f6acba40210`, VA
`a900961a-9b72-4c41-920d-5e530f69a2f2`. **BOSS turn done** (advanced 20 ft L5→**H5**, HELD Bark-an-Order = no free
enemy actions turn 1). **FORGE'S TURN (07-22):** moved B5→C4, **Rage** (bonus action; engine says 1 rage left),
2× Greataxe on the **Door-Minder** (21→5, 23→11 incl. +2 Rage) → **LKT DEAD at D4 — the Alarm-Whistle soft-clock is
neutralized before it ever sounded** — then stepped C4→B5 (25 ft move + reaction still in hand). **His player asked
(To DM): "can i use great weapon mastery to another enemy? I move first."** **RULING posted OOC (201):** NO more
swings this turn — Attack action = both Greataxe swings (spent) + bonus action = Rage (spent); a mastery rides on an
attack, it doesn't conjure one. **Cleave** is engine-auto *on the hit* vs a 2nd enemy within **5 ft** of the struck
target (muscle were ~15 ft off at G4/G6 → no Cleave target; can't bank/carry it — verified engine auto-resolves
Cleave in `internal/combat`); **GWM**'s kill-swing needs the bonus action Rage already used. Told him: 25 ft (5 sq)
to reposition or hold (ending next to Ledger/muscle = they swing first next round, init 21/17 vs his 20); next round
Rage still up, both swings back, Cleave live if two enemies stand within 5 ft. **➤ NOW = FORGE'S TURN STILL OPEN —
awaiting his `/move` or "end turn"; do NOT advance until he declares.** **Windreth has a STANDING reaction
declaration queued** (`a98a4557…`, "uncanny dodge if I get hit") — leave it standing, fires only when an enemy hits
him; do NOT resolve/kill it early. **On advance, next enemy = ENF2 (Front Muscle, init 17), then LKT is dead (skip),
then Windreth 8 → ENF1 8 → Vale 3.** Positions: boss H5, ENF1 G4, ENF2 G6, LKT ☠D4, PCs Windreth A4 / Forge B5 /
Vale A6. Posted opening read-aloud + tactical OOC (telegraphed: only Forge acts before the minder's whistle). **Enemy-turn flow:** `GET .../enemy-turn/{combatantID}/plan` → `POST .../enemy-turn
{combatant_id, steps:[{type,movement/attack/ability}]}` (coords 0-based: DB row R → Row R-1; does NOT auto-advance)
→ `POST .../advance-turn`. **PC HP entered combat near-full** (Vale 36/38, Windreth 38/38, Forge 50/50 — earlier
"17/38" was a misread). Seals intact. **Collector ALIVE = the thread to the far-quarter consolidation front (+ maybe
the countersign to walk in clean); DEAD = Forge's nat-20 map still stands, just no tongue.** **Build-API contract (verified this turn): homebrew POST is strict (exact keys `ac`/`hp_average`/
`hp_formula`/`cr`-STRING/`attacks`/wrapper-form `senses`|`skills`|`abilities`|`bonus_actions` = `{RawMessage:…,
Valid:true}`; campaign_id in QUERY); `/api/encounters/` needs `map_id` (map NOT skippable) + campaign_id in BODY,
resp id field = `id`; `/api/combat/start` auto-pulls staged inits, surprise = `surprised_combatant_short_ids`
[short-ids of the SURPRISED].** _(historical: the pre-combat blind-drop beat — the party worked the drop AND set an ambush on the collector.)_
Actions taken + all 4 checks narrated to #in-character (`1c94d787`/`f7482f9e`/`e34cb598`/`49debeec` → 204):
**Forge took the bundle** (this **snapped the waxed tell-tale** → the drop now reads *serviced* to anyone who
minds it). **Windreth Stealth 32** (nat 19 +10 +Guidance) = hidden, **undetected**, total overwatch — owns
the first move. **Vale Stealth 8** → no cover in the swept corridor, so she pivoted: **Disguise Self (a fourth
dead enforcer) + Minor Illusion (stillness)**, then **Deception nat-1** — the *look* holds at a glance but the
**body-count is wrong** (only 3 enforcers + captain died, all in known spots); she blows the instant the
collector studies/nudges/counts. **Forge Investigation nat-20** (the taken bundle) → cracked the **routing-
countersign slip**: the money consolidates at a **far-quarter market front** where rounds end; this niche is
the **last stop on a round**, so the collector is the **living thread to that front**. Can't name the top hand
(built so no low cutout can) — but names the **next door**. Sealed waxed name-scrap = another cut identity,
**seal warded** (break it → break the countersign) = lead, not page. **The collector has NOW arrived at the
niche** — alone, lantern-less, a low expendable cutout — and stops, clocking the wrongness (snapped thread,
gone bundle, the extra "corpse", Forge crouched in the open with the tribute), **not yet committed** (whistle /
steel / question). Posted #the-story read-aloud (`f1519790`) + **OOC fork** — **awaiting their pick, no forced
roll:** (a) **jump him** — Windreth surprise-strikes from hiding (`/initiative roll:<total>` all → surprise
round), (b) **take him alive** — grapple/subdue to question (`/initiative`, non-lethal), (c) **bluff** —
present the countersign slip as legit business (`/check skill:deception`, uphill: drop visibly serviced),
(d) **let him pass & tail him** to the front (`/check skill:stealth`; Windreth's 32 makes shadowing very
doable), (e) something else. **Collector alive = the thread to the far-quarter consolidation front; dead = the
lead dies with him (still tail-able via the ledger/slip).** Vale can **wear the grey man's face** at will.
**Soft clock now LANDED = the collector himself** (was foreshadowed since Windreth's Perception 19, not
sprung). **Never roll/act/decide for the PCs.** **THE SEAL is OPEN; the buyer's identity, restoration + the
erasure-hand's identity remain the sealed gates.** Full blow-by-blow →
[`sessions/session-01.md`](sessions/session-01.md); live board → DM Console; durable IDs/secrets below._

## Live mechanical state → DM Console (do not hand-copy here)

Round, turn order, every combatant's HP/AC/position/conditions, the pending queue, and the
recent action timeline are **generated** — read them live, never transcribe them into this file:

- **DM Console:** `GET /api/dm/situation` or the `#dm-console` dashboard tab (`next_step`,
  `pending[]`, `state`, `timeline[]`).
- Source tables (read-only sanity checks): `encounters`, `combatants`, `turns`, `action_log`,
  `dm_queue_items`.

## Ops snapshot

- **Stack:** UP via `make local-up` (docker compose). App `localhost:8080`, DB
  `localhost:5432`. Bot `DnDnD` (id `1507904367301496862`) connected to guild `DnDnD`.
- **Last deploy:** `main` (see `git log`); combat state has survived every redeploy.
  Rebuild + redeploy after any code fix: `docker compose up -d --build app`
  (see [`runbook.md`](runbook.md) §1). Redeploy *history* lives in the session logs.
- **Remote-player tunnel:** an **ngrok tunnel on a reserved domain** exposes the local app so
  remote players reach the builder + OAuth. **URL is STABLE**
  (`https://unhustling-cushionless-karan.ngrok-free.dev`), so the OAuth callback is registered
  in Discord **once** and never changes. How-to + `make tunnel-*` targets:
  [`runbook.md`](runbook.md) "Remote players"; one-time setup + `NGROK_DOMAIN`/`NGROK_AUTHTOKEN`
  in `.env`: header of [`scripts/tunnel.sh`](../../scripts/tunnel.sh).
  - **OAuth callback (registered, stable):**
    `https://unhustling-cushionless-karan.ngrok-free.dev/portal/auth/callback` — no per-restart
    Discord change. `make tunnel-up` always yields this URL; `make tunnel-down` restores `.env`
    to `localhost` while keeping the ngrok vars. (Migrated off the old ephemeral cloudflared
    quick tunnel 2026-06-27.)

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
| Sabinnet's Reading Room | `353c58b3-3844-4f4f-8a19-b38a73c0da47` | 12×10. Sesh reader fight (won 07-18). PC breach at the SOUTH wall; Sabinnet's desk far NORTH (F1). |
| Ashfall Waystation — common room | `1ad14481-f938-462d-be75-25764463ff5b` | 12×10 blank grid; 2×2 **Pit** (SW) = cellar mouth. Features narrated. |
| Ashfall Waystation — cellar | `d2fe03c6-9749-4a24-a6e3-cb9d3a77e3cd` | 12×10 blank stone; PC spawn at top-center stairs landing. |
| Ashfall Waystation — the cold vault | `2899165e-3d1b-46e9-962f-9065e4e3529a` | 12×10 blank stone (built 06-29); PC spawn bottom-center = the cold door. |
| Buried Gallery of the Faceless God | `39ecd023-51d8-44bb-bf8e-29e1eff3a231` | 12×12 blank stone (built 07-02). PC spawn bottom-center (the mouth), enemy top-center (the heart); ~8 squares = the 40 ft road. |

_(Other Sesh/Palewatch fights used ad-hoc maps: the grey-man used `cc356cc4…`; the watcher fight used `db0a4d44…`. Build fresh maps live when a fight stands up.)_

## Active encounter (durable refs — live state via the Console)

**No active encounter — the party is OUT OF COMBAT** (aftermath in Sabinnet's reading room; see
Current scene). Recent CLOSED encounters, newest first:

- **CLOSED — "Sabinnet's reading room"**, encounter id `95f98525-3e70-47f0-ad74-583c612a0c73`,
  map `353c58b3-…`, template `db9943fa-ea1e-41ca-976c-d9387bda110b`. Boss homebrew **Sabinnet,
  the Reader-under-glass** (`hb_84d6333d764f`, AC 13 / HP 58, INT+WIS save profs, *Mind Lance*
  +6 3d6 psychic + *Warding Rod* +4 1d8+2 force, psychic-resist + charm-immune, flees/alarms
  when bloodied ≤29) + **2× SRD Thug "Sabinnet's Housecarl"** (`creature_ref_id:"thug"`, HP 32).
  **VICTORY 07-18 (R4)** — both housecarls dead; Sabinnet dropped to 0 by Vale's Eldritch Blast
  but **CAPTURED ALIVE** (Vale pre-declared non-lethal → house rule). Chronology + rulings:
  [`sessions/session-01.md`](sessions/session-01.md).
- **CLOSED — "Sabinnet's approach — the watchers"**, encounter id `8431a89b`, map `db0a4d44`,
  template `8564bc2d`. **2× SRD Thug "Sabinnet's Watcher"**; **VICTORY 07-17 (R3)**, both dead
  (Vale's Eldritch Blast + Windreth's Steady-Aim Sneak shots). session-01.md.
- **CLOSED — "The night road — the follower"**, encounter id `30baba5f-01c1-40f9-b27e-adfa483a0973`,
  homebrew **The Follower** (`hb_9b87c216b7cf`, AC 15 / HP 58, CR 3). A *made* thing built to hunt
  Windreth; **VICTORY 07-09 (R3)**. session-01.md.
- **CLOSED — "Palewatch — the kept vault (the grey man)"**, encounter id `2846a6ca-ab2a-4117-962d-808108dd4f83`,
  boss **Renegade Name-Keeper / grey man** (`hb_ed8093e5cfe4`, AC 15 / HP 104 / WIS +7, CR 6),
  map `cc356cc4…`. **VICTORY 07-07 (R4)** — the party chose the **KILL over the parley**. The
  antagonist is **DEAD, not captured** — do NOT resurrect/recur him. session-01.md.
- **CLOSED — "The Buried Gallery of the Faceless God"**, encounter id `9e558982-697a-4cc8-8c25-abe3d34cf201`,
  map `39ecd023-…`. 1 Ghoul + 2 Zombies; **VICTORY 07-03 (R3)**; faceless-god arc RESOLVED.
- **CLOSED — "The Cold Vault"** `446dce33-…` (map `2899165e-…`, lone Wight, VICTORY 06-30) and
  **"The Cellar"** `8509d1f6-…` (two ghouls, VICTORY 06-28 R11); prior "cellar wretch"
  `6f317490-…` VICTORY 06-26. Full chronology: session-01.md.

## Current scene (narrative framing — non-derivable)

**★ Now: SESH — the name-market canopy, moments after THE SEAL was read.** The party breached the
reader's door, won the fight non-lethally, searched + interrogated + **captured Sabinnet alive**,
extracted her cleanly, long-rested, then walked her + Windreth's warded scrap to **Sesh's name-market**
and had her **read the scrap under the warding-canopy** (07-19). The canopy held — the thread carried
nowhere, **the faceless buyer felt nothing, no bell**. ★★ **The reveal landed: the warded prize is a
living, freshly-cut true name, and it is WINDRETH'S OWN** — the name scraped out of him, the buyer's
most-wanted prize, now spoken aloud for the first time (literal name kept offstage). **Reading ≠
restoration** (he knows it, doesn't have it back; the ward's undoing runs through the god's scattered
name — the endgame lever). The loaded cost: **Sabinnet, the buyer's own reader, now knows whose name it
is.** The scene is the beat *right after* — Windreth hearing his own name, the captive who now holds
it, and the party's next move. **Still sealed: the buyer's true identity, what the faceless god is, the
trilemma stated flat, and how the name is restored.** Handed back: Windreth's response / Sabinnet's fate
now she knows / next heading. Build the next beat live.
**⚠ Loud is loud:** a breach at Sabinnet's door is the buyer's business; if the fight reopens,
build newcomers into a **FRESH encounter** (in-page `POST /api/homebrew/creatures` +
`POST /api/combat/start`; the old encounter is closed).

**How they got here (condensed — full arc in [`sessions/session-01.md`](sessions/session-01.md) +
[`campaign-arc.md`](campaign-arc.md)):** the party is deep in **Sesh**, the gateless face-market
city days west of the Palewatch, chasing the scattered name to Sesh's **name-readers** (the one
place a kept name can be read). They ran a con with the wrapped **Follower corpse** as bait,
followed **Vale's disc** to a fence-shed, took its lookout, and conned the door (Vale Deception
nat-20 wearing the grey-coat buyer's face via **Mask of Many Faces**). They grappled the keeper
"Bertran," took both wardens, secured the room, and broke the back-clerk with **Intimidation 24**.
They now hold the **iron box** (cut name-scraps + coin + a stamped tally-token), the **ledger**
(full scatter-routing + a who-read-whom column for Sesh), and the **prize lead** — **Sabinnet**,
the *Reader-under-glass* (salt-white door behind the fish-stones; **she answers to the faceless
buyer**). The **buyer has no extractable address** (faceless, scatter-doctrine); the clerk gave a
runner countersign (**two slow, one quick** / *"the salt's dry"*), the runner came and took a
packet of loose scraps and left unalarmed, and **Windreth tailed him (Stealth 17)** to *this same
door* — reader-lead and runner-road are the same place. **Forge Investigation 10** netted the
trade's working kit (→ his sheet, gold 15). The party long-rested at the wagon (07-17), moved on
the door by morning **as open customers**, and Forge's Rage-breach turned it loud — through the
watcher fight, the breached inner door, and now this. **THE SEAL never changed hands** (only loose
stock did; the kept prize scrap is unread).

- **Party (durable):** Vale Warlock 4 (Fiend, Pact of the Tome, Mask of Many Faces), Forge
  Barbarian 4 (Berserker + Great Weapon Master), Windreth Rogue 4 (Thief + Defensive Duelist).
  Sheets: [`party/roster.md`](party/roster.md); **live HP → DM Console.** Char IDs: **Vale
  `b6ca7f49…` · Forge `d2d98745…` · Windreth `b2c436da…`**. Pronouns: **Windreth he/him**
  (memory `reference_pc_pronouns`), Vale she, Forge he. Forge's true max HP is **41** (not the
  roster's stale 32).
- **The antagonist arc:** the **Renegade Name-Keeper (grey man) is DEAD** (07-07, party earned
  the kill over a parley); the **Follower** — a *made* thing built to hunt Windreth — is dead too
  (07-09). Arc pressure now runs through the **Order of the Pale-Watch** (wardens still out
  there), **Vale's patron** (a rival collector still steering her — DM-secret), and the physical
  **name-scraps**. Secret spine → [`campaign-arc.md`](campaign-arc.md).
- **★★ THE SEAL — READ 07-19 (was: do NOT resolve on a low/wrong roll):** **Windreth** carries the
  warded **kept prize scrap** — on his sheet as **_The Kept Name-Scrap (warded)_** (`identified:false`,
  flag left as-is). **It has now been read** — Sabinnet read it under Sesh's name-market **warding-canopy**
  (the safe gate: thread drank the sound, buyer felt nothing). **CONFIRMED: it is a living, freshly-cut
  true name, and it is WINDRETH'S OWN stolen name** (was "likely / never confirmed" — the reading
  confirmed it; literal name still offstage, kept as a driver). **Reading ≠ restoration** — he *knows*
  it now, doesn't have it *back*; the ward is a blanket over a candle, and undoing it runs through the
  god's own scattered name (endgame lever, still sealed). **Loaded fallout:** Sabinnet (the buyer's
  reader) now knows whose name it is. Do NOT re-gate this as "unread"; the open question is now
  *restoration* + *what the party does with the knowledge*, not *how to read it*.
- **Vale's kit (on her sheet):** the Faceless God's Token, the ashen face-shard, the **Name-Scrap
  of the Faceless God**, the **Grey Man's Name-Scraps (bundle)**, the defaced **Renegade's
  Warden-Disc**, the Cold Iron Key (×2). (Her **Potion of Healing** was **spent on Forge** during
  the reader fight — patched him to 20/41 mid-combat; consumed, off both sheets.) **⚠ Vale does
  NOT hold the kept prize scrap** — that warded scrap is on Windreth's sheet (see THE SEAL).
- **Windreth's arc-kit (on his sheet):** **_The Kept Name-Scrap (warded)_** (THE SEAL — **now READ
  07-19: confirmed = Windreth's own stolen name**, still warded/unrestored on the sheet) + a **Token
  of Remembrance**. The scrap is no longer a *hidden* prize — he knows what it is; the live thread is
  restoring it (endgame) and that Sabinnet now knows it too.
  **DM-secrets held** (see [`campaign-arc.md`](campaign-arc.md)): the Order's right-to-refuse
  doctrine, Vale's patron = rival collector using her, the reassemble/scatter/hand-over trilemma.

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** (`GET /api/dm/situation` / `#dm-console`) first for `next_step` + the
> live board, then apply this intent. **Never roll/act/decide for the PCs** — players roll their
> own dice; verify any `/command` syntax against `internal/discord/commands.go` before
> advertising it in a coda.

1. **★★ SESH — THE SEAL just read under the canopy; the reveal has landed (07-19).** Reader fight won,
   Sabinnet captured + interrogated, party extracted cleanly, long-rested, walked her + the warded scrap
   to **Sesh's name-market**, and had her **read it under the warding-canopy** — the safe gate held
   (thread drank the sound, **buyer felt nothing, no bell**). ★★ **Confirmed: the warded prize is a
   living, freshly-cut true name = WINDRETH'S OWN** (the buyer's most-wanted prize; literal name offstage,
   still a driver). **Reading ≠ restoration.** Await their RP / rolls — don't act for them. Live beats:
   - **Windreth's response — HIS to play.** He just heard his own stolen name for the first time. Do NOT
     script it (say-it-back / take the scrap / go quiet / reach for restoration — all his). Give it room;
     this is the campaign's emotional fulcrum.
   - **Sabinnet now KNOWS the name.** The buyer's own reader has read Windreth's true name aloud and knows
     whose it is — the most dangerous fact in the arc now sits in a captive's head. The live decision is
     what the party does about that (`/check skill:insight` offered to read whether she'll keep/sell/fear
     it). ⚠ **Prohibited-action guard:** if they move to *kill a bound prisoner* or similar, that's their
     call to declare — adjudicate the fiction, don't push it; and never resolve a PC's moral choice for them.
   - **Restoration is the new sealed gate (endgame lever).** Knowing the name ≠ having it back; undoing the
     ward runs through the god's *own* scattered name (the assembly tracker → convergence). Do NOT hand
     restoration cheaply; it's the trilemma's teeth. Still sealed too: buyer's true identity, what the
     faceless god is, patron=buyer stated flat, the trilemma stated flat (per [`campaign-arc.md`](campaign-arc.md)).
   - **⚠ Soft clock** — a missing fence-reader gets noticed. No pursuit *yet*; the longer they linger in
     Sesh, the more the district stirs (Pale-Watch wardens / the buyer's runners). Foreshadow, don't spring.
   - **Next heading** — the market itself, the buyer's blind-drop (signet + countersign), the fled wardens
     (Mave's five in Sesh's crowd), or a road of their own. Build it live when they point.
   - **Housekeeping:** all PCs full post-rest (**Forge 41/41**). Forge's old thrown handaxe was left in
     the dead housecarl back in the reading room (abandoned on extraction — trivial, ignore unless a player
     flags it). Loot → the finding PC's sheet (standing rule, [`dm-rules.md`](dm-rules.md)).
   - **⚠ Loadout quirk:** thrown weapons leave `equipped_main_hand` NULL (inventory kept) — for a
     repeat throw / melee-after-throw the player must pass `weapon:handaxe` or `/equip handaxe`;
     there is **no DM equipment endpoint** to fix the hand from the console.
2. **Onboard new players** as they arrive (`/register` → build in the portal via the tunnel →
   DM-approve on the dashboard → roster row + `party/<name>.md` sheet). Party scaling toward 5–6;
   see [`big-party.md`](big-party.md).
3. **After every beat:** narrate to #the-story (read-aloud block, OOC coda first, plain simple
   English) + append the play-by-play to [`sessions/`](sessions/) + keep this file's **Next
   action / Current scene** current. Pull the numbers you narrate from the Console; never
   hand-track them.

**Ops quickref:** app `localhost:8080`; tunnel UP (`make tunnel-up`, stable ngrok); DB read-only
sanity via `docker compose exec -T db psql -U dndnd -d dndnd` (heredoc-friendly); redeploy after a
code fix `docker compose up -d --build app`. **Level-ups:** all 3 L4 feats applied (Vale +2
CHA→18, Forge GWM, Windreth Defensive Duelist). **Loadout tip for Windreth:** shortsword main /
dagger off-hand so main-hand Vex + off-hand Nick both fire (ISSUE-061). **Forge's
`player_characters` row is `status=rejected`** (stale since the 07-03 L4 rework — plays fine; his
party-overview card is missing, so out-of-combat status edits go via
`POST /api/character-overview/d2d98745…/status`). **Open bugs to route around:** ISSUE-059 (DM-Queue
Resolve button fires no POST → resolve via `POST /dashboard/queue/<id>/resolve` from the authed
tab), ISSUE-060 (builder omits Warlock pact boon / invocations), ISSUE-070/071 (End-Combat
transient-condition carry / long-coda split — cosmetic).

Durable lore → [`world.md`](world.md); secret spine → [`campaign-arc.md`](campaign-arc.md);
full play-by-play → [`sessions/session-01.md`](sessions/session-01.md).
