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
- **⚠ Known sheet-vs-fiction drift, opened 07-27 — AND IT BIT A PLAYER 07-28.** The **breastplate is
  still in Forge's `equipment` as carried-not-worn** while, in the fiction, it is in the bottom of a
  watch skiff going down-cut. `equipped_armor` is correctly empty and **AC 14 is right either way**,
  so no mutation was made — remove it from the sheet only if the party abandons the chase.
  **07-28: Forge's player read that line and declared "equipping his breastplate" mid-scene.** Ruled
  in the open, charged nothing, AC unchanged, and the line was re-read for the table as a *recovery
  thread, not an item in hand*. If it bites a second time, delete it from `equipment` and keep the
  skiff purely as a fiction hook.

## Maps

| Map | ID | Notes |
| --- | --- | --- |
| The Counting Floor — the stone quay | `83414d56-eba0-4a7b-bf66-ad34d860fe33` | 20×12 @48px. Interior cols H–S rows 4–9; **front door = cols M,N in the south wall, the only gap**. Lighting layer deliberately all-zero (mundane dark is adjudicated in fiction; only `magical_darkness` is engine-enforced). |
| The Night Weigh — the canal weighhouse | `09a89fb3-1885-4f30-9189-8328d3b5fdd2` | 17×13 @48px. Canal rows 1–2, quay 3–5, shed 6–11, office M–O 9–11. |
| Sabinnet's Reading Room | `353c58b3-3844-4f4f-8a19-b38a73c0da47` | 12×10. Sesh reader fight (won 07-18). |
| Palewatch — kept vault | `cc356cc4…` | 14×10, **zero walls** — the reusable open box; re-skinned as the chandlery store-room. |
| Ashfall waystation (common / cellar / cold vault) | `1ad14481…` / `d2fe03c6…` / `2899165e…` | 12×10 blanks. |
| Buried Gallery of the Faceless God | `39ecd023-51d8-44bb-bf8e-29e1eff3a231` | 12×12 blank stone. |
| The Stair Hill Yard — the reading room | `6081877c-258f-4f45-9b05-f90a762a564d` | 18×12 @48px. **Door = west-wall gap rows 6–7; outflow arch = south-wall gap cols I–J.** Two trough rows (water) at rows 4 and 9 spanning D–O, drain channel I–J rows 10–11, reading stand = difficult terrain at Q6. Flood-filled on the persisted bytes: **216/216 reachable, no sealed pockets.** Lighting layer all-zero on purpose — the dark here is ordinary and DM-adjudicated, and all three PCs have darkvision 60. |

**Always flood-fill a hand-authored map on the PERSISTED bytes before using it** — one map once
shipped as five sealed pockets. Walls are edge *segments* (w/h = 0): rasterize by segment.

## Encounters — ⚔ ACTIVE

**The Reading Room** — encounter `caecc5b8-69e0-4f7a-8ad8-b7e54606f791`, template
`53305a10-1cd8-453a-af55-f4032399e56b`, map `6081877c-258f-4f45-9b05-f90a762a564d`.
Started 2026-07-28 on Forge's declaration *"we want to fight them… non-lethal as usual"*.

**Round 1 order:** Windreth 13 (d20 9 +4) → Forge 13 (d20 11 +2, DEX breaks the tie) →
**the man at the stand 4** → Vale 3 (d20 3, +0). Start seats: Vale **O6** (adjacent to the
reader), Forge **N7**, Windreth **M6**, the reader **P6**, the book on its stand **Q6**.
**The reader acts before Vale** — that ordering is load-bearing and was told to the table.

Combatant ids: Windreth `348be098-8e6c-40e3-8a9c-23d2d969c29d` (WI), Forge
`14b6eb94-e481-4280-861a-9b042daddbd3` (FO), the reader
`6a7522cc-0e7b-4333-9a41-a298348643b8` (RDR).

**Statblocks (homebrew, sealed from players):** The Reader `hb_22f8eec8f60e` — an ordinary
unarmed man, AC 11, one token Open Hand attack, no resistances or immunities. His win condition
is **`Hands Off The Page`: shut / take / drown the book and he cannot read a name.** Track that
before tracking his hit points. One of the Seven `hb_8a05eeb7088b` — **deliberately NOT seated.**
The seven are *not in the initiative order*; they enter only via
`POST /api/combat/{enc}/summon` if a name is read off the book. That was disclosed to the table
in full, with the reason (three separate times they failed to react to a *person*) and the honest
conditional: *whether it holds is not up to the DM, it is up to what gets said in this room.*
Promise made publicly: **if a PC swings at a worker, say out loud what that is BEFORE dice.**

Standing this fight: **non-lethal granted flat for everyone, all fight, no re-declaring**;
**Windreth is owed a second bonus action** — the turn-resources override is *refund-only*, so
spend his first, then `POST /api/combat/{enc}/combatants/348be098-.../restore-bonus-action`.

**Round 1, mid-turn: Windreth has the book.** He moved M6 → **P5** (15 ft — the one square that
touches *both* the reader at P6 and the stand at Q6), then spent his **free interact**: *"grab the
book and read."* **Ruled: it lands, no roll** — two fingers *resting* on a page is not a grip and
the reader does not fight, and the book had been publicly declared reachable, so no contest was
invented after the fact. His hand did not close or follow; **the seven did not react** (a name
moves them, an object does not). The silent read was given free and in full: names in many hands,
a date and the recurring number **8**, nearly every entry struck with a single line **in the same
ink and pen as the writing itself**, the top of the page dated tonight, one empty ruled line
below the last entry. That is the fiction answering the party's 8-silver deduction **without
confirming it** — do not explain what a struck line does. **Fork left open and explicitly
unmade:** reading silently is free; reading **a name aloud** was flagged as *not free and not
neutral* **before** it could cost anything, and the choice is the player's. **His turn is still
live** — action, bonus action (+ the owed second), 15 ft, 1 attack, reaction; only the free
interact is spent.

Closed encounters, newest first (full chronology in
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

**They took the loud option, and the room did not care.** Forge declared *"slam the door
forcefully"* and rolled **Athletics 23**. The bar was already loose, so the outcome was never in
doubt — the roll bought the **manner**: he is *through* the door and inside on his feet on a wet
sloping floor, axe up, rather than framed in the opening with his balance gone. Windreth's hand came
out of the seam free, uncharged. The noise was enormous, and **nothing answered it.** Nothing
outside — the yard is still empty and no watcher was retconned in to punish the volume. Not the
washer — she is **unconscious, not asleep**, and noise doesn't undo that, so she still wakes on her
own clock. And, worst of all, not the room: **not one head turned and not one pair of hands
stopped.** They were told plainly that this is the fact rather than the DM withholding, and that
*they are not being punished for the volume, they are being answered by it.*

**What is in there.** A long low room, floor tilting to the outflow arch, so everything spilled
here leaves by the thread of water they walked up. Two trestle rows, troughs of standing water,
warm wet sour air. **Seven figures** in aprons with sleeves wet past the elbow, working paper to
pulp with their hands **in absolute dark, never once feeling for anything** — every reach exact
from repetition. Eyes open. Barely blinking. An **eighth** at a stand with a book open on it, **not
looking at the book**, one finger resting on the page and not moving down it. All three PCs have
**darkvision 60 ft** and this is ordinary dark rather than `magical_darkness`, so everyone sees it;
that was stated up front so nobody had to ask, along with the note that making a light is a *choice
with a price*, not a requirement.

**And with the door open the words came clean — they are not names, they are pieces.**
*"Tallow."* — a slip into the water. *"The lower yard."* — another. *"A woman who limps."* —
another. *"Third of the month."* — another. **One person's day, cut into the smallest parts a day
can be cut into, each part drowned by itself.** That shows the industry of the thing without
confirming or denying the players' book-deduction, which stays theirs. Then the reader stopped
mid-list — **not startled**, the way a man stops reading when someone comes in — **did not turn
around**, and said in exactly the voice it had been using all night: ***"You are early. Shut the
door."*** What "early" means was refused in-post. **No initiative was rolled**: nobody in that room
has done anything hostile, and it stays a conversation with an axe in it until a player flips it.

**Vale obeyed it, and the reader turned around.** Two declarations folded into one beat: Forge's
*"comprehend their speech, chosen language, intonation"* on **Insight 16**, and Vale's *"walks in,
closes the door, and approach the head of the room — 'You've been expecting us?'"* The Insight
bought three real things: the language is **Common**, plain and local and unaccented, with nothing
hidden in the word-choice; the **flatness is a technique rather than a mood** — no emphasis
anywhere, every word the same weight and the same silence after it, the discipline of a smith
counting strikes he cannot afford to miscount, because *you talk like that so that nothing you say
can mean more than anything else you say*; and **he did not change register to address living people
in his doorway**, so either he cannot or he does not hold reading and speaking apart. It explicitly
did *not* buy what he is, whether he is one, or what "early" meant. **Off the seven workers it came
back completely blank** — no hostility, no fear, not even the deliberate blankness of being ignored,
because there is no social signal there to read. That blank was handed over as a finding rather than
allowed to feel like fizzled dice.

Then Vale crossed to ten feet from the stand and asked. **The finger came off the page and he turned
around, and there is nothing wrong with him** — a man in an apron, middle years, thinning at the
front, sleeves wet past the elbow like all the rest, a face you would forget on a stair, looking up
the way a man looks up from work. *Ordinary is the horror here; it should not be walked back into a
monster reveal without a reason.* He answered in the same cadence: ***"Expecting." … "I am expecting
a delivery. You are not it." … "You may wait."*** — which converts the withheld line into a **clock
rather than an explanation**: something is due at that door, he thinks it is due later than now, and
they are standing in the room it arrives at. The timing was withheld and they were told they can
simply ask him. (Private fit, unconfirmed at the table: the Quiet Window takes paper in all night
and the cart brings it up at **first light**, which is what "early" means.)

Then his eyes went to Vale's empty hands and came back to her face, and he asked the only thing he
appears to want: ***"Do you have a name for me?"*** **That question is live and unanswered, and the
cost of answering was deliberately not priced** — said out loud, with the reason: naming is the
exact thing this yard does, and pre-pricing it would hand over the mystery they spent three sessions
walking up a drain to reach. Answering, refusing, lying, and asking him something back are all live
and none of them is the safe one. **The door is shut but not barred** — the bar lies where it fell,
nobody has touched it, and opening it from inside is a pull with no check and no cost. **All three
were ruled inside** when it closed, since nobody declared otherwise and Windreth had his hand on the
door; **his player can put him on the yard side instead, free, with no roll and no retcon tax.**

**She did not answer it.** *"I was hoping you would tell me"* — and then she **showed, without
giving,** the faceless god's name-scrap, held in her own hand at ten feet and never inside his reach.
**No roll was charged and the reason was said out loud:** showing cannot fail, and his reaction was
physical enough that no check was needed to read it. **He tried to read it.** His lips shaped it the
way they had shaped *Tallow* and *Third of the month* — and **no sound came out.** He tried twice,
with no strain and no drama, the way a man tries a door he is certain of, then wet his lips and
stopped trying. **Behind him all seven pairs of hands stopped over the water at once**, dripping
where they were, nobody turning, nobody looking — the room that ignored a door coming off its bar
went completely still over a scrap none of them had looked at — and then went back to work at
exactly the old rhythm without a sound. He took his hand off the book, turned fully toward her (the
first whole attention anything in that room has given anything), and said: ***"I do not give names.
I take them."*** … ***"That is not a name. That is a piece of one."*** … ***"Where did you get a
piece I cannot say?"***

**What that bought:** this yard **cannot unmake that fragment** — he unmade a woman's whole day one
slip at a time tonight and could not get one stroke of the scrap off the page and into the air; and
**the seven do not react to people, they reacted to a name**, which spends part of the blank Forge's
Insight found. **What it did not buy, said in-post:** whether he knows what he was looking at, whose
name it is, or anything about who this room works for — *he asked her*, and the table was told
explicitly that I am not saying whether he can't answer or won't. And it was flagged as **evidence,
not proof**, for their book theory: a room tuned to names is not a book that eats days. Two
assumptions stated as free to reverse: the prop read as the **Name-Scrap (Faceless Mark)** (swap to
the Ashen Face-Shard / Grey Clay Face-Disc / patron-conduit on request, rerun at no cost), and
"shows, not gives" read as never released and never in reach — **he did not reach for it.**

**She did not answer that one either.** She put the fragment away **slowly, mockingly**, and asked to
be taken to his boss — *"maybe he can say it."* He watched the scrap the whole way into wherever it
went, the first time all night his attention has followed an **object** rather than a word. Then he
said **"Boss"** once, flat, testing its shape in his mouth the way he had tested *Tallow* — and **it
came out fine, because it is a title.** He noted *"You did not answer me,"* did not press it, and put
his hand **back** on the open book. Then he quoted a price: *"Names come off this book. Names go on
it. One for one, both aloud, and I do the reading either way."* — ***"Give me a name to put on the
line, and I will say you one off it."*** **The seven did not stop this time**, which is the rule they
just learned holding: *boss* is not a name.

**No roll charged, and why:** putting a thing away cannot fail, and his answer is **character, not a
favour** — a house that trades in names quotes a price when you ask it for one. Given free in-post
because they earned it by standing there: **every word this man has used for the people above him is
a title or a function** — Weighmaster, boss, the third of the month — *never* a name; what that means
was explicitly not confirmed. **What he did not say:** whose name comes off the book, whether he
means a stranger's or one of theirs, and **one word about what happens to a name once written** — he
quoted a price and did not describe the goods, and the table was told flatly that what the book does
to a name on it is theirs to work out. The mockery cost nothing visible, which *is* the cost: they
are now the party carrying a word this room cannot process, refusing its provenance, and asking to be
shown who stands above him — interesting to a house whose whole trade is interest in names.

**Then Forge stepped up and put the axe in the air.** *"Forge comes and make appearance. Equipping his
breastplate. Brandishing his greataxe without hesitation, 'So, anyone want to talk to my boss.'"* Two
rulings, both said in-post. **(a) The breastplate is not there and it cost him nothing.** He went into
the culvert *stripped* — that is what got a dwarf through the drain — and the watch skiff took the
plate down-cut last night; **AC 14 unchanged, no penalty applied**, and the sheet's carried-gear line
was re-read for the table as **a recovery thread, not an item in his hands**. Free reversal offered
and priced honestly: the version where he is wearing it is the version where he is not in the room.
**(b) Brandishing is a display and a display cannot fail**, so no roll — and **the room did nothing.**
Not one of the seven lifted a head; *a greataxe is not a name*, the third time that rule has held.
What it *did* buy is real and was named as such: **his attention came off Vale and onto Forge**, the
first time all night he has changed who he is talking to. **Still no initiative** — a raised axe is
not a swing, and the table was told the word *swings* starts it.

**And the reader turned the threat straight back into the trade.** He did not step back, reach, or
call out. He looked at the **flat** of the blade and along the haft — *at the places where a smith
stamps a mark and a dwarf cuts a word* — and said *"You say boss. That is a title. I can say a
title."* Then he laid two fingers on the empty line: ***"Does it have a name?"*** Flagged in-post
rather than sprung: **the trap is built out of the players' own words** — they held up an object and
called it a boss, and he asked the only question his trade knows. Told them plainly I am still not
saying what the book does to a written name, and pointed out that **he asked about the axe and not
about them** — *mercy or shopping* is theirs to decide.

**Still sealed:** the buyer's identity (a shape filled the lit doorway and did not come out —
deliberately un-described), *what the voice is reading and where a spoken name goes*, how THE SEAL
is restored, and "patron = buyer" stated flat.

**Clocks:** **🔴 the washer** — down, alive, in the open *between them and their only exit*, and
she *remembers*: she wakes up eventually and she is the only witness this system can't erase ·
**🔴 the destroyed grate** — a hole that can't be closed, on iron she salts nightly, and still
their only proved exit · **🔴 Vale's mask has LAPSED** — the minute ran out at ~50 ft and could not
be renewed from the wall, so the yard is now silent and nothing covers them; recastable, but only
*here* · ~~the loud bar~~ (spent — thrown, door open, and nothing answered the noise) ·
**⏰ the delivery** — something is due at that door, they are early for it, they are standing in the
room it arrives at, and the timing is withheld but askable · ~~**🗣 "Do you have a name for me?"**~~
(spent — she answered with a question and a prop, and he let it go for a better one) ·
**🗣 "Where did you get a piece I cannot say?"** — asked, **dodged**, and *left lying there*: he said
*"You did not answer me"* and did not press, so it is still owed and he still wants it ·
**📖 THE PRICE ON THE LINE** — **the live offer, and it is his**: a name written on the book for a
name read off it, one for one, both aloud. **What the book does to a written name is unstated and
stays unstated** · **🪓 "DOES IT HAVE A NAME?"** — the same price, now aimed at **Forge's greataxe**,
because he held it up and called it his boss; his two fingers are on the empty line and the axe is
**unnamed on the sheet**, so whatever gets said is the players' invention ·
**🛡 THE BREASTPLATE IS NOT IN THE ROOM** — ruled out loud when Forge tried to equip it; the culvert
strip is what got him in, the skiff has it, **AC 14 unchanged**, sheet line = recovery thread · **🔥 the fragment he cannot read** — the yard's
first proved limit, now witnessed by him *and* by the seven · **🏷 every word for the people above
him is a title, never a name** — Weighmaster, boss, the third of the month; given free, meaning
withheld · **👁 eight in the room** — seven
pulping, one reading, no alarm, no attack, no initiative; **the seven broke their blank exactly
once, for a name, and went straight back to work** — and did *not* break it for *boss*, which is that
rule holding; *why* is still unspent ·
**🧂 the broken threshold salt** — the second salted boundary they have broken tonight, in a place
that checks its salt nightly · **📄 the voice in the stone building** — now audible and reading a
day apart, piece by piece ·
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
   beat each, zero dice across all four); **the door, the slam, and the room's first conversation
   all resolved 07-28**, the **name question was answered 07-28 with a question and a prop**, and his
   follow-up (*where did you get it*) was **dodged 07-28** — she put the scrap away mockingly and
   demanded his boss, so **he quoted a price instead of answering: a name on the book's next empty
   line, for a name read off it.** **Forge then brandished the greataxe and called it his boss
   (07-28) — no roll, no initiative, the room did not react, and the reader aimed the same price at
   the axe: *"Does it have a name?"*** **They answered that question by declaring a fight
   (07-28, Forge: *"we want to fight them. and we need answer, non-lethal as usual"*), so the
   axe question is now dead as dialogue and live as a threat.** Initiative was rolled by all three
   players and **combat is running** — see *Encounters — ⚔ ACTIVE* above for order, seats and ids.
   **Windreth is up first and the table has been told so.** Next: run his turn when he declares —
   **and the whole faceless-god arc is now standing
   in the room**, so keep the buyer, "patron = buyer", and the book deduction all sealed while
   answering honestly about *this room*. **Do not narrate what a written name costs**; if they write
   one, let the fiction bill them. Two
   offers open and unclaimed: Windreth on the yard side rather than inside (free, no roll), and
   his free rerun of the blade-for-grapple substitution. Don't prompt their levers (the
   Name-Muffle, THE SEAL / Windreth's Kept Name, Vale's Deception +7 and Mask of Many Faces,
   asking *when* the delivery comes, the book on the stand, the washer as someone who can be
   *asked* rather than merely left, the other faces of the building); let them find them.
   **Non-lethal MO held and was honoured on the flip: the party declared the fight themselves
   (07-28) and non-lethal was granted flat for the whole encounter with no roll tax and no
   re-declaring.** **Forge is AC 14** and stays that way — his breastplate left with the skiff (see
   the equipment note below).
   **Fifteen adjudications worth reusing:** hold established physical constraints against a group
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
   lands); **when the players knowingly buy a cost, answer the volume instead of punishing it**
   (the slam got no retconned watcher, no woken washer — unconscious is not asleep, ruled out
   loud — and no alarm; the consequence was that the room *did not care*, which is worse, and a
   roll landing on an outcome the fiction already settled gets spent on the **manner**); and
   **don't roll initiative just because a scene got tense** (nobody in that room had been hostile,
   so it stayed a conversation with an axe in it — say explicitly that a swing starts it);
   **a good roll that comes back blank is information, so hand the blank over** (Insight 16 read
   nothing at all off the seven workers, and saying *there is no signal here to read* beats
   letting the dice feel wasted); **make the operator ordinary** (a forgettable tired man in a wet
   apron is worse than any monster, and the reveal should not be walked back into one); and
   **convert a withheld line into a clock rather than an explanation** ("you are early" became
   *a delivery is due and you are standing where it arrives* — timing withheld, and they were told
   they could simply ask him, which keeps the pressure live and the answer theirs to go get); and
   **when a player spends a campaign-long artifact on a scene, pay it in full and take the answer
   off the DM** (Vale showed the god's name-scrap to a man whose entire trade is reading names
   aloud and drowning them — so he *tried*, and failed, and the seven broke their blank for it,
   which is a real limit of the antagonist proved on the table; then he asked **her** where she got
   it, which converts an interrogation into a trade, hands the next move back, and lets the sealed
   material stay sealed because the NPC is the one who does not know); and
   **when a player demands the sealed thing, quote a price in the setting's own currency instead of
   refusing** (Vale demanded to be taken to the boss; a house whose trade is names answered by
   offering *a name written on the book for a name read off it* — which is not a no, costs the DM
   no seal, is priced in exactly the resource the arc is about, and makes the players' own book
   deduction supply the dread that the DM must not confirm. Say what the price **is** and refuse to
   describe **the goods**); and
   **when a declaration reaches for gear the fiction already took away, rule it in the open and
   charge nothing** (Forge declared he was equipping a breastplate that has been in a watch skiff
   since last night — the drift was surfaced in-post, **AC 14 confirmed unchanged with no penalty
   applied**, the misleading sheet line was re-read for the table as a *recovery thread*, and the
   reversal was priced honestly rather than refused: wearing it means not fitting through the drain,
   i.e. not being in the room. **Never let a stale sheet line silently eat a player's beat**); and
   **flag a trap made of the players' own words instead of springing it** (they held an object up and
   called it a boss inside a house that buys names, so the NPC asked whether it *has* one — and the
   table was told plainly that the hook is theirs, that the book's cost is still unstated, and that
   *he asked about the axe and not about you* is theirs to read as mercy or as shopping).
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
