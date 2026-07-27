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
found, now"* — and **all three are now inside the culvert throat** (Windreth → Vale → Forge;
40 ft of stone, no room to turn around). Forge declared **in, stripped**, kit cached in the reeds.
Water-margin Stealth resolved at **DC 14 — Windreth 26 ✅ / Forge 18 ✅ / Vale 7 ❌.** The failure
paid out exactly as pre-declared, *not* as a capture: her boot went on the salt-polished lip,
Windreth's margin caught her before the scrape became a splash, and **the watch skiff's lantern
swung, stopped, and now dwells on the culvert mouth** — it doesn't know what it heard, but it
sits on the one proved exit. Windreth's +12 also bought the read ahead: **the washer is inside
that grate right now**, sweeping and salting. Lantern behind, washer ahead. Their move.

**Still sealed:** the buyer's identity (a shape filled the lit doorway and did not come out —
deliberately un-described), what is inside the stone building past the chalk line, how THE SEAL
is restored, and "patron = buyer" stated flat.

**Clocks:** **🔴 the dwelling lantern** — a watch skiff's light stopped on the culvert mouth,
blocking the proved exit and sitting over Forge's cached kit · **the washer's night round**,
happening now on the inside of the grate (no weapon, no lantern, *remembers*) · two sunk counters
in the canal ~1.5 days into a 1–3 day fuse · the watch now dragging the cut **at night** · the
gate-post found by the relief watch · **the countersign has rotated — it is now a wrong answer
delivered confidently** · COL the Collector loose with all three faces · a bakery yard man has
seen them at the counter.

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
2. **The culvert run resolved 07-27** (one beat, as designed). Next: whatever they declare in the
   pipe. Two live pressures, both already on the table — the **dwelling lantern** behind and the
   **washer** ahead. Don't prompt their levers (Name-Muffle, Mage Hand, minor illusion, waiting,
   or simply going past her); let them find them. **Non-lethal MO stands until a player flips it.**
   **Forge is AC 14 while stripped** (see the equipment note below).
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
