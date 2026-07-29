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

## Encounters — ✅ NONE ACTIVE (the yard plays with no rounds)

**The Reading Room** — ✅ **CLOSED 2026-07-28 at 13:0xZ, round 2, `status: completed`.** The party
**won it by theft and by walking**: Windreth took the book off the stand, all three crossed to a
doorway, and **@dewa declared *"We are out."*** — the trigger I had published, fired by a player,
honoured on the spot. **Zero damage taken all encounter** (38/38 · 50/50 · 38/38) and **zero damage
dealt** — the only blow struck by anyone was Forge's shove. Encounter
`caecc5b8-69e0-4f7a-8ad8-b7e54606f791`, template
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

**Table stalled on "what is an action even for here" (09:31 player-chat: *"meh action opo?"* /
*"gebuki?"* / *"mosok attack unarmed person"*).** Answered in #in-character
`1531598167852978292` with an explicit **non-exhaustive, non-hint** list: hitting him is on the
table and will be neither lectured against nor punished; the book is an object now, so any
further handling of it (shut, pocket, pass to Vale, put it somewhere it does not return from)
costs the **action** because the free interact is spent; an action can be a question; an action
can set someone else up. Re-flagged the unmade read-aloud fork. Pre-answered **Forge's shove**
idea (from 09:25 player-chat) — legal, and he will be told *what he contests and what a win
does* **before** he rolls, with enemy numbers still sealed. Re-stated the standing promise that
swinging at one of the seven gets narrated **before** dice.

**Fork resolved by the player: silent.** Windreth declared (#in-character 09:40Z, repeated as a
To-DM 09:45Z) *"read the book for himself, he does not speak out loud: study the dates, payments,
handwriting, and crossed-out names for investigation."* **No name has left the room.** That is
the **Study action**; there is no engine command for it, so `action_used` was marked by hand via
`override/combatant/.../turn-resources` (200). **Remaining this turn: bonus action + the owed
second, 15 ft, 1 attack, reaction.**

Committing the action bought the **surface with no roll** (#the-story narration
`eb579c85-8ce0-4615-bf8e-6b96479e0c85`, pointer #in-character `1531623553580007444`): numbers are
mostly **8 but not always**, one far-back figure pressed through to the next page, and **nowhere in
the book is there a currency mark** — no coin stamp, no *s*, no *gp*, just the figure. On tonight's
page **a handful of names are not struck yet**; everything older is. The man's only line: *"You
have it open to tonight."*

**✅ RESOLVED — Investigation 18** (11:26Z, queue `c497c017-a77a-4283-91c6-09e6920d5c0f`,
narrate 204, resolved 11:35Z). Fiction in #the-story `65ed5a99-9a76-4930-a33f-08eccf64e70f`.
**Three tiers released (any / 12+ / 16+), the 20+ tier withheld and said so out loud** — told him
it is still in the book and still findable, since the book is in his hands. Payload table below;
none of it confirms the buyer or says what a struck name costs:

| Roll | Released |
| --- | --- |
| any | On every line the strike is the **same sitting** as the writing — same ink, pen, pressure, unbroken. A name is not recorded and later cancelled; **the crossing is the transaction.** |
| 12+ | **Page numbers skip near the front.** Pages have been cut out cleanly, with a blade. |
| 16+ | The empty ruled line under tonight's last entry **already carries its number in the right column — 8 — and no name.** |
| 20+ | ✅ **RELEASED 07-28 in the culvert, with NO roll**, once light (Forge's hooded lantern) · time · no clock were all met. A few names **repeat, months apart** — four or five out of hundreds, different inks, same hand — and **in every repeat the EARLIER instance is the struck one**; the later entry stands clean and unmarked. Implication left hanging, **unexplained on purpose**. |

**🫥 THE HIDE RULING — the honest one, made in the open.** Windreth then spent his bonus action on
**Cunning Action: Hide, Stealth 25**, and the engine flagged him *Hidden from all hostiles*. **The
reader has blindsight 60**, so hiding does nothing against him. Handled as disclosure-before-cost,
not a retroactive gotcha:

- **The Hidden condition was left in place** — it is genuinely true against everything else in that
  room and anything that comes through the door. Stripping it would have taken away something real.
- **Stated publicly instead: it grants NO ADVANTAGE against the man at the stand**, and that will
  be applied by hand if Windreth swings. Pre-declared, never retroactive.
- **Blindsight was shown, not statted** — his head turned and followed Windreth into the dark
  "with his eyes doing none of the work." Numbers stay sealed; the effect does not.
- **The bonus action was refunded** (`restore-bonus-action`, 200) because he was not told before he
  spent it. **The 07-27 culvert bonus action is STILL OWED on top of that** — he was told so
  explicitly: spend this one and it comes back once more.
- ⚠ Note for the next attack roll: Windreth would have **Sneak Attack anyway** via the ally clause
  (Vale is adjacent to the reader at O6/P6). Do not conflate that with the hide; only *advantage*
  is denied.

**🏃 HE RAN, AND THE HOUSE LET HIM.** Windreth spent the refunded bonus on **Cunning Action: Dash**
(11:40Z) and moved **P5 → G5 — the full 45 ft, straight west, with the book** (11:41Z). He never
attacked. Turn closed 11:41:18Z with **reaction unspent**. Leaving reach at O5 queued an
**opportunity attack for the man at the stand** (`1759def9-e3d3-4b5e-95a9-c4862b899f81`).

**⚔️ THE OA WAS DECLINED — the antagonist's win condition is the book, and he did not reach for
it.** Narrated in #the-story `f8868674-e117-48e8-ab09-0b7bce676d18`, queue resolved 204 at 11:53Z,
mechanical note #in-character `1531630643455328316`. His hand *"has been up since the book left it.
It does not close. It does not follow."* He tracks Windreth the whole 45 ft with his chin level and
**his eyes doing none of the work**, keeps turning **past** him to the west doorway, and says one
line: **"Mind the yard."** Ambiguous on purpose — this room *is* inside the Stair Hill yard, so it
reads as threat or as genuine warning and the DM never picks. **No watcher was added outside; the
pinned promise holds.** The seven did not move and **not one of them turned to watch the book go.**
**His reaction is therefore still up, and the table was told so** — the decline is one ruling, not a
promise about the next one.

**🚪 MAP FACTS PUBLISHED (BFS'd off `maps.tiled_json`, map `6081877c-…`, 18×12, 48 px tiles).** Two
exits, both already visible on the rendered map, so distances were given rather than hidden:
**west doorway = the gap in the west wall at rows 6–7** (Windreth at G5 is **~30 ft** off it — he
ran 45 ft and is **still inside**); **south doorway = the gap in the south wall at cols I–J**
(Forge at N7 = **25 ft**, Vale at O6 = **30 ft**). East and north walls are solid.

**❓ VALE'S TO-DM ANSWERED (11:48Z, "if we go out while carrying the book, do we exit combat?").**
Yes — **all three out a doorway and the encounter ends**, they keep the book, and the outside is
played in the open with no rounds. *"I am not going to make you roll dice to walk down a corridor."*
Three caveats stated **before** anyone moves: his reaction is up and **Vale at O6 is adjacent, so
moving provokes** (she is a warlock — Disengage costs her whole action, and that trade is hers to
make); **leaving the room is not leaving the yard**; and **the delivery clock does not pause.**

**↩️ THE OWED BONUS ACTION CARRIES.** The refunded one went to Dash, which is what it was refunded
for. The **07-27 culvert bonus action is still owed**; his turn is closed and there is no rewind,
so it was booked publicly as **credit on his round 2 turn**. Not lost.

**🪓 FORGE DID NOT RUN — HE CLOSED.** He moved **N7 → O7** (5 ft, 30 left), putting himself
**adjacent to the reader at P6**, and declared a **shove**. He is covering the retreat, not taking it.

**🔧 THE FREEFORM-ACTION TRAP — a real UX bug, ruled in the player's favour and fixed by hand.**
He sent it as `/action "shove RDR"`, which is the **freeform** action: the engine correctly reads
that as *spending the whole Action on something with no command*, so it set `action_used` **and
zeroed both attacks** (`action_log` `freeform_action`, 11:55:15Z; queue
`7d5d84dc-6827-4793-b94a-0754aefe0b69`). His subsequent real `/shove` was then rejected by
`ValidateResource(turn, ResourceAttack)` at `internal/combat/grapple_shove.go:189` for having no
attacks left — and he called it out (#in-character 11:55:40Z, *"i can't shove despite hasn't spend
any attack"*). **He is right.** Shove costs **one attack, not the Action**
(`grapple_shove.go:180-184`). Restored `attacks_remaining` to **2** via
`override/combatant/.../turn-resources` (200); **`action_used` deliberately left true** because he
*is* taking the Attack action, which is where a shove comes from. Queue resolved 204. Ruling posted
#in-character `1531633484899029032` — told him plainly the tooling failed him, not his declaration.

**📏 THE PROMISED PRE-ROLL DISCLOSURE, NOW PAID.** Forge was owed *"what he contests and what a win
does, before he rolls, with enemy numbers still sealed"* — delivered in full: contest is **his
Athletics (STR) vs the reader's Athletics or Acrobatics, whichever is better**; what he can *see*
was given free (*a tired man in a wet apron who has not braced, has not raised his hands once all
night, carries no weapon*) with the honest read **"I am not going to pretend this is a coin flip"**;
and each mode's exact effect spelled out (`prone` → down, half movement to stand, advantage on close
melee; `push` → 5 ft directly away and no further; `grapple` → speed 0). **What a win does NOT do
was said before the dice, not after: it will not make him answer, and it will not end the fight —
"never going to be settled on his balance any more than on his hit points."** Nothing about the
sealed statblock leaked. Correct syntax handed over: `/shove target:RDR mode:prone|push|grapple`.

**💪 THE SHOVE LANDED, AND IT WAS THE PLAY OF THE NIGHT — 20 vs 6** (12:22:17Z, #combat-log).
`mode:push`, so the reader went **P6 → Q5** — pushed *diagonally*, directly away from Forge at O7.
That single push **broke contact before Forge moved**, which is why his run out cost him **no
opportunity attack at all**: the reader's threatened squares no longer touched O7. One attack spent,
one kept. Forge then ran **O7 → O12 → N12**, all 35 ft gone, turn closed at 12:25:14Z with an attack,
his bonus action, free interact and reaction all still unspent. The player saw the trick himself in
#player-chat — *"apik yo, ter-push diagonal"*, *"dirimu terbebas seko opportunity attack"*.

**🚶 THE READER'S TURN — HE WALKED, AND HE DID NOT SWING.** The statblock settled it before I chose
anything: **`Hands Off The Page` is his stated WIN CONDITION** — *"If the book is shut, taken out of
his reach, or in the water, he cannot take Reads a Name. **Track this before tracking his hit
points.**"* Windreth has the book 50 ft away, so **Reads a Name is mechanically locked out**, and
`He Does Not Fight` marks his only attack (Open Hand, 1d4) as *"a last resort, used badly."* So he
took the one turn that is true to both: **movement only, Q5 → K5, 30 ft, no attack, no ability**
(`POST /enemy-turn`, 200, `"Moves 30ft"`). Executed as a hand-authored movement step — note the
payload's `Row` is **0-based** (`turn_builder_handler.go:314` does `dest.Row + 1`), so K5 is
`{Col:10, Row:4}`. That endpoint also auto-resolves the `enemy_turn_ready` queue row.

**🗣️ WHAT HE SAID WHILE WALKING** (#the-story narration `8ee1fa30-753a-4668-afc3-a91658261b5d`) —
*"You are faster than me."* … *"Everyone is faster than me. It has never once mattered."* …
*"Line eight is empty tonight. It was empty in my hands. It is empty in yours."* … and, stopping:
**"I would not read it out there, if I were you. Sound carries in a yard."** That last line
re-arms the established summon trigger **without inventing a watcher in the Stair Hill yard** — it
stays undecidable, advice or threat, exactly as promised. **The seven still have not moved**, and
that was said in the prose deliberately: the book left the room and not one of them looked up. They
remain **out of the initiative order**.

**🚪 EXIT DISTANCES PUBLISHED AGAIN, RE-MEASURED OFF THE WALL DATA.** West gap = **col A, rows 6–7**;
south gap = **row 12, cols I–J** (the `walls` objectgroup is six perimeter segments only — no
interior walls, the floor is open). Windreth **G5 → 30 ft** from the west gap, one ordinary move, no
Dash. Forge **N12 → 20 ft** from the south gap. Vale **O6 → 30 ft**, *exactly* her full move. **New
ruling published: standing IN the gap square and declaring you are going through COUNTS AS OUT** —
no second turn spent on a doorway. Also published: **it does not have to be the same doorway.**
#player-chat has Vale set on the south one — *"Aku penasaran pintu selatan"*, *"Gas"* — while
Windreth is already at the west end, so the party is splitting on purpose.

**🏃 VALE RAN THE WHOLE ROOM AND STOPPED IN THE DOORWAY.** `/move` **O6 → I12**, six diagonal
squares = **exactly 30 ft**, no Dash — and **I12 is inside the south gap** (row 12, cols I–J). She
crossed the row-9 trough on the way (water is flavour here, *not* difficult terrain — the engine
charged her 30 for 6 squares). **She kept everything else**: action, bonus action, reaction, 1
attack, all unspent, turn ended voluntarily. **No opportunity attack** — her path never entered the
reader's reach at K5, and she was moving away from him the whole time. Her turn closed round 1.
⚠ **She is standing IN the gap, not through it — she did not declare going through.** Under the
published ruling that declaration is hers to make on her own turn; **do not make it for her.**

**🕯️ WHAT TAKING THE BOOK ACTUALLY DID TO THE ROOM** (#the-story narration
`8ef84bcf-80fe-4912-b11a-78d5d9c24043`) — the seven's **hands have not stopped and nobody has looked
up**, exactly as before, so this is *not* the seven reacting. But **the voice has stopped, and so has
the small wet sound that came after each word** — there is no word to finish, because the reader is
30 ft from his own stand with empty hands. *"The work goes on. It is simply no longer about anyone."*
An honest consequence of the theft, costing the players nothing and confirming nothing sealed.

**📖 ROUND 2, WINDRETH: HE WALKED INTO THE WEST DOORWAY AND READ THE BOOK AGAIN.** `/move` **G5 →
A6** (12:48:12Z, exactly 30 ft, **A6 is the west gap square**), then a freeform `/action` —
*"read for himself and investigate the book"* (12:49:20Z) — and **Investigation `d20(8) + 6 +
1d4(4) = 18`**, the *same total as his first look*. ⚠ **He acted at 12:48–12:49, two minutes BEFORE
the round-2 post landed at 12:51**, so he never saw the exit measurement or the bonus-action offer.

**⚖ THE RULING ON A REPEATED CHECK — stated plainly rather than fudged** (queue
`f8a868f8-63e8-4060-9191-d1ab4e308473` narrated, 204). **An 18 does not clear the tier above 18, and
repeating the same check at the same DC in the same dark will never grind it out.** It needs
different conditions — light, time, no clock. **Out of the building with the book, he gets the long
look and he gets that tier**; the debt is explicitly still standing. **But the action was not
wasted:** because he asked a *narrower* question from a *new* place, the 18 bought one genuinely new
forensic tell, derived from evidence he already held — **the ruled line numbers 1–8 were all inked in
a single loading of the nib, before any name went in.** Same logic that proved the seven strikes were
one sitting. So it is not handwriting, it is a **count**: tonight was set at eight lines before
tonight had eight names. Seven are filled and all seven struck. **Line eight is not the next one — it
is the one that has not come in yet.** ⛔ **Whose it is was refused out loud, and the book does not
say.** Nothing sealed leaked; the price of a written name still never described. Fiction #the-story
`996e04df-5d5b-4995-8ff3-868a4341ceb1`.

**🛠 PUBLIC CORRECTION — FORGE'S SPEED IS 35, NOT 25.** I published *"you have 25"* off
`characters.speed_ft`; his **turn row** has always said **35** (dwarf 25 **+10 Barbarian Fast
Movement**). Corrected in the open and credited to the engine, not to me. **Read movement off the
`turns` row, never off the sheet's base speed.**

**🚪 "WE ARE OUT." — THE TRIGGER FIRED BY A PLAYER, AND HONOURED WITHOUT HAGGLING.** Board at the
moment it landed: Windreth **A6** (west doorway, turn closed), Forge **J12** (south arch, turn
**live**, 15 ft + action + bonus + reaction + 2 attacks all unspent), Vale **I12** (south arch), the
reader **K5**. At 13:02:54Z **@dewa** answered my own published line *"Say you go through and you
are out"* with **"We are out."** — plural. ⚖ **I took the plural at face value for all three rather
than demanding three separate declarations**, because pedantry there would have been bad faith with
a rule I wrote myself, and because the one-beat directive forbids per-item checklists. **What made
that safe: it spent nothing.** Forge walked out with everything in hand; nobody rolled, nobody took
a point of damage. And the reversal was published as **free and needing no engine work** — *"the
door is fifteen feet behind him and the reader does not fight; walking back in costs you an
encounter's worth of nothing."* ⚠ The one thing I did **not** do is convert anyone's silence into a
declaration on my own; a **player** said it.

**🔎 FORGE'S TO-DM QUESTION ANSWERED WITHOUT TAKING HIS ACTION.** He asked (13:00:28Z) *"should i
perception check toward south entrance, before heading out"*. **Ruled no** — he is standing **in**
the arch, and *"looking through an open arch you are standing in is not a Search, it is having
eyes."* A check is for finding something **hidden**, and **nothing is watching that yard** was
promised in writing on 07-27. **I will not charge a roll to re-confirm my own promise.** The free
look paid the whole yard: the lapsed mask, the runnel, the washer beside the grate, the unbroken
salt on the wall's top course. **No watcher was retconned in — the pin holds.**

**🧾 DEBTS — BOTH SURVIVE THE ENCOUNTER ENDING, SAID SO OUT LOUD.**
› **The culvert bonus action** — *"a debt of mine is not cancelled by a fight ending."* It was
already booked as *credited to his first turn of the next combat*, and that is exactly where it
still sits.
› **The 20+ Investigation tier** — I named **three** conditions (light · time · no clock) and I
**did not add a fourth**. Two are now cleared. What remains is a lamp and a place nobody is due at,
and the payout was upgraded in public: **when he has light and no clock he does not roll again — he
just gets it.** Two 18s bought it outright.
⚠ The freeform `/action` bug (zeroes `attacks_remaining`) stands, unfixed, logged.

**🧭 STATE: NO ROUNDS, NO INITIATIVE, NO TURN ORDER.** The party is in the outer yard with the book.
The reader **did not follow, did not call out, did not raise an alarm** — he walked back to the empty
stand and stood at it, and the seven never looked up. Live in the yard: the **washer** (down, alive,
beside the grate, and she *remembers*), the **unclosable grate**, **Vale's lapsed mask** (nothing is
covering them), the **unbroken wall salt**, the **delivery still coming** (they are early; timing
withheld but askable), **line eight**, the **dodged question**, 48 gp. ⚠ **"Sound carries in a
yard"** was restated unsoftened: **do not read a name aloud out there.**

Closed encounters, newest first (full chronology in
[`sessions/session-01.md`](sessions/session-01.md)):

| Encounter | id | Outcome |
| --- | --- | --- |
| The Reading Room | `caecc5b8-69e0-4f7a-8ad8-b7e54606f791` | **Won by theft, 07-28** — book taken off the stand, all three out a doorway on *"We are out."*; **0 damage dealt or taken**; the reader never fought |
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

**★ Now: THE CLERK IS TAKEN, AND IT WAS CLEAN. Option ① executed with TWO, needing one —
Windreth Stealth 27, Forge Stealth 20 vs DC 13. No combat, no initiative rolled, no noise, no lamp
on Stair Hill: NOBODY KNOWS. He never saw a face. In hand: one old clerk (alive, unhurt,
unrestrained, sitting where the lane bends) and his SATCHEL, UNOPENED. Vale arrived ~60s later to a
finished job — full party on the ground. ~8 minutes to grey. Pending: what they do with the man,
the bag, and the door.**
*(The 07-27 culvert approach below is how they got here and is still all live board state.)*

**🤐 THE TAKE — ONE ROLL, EXACTLY AS PRICED, RESOLVED AS PRICED.** Windreth
`d20(16) + 10 + 1d4(1) = 27` (queue `ebae4719`), Forge `d20(15) + 2 + 1d4(3) = 20` (queue
`6af3da0f`) — both `/narrate` → 204. Declared in #in-character 10:08: *"Take him quietly now.
Windreth + Forge only. Vale catches up after the clerk is secured."* Windreth came in blind-side and
the hand landed before the head turned; **Forge's roll is the one that caught the satchel coming off
the knees strap-first — the bag, not the grab, was the thing that would have made the sound.**
🎲 **Guidance ruling, stated publicly:** the `+1d4` was NOT clawed back after the fact, and it was
irrelevant anyway — **26 and 17 both clear 13 unaided.**
✅ **What the clean version bought (all stated out loud):** nothing on the hill alerted · the door
unspent, still shut and unwatched · **he never saw a face and cannot describe who took him** · Vale
present and unhurt.

**🎒 THE SATCHEL — IN FORGE'S HANDS, UNOPENED.** Described from outside only (soft old leather, strap
worn thin by one shoulder, heavier than paper and lighter than tools, a few small hard things
shifting inside). **Opening it is FREE — no roll, no check — the moment a player says so; the reveal
is theirs to claim, not mine to spend.** *(Intended contents when opened: a folded sheaf of slips in
a DOZEN DIFFERENT HANDS, an inkhorn, cut nibs, a straightedge, a key on a leather cord. The sheaf is
the payoff for the Reading Room observation — **the book is one hand because HE is the one who copies
everybody else's slips into it.** ⚠ This explains the single hand WITHOUT confirming the party's own
book-is-the-mechanism deduction, and WITHOUT narrating what a written name costs. Both guardrails
hold.)*

**🧓 THE MAN.** Older, wool coat, ink to the second knuckle, hands shaking and never once closing
into fists. He did not struggle — went rigid for half a second, then gave it up like a man who had
run the arithmetic. Set down at the bend, hand off his mouth, **he does not shout, does not ask who
they are, does not demand anything.** First words in the world: ***"Am I late?"*** — the thing he is
most afraid of is not the party. **He is not a fight and was said in public not to be one:** no
weapon, no intention of finding one. Offered: he answers anything honestly, inside what he knows.

**🕯 CLOCK + PUBLIC PROMISE.** ~8 minutes to grey. **The stand behind the door stays empty for as
long as he is out in the lane.** Stated verbatim: ***"I will tell you out loud the moment a real
timer starts. I do not run hidden clocks and I am not starting now."***

**📣 LIVE AND FREE (no die attached to any of them):** tie/gag him · talk to him · open the bag ·
take his keys · walk him somewhere darker · go through the door. **No preference stated, again.**
Fiction #the-story `cabac079-6f1d-47ea-a2f3-2f3a706d3cc4`; hand-back #in-character
`1531968887917383680` / `1531968890056216698`.

**Prior (07-29 ~09:45Z) — the tail's second leg and the four priced options:**

**🕵 LEG TWO HELD 2-of-3, AND THE PASS CAME FROM THE MAN WHO FAILED LEG ONE.** Forge
`d20(15) + 2 + 1d4(4) = 21` (queue `fa223f97`), Windreth `d20(10) + 10 + 1d4(1) = 21` (queue
`8502a90d`), Vale `d20(8) = 8` (queue `e36972fe`) — all three `/narrate` → 204. **Forge took the
roll instead of either free out and it paid; the "weak leg" read was RETIRED in public.**

**⚠ VALE'S 8 IS POSITIONAL, NOT A CAPTURE.** A stranger's shutter went up on the water side and put
lamplight across the lane; she folded into a crate gap and lost a street and a half. **Not caught,
not seen, not lost — she is ~60 SECONDS BEHIND and coming in from the water side.** Stated before
any decision: **if the party acts inside that minute, Vale is not in it.** That is the entire price.

**🚪 THE DESTINATION — HE WALKED THEM HOME.** The low lane bends inland and dies against **the foot
of Stair Hill**, the same hill they came off four hours earlier through the unclosable drain mouth.
From below it is retaining stone, tide-line, weed. **There is a low iron-banded door in the flank —
no lamp, no plate, no bell — the kind that exists so somebody need not be seen using the front.**

**🪑 THE BENCH, AND WHY THEY HAVE A WINDOW AT ALL.** He does **not** knock. He sits on a stone bench
beside the door — **worn into a dish by one man sitting in the same place for years** — lifts the
strap over his head, sets the satchel on his knees and **rubs his hands, slowly, one thumb into the
opposite palm.** *"Never once late in eleven years"* is only achievable by being **early every single
morning**, so the washer's mercy-bought detail is what put them here with time in hand and his back
turned. **~10 minutes to grey; then he goes in and walks to a stand with no book on it.**

**⚖️ FOUR OPTIONS PRICED IN PUBLIC BEFORE ANY DIE (`1531960854491435049` / `1531960856844173413`):**
› **① QUIET TAKE = ONE ROLL, NOT A COMBAT** — he is unaware, seated, old, both hands full, so it was
ruled not worth fighting for: **Dex (Stealth) *or* Str (Athletics) *or* Sleight of Hand, player's
pick, DC 13, group half-or-more → taken with no noise, no shout, no initiative.** **Fail = combat
opens with him surprised** (one clerk vs three, they still win) **but the noise is out four feet from
a door, and the noise is the actual cost.** ⚖ Arithmetic given in advance *again*: **acting now it is
Forge + Windreth and half of two is ONE; waiting ~60s for Vale makes it three and three needs TWO** —
waiting costs a minute of ten AND raises the bar, but buys her spells if it goes wrong. Trade named,
**no DM opinion attached.**
› **② TALK TO HIM ON THE BENCH** — **no roll to open**; the price is spending the surprise
irrecoverably. *He is not a fighter, but he only has to be loud.* Offered to answer as him honestly
within what he knows.
› **③ LET HIM GO IN AND FOLLOW** — buys **the place where the writing happens**, the one thing the
night has not shown them; costs the open street and means meeting the inside **after** it knows.
› **④ WATCH AND DO NOTHING** — free, real, ends at the grey.
🏃 **Bolt clause updated for the ground:** seated, bag on knees, bad knees — **the door is the only
thing he could realistically reach, and going for the door IS the "running away" their declared
capture clause covers.**
🕯 **Clock real but not a whip.** **No preference stated, and it was said out loud that none will
develop.**

Fiction #the-story `1350bcd0-f26e-4533-b5fb-7da905351e44`.

**Prior (07-29 ~04:00Z) — the pickup, the plate, and leg one:**

**🛡 FORGE IS AC 16 — WRITTEN TO THE SHEET, NOT PROMISED.** The pickup happened (Windreth crossed
back with the plate), he strapped it on, and the write went in: builder PUT with
`worn_armor:"breastplate"` on `d2d98745-…` → **`ac` 14 → 16, `equipped_armor` = breastplate**,
verified in DB with HP 50/50, gold 26 and all 20 inventory items preserved (additive edit, so
`preserveInventory` held). `ac_formula` is now empty by design — the armour branch supersedes
`10 + DEX + CON`. **Breastplate carries no Stealth penalty in the rules and that was said out loud**,
so nothing that happens on this tail is the armour's fault.

**🕵 LEG ONE HELD — 1 of 2, which is exactly half, which is a pass under the published rule.**
Vale `d20(18) = 18` vs DC 13 (queue `6b03b3a8`, `/narrate` → 204); Forge `d20(2) + 2 + 1d4(2) = 6`
(queue `0c289930`, `/narrate` → 204). The d4 was counted, it just had nothing to save. **Nobody was
lost and nothing has to be re-run.**

**🔍 FORGE'S FAILURE BOUGHT A CLUE, NOT A PUNISHMENT — and that was the promise, kept.** One boot
edge on a wet flag; the old man stopped for three breaths without turning around, then **walked a
pointless loop up between two blind warehouse walls and came out on the same lane further along.**
Ruled in public: **that is counter-surveillance, and a clerk does not know it.** *Who taught him,
or what did,* is deliberately left unexplained. He now walks slower and stands at corners.

**🧭 WINDRETH JOINED — declared 07-29 03:44Z, and the catch-up was ruled FREE, no roll.** Crossing
the quarter alone in the dark is the one thing he is better at than anyone at the table.

**📐 LEG TWO PRICED, ARITHMETIC SHOWN BEFORE THE DICE:** Dex (Stealth) **DC 13** each — he is more
careful, they know he checks, those cancel. **Three tailers means half-or-more is now TWO of three**
(it was one of two). **If Forge takes an out it is Vale + Windreth and the bar is back to one of
two.** Stated explicitly as arithmetic, **not** as steering. Forge's two free outs re-offered with
no judgement attached: hang back a street (no roll, no eyes on the ending) or **walk it openly**
(no roll, price is being *seen* and describable later).

**🏃 THE BOLT CLAUSE — PRICED, AS DECLARED.** @dewa in-character 02:36Z: *"We try to follow, but if
we notice him running away, we will capture him."* Ruled: **running him down is not the risk and I
refused to pretend it was** — he is old, slow, satchel on a long strap, and Forge alone moves 35.
**The risk is that he does not run *away*, he runs *somewhere*** — a door, a yard, earshot of people
paid to be awake early. **Taking him is cheap; taking him quietly where nobody hears is the real
roll, and it gets priced once there is an actual place to price it against.** Grabbing him **opens
combat**, and combat at dawn on a waterfront is loud.

**🎯 AMBUSH AT THE BEND IS STILL FULLY LIVE** — @JonathanEka's original shape (ride it to the
destination, then take him there) is untouched; they may walk leg two and still decide at the
corner. **Also named neutrally and then left alone: they have never ruled out simply speaking to
him.** *No DM preference, and it was said in public that no preference is going to develop.*

**🕊 THE WASHER IS GONE, AND THE PROMISE WAS KEPT.** She was still at the culvert when Windreth came
down the bank — note folded up her sleeve — and **she handed it over even though he had already
arrived and the message was now pointless, because that was the arrangement.** Then she went up the
bank and away in no particular direction. **She is not a trap and she was not made into one.**

Fiction #the-story `42b36e3d-48dc-4f16-abfb-bb92286ba107`; leg-two pricing + bolt clause
#in-character `1531873346575143067` / `1531873348751986818`.

**Prior (07-29 02:24Z) — the lift, the mercy, and the tail pricing:**

**🤏 THE QUIET LIFT LANDED BY EXACTLY ONE.** Windreth declared it in-character 07-29 02:03Z — *waits
for the duty hand's attention to settle on the slate, then slips the breastplate from the salvage
crate without touching anything else.* **`d20(5) + 7 + 1d4(4) = 16` vs the published DC 15.** Said
on the record: **without the d4 this is a 12 and the duty hand looks up.** Queue row `3db3a05a`
resolved via `/narrate` → 204. **🔴→✅ THE PLATE CLOCK IS CLOSED, NOT DEFERRED** — it never reached
the slate, so there is no description, no night, no dwarf-cut anything for anyone to read back.
**They beat the double-booked dawn by being fast, and that was named in public as a real win.**
🛡 **Forge goes AC 14 → 16 the moment he straps it on — not one second before; Windreth is carrying
it.**

**🕊 THE WASHER IS FREE — MERCY BRANCH TAKEN, both sides named out loud.** Declared by @dewa
in-character 02:19Z: Vale + Forge **write Windreth a note**, ask her to relay it when he returns,
tell her she is then free to go wherever she wants, and **Vale casts Guidance as a parting gift**.
*Cost, stated:* they gave up custody of **the only witness this system cannot erase**. *Bought:*
**she gave them THE LOW LANE unprompted** — *"He does not come up the stair… he comes up the low
lane, from the water side. On foot. Alone — always alone. He is old, and he walks slow, and in
eleven years he has never once been late."* plus *"If you are going to be somewhere, be there before
the grey."* ⚠ **A BINDING DM PROMISE MADE IN PUBLIC: "I am not going to turn her into a punishment
for letting her go. Her fate is not a trap I am holding in my back pocket."** Do not retcon her into
a consequence. ✨ **Guidance checked and legal** — Vale has it via the Tome (`granted_spells`);
counted as cast and **honoured as a gesture, not inflated into mechanics** (nothing for it to modify).
📄 **The note's wording is left to the players** — offered free either way, default reading is
*"here is where we went and why."*

**🕵 THE TAIL ON THE HAND — PRICED BEFORE ANY DIE, and it is a new scene type (tail, not combat).**
› **The pickup is free** — on the low lane before the grey costs time and no roll; they have the
time and she bought them the position. › **The follow is TWO LEGS, each a Dex (Stealth) check
DC 13**, group rule as already published (half or more clears the leg). › **Failure ≠ caught.**
It means he *notices something* — stops, doubles back, takes a turn he did not need. **Losing him is
the failure state; it is explicitly not a fight.** › **Forge named as the weak leg up front, with
two FREE outs:** hang back a full street (out of the check and out of reach) **or walk it openly** as
a dwarf heading to work at first light — **no roll at all, at the price of being SEEN by a man who
can describe him later.** › **Ambush instead of follow stays on the table** (floated by
@JonathanEka in #player-chat): *a follow buys a place, an ambush buys a man* — stated neutrally,
**no DM preference, no steering.**

**🧭 WINDRETH OPEN AND GENUINELY OPEN** — he has the plate and the whole night; said in public that
**a tail does not need three and two is often better than three, so joining is a choice, not a
repair job.**

Fiction #the-story `57337b88-1511-4f93-af55-b80de9d3a9a8`; mechanics via the `/narrate` resolution
on `3db3a05a`.

**Prior (07-29 01:53Z) — the split, the persuasion, and the hand:**

**🧭 THE SPLIT WAS DECLARED IN-CHARACTER — I did not decide it.** Windreth, #in-character 07-28
23:04Z: *"The woman can wait. The plate cannot."* … *"Let her wake. Keep her calm, give her water,
but ask no names and show her nothing from the book. I will be back before dawn."* **He goes alone.**
Vale + Forge hold the room. @JonathanEka 01:47Z declared **Forge Helps** Vale (pre-built cover
story); @dewa 01:53Z declared the approach: *wait until she is fully awake, then with kindness
persuade her to tell what she knows — operations, owners, any clue.*

**🗣 PERSUASION 17 — LANDS. DC 15, and I gave them the number plus the reason it was 15.** Roll and
declaration arrived together so I never got to pre-price it; **I said that out loud and then priced
it exactly as I would have beforehand.** 15 *because of how they did it* — waited for her to be
fully awake · led with kindness and water · **asked no names** · **showed her nothing from the
book** (Windreth's own instruction, actually followed). Said publicly: opening that book in front of
her is **not** a DC 15 conversation, and I would have warned them before the roll. **Forge's Help was
already inside the roll** — `d20(6/13 → 13) + 4 = 17`, advantage applied, Help spent, no re-roll
owed. Queue row `af9ad04c` resolved via `/narrate` → 204.

**🕯 WHAT SHE GAVE — and the wall a 25 would not have broken either.** *Bought:* the **quiet window
from the inside** (*"an hour they send us out"* — bar on the side door, bucket to the lane, come back
to a used room, chairs moved, boots on the wet; *"you learn not to look at the stand"*) · **the
count** — she cannot read, which is exactly why they let her stay while it is read aloud: she holds
the tally out loud while **the hand** writes · **the night it went to eight and did not go on** —
pen down, room silent, she was sent to the lane and stood there a long time. **She does not know
what is on line eight and never has.** · **THE HAND** — *"He is the one whose writing it is. All of
it. The whole book is one man's hand."* No name. **When the stand is empty and a delivery comes,
they send for him, and he comes at FIRST LIGHT to write.** *"They will have sent for him already."*
*Not bought — and stated as a real ceiling, not a stonewall:* **the owners.** She is a washer; they
put her in the lane before that door opens, every time, without exception. Heard the boots, never
the face. **Said in public: no d20 result makes a person have seen a face they were never in the
room for — the owners are up the ladder, not through her.** ⚠ Guardrails held: **nothing confirmed
about the buyer, nothing about what a struck name costs, and their own book/unlisted deduction
explicitly left to them.** What she *is* still unanswered — never touched. **Her one ask, flat, the
only thing she requested: "Do not take me back there."**

**⚔ WINDRETH ON THE PONTOON — TAKING ROLL PRICED OUT LOUD, NO DIE TOUCHED YET.** Travel ran **free,
25 min, no roll** (public towpath, as pre-promised). Board: plank shelf on floats, six skiffs in on
their lines, shift's end, crews gone up the stair. **One duty hand with a hooded lamp and a slate,
working the night's salvage crate.** Crate open, three feet from his boot; **the breastplate is in it
and NOT YET ON THE SLATE.** Window closes at the bottom of the crate or sunrise, whichever is first.
Four ways published, all binding: › **quiet lift** — Sleight of Hand *or* Stealth, player's pick,
**DC 15**; fail = he looks up, **not a fight** (municipal worker at end of shift), it becomes a
conversation with Windreth's face in it › **from the water** — no roll to get under the pontoon,
**Athletics DC 13** to bring a breastplate up out of water quietly; fail = noise + he comes to the
rail, and either way soaked, cold, more night gone › **straight at him** — **Persuasion/Deception
DC 13**, will probably clear, **the cost is not the roll: he writes a description on the slate**,
a name-shaped thing in the week they are avoiding name-shaped things › **walk away free**, no roll,
no cost, Forge stays AC 14. Also said as information not threat: **he is alone and help is 25 min
out — nobody knows his face and nobody is coming.**

**⏳ THE DAWN CLOCK IS NOW DOUBLE-BOOKED — and that is the pressure, not an ambush.** At first light:
**the plate stops being unlogged** AND **the hand arrives at the stair hill to write.** Same sunrise,
two places, and it was said in public that he cannot be in both.

**🕯 LINE EIGHT REMAINS A PLAYER CHOICE AND NEVER A DIE ROLL** — re-stated in the open: reading it,
or saying aloud what is written on it, is elected, never sprung.

Fiction #the-story `c10ff231-1497-4c1a-9fe4-17e1e16e50ba`; mechanics #in-character
`1531843400104677507` / `1531843402117808159` + the `/narrate` resolution.

**Prior (07-28, 14:16Z) — the lamp correction and the book read to the bottom:**

**🧭 DESTINATION CHOSEN — ③ THE OLD CULVERT.** Worked out by the players in #player-chat (Javanese)
at 14:02–14:04Z: Windreth proposed *the old culvert first, wait for the washer to wake so she can be
questioned, then read the book at open water at dawn*; **@dewa** agreed and proposed the split
(*"me and Jona wait for her to wake, you go find Jona's armour with stealth?"*); Windreth answered
*"boleh."* Ran the move as **free — 20 ft, no hurry, nobody watching, no rolls.** They are 40 ft in,
**past where the stone changes hands**, on a dry ledge above the waterline. No salt anywhere.

**🏮 A DM CORRECTION MADE IN PUBLIC, AND IT WENT THEIR WAY: FORGE HAS BEEN CARRYING A LANTERN ALL
NIGHT.** `lantern-hooded` ×1 and `lamp-oil` ×1 are on Forge's sheet, in the pack he cached at the
reeds and got back an hour ago. **I had published that option ③ was "pitch dark, so it does not buy
the tier" — that was wrong and I said so plainly rather than making them argue me out of it.**
Darkness plus a lamp is lamplight; the condition I named was *light*, not *sunlight*. **All three
conditions met: light ✅ (hooded to a coin, nothing reaches the mouth) · time ✅ (2 hrs to dawn,
nothing to spend them on) · no clock ✅ (a stone room nobody in Sesh knows is a room).**

**📖 THE 20+ INVESTIGATION TIER — RELEASED, WITH NO ROLL, EXACTLY AS PROMISED.** Payload delivered
verbatim to the table row on `game-state.md:209`: **a few names repeat** — four or five out of
hundreds, months apart, different inks, same careful hand — **and in every repeat it is the EARLIER
entry that is struck through**; the later one stands clean and unmarked. **Left unexplained on
purpose.** Nothing confirmed about the buyer, about what a struck name costs, or about the party's
own book/unlisted deduction (`campaign-arc.md` guardrail holds). ⚠ **Said explicitly in public: the
plate deadline is a clock on the PLATE, not on the reading — no fourth condition added through the
back door after the fact. The tier is paid and stays paid.**

**🛡 THE BREASTPLATE QUESTION ANSWERED IN FULL** (JonathanEka To-DM 14:04:11Z: *"is it simply a check
to find the right boat? or need to travel to a pier nearby (if such exist)"*). Answered as **a
place, a clock, and one uncertain moment — not one check, not a dungeon**: › **the pier exists** —
night-watch skiffs tie up at a **watch pontoon down-cut** at shift's end; municipal, not secret, so
**finding it costs time not a roll**, **~25 min each way** from the culvert mouth · › **the clock is
dawn and it is real** — loose salvage in a boat bottom belongs to nobody, but **once it comes off at
shift's end it gets logged**, and a dwarf's breastplate out of the canal in the same week the watch
is dragging that canal for two sunk bodies gets looked at twice (built from existing canon, not
invented pressure) · › **the roll, if any, is TAKING it, not finding it** — moored skiff, a hand
maybe asleep aboard, maybe a watchman on the pontoon; Stealth / Sleight of Hand, **to be priced out
loud before anyone goes** · › **the honest cost of the split: ~1 hour, and Windreth is not in the
room when she wakes.**

**⏳ SHE IS COMING UP — inside the hour, well before dawn.** Set concretely as a DM call, **no roll**,
consistent with the published "no repeated rolls to keep her stable." Her hand closes and opens on
the ledge. **Not played out** — the interrogation waits on whether Windreth is present for it.

**🧭 PENDING — THE ONE OPEN DECLARATION: does Windreth go for the plate, or do all three stay?** The
split was agreed in #player-chat but **not declared in-character**, so I asked for it in the open
and stated **I have no preference and both are real plays**. Not deciding it for them.

Fiction #the-story `aac4cec8-8ae0-4f27-92b6-0baf5d68880a`; mechanics #in-character
`1531666661529223338` / `1531666663324389419` / `1531666665367142400`.

**✅ THE GROUP STEALTH CHECK PASSED — resolved exactly as pre-priced.** **Windreth 17** (`d20 6 + 10
+ 1d4 1`) ✅ · **Forge 23** (`d20 **20** + 2 + 1d4 1`) ✅ vs **DC 15**. Two of three is half or more
⇒ **the party succeeds**; **Vale never needed to roll and owes nothing**. Windreth's player made
exactly this argument in a To-DM at 13:42:04Z and **I confirmed his read out loud rather than
re-litigating a line I published before any die was touched** — moving the bar after seeing the
numbers would have been bad faith. Forge's **natural 20 while carrying an unconscious adult** was
paid back in the fiction (the man with a body on his shoulder is the quietest thing on the reed
line). Both `dm_queue_items` resolved via `POST /dashboard/queue/{id}/narrate` → 204.
**Consequence: the lantern crossed the reed line and kept going.** No mark, no report, nobody
looking for them, **and the destination stays free — all three options still on the table.**

**🎒 FORGE'S KIT — "can we grab Forge's gears back?" answered YES, and it was already free.**
@dewa asked at 13:44:47Z (JonathanEka in #player-chat: *"nganti lali klambi ku"*). Ruled: the
re-equip was **already applied last beat as a published default** — belt, pack, boots, clothes, no
roll, no cost, no time, because the kit was cached in the open at the reed line and they came out
on top of it. ⚠ **The breastplate is the one thing not there, and that is planted-not-sprung** —
the skiff-hand hooked it as salvage on **07-27**, in the same beat the kit was stated to be
recoverable *and findable*. **Forge is AC 14 and stays AC 14 until it is back on his chest.** Told
them plainly it is **not gone forever: boats tie up somewhere** — a thread to pull, not a loss to
eat.

**🔢 "EIGHT." — SHE TALKS IN HER SLEEP, AND IT IS NOT A NAME.** Carried out of the reeds, the
washer says one word twice, flat, *the way people count*: **"Eight."** Deliberately **a number and
not a name**, so it plants **LINE EIGHT** without touching the summon trigger (which stays a
player choice) and without confirming the party's own book/unlisted deduction (`campaign-arc.md`
guardrail). **Refused to explain it in public** and will keep refusing.

**🧭 THE FORK — the only thing pending is a player declaration.** Forty feet on, the mud path
splits three ways in the dark: **left** bends with the cut toward open water and a dawn ~2 hrs off;
**right** climbs a broken towpath toward the one warm chimney in the quarter (flour on the wind);
**behind them** the old culvert mouth breathes cold. Same three options, same honest prices, **not
changed by the roll**. Also still theirs and unsteered: **she is going to wake up — mercy or
shopping.**

Fiction #the-story `5891fd33-7cc7-4d31-93df-57fc0ed56805`; mechanics #in-character
`1531660440625479742` / `1531660442496274603`.

**🌾 THE EXIT BEAT — "take the washer alive, out through the grate, before the delivery, no names
aloud."** Windreth declared it at 13:16:57Z, **@dewa** answered *"We follow Windreth"* at 13:21:25Z,
and Forge asked *"where can Windreth go to find a place with better light?"* — **one plan, one
beat.** Everything up to the reeds resolved **with zero rolls**, and the reason was published: the
grate was already ruled an **open drain mouth, passable at a crouch, both ways, unlimited**;
**nothing was watching that yard** (my own written promise, not walked back); and **carrying her is
slow and awkward, not uncertain** — consequences, not checks.

**⚙ Defaults applied, all published as freely reversible at no cost:** **Forge carries her** (STR 15
→ 225 lb capacity, nowhere near it, **no encumbrance, no speed penalty**); **the book rode inside
Windreth's shirt** and is dry; **Forge re-equipped his cached kit free** in the reeds (⚠ still **no
breastplate** — watch skiff, **AC 14 unchanged**); **she is alive, unconscious and stable**, no
repeated rolls to keep her that way.

**⚠ THE PRICE OF TAKING HER, STATED NOT SPRUNG.** An unconscious washer who wakes in that yard is a
confused woman with a story nobody credits. A washer who is **simply gone**, over a destroyed grate
and broken threshold salt, is **a hole in a nightly routine** — they have upgraded a survivable mess
into **a disappearance**. In exchange they hold **the only witness this system cannot erase**, in
custody, able to answer questions when she wakes. Told them plainly it was a real trade made with
open eyes. *Mercy or shopping* still theirs. ⚠ **Whether she is staff, warden, or something the yard
keeps stays deliberately unanswered** (orphan thread, `campaign-arc.md`).

**🗣 "No names aloud until we have light, distance, and time."** — logged and honoured, with the
mechanism published: **the summon trigger is a player choice, not a random event.** It will never be
fired on a die roll or a whim. It is theirs to not pull.

**📣 THE DELIVERY ARRIVED WHILE THEY WERE UNDER IT** — heard through the stone from inside the
culvert, **never seen, nothing described**: *"Something arrives at the stair gate."* Buyer stays
sealed; the clock paid off as tension, not as an ambush, and **no watcher was invented in the yard**.

**🪨 THE CISTERN SEED SURFACED** (planted 07-27, orphan thread): halfway down, Windreth places what
he noticed on the way up — **the culvert changes hands.** Yard-end stone is squared, warded, young;
this stone is older than the wall, older than the salt. Never explained.

**🎲 ✅ RESOLVED — the one roll, pre-priced before any die was touched:** **GROUP Stealth check, DC 15**
(`/check skill:stealth`, one each; **half or more succeed ⇒ party succeeds**, RAW group check, used
deliberately). **DC 15 not 14 because "a body does not crouch when you tell it to."** ✅ Pass = gone
clean, destination free. ❌ Fail = **not a capture** — the lantern sits on the reed line, the watch
marks the spot, the **lit** options die for tonight and they go to ground close and dark.
**Outcome: PASSED 2-of-3 (17 / 23), pass branch paid in full, nothing spent.**

**💡 "WHERE IS BETTER LIGHT?" ANSWERED WITH THREE REAL OPTIONS + HONEST PRICES** (and the tier
payout restated: *with a lamp and no clock he does not roll again, he just gets it*):
**① dawn on open water down the cut** — free light, no owner; costs **~2 hours** holding a stolen
book and a stolen woman · **② the bakery / the Quiet Window** — lit oven right now, indoors, warm;
costs the **yard man who has already seen them** and means reading the book **inside the chain that
made it** · **③ the old culvert, the stone that changes hands** — dry, private, unknown, **20 ft
behind them**; costs **pitch dark, so it does not buy the tier** (buys privacy, time, and a place to
let her wake).

Fiction #the-story `1af887a2-29f1-4f3d-8152-30ee5488ce18`; mechanics #in-character
`1531653947591360512` / `1531653949160034385` / `1531653951437406290`.

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

**Clocks:** **🔴 ~8 MINUTES TO THE GREY — AND THE MAN WHO WAS SUPPOSED TO WALK THROUGH THAT DOOR IS NOW IN THEIR HANDS.** ✅ **THE TAKE LANDED CLEAN** (Windreth 27 / Forge 20 vs DC 13, needed one of two): no combat, no noise, nothing on the hill alerted, **and he never saw a face.** The stand behind the door stays empty for exactly as long as he is out in the lane with them. **Public promise attached: no hidden clocks — a real timer will be announced out loud before it runs** · ~~🔴 THE TAIL IS RUNNING~~ (**✅ BOTH LEGS HELD** — 1-of-2 then 2-of-3; he checked for company once and found none — **he knows how to check for a tail and did it once, on Forge's scuffed flag, and found nothing**) · ~~🔴 THE TAIL IS RUNNING~~ (**✅ BOTH LEGS HELD** — 1-of-2 then 2-of-3; he checked for company once and found none — **he knows how to check for a tail and did it once, on Forge's scuffed flag, and found nothing**) · **🔴 THE HAND COMES AT FIRST LIGHT** — new and the sharpest one on the board, given by
the washer herself under a DC 15 Persuasion on 07-29 01:53Z: the whole book is **one man's hand**,
no name known to her, and **when the stand is empty and a delivery comes they send for him — he
arrives at dawn to write.** *"They will have sent for him already."* **Same sunrise as the plate
deadline**, so the two clocks are double-booked and it was said in public that Windreth cannot be in
both places — **and they beat it: the plate was recovered early and unlogged, so the hand is now the
ONLY thing landing at dawn.** ⚠ **07-29 UPDATE — the washer gave up his approach for free, earned by
mercy: he comes up THE LOW LANE from the water side, NOT the stair gate. On foot, alone, always
alone, old, slow, and never late in eleven years.** *"Be there before the grey."* **The party is
setting a tail on him** (two legs, Dex Stealth DC 13 each, priced in the ★ Now block). What he *is*
beyond "the one whose writing it is" stays sealed; **do not narrate what a written name costs** · **✅ THE WASHER HAS TALKED AND BEEN LET GO** — mercy branch taken 07-29 02:19Z; she is out of
their custody and out of the yard, carrying a folded note for Windreth and nothing else they owe her.
⚠ **BINDING PROMISE: she is not a trap and her fate will not be used as a punishment for the mercy.**
What she gave before she walked: awake, calm, given
water, **asked no names and shown nothing from the book** (Windreth's standing instruction, followed
by his players): she gave the quiet window from the inside, the stand, the count, the night it
stopped at eight, and the hand. She did **not** give the owners and **cannot** — a washer is put in
the lane before that door opens, every time; ruled in public as a **real ceiling, not a stonewall**,
and that no d20 result changes it. **Her one ask: "Do not take me back there."** *Mercy or shopping*
still undecided; **what she is stays unanswered** · no longer a clock in the yard, now a **person in their
custody**: alive, awake, stable, carried out over Forge's shoulders on 07-28. She *remembers*,
she wakes up eventually, and she is still the only witness this system can't erase. ⚠ Taking her
**converted a survivable mess into a disappearance** — a missing washer is a hole in a nightly
routine in a way a confused one is not. *Mercy or shopping* undecided; **what she is stays
unanswered**; **🔢 she talks in her sleep and said a NUMBER, not a name — "Eight," twice, flat,
the way people count** (plants LINE EIGHT without touching the summon trigger; refused explanation
in public) · **🔴 the destroyed grate** — a hole that can't be closed, on iron she salted nightly,
and the route they just used **out** · ~~🔴 Vale's mask~~ (lapsed and now moot — they are out of the
yard) · ~~the loud bar~~ (spent — thrown, door open, and nothing answered the noise) ·
**✅ ⏰ THE DELIVERY LANDED** — it arrived at the stair gate 07-28 while the party was face down in
the culvert **eight feet under it**: heard through stone, **never seen, nothing described**, buyer
still sealed. They got out ahead of it by minutes. **What it was, and what it does now it has found
an empty stand and a missing book, is entirely open** · ~~**🗣 "Do you have a name for me?"**~~
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
**✅ FORGE'S BREASTPLATE IS BACK AND THE CLOCK IS CLOSED** — quiet lift on the pontoon 07-29 02:03Z,
**Sleight of Hand `d20(5)+7+1d4(4)=16` vs DC 15, by exactly one** (without the d4 it is a 12 and the
duty hand looks up). **It never reached the slate**, so there is no description, no night and no
dwarf-cut anything on any municipal record — the watch's thread is CUT, not deferred. Windreth is
carrying it; **Forge goes AC 14 → 16 the moment he straps it on, not before.** *(History:*
taken as salvage and
going down-cut: recoverable if they chase where it ties up, and meanwhile a dwarf's breastplate
pulled from a canal is a thread the watch now holds — **07-29: Windreth is STANDING ON THE PONTOON,
alone, help 25 min away.** Travel ran free and no-roll as promised. The plate is in an **open salvage
crate three feet from a duty hand's boot**, lamp and slate in his hands, **and it is not on the slate
yet** — the window closes at the bottom of the crate or at sunrise. **Four ways priced out loud and
binding before any die:** quiet lift **DC 15** (Sleight of Hand *or* Stealth, player's pick; fail =
he looks up, **not a fight**, a conversation with Windreth's face in it) · from the water, no roll to
get under, **Athletics DC 13** to lift it out quietly (fail = noise + rail, either way soaked and
cold) · straight at him **Persuasion/Deception DC 13** (will likely clear — **the cost is the slate**,
a written description in the week they are dodging written things) · **walk away free.** No die
touched yet — all four options are now moot, the quiet lift was chosen and it landed.)* · ~~the dwelling lantern~~ (cleared by the
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
ISSUE-060 (builder omits Warlock pact boon / invocations);
**NEW 07-28 — freeform `/action` silently eats a turn when the action *does* have a command.**
`/action "shove RDR"` spends the Action and zeroes `AttacksRemaining`, so the player's real `/shove`
is then rejected for having no attacks (it cost Forge his whole turn mid-fight, restored by hand).
`internal/refdata/action_catalog.go` already knows every action's key, economy and command string
(`{Key: "shove", Economy: EconomyAction, Command: "/shove <target> [prone|push]"}`), so the freeform
handler can match the description against the catalog and **reject before spending anything** with
*"that is `/shove` — use it instead"*. Fix is bounded and has an obvious SSOT; not yet written.

Durable lore → [`world.md`](world.md); secret spine → [`campaign-arc.md`](campaign-arc.md);
play-by-play → [`sessions/session-01.md`](sessions/session-01.md).

> **⚠ Session-log coverage gap.** `sessions/session-01.md` stops at **2026-07-19** (the canopy
> reading). The entire Night Weigh / Far Quarter leg — 07-21 through 07-27 — was played out of
> the DM's campaign memory note and **never appended here**. Treat the session log as complete
> only up to 07-19; for anything after that, the memory note
> `project_night_weigh_weighhouse_staged` is the record. Backfilling it is worth doing, but do
> not reconstruct beats from summary — pull them from the #the-story message ids recorded in
> the memory note.
