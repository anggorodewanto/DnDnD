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

_Last updated: 2026-07-17 — **SESH, the deep market — IN COMBAT at Sabinnet's door. Encounter `8431a89b`, end of Round 1;
Vale's turn open, Round 2 leads with Windreth.** The morning con collapsed into a **Rage door-breach** (Forge, 14:11); the
table went LOUD (all three `/initiative` in: **Windreth 23 → Forge 18 → watcher 7 → Vale 6 → watcher 1**). DM built the
fight via in-page API (map `db0a4d44` + template `8564bc2d` = **2× Thug as "Sabinnet's Watcher"**, HP/AC secret; enemy
init auto-rolled), opened it (#the-story `1527585506932559906`), **no surprise round.** **Round 1 played:** Windreth
(Cunning-Action Hide → Sneak, Stealth 21) & Forge chopped the **door-blocker** to 2 HP; coat-watcher (init 7) maced
**Forge for 7** (Forge 34/41 — **NOT raging**: his bonus action went to the off-hand handaxe, so the hit landed full);
**Vale's Eldritch Blast dropped the blocker (DEAD, 0 HP).** **➤ NEXT: Vale may reposition (K6) then click End Turn →
Round 2 leads Windreth (23). The coat-watcher (J2, untouched 32 HP) is the last enemy standing; the barred door is still
shut.** Don't act for PCs; enemy HP/AC behind the screen.
Prior beat: rested at the wagon (bot-applied long rest), walked up
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

**★ Now: SESH — Sabinnet's door, IN COMBAT. Encounter `8431a89b`, end of Round 1 (Vale's turn open); Round 2 leads Windreth.** The morning con
(walked up as customers; no Mask) collapsed when **Forge pivoted to a Rage door-breach (14:11)**; DM froze at the brink,
the **table went LOUD** (all three `/initiative` in). DM built the fight via in-page dashboard API and opened it
(#the-story `1527585506932559906`), **no surprise round** (open walk-up vs posted watchers). **The board (live state →
DM Console, do NOT hand-track here):** enemy = **2× SRD Thug** dressed as **"Sabinnet's Watcher"** — `watch1` (blocker,
H2) **now DEAD** + `watch2` (coat, J2, untouched **32 HP / AC 11 — SECRET from players**); current PC positions
**Windreth G2, Forge I3 (34/41, took a 7 mace), Vale K6.** **Turn order: Windreth 23 → Forge 18 → watch2 (7) → Vale 6.**
**Round 1 played out:** Windreth (Cunning-Action Hide, Stealth 21 → Sneak) + Forge chopped the blocker to 2; watch2 (coat)
maced Forge for 7 (Forge **NOT raging** — his bonus went to the off-hand handaxe, so full damage landed); **Vale's Eldritch
Blast finished the blocker.** **➤ NEXT — don't act for PCs:** Vale may reposition (K6) then click End Turn → **Round 2
leads Windreth (23).** Only the coat-watcher remains up and the door is still barred. ⚠ Loud is loud — a breach at
Sabinnet's door is the buyer's business; the sealed scrap still stays hidden (reading THE SEAL here thins the ward, the
buyer feels it, Mave). Sabinnet + any interior muscle can escalate as the fight drags. The party is deep in
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
   - **✅ COMBAT LIVE (07-17) — encounter `8431a89b`, end of Round 1 (Vale's turn open) → Round 2 leads Windreth.** The
     con collapsed on Forge's 14:11 Rage-breach; DM froze at the brink (`1527574640061583391`), the table went LOUD (all
     three `/initiative` in: Windreth 23, Forge 18, Vale 6), DM built the fight via in-page dashboard API and opened it
     (#the-story `1527585506932559906`), **no surprise round**. **Durable IDs:** map `db0a4d44` · template `8564bc2d` ·
     encounter `8431a89b`. **Enemy = 2× SRD Thug** (`creature_ref_id:"thug"`) as **"Sabinnet's Watcher"**: `watch1`
     blocker @H2 **now DEAD**, `watch2` coat @J2 **untouched (HP 32 / AC 11 — SECRET)**. Current PC positions Windreth
     G2, Forge I3 (34/41), Vale K6. **Turn order: Windreth 23 → Forge 18 → watch2 (7) → Vale 6.** **Round 1 played:**
     Windreth Cunning-Action Hide (Stealth 21) → Shortsword Sneak 15 + Nick-dagger 4 on blocker; Forge two handaxes (11)
     on blocker (→ 2 HP); watch2 (coat) maced **Forge for 7** (Forge **NOT raging** — his bonus went to the off-hand
     handaxe, so no resistance; #combat-log + #the-story `1527589987988672653`); **Vale's Eldritch Blast (8 force)
     finished the blocker.** **➤ NEXT — live state from the DM Console (don't hand-track HP/positions here):**
     - **Vale's turn is open** — she cast EB (her action) and may still move; on her word, click **End Turn** in the
       Combat Manager → **Round 2, Windreth up (23).** Resolve each PC action off their own `/`-command; **never
       roll/act for them.** Re-seat via `POST /api/combat/{enc}/set-active-turn {combatant_id}` if the tracker desyncs.
     - **Enemy turns = Combat Manager → yellow "Run Enemy Turn" → Review (server rolls) → Confirm & Post → End Turn.**
       The engine picks target + rolls; keep the rolled numbers (no fudging). Only the **coat-watcher (init 7)** remains.
     - **⚠ Forge not raging:** his declared Rage never went up (bonus spent on the off-hand swing). Flagged to Jonathan;
       he can `/bonus`-Rage on a future turn. Do NOT retro-apply it.
     - **Escalation:** Sabinnet + any interior muscle can join as the fight drags; she may flee / alarm the buyer.
       Build extra combatants into the same encounter if they enter.
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
