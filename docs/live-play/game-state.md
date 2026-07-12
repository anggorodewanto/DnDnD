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

_Last updated: 2026-07-12 — **SESH, the keeper's shed (out of combat).** The party ran the **con** on the buyer's runner at the speak-slot: **Vale Deception 15** (advantage, Forge Helping as illusion-"Bertran") **held** — the runner took a packet of loose scraps as the "night's read," raised no alarm, never entered, and is **walking back into the lane, sure he's alone.** ➤ Live beat: **Windreth's tail** (`/check stealth`, to follow the runner up-chain toward the buyer) + **Forge's `/check investigation`** of the shed — both awaiting player rolls; Vale holds the doorway. The runner is a careful carrier with **no buyer-address on him** (the buyer sits at the end of his route, not on his tongue), so following is the only thread up. The full night-road → Follower-kill → Mave's-wardens → Sesh → the-con arc lives in [`sessions/session-01.md`](sessions/session-01.md). Live board → DM Console; durable IDs/secrets below._
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

**★ Now: SESH — inside the keeper Bertran's fence-shed, mid-infiltration (out of combat).** The party is deep in
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
alone.** He gave up nothing to talk — a courier has no buyer-door; the buyer sits at the end of his *route*. **➤ The
live beat (07-12): the up-chain is now a chase — Windreth's `/check stealth` to tail the runner, plus Forge's
`/check investigation` of the shed; Vale holds the doorway.** **THE SEAL stays shut** (only loose stock changed
hands; the kept prize scrap never in play). (Full
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
- **⚠ THE SEAL (do NOT resolve on a low/wrong roll):** Vale carries the warded **kept prize scrap**
  (`identified:false`) — likely **Windreth's own stolen name**, strongly implied but **never confirmed**. It
  opens ONLY on a **proper reading**: a high `/check arcana` (risky), a higher warden, or **Sesh's name-market**
  — NOT insight / survival / perception / a failed roll. **Windreth's stolen true name** is recoverable only via
  this scattered-name arc (the man who could simply hand it back is dead).
- **Vale's kit (on her sheet):** the Faceless God's cold-iron token, the ashen face-shard, name-scraps (×2 + the
  grey man's cut-scrap bundle), the defaced **warden-disc**, the **kept prize scrap**, a **Potion of Healing**
  (2d4+2, unused), the Cold Iron Key. **DM-secrets held** (see [`campaign-arc.md`](campaign-arc.md)): the Order's
  right-to-refuse doctrine, Vale's patron = rival collector using her, the reassemble/scatter/hand-over trilemma.

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** (`GET /api/dm/situation` / `#dm-console`) first for `next_step` + the live board, then
> apply this intent. **Never roll/act/decide for the PCs** — players roll their own dice; verify any `/command`
> syntax against `internal/discord/commands.go` before advertising it in a coda.

1. **★ SESH, the keeper's shed — the con held; await Windreth's tail + Forge's search (out of combat).**
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
   - **➤ Await two player rolls (out of combat, queue empty) — never roll/decide either for them:**
     - **Windreth `/check stealth`** — peel out + **tail the runner** up-chain. He's a pro carrier watching his own
       back: a strong roll follows him to his drop / the next hand toward the buyer; **a blown roll marks Windreth**
       (the carrier clocks the tail → leads him wrong, bolts, or turns it into a fight). This is the live thread to the
       buyer now the con's done. If it turns into a fight, stand up an encounter (`POST /api/combat/start` with the char
       ids above + PC initiative per [[project_combat_start_pc_init_seat_repair]]).
     - **DM-SECRET — Forge's `/check investigation` (shed search, still pending), resolve on the roll:** box + ledger
       already theirs; this is hidden kit. **≥15** = false-bottom crate / floor-cache — a purse of real coin, a spare
       grey-clay buyer-chit, **plus a *sealed* courier-packet Bertran had prepped but not yet sent** (now **evidence /
       leverage**, NOT something to hand the runner — the handoff already happened). **10–14** = coin + working supplies
       (rope, hooded lantern, spare wax/thread for seals, travel food). **<10** = only bulky face-trade contraband he
       can't carry, and the rummaging is **loud** — but the runner's already walking, so it risks drawing the *lane* /
       a second set of ears, not the con.
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
