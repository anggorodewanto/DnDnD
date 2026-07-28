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

_Last updated: 2026-07-27 — **compaction pass.** The previous revision had grown a
~150-line round-by-round transcript of an encounter that closed on 07-23, in flat
violation of the charter above; that belongs in [`sessions/`](sessions/), not here, and
it is gone. Scene, party board, and Next action were **eight days stale** (they still
read "SESH, moments after THE SEAL was read", party at L4, Forge 41 max HP) and are now
current to the live beat. **Party is L5**; live board below is DB-verified._

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
  (see [`runbook.md`](runbook.md) §1). `make local-up` is attached — scripted redeploys
  need `docker compose up -d --build`.
- **Remote-player tunnel:** an **ngrok tunnel on a reserved domain** exposes the local app so
  remote players reach the builder + OAuth. **URL is STABLE**
  (`https://unhustling-cushionless-karan.ngrok-free.dev`), so the OAuth callback is registered
  in Discord **once** and never changes. How-to + `make tunnel-*` targets:
  [`runbook.md`](runbook.md) "Remote players"; one-time setup + `NGROK_DOMAIN`/`NGROK_AUTHTOKEN`
  in `.env`: header of [`scripts/tunnel.sh`](../../scripts/tunnel.sh).
  - **OAuth callback (registered, stable):**
    `https://unhustling-cushionless-karan.ngrok-free.dev/portal/auth/callback` — no per-restart
    Discord change. `make tunnel-up` always yields this URL; `make tunnel-down` restores `.env`
    to `localhost` while keeping the ngrok vars.
- **All live-play mutations go through the dashboard's authenticated in-page `fetch`**
  (claude-in-chrome, `credentials:"include"`) — never raw SQL or curl. DB reads for sanity
  checks are fine.

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

## Party (durable — live HP/slots via the Console)

| PC | Player | Class (L5) | Char ID | The numbers that keep mattering |
| --- | --- | --- | --- | --- |
| **Windreth** (he/him) | posts as `Windreth` | Rogue 5 Thief, elf | `b2c436da-6762-458f-8016-3fe8f18e35e6` | Stealth **+10** & Investigation **+6** (both expertise) · SoH +7 · Perception +5 · STR 8 |
| **Forge Anvilbearer** (he/him) | `JonathanEka` | Barbarian 5 Berserker, dwarf | `d2d98745-d322-4380-924f-3296a0c447b7` | Athletics +5 · Insight +3 · greataxe is two-handed (a shield costs him GWM) · **AC 14 while stripped in the culvert**, 16 with the breastplate back on |
| **Vale** (she/her) | `dewa` (the user) | Warlock 5 Fiend, tiefling | `b6ca7f49-c173-4290-8c80-6fb785fbe733` | Deception **+7** (proficient) · Persuasion +4 (raw CHA 18, *not* proficient) · Stealth **+0** · AC from *Armor of Shadows*, no armour equipped · spell DC 15 |

`dewa` is the user's own account and also speaks for the party as a whole ("we pivot…") —
a declaration from `dewa` is not automatically Vale's action.

- Tome cantrips live in `granted_spells`, not `spells` (ISSUE: builder omits pact boon/invocations).
- **Forge's `player_characters` row is `status=rejected`** (stale since the 07-03 L4 rework — plays
  fine; his party-overview card is missing, so out-of-combat status edits go via
  `POST /api/character-overview/d2d98745…/status`).
- **⚠ Never read worn/held gear from the `inventory` jsonb.** The scalar columns
  `equipped_main_hand` / `equipped_off_hand` / `equipped_armor` are what combat actually reads, and
  the two stores drift. Caught live 07-27: Forge's jsonb listed **no armour at all** while
  `equipped_armor` said `breastplate` (the real source of his AC 16) — the DM narrated a strip as
  costing nothing and had to correct it publicly. In the same sheet the jsonb had **Greataxe *and*
  Handaxe both `equipped:true` in `main_hand`** while the scalar correctly held only the greataxe —
  the recurring duplicate-slot bug. Both reconciled by a builder PUT (`worn_armor:""`, plus pushing
  `breastplate` onto `equipment` so it survives as carried-not-worn); the PUT is additive, so HP,
  gold, rage uses and all quest items were preserved. Note the field names differ between layers:
  the builder submission calls it `equipped_weapon` / `worn_armor`, the table calls it
  `equipped_main_hand` / `equipped_armor`.
- **⚠ Known sheet-vs-fiction drift, opened 07-27:** the **breastplate is still in Forge's
  `equipment` as carried-not-worn** while, in the fiction, it is in the bottom of a watch skiff
  going down-cut. **AC 14 is correct either way**, so no mutation was made — remove it from the
  sheet only if the party abandons the chase. Flagged here so it doesn't quietly rot into the same
  two-stores drift described above.

## Maps

| Map | ID | Notes |
| --- | --- | --- |
| The Counting Floor — the stone quay | `83414d56-eba0-4a7b-bf66-ad34d860fe33` | 20×12 @48px. Interior cols H–S rows 4–9; **front door = cols M,N in the south wall, the only gap**. Lighting layer deliberately all-zero (mundane dark is adjudicated in fiction; only `magical_darkness` is engine-enforced). |
| The Night Weigh — the canal weighhouse | `09a89fb3-1885-4f30-9189-8328d3b5fdd2` | 17×13 @48px. Canal rows 1–2, quay 3–5, shed 6–11, office M–O 9–11. |
| Sabinnet's Reading Room | `353c58b3-3844-4f4f-8a19-b38a73c0da47` | 12×10. Sesh reader fight (won 07-18). |
| Palewatch — kept vault | `cc356cc4…` | 14×10, **zero walls** — the reusable open box; re-skinned as the chandlery store-room. |
| Ashfall waystation (common / cellar / cold vault) | `1ad14481…` / `d2fe03c6…` / `2899165e…` | 12×10 blanks. |
| Buried Gallery of the Faceless God | `39ecd023-51d8-44bb-bf8e-29e1eff3a231` | 12×12 blank stone. |

**Always flood-fill a hand-authored map on the PERSISTED bytes before using it** — one map once
shipped as five sealed pockets. Walls are edge *segments* (w/h = 0): rasterize by segment.

## Encounters — none active

The party is **out of combat**. Closed encounters, newest first (full chronology in
[`sessions/session-01.md`](sessions/session-01.md)):

| Encounter | id | Outcome |
| --- | --- | --- |
| The Collector (counting floor, R2) | `4c0e014c-43ae-4017-9243-b6771e7cd710` | **Took terms** — walked alive on the party's word, 07-26 |
| The Counting Floor | `6a7ba24e-aca9-4415-8009-2fbcff83f285` | Victory 07-26 — CT2 + tally-runner dead, CT1 alive; alarm never went out |
| The Far-Quarter Front — the breach | `a46b1472-72c6-49aa-a36c-2343bd613299` | Victory 07-23 — 4 captured alive |
| The Night Weigh | `d23ab3d7-127a-4fbe-b689-b380ae617252` | Victory 07-24 — all 5 KO'd **non-lethal** |
| The Unlit Rows (walls + fog) | `6fffbb99…` | Victory 07-21 — party milestone-levelled L4→L5 |
| Sabinnet's reading room / her watchers | `95f98525…` / `8431a89b…` | Victory 07-18 / 07-17 — Sabinnet **captured alive** |
| The night road — the Follower | `30baba5f…` | Victory 07-09 |
| Palewatch — the kept vault (the grey man) | `2846a6ca…` | Victory 07-07 — the party chose the **kill over the parley**; do NOT resurrect/recur him |
| Buried Gallery / Cold Vault / Cellar | `9e558982…` / `446dce33…` / `8509d1f6…` | Victories 07-03 / 06-30 / 06-28 |

## Current scene (narrative framing — non-derivable)

**★ Now: THE FAR QUARTER, night of 07-27 — the culvert under the Stair Hill yard.**

The party is deep in **Leg 3b, "the paper trail"** — hunting the faceless buyer's *money and
paper*, never his face, exactly as the leg's design intends. The chain they have walked:
weighhouse → drop-barge → the stone-quay counting floor → a cover-up on a clock → **the Quiet
Window** (a night-baker's back shutter where every collector's paper goes in and nothing comes
out) → the bakery → **the bread cart's four-stop round** → **the private yard at the top of
Stair Hill**.

**What the yard is (earned on rolls, table-confirmed):** it **unmakes paper**. The culvert silt
below it is packed with years of pulped paper and names in metallic ink that will not stay in
the eye. It also **eats memory** — day-hands hired at the gate-clerk's stool for 8 silver (the
docks pay 2), one morning only, never twice, cannot remember anything past the **chalk line**.
The clerk's book takes a **name**, in dark metallic ink, nib wetted in **grey salt** — the same
salt laid unbroken along the wall's top course, and the same salt as the Name-Muffle in Vale's
kit. A name spoken inside the yard slides too: Windreth heard one from the culvert and *"by the
time it reaches him it isn't there — not forgotten, never arrived."*

**The players' own deduction (theirs, deliberately unconfirmed):** the carter and the washer
both cross that line routinely and neither is a blank; only the *listed* are. So it's the
**book** — your day goes with your name when it's struck out. Which means **Forge (the carter's
unlisted crew) and Vale (riding the cart's under-bed sling) might be the first in years to go in
and keep it.**

**The live beat:** they pivoted off the dawn plan — *"we sneak in through the way Windreth
found, now"* — and went down the culvert, all three (Windreth → Vale → Forge; 40 ft of stone, no
room to turn around). Forge declared **in, stripped**, kit cached in the reeds. Water-margin
Stealth resolved at **DC 14 — Windreth 26 ✅ / Forge 18 ✅ / Vale 7 ❌**; the failure paid out as
pre-declared and *not* as a capture — a scrape, not a splash — but a watch skiff's lantern
stopped and dwelt on the culvert mouth. **Vale answered it with a minor illusion** (a rat falling
in and swimming off) and it worked cleanly — **no roll owed**, since seeing through it requires a
creature to take the **Study** action against her **DC 15**, and a bored skiff-hand at 40 ft in
the dark is not studying anything. He said *"Rat,"* took the light off the mouth, and poled on.
**The cost landed on the kit instead:** the lantern raked the reed line on the turn and he hooked
**Forge's breastplate** out of the mud as *salvage* — it is now in the bottom of a watch skiff
going down-cut, unconnected to anyone, and boats tie up somewhere. That consequence was
**planted, not sprung** (the kit was stated as recoverable *and findable* when it was cached).

**They are now inside the outer yard.** They crossed the forty feet under the illusion's own
noise, waited out the washer's round at the eaten grate, and took her: Vale's second illusion put
a small sound at her back, and **Windreth alone** — Forge was thirty feet down a one-man-wide
throat and could not reach — took her through the bars with the Silent Blade, **non-lethal per
the standing house rule**, catching her weight so the flagstone got a lowering and not a fall.
Again **no rolls**: a tired washerwoman glancing at a noise never takes the *Study* action, and
the take was surprise + unaware + unarmed + unarmoured at arm's reach. Everything uncertain in
that beat was a **consequence**, not a check — and the consequences are real. **The grate was
destroyed, not opened** (pencil-lead iron came away in lengths), so their exit is a hole that
cannot be closed, on a grate she salts nightly. **She is unconscious in the open**, alive, and she
is the one person in this operation who **keeps her memory** — the only witness the system cannot
erase, pointed at them. **Vale is holding the yard's sound** with a working-noise illusion copied
from twenty minutes of listening (1 min / 30 ft / max 3 active — it needs renewing). And with the
yard quiet and theirs, they heard what her broom had been covering every night: **from inside the
windowless stone building, past the chalk line, at this hour with no lamp lit — paper, a great
deal of it, worked by hands.**

**Then they crossed the chalk line, and nothing happened.** Their declaration was *"we approach
the building, trying to see what is going on inside"* — and the building sits past the line, so
the line went with it. **No roll:** a windowless building has no eyes, they had already removed
the only watcher in that yard, and Vale's mask was manufacturing their exact kind of noise on wet
flagstone while they walked on wet flagstone. Crossing was never a check; it was a bet they had
placed an hour earlier. **Nothing took anything from them — and I told them plainly that this is
evidence for their theory and not proof of it**, because they do not know whether the bill is
presented at the line, at the gate, or at dawn. It stays unconfirmed.

They followed a **hand's-width runnel** cut arrow-straight from the building's foot back to the
drain they came up through — the grey water's path, and therefore the source of the pulped paper
in the culvert silt: they walked up the far end of the thing they crawled in through. **The
building has one door** — iron-strapped, no keyhole outside, barred from within — with the
flagstone before it scrubbed to a pale blister about four feet wide, worn like a threshold one man
crosses daily carrying something flat. **No window, no vent, no seam of light anywhere.** So the
honest answer to *see inside* was **there is nothing to see**: the work happens in the pitch dark.
The answer arrived through a different sense instead of being refused — at the wall's foot the
runnel ends in a **low arch a hand's breadth high**, the water's exit, breathing warm wet air sour
with soaked paper and grey salt. Through it: **several pairs of hands, not one. Nobody talking.**
And under the hands **a single flat unhurried voice reading one word at a time, with a pause after
each — and after each pause, a small wet sound. Paper going into water. Then the next word.** The
words themselves are unintelligible through a hand's breadth of stone. Something in there is
**named aloud and then unmade, item by item.** Their move.

**The cost of their own success, surfaced not sprung:** at the wall Vale is ~50 ft from the
washer-noises, and minor illusion is 1 min / 30 ft — **she cannot renew the mask from where she is
standing.** Walk back into range, or cast fresh working-noises at the wall and put the washer's
broom somewhere it has never been in twenty years. Theirs to solve.

**Then Windreth went at the door — and the fuse landed while he worked it.** He declared *"trying
to lockpick the door"* and rolled **Sleight of Hand 13**. Two things came out of that. First,
Vale's minute ran out: the broom finished its stroke, the bucket settled, and **the yard went
properly silent**, which is the abnormal state in a place where someone inside has heard that broom
every night for years. It was narrated as a minute simply running out — the price had been posted
in advance, so it landed as a clock, not a gotcha. Second, **the declared action turned out not to
exist.** That door has no keyhole, no plate, no ward, no escutcheon and **not one scratch of a key
ever turned on the outside face** — it has never been locked, only ever *shut*. So there was no
check to fail: the picks were charged nothing and became the free means of *learning* that, and the
declaration was read generously as *work the door with his tools*. What the 13 actually bought was
the seam. His blade found **one timber across two iron cradles, unfastened, merely heavy** — it
moves. Through the gap came three things: **no lamp, candle, rushlight, oil or smoke, and no warmth
at all on a draught blowing outward** (the work is fast and it is done in absolute dark, on
purpose); **the reading has never once faltered or hurried** since they reached the wall, so nothing
in there has noticed them; and packed into the threshold joint at the door's foot, **a line of grey
salt** — the same grey as the wall's top course and the clerk's inkpot. Every boundary of this place
is salted. **His blade broke that one to reach the bar**, and they were told so on the spot rather
than having it banked for a later ambush. The roll bought **capability and the price, never the
outcome**: he can lift the bar, he cannot lift it quietly — ruled impossible rather than hard, since
two feet of wet oak has to clear iron cradles one-handed on a door he is also holding. The choice
went back to the table with a standing offer attached: *say "open it regardless of the noise" and it
runs straight from the bar-lift with no new roll.*

**Still sealed:** the buyer's identity (a shape filled the lit doorway and did not come out —
deliberately un-described), *what the voice is reading and where a spoken name goes*, how THE SEAL
is restored, and "patron = buyer" stated flat.

**Clocks:** **🔴 the washer** — down, alive, in the open *between them and their only exit*, and
she *remembers*: she wakes up eventually and she is the only witness this system can't erase ·
**🔴 the destroyed grate** — a hole that can't be closed, on iron she salts nightly, and still
their only proved exit · **🔴 Vale's mask has LAPSED** — the minute ran out at ~50 ft and could not
be renewed from the wall, so the yard is now silent and nothing covers them; recastable, but only
*here* · **🚪 the loud bar** — the door was never locked, and the timber can be lifted but not
quietly (standing offer: *"open it regardless"* runs from the bar-lift with no new roll) ·
**🧂 the broken threshold salt** — the second salted boundary they have broken tonight, in a place
that checks its salt nightly · **📄 the voice in the stone building** — reading and
drowning things one at a time in the dark, words unintelligible from outside, and it has not
faltered once ·
**🟢 the line, crossed** — nothing happened, which is not the same as nothing being owed ·
**🔴 Forge's breastplate on a watch skiff**, taken as salvage and
going down-cut: recoverable if they chase where it ties up, and meanwhile a dwarf's breastplate
pulled from a canal is a thread the watch now holds · ~~the dwelling lantern~~ (cleared by the
illusion) · two sunk counters
in the canal ~1.5 days into a 1–3 day fuse · the watch now dragging the cut **at night** · the
gate-post found by the relief watch · **the countersign has rotated — it is now a wrong answer
delivered confidently** · COL the Collector loose with all three faces · a bakery yard man has
seen them at the counter.

**Pinned by a To-DM answer (Forge's player asked whether the grate is gone and whether they can
roam):** answered OOC in #in-character, no beat spent, and the answers are now table-facts that
must be honoured. **The grate is an open drain mouth, passable at a crouch, both ways, unlimited** —
a guaranteed exit that also guarantees discovery. **Nothing is watching that yard: no dog, no
patrol, no watchman, no alarm raised — nobody in the world knows they are in there**, because the
windowless building cannot see out (the same fact that stopped them seeing in). **Moving around an
empty yard costs no check.** What "freely" does *not* cover was stated too: the building itself is
shut and barred from within; they have looked at exactly **one face of it**; the stair gate is the
clerk's gate and this is not first light; the wall's top course is still unbroken grey salt. **Do
not retcon a watcher into this yard later** — the quiet was earned and was promised out loud.

**Owed to a player:** **Windreth's second bonus action**, credited to his first turn of the next
combat (a debt of the DM's isn't cancelled by a fight ending).

Beat-by-beat state, props, NPC ledger, and the reusable rulings this arc produced live in the
DM's campaign memory note `project_night_weigh_weighhouse_staged`; the secret spine is
[`campaign-arc.md`](campaign-arc.md).

## Next action (DM intent — the one thing the Console can't infer)

> **Never roll/act/decide for the PCs** — players roll their own dice; verify any `/command`
> syntax against `internal/discord/commands.go` before advertising it in a coda.

1. **Poll before you write.** Every beat: `dm_queue_items` **and** #in-character **and**
   #roll-history — and again *immediately before* writing a compressed beat. The one-beat tempo
   ([`dm-rules.md`](dm-rules.md) "At the table") multiplies the cost of a stale read, because one
   beat now asserts a dozen steps. When it happens, **rewind publicly and free.**
2. **The culvert run, the lantern, the washer ambush, and the crossing all resolved 07-27** (one
   beat each, zero dice across all four); **the door resolved 07-28 on a single roll.** The open
   question posted to the table is verbatim ***"The bar is loose under his blade. What do you
   do?"*** Next: whatever they declare there — a door that was never locked, a bar that will come
   up loud, a silent yard with no cover left, an unlit room of hands that has not noticed them, a
   broken salt line at their feet, and a body in the open between them and their exit. **Two
   offers are open and unanswered:** *"open it regardless of the noise"* (runs from the bar-lift,
   no new roll) and Windreth's free rerun of the blade-for-grapple substitution. Don't prompt
   their levers (the Name-Muffle, the washer as someone who can be *asked* rather than merely
   left, fresh illusion at the wall, Mage Hand through the outflow, the other faces of the
   building, making light); let them find them. **Non-lethal MO stands until a player flips it.**
   **Forge is AC 14** and stays that way — his breastplate left with the skiff (see the equipment
   note below).
   **Six adjudications worth reusing:** hold established physical constraints against a group
   declaration (the one-man-wide throat meant Forge simply could not join the grab — surfaced,
   with the reversible alternative priced); **substitute the tool while honouring the intent**
   (a grapple through bars at STR 8 is the weakest thing Windreth owns, so the declared
   subdue-and-KO was resolved with the blade, flagged in-post, free rerun offered);
   **answer the declared question through a different sense rather than refusing it** (they asked
   to *see* into a building that physically has nothing to see into, so the answer came by ear
   through the water outflow — a real answer, and the *next* step priced honestly rather than
   rolled); **charge their own success** (the mask that bought them the yard cannot reach the
   yard from where the mask's success let them stand); **never charge a roll against an action
   the fiction has already ruled out** (a lockpick on a door with no lock is not a failure — make
   the attempt the free means of *learning* that, read the declaration generously, and let the
   roll buy **capability and the price rather than the outcome**, then hand the choice back with a
   no-new-roll offer attached); and **give them the damage they just did, immediately** (the
   broken threshold salt was reported in the same breath as the discovery, not saved for a later
   ambush — the same reason a telegraphed fuse like the lapsing mask gets narrated plainly when it
   lands).
3. **Keep the buyer sealed by observation, not refusal.** Every seal so far has held because the
   fiction genuinely doesn't show a face, not because the DM said no.
4. **48 gp is still unspent** (Forge 26 / Vale 11 / Windreth 11) against a 50 gp potion — a
   deliberate two-gold squeeze left as a problem, not a wall.
5. **The Pike & Iron waybill book** (sender / destination / date — no salt, no memory) is the
   live alternative target if the yard turns out to cost too much.
6. **Onboard new players** as they arrive (`/register` → portal build via the tunnel →
   DM-approve → roster row + `party/<name>.md`). Party scaling toward 5–6; see
   [`big-party.md`](big-party.md).
7. **After every beat:** narrate to #the-story (`:::read-aloud` fence via
   `POST /api/channel/post`), append the play-by-play to [`sessions/`](sessions/), and keep this
   file's **Current scene / Next action** current. Then **commit and push** — that is a standing
   order, not an errand to be asked for.

**Open bugs to route around:** template creature-placement validator is 0-based at
`internal/combat/service.go:948` vs 1-based elsewhere ⇒ **the last row of every map rejects
creatures** (and the template PUT needs `campaign_id` as a *query* param too); enemy-turn
`RollAttack` ignores target conditions ⇒ Reckless's downside never applies on NPC turns;
mastery is keyed by weapon **ID** not **kind**, so a homebrew clone burns its own mastery slot;
`/enemy-turn` does not auto-apply a defender's standing Uncanny Dodge (halve by hand);
ISSUE-059 (DM-Queue Resolve button fires no POST → use `POST /dashboard/queue/<id>/resolve`),
ISSUE-060 (builder omits Warlock pact boon / invocations).

Durable lore → [`world.md`](world.md); secret spine → [`campaign-arc.md`](campaign-arc.md);
play-by-play → [`sessions/session-01.md`](sessions/session-01.md).

> **⚠ Session-log coverage gap.** `sessions/session-01.md` stops at **2026-07-19** (the canopy
> reading). The entire Night Weigh / Far Quarter leg — 07-21 through 07-27 — was played out of
> the DM's campaign memory note and **never appended here**. Treat the session log as complete
> only up to 07-19; for anything after that, the memory note
> `project_night_weigh_weighhouse_staged` is the record. Backfilling it is worth doing, but do
> not reconstruct beats from summary — pull them from the #the-story message ids recorded in
> the memory note.
