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

_Last updated: 2026-07-18 — **SESH, the deep market — ⚔️ COMBAT LIVE in Sabinnet's reading room (Round 2, encounter
`95f98525`). The inner door was breached; the climactic reader fight is on. **Round 1 FULLY played: Windreth Sneak-shot HC1 (14),
Forge raged + axed HC1 (10 → reeling 8/32), Sabinnet Mind-Lanced Forge (16 psychic → 18/41), Vale's Hold Person FIZZLED (Sabinnet
WIS 25 vs DC 14), then both housecarls maced Forge (Rage halved both → Forge 13/41) and HC1 charged D4→F5 (Forge now flanked).
➤ ROUND 3, Vale's turn (PC) — **she cast Hold Person on Sabinnet; DM resolved the enemy WIS save (10 vs DC 14, FAIL) → SABINNET PARALYZED (save-ends WIS DC 14). Vale's action + pact slots spent; movement/bonus left, AWAITING her move-or-`/done`. Then Windreth into a HELD boss (advantage on all attacks; melee-within-5ft = AUTO-CRIT; Forge already adjacent G2). Forge is at 9/41 after Sabinnet's R3 Mind Lance (11 psychic).** Windreth (R3) moved C9→G5 + Cunning-Action Hide (Stealth 23, hidden) + Shortsword (adv, HIT, +Sneak, 7) + Dagger NAT-20 crit (5) → **HC2 to 4 HP (SURVIVED)**; **Forge's action handaxe (5+Rage) then dropped HC2 → DEAD (his kill)**. **BOTH HOUSECARLS DEAD — only Sabinnet left (58/58, F1, untouched).** Sabinnet flees/alarms at HP ≤29. ⚠ CORRECTION (07-18): an earlier note wrongly credited Windreth's crit with the HC2 kill — the crit left HC2 at 4 HP; **FORGE's action-attack killed it**. Forge's off-hand BONUS handaxe then whiffed the already-dead guard; per Forge's request the wasted bonus was **refunded** (one-time SQL `bonus_action_used=false`, user-approved) — proper dashboard `restore-bonus-action` endpoint being built. ⚠ Windreth double-spent his bonus (Hide + off-hand dagger) — engine allowed it; mook already doomed, left as-is (engine gap, not reversed).** The prior watcher fight (`8431a89b`) is COMPLETED (both watchers down); that history is recapped below.** The morning con collapsed into a **Rage door-breach** (Forge,
14:11); the table went LOUD (all three `/initiative` in: **Windreth 23 → Forge 18 → watcher 7 → Vale 6 → watcher 1**). DM
built the fight via in-page API (map `db0a4d44` + template `8564bc2d` = **2× Thug as "Sabinnet's Watcher"**, HP/AC secret;
enemy init auto-rolled), opened it (#the-story `1527585506932559906`), **no surprise round.** **R1:** Windreth (Hide→Sneak)
& Forge cut the **blocker** to 2; coat-watcher maced **Forge for 7** (Forge **NOT raging** — bonus went to the off-hand
axe); **Vale's Eldritch Blast killed the blocker.** **R2:** Windreth **Steady Aim → Shortbow Sneak 14** (coat 18/32); Forge
whiffed both handaxes; coat (DM-run enemy turn) **missed Forge** (nat 1) and — DM fiction — **shouted through the barred
door for help**; **Vale's EB 14** dropped the coat to **4**. **R3:** Windreth **Steady Aim → Shortbow Sneak 21** → **coat
DEAD, last enemy down.** DM **ended combat** (status `completed`; End-Combat auto-carried HP to sheets: **Forge 34/41**,
Vale & Windreth full, no conditions). Aftermath posted (#the-story `1527600138061615175`). **➤ NEXT (out of combat):** the
coat's shout **was heard** — the **barred salt-white inner door is still shut** but the interior is **roused and waiting**;
Sabinnet (Reader-under-glass) + any interior muscle may be behind it. **Party is regrouping at the threshold — awaiting
their approach** (breach / listen / parley / pull back). Don't act for PCs. If the fight reopens, build newcomers into a
FRESH encounter (in-page `POST /api/homebrew/creatures` + `POST /api/combat/start`). Prior beat: rested at the wagon
(bot-applied long rest), walked up
as customers, got the slot open on a woman's voice offering the reading trade — before Forge went loud. The runner-tail
converged both threads on this door (Windreth Stealth 17: a courier's road ends at a hand, not a face — **the same
salt-white door the clerk named as Sabinnet's**, the *Reader-under-glass*, who answers to the faceless buyer). The full
night-road → Follower-kill → Mave's-wardens → Sesh → the-con → the-tail arc lives in
[`sessions/session-01.md`](sessions/session-01.md). Live board → DM Console; durable IDs/secrets below._
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

_(Sesh + the Palewatch have been played out of combat / on ad-hoc built maps — the grey-man fight used map `cc356cc4…`; no standing Sesh combat map yet. Build fresh maps live when a fight stands up.)_

## Active encounter (durable refs — live state via the Console)

**No active encounter — the party is OUT OF COMBAT** (mid-infiltration in Sesh; see Current scene). Recent
CLOSED encounters, newest first:

- **CLOSED — "The night road — the follower"**, encounter id `30baba5f-01c1-40f9-b27e-adfa483a0973`, homebrew
  creature **The Follower** (`hb_9b87c216b7cf`, AC 15 / HP 58, CR 3 skirmisher). Ambush fight on the ash-road to
  Sesh (a *made* thing built to hunt Windreth); **ended in VICTORY 07-09 (R3)** — the Follower dropped to 0 on
  Forge's turn, encounter `completed`. ISSUE-038 auto-carried HP; sheets clean, no transient leak.
  Chronology: [`sessions/session-01.md`](sessions/session-01.md).
- **CLOSED — "Palewatch — the kept vault (the grey man)"**, encounter id `2846a6ca-ab2a-4117-962d-808108dd4f83`,
  boss homebrew **Renegade Name-Keeper / grey man** (`hb_ed8093e5cfe4`, AC 15 / HP 104 / WIS +7, CR 6, no
  Legendary Resistance), map `cc356cc4…`. **ended in VICTORY 07-07 (R4)** — the party chose the **KILL over the
  parley** (Windreth's Sneak-Attack from hiding dropped him). The antagonist is **DEAD, not captured** — do NOT
  resurrect/recur him; arc pressure now runs through the Order of the Pale-Watch, Vale's patron, and the physical
  scraps (see [`campaign-arc.md`](campaign-arc.md)). Chronology: [`sessions/session-01.md`](sessions/session-01.md).
- **CLOSED — "The Buried Gallery of the Faceless God"**, encounter id `9e558982-697a-4cc8-8c25-abe3d34cf201`, map
  `39ecd023-…`. 1 Ghoul + 2 Zombies; **VICTORY 07-03 (R3)**; the dormant god released (no final combat) — the
  faceless-god arc RESOLVED. Chronology: [`sessions/session-01.md`](sessions/session-01.md).
- **CLOSED — "The Cold Vault"**, encounter id `446dce33-e221-4d1f-a88b-4e81534b3399`, map `2899165e-…`. Lone
  **Wight** keeper; **VICTORY 06-30 (R3)**, no casualties. Chronology: [`sessions/session-01.md`](sessions/session-01.md).
- **CLOSED — "The Cellar"**, encounter id `8509d1f6-da9d-451c-bb2e-8571b9402e9e`, map cellar. Two ghouls;
  **VICTORY 06-28 (R11)**. Prior fight — "Waystation — the cellar wretch" (`6f317490-…`) — VICTORY 06-26. Full
  chronology: [`sessions/session-01.md`](sessions/session-01.md).

## Current scene (narrative framing — non-derivable)

**★ Now: SESH — Sabinnet's reading room, ⚔️ COMBAT LIVE (Round 1).** The salt-white INNER door was breached (Forge shove #2
= Athletics 20 w/ Vale's Help vs secret DC 15); reveal + initiative call posted; **all three `/initiative` totals came in
via `/roll` in #roll-history — Forge d20+2=19, Vale d20=19, Windreth d20+4=19.** DM started the fight via in-page `POST
/api/combat/start` (dice verbatim, APP-5 zero-override) — **encounter `95f98525-3e70-47f0-ad74-583c612a0c73`** ("Sabinnet's
reading room") on map `353c58b3`, template `db9943fa`. **Turn order (all 19 → DEX tiebreak): Windreth(1) → Forge(2) →
Sabinnet(3) → Vale(4) → Housecarl HC2(5) → Housecarl HC1(6)**, no surprise. **Board:** party breached the SOUTH wall —
**Windreth C9** (wide-west flank), **Forge F9** (breach), **Vale F10** (behind); enemies NORTH — **Sabinnet F1** (far back at
her glass), **housecarls HC1 D4 + HC2 H4** (mid-floor lane). Combat-open beat (5:24 PM) + Round-1 recap beat (6:25 PM) posted
(#the-story). **Enemy HP/AC SECRET:** Sabinnet AC13 / HP58, INT+WIS save profs, *Mind Lance* +6 3d6 psychic + *Warding Rod* +4
1d8+2 force, psychic-resist + charm-immune, **flees/alarms when bloodied (≤29)**; 2× Thug housecarls HP32 each.
**Round 1 (COMPLETE):** (1) **Windreth** — Steady Aim → Shortbow **Sneak on HC1 (D4) = 14** (HC1 32→18). (2) **Forge** — **Rage**
(bonus, active) → moved F9→**G5** → Handaxe **HC1 = 10 incl +2 Rage** (HC1 18→**8/32, reeling**) + vex applied. (3) **Sabinnet**
(DM-run via Turn Builder) — **Mind Lance vs Forge: to-hit 24 HIT, 16 psychic** (rage does NOT resist psychic; **Forge 34→18/41**);
stayed seated at F1. (4) **Vale** — cast **Hold Person on Sabinnet**; DM resolved her save via **Combat Manager → Pending monster
saves → Resolve save** → **WIS 25 vs DC 14, SUCCESS** → spell FIZZLED, **Vale lost concentration, pact slot spent, 0 held** (then
her turn auto-resolved on timeout, no move — still F10). (5) **HC2** (DM-run, Turn Builder) — **Mace vs Forge: to-hit 22 HIT, 2
bludg** (Rage halved 4→2; Forge 18→16). (6) **HC1** (DM-run) — **Mace vs Forge: to-hit 17 HIT, 3 bludg** (Rage halved 6→3; Forge
16→**13/41**); HC1 charged **D4→F5** (DM position-override, board-sync to melee reach). **Forge now FLANKED at G5** (HC2 H4 E-side,
HC1 F5 W-side). Both housecarl beats posted #combat-log; Round-2 open beat posted #the-story (7:39 PM).
**Round 2 so far:** (1) **Windreth** (PC) — `/bonus` Steady Aim → `/attack` Shortbow on **HC1 (F5)**: adv, **18 to hit, 17 piercing
Sneak Attack** → **HC1 DROPS to 0, DEAD** (one housecarl down). Kill beat posted #the-story (7:55 PM). (2) **Forge** (PC, turn
ACTIVE) — freeform "take my thrown handaxe from the deceased F5 enemy" + "approve if makes sense"; **DM approved as a free object
interaction** (G5 adjacent to corpse F5 → axe back in hand, no action/move cost; both his freeforms resolved in DM Queue). Forge
then moved **G5→I5** (adjacent to HC2 H4) and `/attack`ed — but engine gave **Unarmed Strike (5)** because the thrown handaxe left his
`equipped_main_hand` NULL (loadout quirk; axe still in off-hand/inventory). **Player asked to redo with the axe; DM approved.** ⚙️ Undo
applied via in-page fetch: **HP-override HC2 27→32** (undo-last-action can't revert an attack — no before_state → 422; used HP-override
instead) + **`restore-action` on Forge** (action_used→f, attacks_remaining→1; movement 15ft kept). Redo-prompt posted #the-story (8:06 PM).
(3) **Forge handaxe redo RESOLVED** — he re-attacked with `/attack ...weapon:handaxe`: **MISS (7, action)** then a bonus-action follow **HIT (23 → 5 slashing)** on HC2 (**32→27**). Redo beat posted #the-story (5:58 AM 07-18). His turn ended (action + bonus spent). (4) **Sabinnet** (DM-run, Combat Manager) — **Mind Lance vs Forge, 10 to hit → MISS** (Rage doesn't blunt psychic, he just rolled low; ranged from F1, never left the table). Whiff beat posted #the-story (6:03 AM).
(5) **Vale (PC) DONE.** `/move` F10→**J6** (adjacent Forge); bonus = Potion of Healing → **Forge 13→20/41**; mis-cast Hex → **UNDONE** (concentration dropped, `hexed` stripped, pact 0→1 refunded); **consumed potion removed from Vale's sheet** via `POST /api/inventory/remove`. Action = `/cast` **Eldritch Blast + Agonizing Blast** on HC2 → hit 15, **11 force → HC2 27→16**. (6) **HC2 (DM-run, Combat Manager) DONE.** Mace vs Forge, **13 to hit → MISS**; posted #combat-log. End Turn → dead HC1 (init 9) auto-skipped → **Round 3.** R2-close beat posted #the-story (7:07 AM). **R3 (1) Windreth (PC) DONE:** `/move` C9→**G5**; `/bonus` Cunning-Action Hide (Stealth 23, hidden); `/attack` Shortsword vs HC2 (adv-hidden, 23 HIT, 7+Sneak); `/attack` Dagger vs HC2 (adv-vex, **NAT 20 CRIT**, 5) → **HC2 to 4 HP (SURVIVED — the crit did NOT kill).** (⚠ he double-spent bonus: Hide + off-hand dagger — engine allowed; not reversed.) **R3 (2) Forge (PC) — action = the KILL:** `/attack` Handaxe (21 HIT, 5+2 Rage) → **HC2 4→DEAD (Forge's kill).** Bonus Handaxe (9 MISS) then whiffed the already-dead guard → **wasted bonus REFUNDED** per Forge's request (one-time SQL `bonus_action_used=false`, user-approved; proper `restore-bonus-action` dashboard endpoint being built). **R3 (2 cont.) Forge DONE:** moved I5→**G2** (adjacent Sabinnet F1), held bonus, `/done` (12:20 PM). **R3 (3) Sabinnet (DM-run, Combat Manager):** Mind Lance vs Forge, **24 to hit HIT, 11 psychic** (Rage doesn't blunt psychic) → **Forge 20→9/41**; never left F1. **R3 (4) Vale (PC):** `/cast` **Hold Person on Sabinnet** (12:36 PM; pact slot → **0 held**, concentrating on Hold Person). DM resolved the enemy WIS save via sanctioned in-page `POST /api/combat/{enc}/pending-saves/{id}/resolve` (ISSUE-043 monster-save path; engine rolled) → **WIS 10 vs DC 14, FAIL → SABINNET PARALYZED** (save-ends WIS DC 14, indefinite; fresh save at end of each of her turns to break free). Paralysis beat posted #the-story (12:48 PM). **➤ NOW — Vale's turn OPEN: action + both pact slots spent, movement + bonus left. AWAITING her move-or-`/done` — don't act for her. Then Windreth (R3) into a PARALYZED Sabinnet — every attack has advantage, melee-within-5ft = AUTO-CRIT; Forge already adjacent at G2.** **BOTH HOUSECARLS DEAD — only Sabinnet left, now HELD.** Live HP: WI 31/31 (**G5**), FO **9/41** (**G2**, raging), VA 31/31 (**J6**), **SAB 58/58 (secret, F1, PARALYZED)**, HC1+HC2 DEAD.
**LOADOUT NOTE:** thrown weapons NULL `equipped_main_hand` (inventory kept) — for repeat throws/melee-after-throw the player must pass
`weapon:handaxe` or `/equip handaxe`; there is **no DM equipment endpoint** (can't fix the hand from the console).
**Run enemy turns via Combat Manager → Run Enemy Turn** (engine rolls; keep the numbers). **Sabinnet flees/alarms at ≤29 HP.** ⚠ Loud is loud — a breach at Sabinnet's door is the buyer's business; the sealed
scrap still stays hidden (reading THE SEAL here thins the ward, the buyer feels it, Mave). **If the fight reopens, build the
newcomers into a FRESH encounter** (in-page `POST /api/homebrew/creatures` + `POST /api/combat/start`; the old encounter is
closed). The party is deep in
**Sesh**, the gateless face-market city days west of the Palewatch, chasing the scattered name toward Sesh's
**name-readers** (the one place a kept name can be read). They ran a "show, not a sale" con — the wrapped
**Follower corpse** as bait — and followed **Vale's disc** to a fence-shed, took its lookout, then conned the
door: **Vale's Deception nat-20** (wearing the grey-coat buyer's face via **Mask of Many Faces**) got them
**welcomed in**. Once inside, **Forge grappled the keeper, "Bertran,"** and Vale's **Minor Illusion (sound)**
keeps Bertran's voice chatting at the front — **now dropped**. Both wardens taken
(Windreth Stealth 19 to the back; Athletics 5 botch → **Vale's Hold Person landed**, keeper failed WIS 10 vs
DC 14; **Vale pact 1/2**). Party **secured the room** (door barred, both captives bound + gagged, Hold Person +
illusion dropped), then Vale ran a tongues-garland intimidation — **Intimidation 24** (nat 20 + Forge's Help).
**The little back-clerk folded completely (07-12):** the party now holds the **iron box** (cut name-scraps +
coin + a stamped tally-token), the **ledger** (full scatter-routing + a who-read-whom column for Sesh), and the
**prize** — a lead to Sesh's *sealed-name* reader **Sabinnet** (the Reader-under-glass; salt-white door behind
the fish-stones; **she answers to the faceless buyer**). Vale then pressed (RP, no roll — he'd already folded):
the **buyer has NO extractable address** (faceless, never seen; scatter-doctrine keeps a shed-clerk from knowing
one), but the clerk gave the **runner's countersign** — knock **two slow, one quick**, answer **"the salt's dry"**
— and the runner came **due, minutes later**. **The party chose the CON (07-12):** Forge Helped (cloaked in Vale's
illusion as "Bertran" behind her → advantage), **Vale answered the slot in the grey-coat's face** and gave the
countersign — **Deception 15 held.** The runner took a **packet of loose scraps** (the bought-not-placed stock from
the open box) as the night's read, **raised no alarm, never entered**, and **turned back into the lane sure he's
alone.** He gave up nothing to talk — a courier has no buyer-door; the buyer sits at the end of his *route*. **➤ Both shed
rolls resolved (07-13): Forge Investigation 10** netted the working kit (→ his sheet, gold 15) and **Windreth
Stealth 17** (advantage, Vale's illusion) **tailed the runner clean** — no buyer (scatter-doctrine), but the
runner's road **ends at Sabinnet's own salt-white door behind the fish-stones**, so the reader-lead and the
runner-road are **the same place** (warded, watched, hers — the enemy's gatekeeper). **The party then long-rested at
the wagon (07-17) and moved on the door by morning — they now sit at a covered daylight vantage on it, choosing how
to take a threshold that belongs to the enemy.** **THE SEAL stays shut** (only loose stock changed hands; the kept
prize scrap never read). (Full
arc: night road → Follower kill →
Mave's five wardens → travel to Sesh → the con → this shed — in [`sessions/session-01.md`](sessions/session-01.md).)

- **Party (durable):** Vale Warlock 4 (Fiend, Pact of the Tome, Mask of Many Faces), Forge Barbarian 4 (Berserker
  + Great Weapon Master), Windreth Rogue 4 (Thief + Defensive Duelist). Sheets:
  [`party/roster.md`](party/roster.md); **live HP → DM Console** (on the 07-11 resume: Vale 31/31, Windreth
  31/31, Forge 41/41 — Forge's true max is **41**, not the roster's stale 32). Char IDs: **Vale `b6ca7f49…` ·
  Forge `d2d98745…` · Windreth `b2c436da…`**. Pronouns: **Windreth he/him** (memory `reference_pc_pronouns`),
  Vale she, Forge he.
- **The antagonist arc:** the **Renegade Name-Keeper (grey man) is DEAD** (killed 07-07 at the Palewatch — the
  party earned the kill over a parley). The **Follower** — a *made* thing built to hunt Windreth — is dead too
  (07-09). Arc pressure now runs through: the **Order of the Pale-Watch** (wardens still out there; the party
  killed a man in their cloister over a kept scrap), **Vale's patron** (a rival collector still steering her —
  DM-secret), and the **physical name-scraps**. Secret spine → [`campaign-arc.md`](campaign-arc.md).
- **⚠ THE SEAL (do NOT resolve on a low/wrong roll):** **Windreth** carries the warded **kept prize scrap** — on
  his sheet as **_The Kept Name-Scrap (warded)_** (`identified:false`) — likely **his own stolen name**, strongly
  implied but **never confirmed** (verified on the sheets 07-14: it is on Windreth, NOT Vale). It
  opens ONLY on a **proper reading**: a high `/check arcana` (risky), a higher warden, or **Sesh's name-market**
  — NOT insight / survival / perception / a failed roll. **Windreth's stolen true name** is recoverable only via
  this scattered-name arc (the man who could simply hand it back is dead).
- **Vale's kit (on her sheet):** the Faceless God's Token, the ashen face-shard, the **Name-Scrap of the Faceless
  God**, the **Grey Man's Name-Scraps (bundle)**, the defaced **Renegade's Warden-Disc**, a **Potion of Healing**
  (2d4+2, unused), the Cold Iron Key (×2). **⚠ Vale does NOT hold the kept prize scrap** — that warded scrap is on
  **Windreth's** sheet (see THE SEAL above).
- **Windreth's arc-kit (on his sheet):** **_The Kept Name-Scrap (warded)_** (`identified:false`, THE SEAL — likely
  his own stolen name) + a **Token of Remembrance**. He keeps the warded scrap buried/cold (his 07-13 tail RP); it
  opens only on a proper reading — never at the scene, never on a failed roll. **DM-secrets held** (see [`campaign-arc.md`](campaign-arc.md)): the Order's
  right-to-refuse doctrine, Vale's patron = rival collector using her, the reassemble/scatter/hand-over trilemma.

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** (`GET /api/dm/situation` / `#dm-console`) first for `next_step` + the live board, then
> apply this intent. **Never roll/act/decide for the PCs** — players roll their own dice; verify any `/command`
> syntax against `internal/discord/commands.go` before advertising it in a coda.

1. **★ SESH, the deep market — rested; at a daylight vantage on Sabinnet's door, choosing the approach (out of combat).**
   Vale's **Intimidation 24** (nat 20 + Forge's Help) broke the back-clerk fully — **all intel delivered** (see
   Current scene): the **iron box** (cut name-scraps + coin + a stamped tally-token — NOT Windreth's name), the
   **ledger** (full scatter-routing + a who-read-whom column for Sesh this season), and the **prize** — **Sabinnet**,
   the *Reader-under-glass*, who reads the **sealed** names from a salt-white door behind the fish-stones in the deep
   market, and who **answers to the faceless buyer**. She's the gatekeeper for THE SEAL — but she's the enemy's hand,
   so reaching her is its own problem (deferred to a later beat).
   - **✅ CON RESOLVED (07-12):** the party chose the con — **Vale Deception 15** (advantage, Forge Helping as
     illusion-"Bertran" behind her) **held**. The runner took a **packet of loose scraps** as the night's read, raised
     no alarm, **never entered**, and is **walking back into the lane sure he's alone.** He gave up nothing — a courier
     has no buyer-address; **the buyer sits at the end of his route, not on his tongue.** Vale handed over only loose
     stock, so **THE SEAL stayed wrapped.** The con bought a clean exit and cover; the thread up-chain is now the tail.
   - **✅ BOTH SHED ROLLS RESOLVED (07-13) — queue empty, no pending roll:**
     - **Windreth `/check stealth` = 17** (advantage, Vale's illusion) — **tailed the runner clean, unmarked** past a
       carrier who checks his back twice. **No buyer** (scatter-doctrine: a courier's road ends at a hand, not a face),
       **but the tail converged the two threads** — the runner feeds the night's packet through a **slot in Sabinnet's
       own salt-white door behind the fish-stones**, so the reader-lead and the runner-road are **the same place**.
       Windreth marked the door, its **two watchers**, and the approach, then slipped off. **THE SEAL stayed shut** (he
       kept the living scrap buried, did not read it).
     - **Forge `/check investigation` = 10** (middle band) — no false-bottom cache, but netted the trade's **working
       kit**: rope, hooded lantern, travel food, a small purse of coin, a **fence's seal-kit**. **→ written to Forge's
       sheet** (`characters` id `d2d98745…`): `gold 15`; rows `rope-50ft ×1`, `lantern-hooded ×1`, `rations-1day ×3`,
       custom **Fence's Seal-Kit ×1**. (Loot-to-sheet is now a standing rule — [`dm-rules.md`](dm-rules.md).)
   - **✅ REST + APPROACH (07-17):** the party chose to **long-rest at the wagon and move on Sabinnet in the morning**
     (dewa; all three ran `/rest`, **bot-applied** — Vale's pact slots restored, death saves reset, HP full; **no DM
     mutation owed**). Narrated the quiet night (no ambush decreed on a rest they chose) → grey morning → the party
     down to a **covered daylight vantage on Sabinnet's salt-white door** (#the-story `1527548072874479727`).
   - **✅ APPROACH CHOSEN (07-17):** the party went **open — walked up as customers** (dewa: "approach openly,
     knocking as a regular customer… ask for reading service and probe for info about the buyer"; Forge "Buy"; **no
     Mask, own faces**). Narrated to the slot (#the-story `1527564400716677191`): watchers flank, a woman's voice
     opens the reading trade and asks who sent them.
   - **✅ COMBAT WON (07-17) — encounter `8431a89b` COMPLETED, both watchers dead, back out of combat.** The con collapsed
     on Forge's 14:11 Rage-breach; DM froze at the brink (`1527574640061583391`), the table went LOUD (all three
     `/initiative` in: Windreth 23, Forge 18, Vale 6), DM built the fight via in-page dashboard API and opened it (#the-story
     `1527585506932559906`), **no surprise round**. **Durable IDs (now closed):** map `db0a4d44` · template `8564bc2d` ·
     encounter `8431a89b`. **Enemy = 2× SRD Thug** (`creature_ref_id:"thug"`) as **"Sabinnet's Watcher"** (HP/AC SECRET) —
     `watch1` blocker + `watch2` coat, **both DEAD**. **R1:** Windreth Hide (Stealth 21) → Shortsword Sneak 15 + Nick-dagger
     4 on blocker; Forge two handaxes (11) → 2 HP; coat maced **Forge for 7** (Forge **NOT raging**; #the-story
     `1527589987988672653`); **Vale's Eldritch Blast (8) killed the blocker.** **R2:** Windreth **Steady Aim → Shortbow Sneak
     14** (coat 18/32); Forge whiffed both axes; **coat (DM-run) missed Forge** (nat 1) + **shouted through the door for
     help** (DM fiction; #the-story `1527598322980880436`); **Vale's EB 14** dropped the coat to **4**. **R3:** Windreth
     **Steady Aim → Shortbow Sneak 21** → **coat DEAD, last enemy down.** DM **ended combat** (Confirm End; status
     `completed`; End-Combat **auto-carried HP to sheets** — verified `characters`: **Forge 34/41**, Vale 31/31, Windreth
     31/31, no lingering conditions). Aftermath narrated (#the-story `1527600138061615175`). **➤ NEXT — out of combat, don't
     act for PCs:**
     - **The barred salt-white INNER door is still shut, but the interior is roused and waiting** — the coat's dying shout
       carried. Sabinnet (the Reader-under-glass) + any interior muscle are likely behind it.
     - **✅ APPROACH CHOSEN — BREACH (07-17, 16:00):** Forge unequipped both handaxes to **shoulder the door** (Jonathan).
       DM set the last-quiet beat + prompted the roll (#the-story `1527601414174674945`), Vale/Windreth given the go-loud
       window. **DC held secret = 15** (stout iron-banded wax-sealed barred door).
     - **🚪 FIRST SHOVE FAILED (16:15):** Forge `/check athletics` **= 10 vs DC 15 → door HELD** (resolved via DM Queue →
       Send Narration → #in-character; queue clear). Big noise — **interior now fully braced, no quiet entry left.**
       **Windreth (16:05) slid wide to flank the opening and READIED a shortbow shot** at the first hostile who reaches
       for names / casts / flees (ready persists until the door gives). **Forge (16:15) asked for a Help to retry with
       advantage.**
     - **🚪 SECOND SHOVE — DOOR BREACHED (16:20):** **Vale gave the Help** (advantage; Windreth kept his readied shot).
       Forge `/check athletics` **= 20 vs DC 15 → door GIVES.** Resolved via DM Queue → Send Narration (#in-character;
       queue clear). **➤ COMBAT TRIGGERED — reveal scene + initiative call posted** (#the-story `1527611410945671259`).
     - **⚔️ FRESH ENCOUNTER BUILT (07-17, 16:40) — awaiting the party's `/initiative` to start:** built via in-page API
       (map → homebrew creature → template). **Durable IDs:** map **`353c58b3-3844-4f4f-8a19-b38a73c0da47`** (Sabinnet's
       Reading Room, 12×10) · Sabinnet creature **`hb_84d6333d764f`** · template **`db9943fa-ea1e-41ca-976c-d9387bda110b`**.
       **Enemies (HP/AC SECRET):** **Sabinnet, the Reader-under-glass** (homebrew CR3 caster — AC 13, HP 58, INT/WIS save
       profs, *Mind Lance* +6 ranged 3d6 psychic + *Warding Rod* +4 melee 1d8+2 force; psychic-resist, charm-immune) at
       template **F/row1** (far north); **2× SRD Thug reflavored "Sabinnet's Housecarl"** (`creature_ref_id:"thug"`) at
       **D/row4** and **H/row4** (mid-room). *(template rows are 0-based.)*
     - **✅ COMBAT STARTED (07-17, ~17:24) — encounter `95f98525-3e70-47f0-ad74-583c612a0c73`.** All three `/initiative`
       came in via `/roll` in #roll-history (Forge d20+2=**19**, Vale d20=**19**, Windreth d20+4=**19**). Fired `POST
       /api/combat/start` (dice verbatim, APP-5 zero-override): `template_id:"db9943fa-…"`, the 3 PC ids,
       `character_initiatives` all `{roll:19}`, `character_positions` Windreth `C/9` (wide-west) · Forge `F/9` (breach) ·
       Vale `F/10` (behind) — combat-start rows 1-based. **Turn order (all 19 → engine DEX tiebreak):
       Windreth(1) → Forge(2) → Sabinnet(3) → Vale(4) → HC2(5) → HC1(6)**; no surprise. Enemies from template: Sabinnet
       **F1**, housecarls **HC1 D4** + **HC2 H4**. Combat-open beat (5:24 PM) + Round-1 recap beat (6:25 PM) posted (#the-story).
     - **✅ ROUND 1 played through Sabinnet:** (1) **Windreth** — `/bonus` steady-aim → Shortbow **Sneak on HC1 = 14** (HC1
       32→18); turn done. (2) **Forge** — `/action rage` (bonus; **Rage active**) → moved F9→**G5** → Handaxe **HC1 = 10 incl
       +2 Rage** (HC1 18→**8/32, reeling**) + vex; turn done. (3) **Sabinnet** (DM-run, Turn Builder → Review → Confirm & Post)
       — **Mind Lance vs Forge: 24 to hit → HIT, 16 psychic** (rage doesn't resist psychic; **Forge 34→18/41**); stayed at F1;
       End Turn. Live HP: WI 31/31 · FO 18/41 · VA 31/31 · SAB 58/58 · HC1 8/32 · HC2 32/32 (enemy HP secret).
     - **(4) Vale** — cast **Hold Person on Sabinnet**. Save resolved DM-side (Combat Manager → **Pending monster saves →
       Resolve save**): **Sabinnet WIS 25 vs DC 14 → SUCCESS → Hold Person FIZZLED**, Vale **lost concentration, pact slot spent,
       0 targets held.** Fizzle beat posted (#the-story 7:16 PM). **Her turn then auto-resolved on timeout** (no move — still F10).
     - **✅ (5) HC2 + (6) HC1 (DM-run, Combat Manager → Run Enemy Turn → Turn Builder → Review → Confirm & Post):** both maced
       **Forge**. **HC2: 22 to hit HIT, 2 bludg** (Rage halved 4→2; Forge 18→16). **HC1: 17 to hit HIT, 3 bludg** (Rage halved
       6→3; Forge 16→**13/41**). **HC1 charged D4→F5** via DM position-override (`POST …/override/combatant/{id}/position`, board-sync
       to 5ft reach). **Forge now FLANKED at G5** (HC2 H4, HC1 F5). Both beats → #combat-log. **End Turn ×2 → Round 2.**
     - **✅ ROUND 2 (1) Windreth (PC)** — `/bonus` Steady Aim → `/attack` Shortbow on **HC1 (F5)**: adv, **18 to hit → 17 piercing
       Sneak Attack** → **HC1 DEAD (0/32).** One housecarl down. Kill beat posted (#the-story 7:55 PM).
     - **✅ ROUND 2 (2) Forge (PC) freeform** — "take my thrown handaxe from the deceased F5 enemy" + "approve if makes sense."
       **DM approved as a FREE object interaction** (G5 adjacent to corpse F5): axe back in hand, no action/move cost. Both Forge
       freeforms ("rage" + handaxe) resolved & cleared in DM Queue.
     - **↩️ ROUND 2 (2b) Forge mis-swing + DM undo.** Forge moved **G5→I5** (adjacent HC2) and `/attack`ed → engine defaulted to
       **Unarmed Strike (5, HC2→27)** because throwing the axe last round left `equipped_main_hand` NULL. Player asked to redo w/ the
       axe. **DM undo (in-page fetch):** HP-override **HC2 27→32** (undo-last-action 422s on an attack — no before_state; used HP-override)
       + **`restore-action` on Forge** (`POST …/combatants/{FO}/restore-action`, no body → action_used=f, attacks_remaining=1; move NOT
       refunded, 15ft kept). Redo-prompt posted #the-story (8:06 PM).
     - **✅ ROUND 2 (2c) Forge handaxe redo RESOLVED.** `/attack ...weapon:handaxe` → **MISS (7, action)**, then bonus-action
       follow **HIT (23 → 5 slashing, +2 Rage)** on HC2 (**32→27**). Redo beat posted #the-story (5:58 AM 07-18). Turn ended.
     - **✅ ROUND 2 (3) Sabinnet (DM-run, Combat Manager → Run Enemy Turn).** **Mind Lance vs Forge, 10 to hit → MISS** (ranged
       from F1; Rage doesn't blunt psychic, just a low roll). Confirmed & posted #combat-log; whiff beat #the-story (6:03 AM). End Turn → Vale.
     - **✅ ROUND 2 (4) Vale (PC) DONE.** `/move` F10→**J6** (adjacent Forge; 10ft move left). **Bonus = administer Potion of Healing
       to Forge** (2024: within-5ft + bonus action), `/roll 2d4+2 = 7` → **Forge 13→20/41** (DM HP-override). Then **mis-cast Hex** (bonus)
       and asked to undo → **DM undo (in-page fetch):** `POST …/combatants/{VA}/concentration/drop` (broke Hex, removed `hexed` from
       Sabinnet) + `POST …/override/character/{VA}/slots` **pact 0→1 refunded**. **Consumed potion removed from Vale's sheet:**
       `POST /api/inventory/remove` {character_id:VA, item_id:potion-of-healing, qty:1} (DM-requested). **Action = `/cast` Eldritch Blast
       + Agonizing Blast on HC2** → hit 15, **11 force → HC2 27→16**. Heal+undo beat posted 6:50 AM; EB folded into R2-close beat 7:07 AM.
     - **✅ ROUND 2 (5) HC2 (DM-run, Combat Manager → Run Enemy Turn).** Turn Builder proposed **Mace vs Forge** (only PC in 5ft reach,
       Forge adjacent at I5). Review: **13 to hit → MISS** (Damage 0). Confirm & Post → #combat-log. **End Turn → dead HC1 (init 9)
       auto-skipped → Round 3.** R2-close beat (Vale EB + HC2 whiff) posted #the-story (7:07 AM 07-18).
     - **✅ ROUND 3 (1) Windreth (PC) DONE.** `/move` C9→**G5** (flank on HC2); `/bonus` Cunning-Action **Hide** (Stealth 23 → hidden
       from all hostiles); `/attack` **Shortsword** vs HC2 (adv, attacker hidden): 23 HIT, 7 piercing + **Sneak Attack**; `/attack`
       **Dagger** vs HC2 (adv, vex): **NAT 20 — CRITICAL**, 5 piercing → **HC2 to 4 HP — SURVIVED (the crit did NOT drop it).** ⚠ He double-spent his bonus
       (Cunning-Action Hide **and** the off-hand dagger both cost a bonus action) — the engine allowed both; HC2 was near-dead anyway,
       so **left as-is (engine action-economy gap), NOT reversed.** ⚠ CORRECTION (07-18): earlier text wrongly said this crit killed HC2.
     - **✅ ROUND 3 (2) Forge (PC) — action = the KILL; bonus refunded.** `/attack` **Handaxe** (21 HIT, 5 slashing + 2 Rage) → **HC2 4→0 — DEFEATED (Forge's kill).** Bonus
       `/attack` Handaxe (9 MISS) then whiffed the **already-dead** guard → wasted; per Forge's request that **bonus was REFUNDED** (one-time SQL `bonus_action_used=false`,
       user-approved; proper `restore-bonus-action` dashboard endpoint built + tested this session, redeploy pending). Guard-fall (11:57 AM) + undo-approval (12:19 PM) beats posted #the-story.
     - **✅ ROUND 3 (2 cont.) Forge DONE.** Moved I5→**G2** (adjacent Sabinnet F1), held the bonus, `/done` (12:20 PM) — declined the thrown-axe shot.
     - **✅ ROUND 3 (3) Sabinnet (DM-run, Combat Manager).** Mind Lance vs Forge: **24 to hit HIT, 11 psychic** (Rage doesn't blunt psychic) → **Forge 20→9/41**; stayed at F1.
     - **✅ ROUND 3 (4) Vale (PC).** `/cast` **Hold Person on Sabinnet** (12:36 PM; pact slot → **0 held**, concentrating). DM resolved the enemy WIS save via sanctioned in-page `POST /api/combat/{enc}/pending-saves/{saveID}/resolve` (ISSUE-043 monster-save resolver; engine rolled d20+5) → **WIS 10 vs DC 14, FAIL → SABINNET PARALYZED** (save-ends WIS DC 14, indefinite; fresh save at end of each of her turns). Paralysis beat posted #the-story (12:48 PM).
     - **➤ NOW — ROUND 3, Vale's turn still OPEN:** action + both pact slots spent (Hold Person holds her), **movement + bonus left. AWAITING her move-or-`/done` — do NOT act for her.** After Vale: **Windreth (PC)** into a **PARALYZED** Sabinnet — **every attack has advantage, any melee hit within 5ft is an AUTO-CRIT**; Forge already toe-to-toe at G2. **Sabinnet flees/alarms when bloodied (HP ≤29) — but held, she can't flee until she saves out.** **Only 1 enemy left: Sabinnet (F1, HELD).** Enemy HP/AC SECRET.
     - **⚠ Old encounter `8431a89b` is CLOSED** — this is a NEW encounter/template; do not touch the old one.
     - **⚠ Forge not raging:** his declared Rage never went up (bonus spent on the off-hand swing). Flagged to Jonathan;
       he can `/bonus`-Rage next fight. Do NOT retro-apply it. (Rage also would have ended by now — out of combat.)
       **⚠⚠ CONTRADICTS ENGINE (07-18):** in THIS Sabinnet fight Forge **IS** raging — `combatants.is_raging=t` + his handaxes log **+2 Rage**.
       This "not raging" note reads like a leftover from the prior watcher fight (`8431a89b`, where he wasn't). Left untouched (may be a
       concurrent DM-agent's edit — do not revert without confirming); flagged for the user to resolve.
     - **⚠ Standing hazard — the sealed scrap stays hidden:** do NOT let Windreth's warded scrap be read on Sesh's
       floor — it **thins the ward → the buyer feels it** (Mave). Loud means the buyer already has a reason to look
       this way; keeping the SEAL out of sight matters more, not less, now.
   - **THE SEAL still opens ONLY on a proper name-reading actually rolled** — the Sabinnet lead is a *route* to that,
     not the reading itself.
   Durable lore → [`world.md`](world.md); secret spine → [`campaign-arc.md`](campaign-arc.md).

2. **Onboard new players** as they arrive (`/register` → build in the portal via the tunnel → DM-approve on the
   dashboard → roster row + `party/<name>.md` sheet). Party scaling toward 5–6; see [`big-party.md`](big-party.md).

3. **After every beat:** narrate to #the-story (read-aloud block, OOC coda first, plain simple English) + append
   the play-by-play to [`sessions/`](sessions/) + keep this file's **Next action / Current scene** current. Pull
   the numbers you narrate from the Console; never hand-track them.

**Ops quickref:** app `localhost:8080`; tunnel UP (`make tunnel-up`, stable ngrok); DB read-only sanity via
`docker exec -i dndnd-db-1 psql -U dndnd -d dndnd` (`-i` for stdin/heredoc); redeploy after a code fix
`docker compose up -d --build app`. **Level-ups:** all 3 L4 feats applied (Vale +2 CHA→18, Forge GWM, Windreth
Defensive Duelist). **Loadout tip for Windreth:** shortsword main / dagger off-hand so main-hand Vex + off-hand
Nick both fire (ISSUE-061). **Forge's `player_characters` row is `status=rejected`** (stale since the 07-03 L4
rework — plays fine; his party-overview card is missing, so out-of-combat status edits go via
`POST /api/character-overview/d2d98745…/status`). **Open bugs to route around:** ISSUE-059 (DM-Queue Resolve
button fires no POST → resolve via `POST /dashboard/queue/<id>/resolve` from the authed tab), ISSUE-060 (builder
omits Warlock pact boon / invocations), ISSUE-070/071 (End-Combat transient-condition carry / long-coda split —
cosmetic).
