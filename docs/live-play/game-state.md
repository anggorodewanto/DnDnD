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

_Last updated: 2026-07-18 — **SESH, the deep market — OUT OF COMBAT, searching Sabinnet's
reading room.** The reader fight is **WON** and **Sabinnet is CAPTURED ALIVE** (unconscious +
stable — pre-declared non-lethal; new house rule, see [`dm-rules.md`](dm-rules.md)); encounter
`95f98525` is `completed` (Windreth 31/31, Vale 31/31, **Forge 9/41**). **First search resolved:**
Forge (Sleight of Hand 17) frisked her + opened the locked desk drawer → key-ring, reading-lens,
oilcloth packet, waxed name-scraps, black sealing-wax, faceless signet; Vale (Investigation 8, weak)
got only 2 unlabeled vials + confirmation the ledgers/scraps are in the fence's faceless cipher —
the real intel sits **behind Sabinnet herself**. Loot **written to sheets 07-18** (Forge: key-ring,
reading-lens, oilcloth packet, waxed name-scraps, black wax, faceless signet; Vale: 2 vials — arc
items recorded `identified:false`/sealed so possession ≠ reading).
**➤ NOW:** room theirs, **not yet bound/woken**; alarm-cord **never pulled** → interior **not
alerted**. Await the party's next move (bind / wake + interrogate / search harder / short rest) —
don't act for them. **THE SEAL stays shut** — Windreth's warded scrap opens only on a proper
reading, never at the scene. Full blow-by-blow → [`sessions/session-01.md`](sessions/session-01.md);
live board (when a fight stands up) → DM Console; durable IDs/secrets below._

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

**★ Now: SESH — Sabinnet's reading room, OUT OF COMBAT (aftermath).** The party breached the
barred salt-white inner door (Forge Athletics 20 w/ Vale's Help vs secret DC 15) and won the
reader fight over four rounds (both housecarls down; Sabinnet paralyzed by Vale's Hold Person,
crit-battered while held, then finished at range). **Sabinnet is CAPTURED ALIVE** — senseless
behind her desk, her pale **rod fallen out of reach**, the **alarm-cord never pulled** (nothing
outside alerted *yet*). The room is full of paper, ledgers, and glass cases. The party can
**secure/bind her**, **search the room**, or **wake and interrogate** her — she's the sealed-name
reader they came for, and Windreth carries **THE SEAL** that may belong to this very desk.
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
- **⚠ THE SEAL (do NOT resolve on a low/wrong roll):** **Windreth** carries the warded **kept
  prize scrap** — on his sheet as **_The Kept Name-Scrap (warded)_** (`identified:false`) — likely
  **his own stolen name**, strongly implied but **never confirmed** (verified on the sheets 07-14:
  it is on **Windreth, NOT Vale**). It opens ONLY on a **proper reading**: a high `/check arcana`
  (risky), a higher warden, or **Sesh's name-market** — NOT insight / survival / perception / a
  failed roll. The Sabinnet lead is a *route* to that reading, not the reading itself.
- **Vale's kit (on her sheet):** the Faceless God's Token, the ashen face-shard, the **Name-Scrap
  of the Faceless God**, the **Grey Man's Name-Scraps (bundle)**, the defaced **Renegade's
  Warden-Disc**, the Cold Iron Key (×2). (Her **Potion of Healing** was **spent on Forge** during
  the reader fight — patched him to 20/41 mid-combat; consumed, off both sheets.) **⚠ Vale does
  NOT hold the kept prize scrap** — that warded scrap is on Windreth's sheet (see THE SEAL).
- **Windreth's arc-kit (on his sheet):** **_The Kept Name-Scrap (warded)_** (THE SEAL) + a **Token
  of Remembrance**. He keeps the warded scrap buried/cold; it opens only on a proper reading.
  **DM-secrets held** (see [`campaign-arc.md`](campaign-arc.md)): the Order's right-to-refuse
  doctrine, Vale's patron = rival collector using her, the reassemble/scatter/hand-over trilemma.

## Next action (DM intent — the one thing the Console can't infer)

> Open the **DM Console** (`GET /api/dm/situation` / `#dm-console`) first for `next_step` + the
> live board, then apply this intent. **Never roll/act/decide for the PCs** — players roll their
> own dice; verify any `/command` syntax against `internal/discord/commands.go` before
> advertising it in a coda.

1. **★ SESH, Sabinnet's reading room — AFTERMATH, out of combat.** The fight is won and
   **Sabinnet is captured alive**; the party has the room and the initiative. Await their RP /
   rolls — don't act for them. Likely beats to adjudicate:
   - **Secure her / the room** — bind + gag Sabinnet before she wakes, take the fallen rod. **First
     search already resolved 07-18** (Forge SoH 17 → key-ring, reading-lens, oilcloth packet, drawer:
     waxed name-scraps + black wax + faceless signet; Vale Inv 8 → 2 unlabeled vials, ledgers stay
     ciphered). Loot **written to sheets 07-18** — Forge holds the desk kit (key-ring, reading-lens,
     oilcloth packet, waxed name-scraps, black wax, faceless signet), Vale the 2 vials; the arc pieces
     (packet, scraps, signet, vials) are recorded `identified:false`/sealed so **possession ≠ reading**
     — the SEAL and the deep intel stay gated behind Sabinnet, not a search roll. Deeper digging =
     another `/check skill:investigation` (with time).
   - **Wake + interrogate** — she's the sealed-name Reader-under-glass and the whole reason they
     came; question her (`/check skill:intimidation` / `skill:persuasion`) about the buyer, the
     sealed names, and how a kept name gets read. Follow the adjudication patterns in
     [`dm-rules.md`](dm-rules.md) (reward unforced RP; a strong check shows the *body*, not the
     *name*; don't dump the sealed core).
   - **Housekeeping:** Forge is at **9/41** (short-rest / heal candidate); his old thrown handaxe
     is still in the dead housecarl (trivial post-combat pickup). Loot → the finding PC's sheet
     (standing rule, [`dm-rules.md`](dm-rules.md)).
   - **⚠ Standing hazard — keep THE SEAL hidden:** do NOT let Windreth's warded scrap be read on
     Sesh's floor — reading it thins the ward and **the buyer feels it** (Mave). Loud already gives
     the buyer a reason to look this way, so keeping the SEAL out of sight matters *more* now.
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
