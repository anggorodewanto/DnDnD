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

_Last updated: 2026-07-01 — **IN THE BURIED GALLERY OF THE FACELESS GOD — OUT OF COMBAT, PARLEY LIVE; THE FACELESS GOD HAS ANSWERED (a TESTING half-success). Vale chose PERFORMANCE and rolled 11 (d20 6 +5) — middling tier: her telling wavers, so she buys the thing's ATTENTION, not its trust or safe passage. For the first time the dormant HEART does more than attend — it ANSWERS (not in words — it has unmade every mouth — but through the pull behind her ribs + the dead): the sentinel sinks to her level, emptiness eye-to-eye; the meaning is hunger dressed as courtesy — it will hear her (starved for exactly this), but a god of stolen faces TRADES ONLY IN KIND and doesn't yet believe she can pay — PROVE IT (a true lost story / a name / a face) or it will simply TAKE. Its blank attention flicked once to Forge's readied axes, then back. The waking dead lean a half-step closer and HOLD — "the offer bought attention, not leaving." Combat still NOT sprung. Forge (3:35 PM) has both handaxes drawn, guarding Vale, holding to strike ONLY if it sours (no attack → no initiative). AWAIT the players' next move — Vale: give proof / offer a costly token (name/face) / press-or-change the bargain / strike first; Forge: hold guard / add his voice / interpose / open the dance. On a strike or a soured bargain, BUILD THE GALLERY MAP + ENCOUNTER LIVE (masked sentinel + 2–3 nearest faceless dead, reserve Zombies; heart + crowd dormant; scale per big-party.md, 2-PC party). Never roll/act/decide for them. **PRODUCT BUG — FIXED (ISSUE-052, redeployed 07-01):** the #dm-queue resolver `POST /dashboard/queue/<id>/narrate` returned 503 while BOTH delivering the stub (#in-character 3:40 PM) and resolving the item (queue empty). Root cause: a cosmetic #dm-queue message edit gated the HTTP status, and the resolve was non-idempotent (retry double-posts). Fixed TDD — `editHandled` now best-effort; `ResolveSkillCheckNarration`/`ResolveWhisper` short-circuit on already-resolved. Redeployed. (Belt-and-suspenders: if a resolver ever errors again, still verify delivery + empty queue before any retry.)** _Prior:_ **IN THE BURIED GALLERY OF THE FACELESS GOD — OUT OF COMBAT, PARLEY IN PROGRESS. Vale answered the "you've been seen" cliffhanger by KNEELING and offering the faceless god a story-trade (common+infernal: "the gift and burden of stories lost — regale us, and we will spread your tale upon the realm") — the exact trade her patron deals in (a god of STOLEN FACES addressed by a STORY-COLLECTOR). DM gated it on a player-rolled CHARISMA check (Vale's pick: Persuasion / Deception / Performance; secret tiered DC — a low roll will NOT drop her into its jaws, it decides whether the heart takes her for a TELLER or a TRESPASSER, and the waking dead are only PAUSED not stopped). DM narrated the offer + the world leaning in (#the-story read-aloud 3:33 PM, Console timeline top): the dormant HEART attends for the FIRST time — starved of exactly "Story. Spread. Un-forgotten."; the nearest waking dead go still, poised (a held breath); the sentinel's faceless head tilts, listening past Vale to whatever owns it; ends "the dark is deciding whether it wants a teller — or one more face." Combat still NOT sprung. AWAIT (a) Vale's Charisma roll (#roll-history) and (b) Forge's declared stance — he has NOT reacted yet (last IC line predates detection); DM prompted him (back her / add his voice / interpose vs the nearest waking dead / ready a strike if it sours). On the roll, narrate the heart's verdict per tier (grants a hearing / stalls the waking / offer isn't enough and the dead resume); if it sours, BUILD THE GALLERY MAP + ENCOUNTER LIVE — masked sentinel + 2–3 nearest faceless dead (reserve Zombies), heart + rest of crowd dormant (dread reserve); scale per big-party.md (2-PC party). Never roll/act/decide for them.** _Prior:_ **IN THE BURIED GALLERY OF THE FACELESS GOD — OUT OF COMBAT, BUT CONTACT MADE; A FIGHT IS ONE BREATH AWAY. The party crept the idol's drag-scar toward the one aware "waiting shape," lantern held out as a decoy — then Vale's Stealth came up 6 (d20+0): her creep is HEARD, the cold attention slides off the lantern and levels the sentinel's blank face at HER. Forge's Perception 14 caught the tell in time — the nearest bowed "statues" are NOT stone, they are WAKING dead (the heart still dormant) — so the party is NOT flat-footed (first move theirs if they strike). Both checks resolved via the #dm-queue resolver (outcome stubs delivered to #in-character 1:26 PM; full beat #the-story). DM narrated the detection + posted a CLIFFHANGER ("you've been seen — what do you do?": strike / flee the entry throat / parley-turn it / clever). Combat still NOT sprung unilaterally — spotlight on the players for the break. ON THEIR DECLARATION, BUILD THE GALLERY MAP + ENCOUNTER LIVE: masked sentinel + 2–3 nearest faceless dead rousing now (reserve Zombies), rest of crowd + heart dormant (dread reserve); scale per big-party.md (2-PC party). Never roll/act/decide for them.** _Prior:_ **TWO PLAYER ROLLS PENDING — party crept toward the waiting shape; patron answered (faces lifted-off-and-worn; the faced-made-faceless-and-kept); called Vale Stealth + Forge Perception, tiered secret DCs.** _Prior:_ **IN THE BURIED GALLERY — Vale floated the lantern out with Mage Hand (auto-success) revealing the FACELESS STANDING CROWD bowed to the heart; the aware thing turned to the floating LANTERN not Vale; combat not sprung; spotlight to players.** _Prior:_ **THE PARTY CROSSED PAST THE COLD VAULT into the deeper dark — OUT OF COMBAT, standing at the lip of a vast buried gallery of the faceless god.** Both PCs short-rested (Forge's d12 hit-die rest now works — **ISSUE-051 fixed**), then both declared ready (#in-character: Vale *"Ready to go? …we are close"*; Forge *"more than ready"*) and crossed. DM narrated the crossing to #the-story (read-aloud) — the drag-mark of the pried idol leads down a worked-stone throat, past where the keeper's frightened graffiti stops, into a gallery too vast for the lantern where **something notices the light**; Vale's patron pull points dead ahead. **Spotlight on the players — await their next action in the gallery; never act/roll/decide for them.** This is NEW, unprepped territory. Full history below, newest at the end. — **COMBAT WON — the Cold Vault keeper is DESTROYED; the party is OUT OF COMBAT.** (History of the fight below, newest at the end.)**COMBAT LIVE — the Cold Vault boss fight is ON (Round 2).** The players turned the key (Forge *"follows to descend below"* 7:17 PM; Vale *"inserts the key… nodded to Forge and turns the key"* 7:54 PM), so the beat ran end-to-end: posted **block B** (the vault read-aloud — the door opening on grave-cold air, the chiselled-out shrine, the keeper rising in its frost-grey clothes; 7:55 PM, Post History confirmed) → opened **"Cold Vault — the keeper"** → **Start Combat** (~7:56 PM). **Live encounter `446dce33-e221-4d1f-a88b-4e81534b3399`.** Surprise adjudicated live = **none** (keeper dormant, PCs deliberately opened — standard initiative). **Round 1 order (from the Console): Forge (14) → Wight keeper (14, tie→Forge first) → Vale (4).** PCs entered bottom-center (the cold door); keeper top-center — ~40 ft gap to close. **R1 so far (06-30):** Forge advanced to F4 and **raged** — but ended his turn without attacking/taking damage, so by RAW the **rage lapsed at end of turn** (`is_raging=f`). The silent drop (no #combat-log / no DM-timeline notice) was **fixed live — ISSUE-041 FIXED**: rage expiry now posts to #combat-log + writes a `rage_expired` action_log row. The **keeper's turn was run from the workspace Turn Builder**: Longsword **hit Forge — 7 slashing** (Forge 25/32), posted to #combat-log + narrated to #the-story (read-aloud). **It is now Vale's turn (PC) — awaiting her slash command; do NOT act/roll for her.** **DM ruling applied (06-30):** Forge's player asked to undo his wasted R1 rage (his F4 move left him 15ft short, rage lapsed for nothing); **granted — rage charge refunded 2→3** via the in-combat Manual Override → Feature Uses (audited `dm_override` + auto #combat-log correction, player-👍'd). He keeps F4 + the hit stands; **DM Queue now empty.** Keep the keeper's HP/AC SECRET. Live board → DM Console. **Escalation in play: the keeper is UNDEAD → Vale's *hold person* will FAIL** (telegraph it the first time she tries). Live round/turn/HP/positions → **DM Console** (`#dm-console`), not this file. ISSUE-038 fixed: End Combat now AUTO-carries PC HP/conditions to the sheets (the manual carry-out footgun is gone). **Latest beat: Vale's Shatter hit the keeper — it failed its CON save (nat 1+3=4 vs DC 13), took 3d8=16 thunder, narrated to #the-story; still Vale's turn.** That cast surfaced + fixed ISSUE-042 (pact-slot cast-log), ISSUE-043 (DM monster-save resolver — new Combat/DM-Console "Resolve save" UI; see [`runbook.md`](runbook.md) §4), and the CRITICAL ISSUE-044 (AoE save-for-half damage never applied in prod — now fixed + idempotent). **R2 (06-30):** Round 2 is live — Forge's Greataxe (16) + Vale's dagger (1) left the keeper badly wounded; the keeper's **R2 Longsword MISSED Forge (11 vs AC 14)** and was narrated; **now Vale's turn (R2)**. Vale readied `hellish rebuke` via `/reaction` (still active — the miss didn't trigger it). ISSUE-045 fixed (`/reaction declare` now announces publicly, was ephemeral); ISSUE-046 logged OPEN (no path executes a reaction *spell* — "Resolve" is bookkeeping, `/cast` is turn-gated — so the DM hand-assembles it; proposed resolver mirrors ISSUE-043). **Undo grant (06-30):** Vale's R2 Shatter blast caught ally Forge; her `/undo` was **GRANTED** — the cast was **voided** (new **ISSUE-048** dashboard *Cancel* on the pending save → both `s2c3` saves forfeited, **no damage**, Forge unhurt 25/32), her **pact slot refunded 0→1**, and the `undo_request` queue item resolved. **Follow-up (06-30):** Vale reported she still couldn't recast — the cast had spent her turn's **action** and nothing restored it; surfaced + fixed **ISSUE-049** (new dashboard **"Restore Action"** button → her turn's `action_used`/`action_spell_cast` cleared, `attacks_remaining` reseeded to 1, movement untouched, audited, no HP leaked). **Now fully clear: Vale's action + pact slot are back and the blast is voided — awaiting her recast** (Shatter, further right, clear of Forge) — do NOT roll/act for her. **RECAST RESOLVED (06-30):** Vale recast Shatter clear of Forge (#combat-log *"Affected: Wight"* only); the keeper's CON save was resolved from the workspace (**5 vs DC 13 — Failure, 11 thunder**) and the blast left it **a breath from collapse — reeling, still upright** (HP secret → Console; Forge + Vale unhurt). Narrated to #the-story (read-aloud, 6:40 PM). **Still Vale's turn (R2), her ACTION now spent on the recast** (movement/bonus/reaction remain; *hellish rebuke* still readied) — **await her next command (`/move`/`/done`/…); never roll/act for her.** When she ends her turn, Round 3 opens with **Forge**, who will almost certainly finish the reeling keeper. **VICTORY (06-30, R3):** Vale's closing dagger throw **missed**; Round 3 opened with Forge, who **destroyed the keeper** with two thrown handaxes — first **hit (8) → "💀 Wight drops to 0 HP — defeated"**, second a **NAT 20 vex-advantage crit** (overkill). Narrated the kill to #the-story (read-aloud, 9:25 PM). **Combat ENDED** via End Combat → encounter `446dce33-…` `status=completed`, *"Combat ended — The Cold Vault"* in #combat-log. **ISSUE-038 auto carry-out worked** — sheets now **Forge 25/32, Vale 24/24, no conditions** (no manual reconcile). **No active encounter.** The Cold Vault is **cleared and theirs**: the shrine stands hollowed/empty, the cold door open, the deeper dark unexplored. **Spotlight to the players — await their next action; never narrate their choices.** **POST-COMBAT BEAT (06-30):** Vale's player opened the exploration — *"Vale gives Forge a thumb up. Then examines the shrine"* (#in-character, 9:44 PM, Discord-only). Board reconciled quiet (no encounter, queue empty, action_log ends at the kill). Narrated the shrine read-aloud (#the-story, 9:50 PM — worn altar, a niche with **something recently pried/chiselled out**, the forgotten god's name gouged out over and over by a frightened hand, the cold off the stone, **Vale's patron-pull drawing tight: *this* is the forgotten god it set her chasing**) and called for an **Investigation check** — secret **DC 13**, ruled **tiered** (low roll still gives the obvious + her patron's certainty; the breadcrumb is never hard-blocked). **SHRINE FIND RESOLVED (06-30):** Vale called Forge in to help (#in-character 9:53 PM); both rolled Investigation (#roll-history) — **Vale 22 (NAT 20)** smashes DC 13, **Forge 9** under it. Narrated the find (#the-story read-aloud, 9:57 PM): Forge's craft eye reads the niche idol was **recently pried out** (pry-bar/cold-chisel, fresh cuts, keeper's tools nearby); Vale's nat-20 reads the **ritual** carve/erase (a name carved to *call* the god, scratched out to *un*-call it, for years), a **surviving fragment of the name**, and a **faceless-god** relief — a **forgotten god of stolen faces**, its idol carried off **through the cold door**; her **patron surges (hot/fed)** in recognition — this is the story it set her chasing. **Threads now tied:** forgotten god + "wearing their own faces" + the pried idol gone into the dark = the campaign's next pull, pointing past the open cold door. **Spotlight back to the players — await their next action; never narrate their choices.** **REST BUG FIXED (ISSUE-050, 06-30):** both PCs' `/rest short` was silently gated waiting for DM approval (no resolvable rest path) — root cause a contradiction in the rest auto-approve default (`AutoApproveRestEnabled()` returned false on the nil default though the field docs say nil = auto; this campaign's settings omit `auto_approve_rest`). Fix-now TDD: flipped nil-default → true, redeployed; cleared the 2 stale `rest_request` queue items. **Players re-run `/rest short`** → ephemeral hit-dice buttons (they pick HD, bot rolls 1dX+CON per click; HD return only on `/rest long`, half level). **HIT-DICE BUG FIXED (ISSUE-051, 07-01):** Forge's `/rest short` hit-die click crashed with *"invalid hit die type: barbarian"* — `HitDiceRemaining` was persisted keyed by class name (`{"barbarian":3}`) not die string (`{"d12":3}`); two producers (builder persist path + `DeriveStats`) keyed by `c.Class` while every consumer keys by die. Fixed both to key by `ClassHitDie()` with `+=` (TDD, committed `03642e2`, redeployed); re-keyed the two corrupt live rows out of band (Forge `{"d12":3}`, Vale `{"d8":3}`, counts preserved). Both PCs' hit-dice spends now heal. **CROSSED INTO THE DEEPER DARK (07-01):** after resting, both players declared ready and the party crossed past the cleared Cold Vault; DM narrated the crossing (#the-story read-aloud, Console timeline top) — the pried idol's drag-mark leads down a worked-stone throat, past where the keeper's carve/erase graffiti stops, into a **vast buried gallery of the faceless god** (a hundred eyeless ovals) too big for the lantern, its floor lost under unnamed standing shapes; out in the dark the drag-mark ends and **something notices the light — a slow turning of cold attention**; Vale's patron pull points dead ahead, hot and close. **Now at the lip of the gallery — spotlight on the players; await their next declared action (advance / light / stealth / call out / examine the shapes); never act/roll for them.** NEW unprepped territory — if it becomes a fight, build the map + encounter live (cold-vault design has reserve Zombies; scale per big-party.md). **GALLERY LIT (07-01):** Vale's next declared move was **Mage Hand to float the lantern up and out** (12:21 PM #in-character) — cantrip, lantern under the 10-lb limit ⇒ **auto-success, no roll**. DM narrated the reveal (#the-story read-aloud, Console timeline top): the raised light pushes the dark back only a slice (gallery too vast for it) and the **near standing shapes resolve into figures — a close-packed crowd, men/women/one child-height, worked from (or wearing) grey stone, all bowed toward the gallery's unseen heart, all FACELESS** (eyes+mouth smoothed to blank ovals, bowed the way the idol's drag-mark runs); the light does NOT reach the ranks behind or whatever they face. **Tactic rewarded:** the aware thing tracks the **floating lantern, not Vale** — the cold leans toward the light and **one smooth grey face far back turns to point at the lantern** while the rest stay bowed (telegraph, no stat line; specifics secret). A dread/choice beat — **combat NOT sprung** on a scouting action. **Spotlight back on the players — await their next move (advance / pull the light back / examine the faceless figures / stealth toward the heart / retreat); never roll/act/decide for them.** If they advance or the aware figure closes, **build the gallery map + encounter live** then (reserve **Zombies** = the faceless standing dead; scale per big-party.md). **DM narration — Forge's darkvision (60 ft), #the-story read-aloud:** colorless-grey extension of the reveal — the figures **fill the hewn gallery rank on rank** (not a knot at the door), walls carry the eyeless ovals, the drag-mark runs a road through the crowd, but the **heart they bow to sits past even his 60 ft (still black)** — mystery kept; he confirms low + certain the **one face tracking the floating lantern**. Posted to #the-story as narration at the player's request (Console timeline top). Board: out of combat, DM Queue empty, no active encounter.
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

1. **▶ IN THE BURIED GALLERY OF THE FACELESS GOD — out of combat, PARLEY LIVE; THE FACELESS GOD HAS ANSWERED (a
   TESTING half-success); a fight is one breath away. Spotlight on the players.** **Latest (07-01):** Vale chose
   **Performance** and rolled **11** (#roll-history 3:36 PM) — middling tier: her telling wavers, buying the thing's
   **attention, not trust or safe passage**. DM narrated the god **ANSWERING for the first time** (#the-story read-aloud
   3:40-ish, Console timeline top — `POST /api/narration/post → 201`): the sentinel sinks to her level, emptiness
   eye-to-eye; *hunger dressed as courtesy* — it will hear her (starved for exactly this) but **trades only in kind**
   and doesn't believe she can pay — **PROVE IT** (a true lost story / a name / a face) or it will **TAKE**; its blank
   attention flicked once to Forge's drawn axes, then back; the dead lean a half-step closer and **HOLD** ("the offer
   bought attention, not leaving"). **Combat still NOT sprung.** Forge (3:35 PM) has both handaxes out, guarding Vale,
   holding to strike **only if** it sours (no attack → no initiative). **AWAIT the players' next move** — Vale: give
   proof (another Performance/Persuasion, or name the tale) / offer a costly token (name/face) / press-or-change the
   bargain / strike first; Forge: hold guard / add his voice / interpose vs the nearest waking dead / open the dance. On
   the move, narrate the heart's verdict per tier; **on a strike or soured bargain, BUILD THE GALLERY MAP + ENCOUNTER
   LIVE** (masked sentinel + 2–3 nearest faceless dead, reserve Zombies; heart + crowd dormant; scale per big-party.md,
   2-PC party). **Never roll/act/decide for them.** **✓ PRODUCT BUG FIXED (ISSUE-052, redeployed 07-01):** the #dm-queue
   resolver `POST /dashboard/queue/<id>/narrate` returned **503 while SUCCEEDING** — Vale's Performance stub both
   delivered (#in-character 3:40 PM) and marked the item resolved (`pending: []`). Root cause: a cosmetic #dm-queue
   message edit gated the HTTP status + a non-idempotent retry double-posts. Fixed TDD (`editHandled` best-effort +
   already-resolved short-circuit in the skill-check/whisper resolvers), redeployed. (Prior CONTACT MADE → parley-offered beats now history — ✓ below.) The Cold Vault fight is long won (keeper `446dce33-…` destroyed,
   `status=completed`); shrine find resolved; both PCs short-rested and crossed into new, unprepped territory; the
   gallery is LIT (Vale's Mage Hand lantern) + Forge's darkvision read it rank-on-rank. **Newest beat (07-01):** the
   party crept the idol's drag-scar toward the one aware **"waiting shape,"** lantern held out as a decoy — then the two
   checks landed (#roll-history):
   - **Vale Stealth d20+0 = 6 → creep HEARD.** The cold attention slides off the decoy lantern and levels the sentinel's
     blank face at **her** (dry breath quickening). She is **detected**.
   - **Forge Perception d20+0 = 14 → good read.** Dwarf-eyes catch the tell: the nearest bowed "statues" are **not
     stone** — a grey hand uncurls, dust sifts from a rising shoulder — the gallery's dead are **waking**; the heart
     stays dormant. Party is **NOT flat-footed** (first move theirs if they strike).
   - (Both flat +0 = real char mods — `/check` does apply mods, e.g. Vale's Investigation +2; no bug.) Both
     `skill_check_narration` queue items **resolved via the #dm-queue inline resolver** (Send Narration → outcome stubs
     delivered to #in-character 1:26 PM; full beat #the-story). **DM Queue empty.**
   - DM narrated the **detection beat** (#the-story read-aloud, Console timeline top) + posted a **CLIFFHANGER**:
     *"Vale, Forge — you've been seen. What do you do?"* (strike first / break for the entry throat / speak-to-or-turn
     it / something cleverer). **Combat NOT sprung unilaterally** — the tiered low-Stealth outcome = detection + rising
     stakes, players keep the choice of the break.
   **NEXT: await the players' declared reaction. ON THEIR DECLARATION, BUILD THE GALLERY MAP + ENCOUNTER LIVE** — the
   aware **masked sentinel** (the "waiting shape") + the **2–3 nearest faceless dead** rousing now (reserve **Zombies**),
   the rest of the crowd + the **heart** still dormant (dread reserve). No board prepped past the Cold Vault; scale per
   [`big-party.md`](big-party.md) (currently a 2-PC party). **Never act, roll, or decide for them** (per
   [`dm-rules.md`](dm-rules.md)). The old lit-gallery / cleared-vault / shrine-find beats below are now **history** (all
   ✓), kept for reference.
   - **✓ SHRINE FIND RESOLVED (06-30).** Vale + Forge examined the shrine together — **Vale Investigation 22 (NAT 20)**
     vs DC 13, **Forge 9**. Narrated (#the-story 9:57 PM): the idol was **recently pried out** (Forge's craft read); the
     scarring is a **call/un-call ritual**, a **fragment of the name survives**, and the relief is a **faceless god — a
     forgotten god of stolen faces**, its idol gone **through the cold door**; Vale's **patron surges in recognition**.
     This is now the campaign's central pull, pointing past the open cold door into the deeper dark. **Spotlight is back
     on the players** — await their next action (follow the pull through the cold door, search the keeper's remains, or
     rest); never narrate their choices. The scene to react to (don't pre-empt
   their search): the **shrine stands hollowed and empty** — *something was chiselled out of it* (the journal's
   "wearing their own faces" thread); the **cold door is open** behind where the keeper fell; the deeper dark is
   **unexplored**. When they search/examine, adjudicate their rolls (they roll their own dice) and narrate the find to
   #the-story (read-aloud). Likely next beats: examine the shrine / the keeper's remains, push past the cold door into
   the deeper dark, or pull back to rest. Vale's patron is steering her downward (her stated *"i have no choice"*).
   No reserve husks were used; if a new threat is warranted later, the design has Zombies in reserve
   ([`encounters/cold-vault.md`](encounters/cold-vault.md)). **Still-live post-fight hooks:**
   - **✓ Journal read (1:38 PM).** It surfaced the **cold door** — an old vault lower than the cellar that the
     keeper unlocked; the wretches **fled up from it**, so the cellar door was clawed from inside to *escape*
     the cold door, not to reach the keeper. The keeper's last line: *"the cold iron key locks the cold door.
     Do not turn it."* This is now the campaign's central pull downward. (Beat logged in `sessions/session-01.md`.)
   - **✓ The cold iron key — USED.** Vale turned it (against the keeper's warning), the cold door opened, and the
     keeper rose: the Cold Vault boss beat. Key spent on the door; still on Vale's sheet as a quest token.
   - **✓ Descend + the boss beat — DONE (06-30).** The party descended, turned the key, and **won** the Cold Vault
     fight (the Wight keeper destroyed; combat ended). What's *deeper* than the vault — past the now-open cold door,
     the chiselled-out shrine, the journal's "wearing their own faces" — is the **new** unexplored thread. Design /
     history for reference: [`encounters/cold-vault.md`](encounters/cold-vault.md), `sessions/session-01.md`.
   - **The healing draught (on the sheet, unused):** if a PC drinks it later it restores **2d4+2** — **the
     players roll it** ([`dm-rules.md`](dm-rules.md)). Apply via **Party → Edit status** (add to current HP,
     capped at max) and decrement the potion in Manage inventory. At full HP now it's a saved resource for the
     next fight.
   - **Drop-to-0 logging gap — looks RESOLVED (06-30, verify before closing).** The same scenario that silently
     failed on 06-28 (G1) now **fired correctly**: Forge's `/attack` handaxe dropping the keeper posted
     *"💀 Wight drops to 0 HP — defeated"* to #combat-log + a `downed` action_log row. So the player `/attack` path
     now does funnel the drop notice. Confirm against the ISSUE log / code before marking the old gap closed.
2. **Onboard new players** as they arrive (`/register` → build → DM-approve → roster row + sheet →
   fold into the fiction). 3-4 more PCs expected. See [`runbook.md`](runbook.md) "Onboarding
   players" + [`big-party.md`](big-party.md).
3. **After every beat:** narrate to #the-story (read-aloud) + append the narrative
   [`sessions/`](sessions/) log. Do **not** transcribe HP/positions back into this file.
