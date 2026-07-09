# Combat-Ops Improvement Backlog — Handover (opened 2026-07-09)

A retrospective from the first live combat start (encounter `30baba5f`, "The Follower"
fight). It records where the current tooling fought the DM and turns each pain point into
a **pickup-able** item for the next agent. Two tracks, per the request that opened this doc:

- **Part A — App** (product/eng): defects and missing endpoints in the DnDnD service.
  Graduate accepted items into [`issues.md`](issues.md) as `ISSUE-070+`.
- **Part B — DM harness** (procedure): how a DM agent should operate. Encode accepted
  items into [`dm-rules.md`](dm-rules.md) (rules) or [`runbook.md`](runbook.md) (how-to).

Every item cites the concrete evidence from this session so nothing here is speculative.
An **Appendix** captures the verified API/DB facts so future agents execute instead of
re-investigating.

**Priorities:** P1 = blocks or forces an unsafe workaround every combat · P2 = notable
friction · P3 = polish.

---

## Context — what happened this session

1. A player-reported bug: `/roll d20+2` silently failed (bare die, no count). Fixed
   red/green (`e967364`), pushed, redeployed. Root class: the command help advertised
   `d20` as valid while the parser rejected it, and **no test covered the advertised
   forms**.
2. First live combat start. The three players rolled their own initiative in
   #roll-history. Starting the encounter required a **discard → override → DB seat-repair**
   dance because `POST /api/combat/start` auto-rolls initiative for everyone (PCs included)
   with no opt-out, which collides head-on with the standing rule "never roll a PC's die."
3. A raw DB `UPDATE` was needed to seat the correct round-1 first actor, because no app
   endpoint can re-seat or rewind the active turn.

The outcome was correct and faithful, but it took ~a dozen investigative round-trips and
one guarded DB write. Most of that is avoidable.

---

## Part A — App improvements

### APP-1 (P1) — Player-authoritative initiative at combat start — ✅ DONE 2026-07-09
- **Shipped:** `POST /api/combat/start` now accepts an optional `character_initiatives`
  map (`{ "<charID>": { "roll": 19, "order": 1 } }`, order optional). Supplied PCs skip the
  auto-roll and their reported total is used verbatim; only combatants without an entry
  (NPCs, un-supplied PCs) auto-roll. Seat order is derived from the rolls (roll → DEX →
  name → uuid via the new pure `AssignInitiativeOrder`) or pinned by the optional `order`.
  Malformed orders (non-positive / duplicated) 400 before any DB write; an out-of-range
  seat is rejected in `AssignInitiativeOrder`. `RollInitiative` kept its signature via a
  thin wrapper over the new private `rollInitiative(…, supplied)`.
- **Problem (original):** `POST /api/combat/start` always auto-rolls initiative for **all**
  combatants, PCs included, with no opt-out, and seats the first turn on the app's highest
  roll. There is no way to start combat from player-supplied initiative values.
- **Evidence:** this session the app auto-rolled Windreth to 22 and seated him first; the DM
  had to override all four combatants to the players' real totals (Forge 19 / Windreth 14 /
  Vale 14 / Follower 5) and then repair the seat.
- **Proposed fix:** accept an optional `character_initiatives` map on the start body, e.g.
  `{ "<charID>": { "roll": 19, "order": 1 } }` (order optional — derive from rolls when
  omitted). For each supplied PC skip the auto-roll and use the value; auto-roll only
  combatants **without** a supplied value (i.e. NPCs). Seat the first turn on the true
  order-1. NPC auto-rolls (a legitimate DM-side die) are unchanged.
- **Acceptance:** start with supplied PC inits → tracker order and `first_turn` match the
  supplied values with **zero** override calls and **no** DB touch.

### APP-2 (P1) — A re-seat / set-active-turn capability — ✅ DONE 2026-07-09
- **Shipped:** new `POST /api/combat/{enc}/set-active-turn { combatant_id }` (chose design
  variant a). It reseats **in place**: guards (encounter has an un-acted active turn; target
  is a living, non-summon combatant in the encounter that hasn't already acted this round;
  target ≠ current), then reassigns the current active turn row to the target via a new
  `ReseatTurn` sqlc query that resets every per-turn column (movement/attacks/action flags/
  timer). Because the row is reassigned, `current_turn_id` already points at it — no pointer
  write, no double active row, no FK/delete hazard. The displaced combatant loses its turn
  row and is picked again at its true initiative order. Guard conflicts → 409, missing target
  → 404; route added to BOTH `RegisterRoutes` + `mountCombatDashboardRoutes` (parity test
  green). Enemy-turn-ready stub hygiene handled; zone/summon/death-save side effects
  intentionally out of scope for a pre-action seat-repair.
- **Problem (original):** the initiative-override writes `initiative_order` but never moves
  the current-turn pointer, and no endpoint rewinds or re-seats the active turn (only
  `advance-turn` forward and `undo-last-action` for in-turn effects). So when `start` seats
  the wrong first actor, the only fix is a raw DB write.
- **Evidence:** `UPDATE turns SET combatant_id=<Forge>, movement_remaining_ft=25 WHERE
  id=<the auto-seated Windreth turn>` — done by hand this session.
- **Proposed fix (pick one):**
  - (a) `POST /api/combat/{enc}/set-active-turn { combatant_id }` that, when the target has
    not yet acted this round, discards any premature same-round turn row for a
    not-yet-acted combatant and seats the target via the normal `createActiveTurn` path
    (so timer, movement, effect-slot reset all run correctly); **or**
  - (b) make the initiative-override, when it changes order **before anyone has acted in
    round 1**, re-derive the seat from the new order-1.
- **Acceptance:** after correcting order pre-action, the active turn is order-1 with no DB
  write, and the displaced combatant still takes its turn later this round.
- **Note:** APP-1 + APP-2 together retire the entire discard/override/repair dance.

### APP-3 (P2) — Initiative-override endpoint safety
- **Problem:** the override request struct uses non-pointer ints, so omitting
  `initiative_order` silently writes `0` (jumping that combatant to the front). It writes
  both fields unconditionally and enforces no uniqueness/contiguity, so duplicate orders are
  possible.
- **Evidence:** the investigation flagged the `0`-write footgun; this session every call had
  to send both fields defensively.
- **Proposed fix:** make `initiative_roll` / `initiative_order` pointers (omit = leave
  unchanged); reject a duplicate `initiative_order` (or auto-renumber to keep a clean 1..N);
  optionally renumber siblings on an order change.
- **Acceptance:** a roll-only override leaves order intact; a duplicate order is rejected or
  cleanly resolved.

### APP-4 (P2) — Coordinate-model consistency + better 400s
- **Problem:** `character_positions` uses `Position{ Col string /* letter */, Row int /*
  1-based */ }`, while encounter-template creature placement uses integer `position_col` /
  `position_row`. Sending a numeric `col` to `start` returns a bare `400 "invalid JSON
  body"` with no field named. There is also an off-by-one risk (template row vs combatant
  row base).
- **Evidence:** first `start` attempt this session 400'd because `col` was sent as `3`
  instead of `"D"`; the cause was only found by reading the struct.
- **Proposed fix:** unify on one coordinate representation across template + start, or accept
  both and document it; return a field-level decode error (e.g. `col must be a letter
  A–P`).
- **Acceptance:** consistent coords across the two endpoints; a type mismatch names the
  offending field.

### APP-5 (P2) — A player self-initiative command that feeds the tracker
- **Problem:** `/roll d20+N reason:initiative` is cosmetic — it posts to #roll-history but
  does not feed the combat tracker. Players have no way to submit their own initiative into
  an encounter, which is the root reason the DM must roll-then-override.
- **Evidence:** all three players posted `initiative` rolls to #roll-history that the tracker
  never saw; the DM transcribed them by hand.
- **Proposed fix:** an `/initiative` command (or a pre-combat "everyone roll initiative"
  prompt) that, when an encounter is pending/starting, records the player's own roll into
  the tracker. Pairs with APP-1.
- **Acceptance:** player `/initiative` seats their own value; the DM performs no override.

### APP-6 (P3, PARTLY DONE) — Test the forms the help advertises
- **Problem:** `/roll`'s help, command-option description, and `rollExamples` advertised
  `d20`, which `ParseExpression` rejected; there was no test over the advertised forms.
- **Evidence:** the `/roll d20+2` bug (fixed this session in `e967364`).
- **Proposed fix (remaining):** add a test that iterates every example string in
  `rollExamples` / the command help through `ParseExpression` and asserts each parses. This
  guards the whole class, not just today's case.
- **Acceptance:** a regression test fails if any advertised example stops parsing.

### APP-7 (P3) — Opaque decode errors on write endpoints
- **Problem:** `start` returns bare `"invalid JSON body"`; homebrew-creature create uses
  `DisallowUnknownFields` plus `sql.Null*` / `pqtype.NullRawMessage` wrapper shapes that
  `400` on any drift. Both are painful to debug blind.
- **Evidence:** the `col`-type 400 (APP-4) and the documented null-wrapper hazards from prior
  combat builds ([[project_liveplay_build_combat_via_api]]).
- **Proposed fix:** surface field-level decode errors on these endpoints.
- **Acceptance:** a wrong field/type is named in the response.

### APP-8 (P3) — A read-side combat-state endpoint
- **Problem:** there is no clean GET for "current active turn + combatant order + hp"; the DM
  queried Postgres directly to verify the seat.
- **Evidence:** this session verified the repair via `psql` against `encounters` / `turns` /
  `combatants`.
- **Proposed fix:** `GET /api/combat/{enc}/state` → round, current-turn combatant, and each
  combatant's order/roll/hp/active flag.
- **Acceptance:** one authenticated GET returns the tracker state the DM currently derives
  from the DB.

---

## Part B — DM harness improvements (how the agent should operate)

> **Status (2026-07-09): Part B encoded.** DMH-1/-4/-6 → [`dm-rules.md`](dm-rules.md);
> DMH-2/-3/-6/-7 → [`runbook.md`](runbook.md); DMH-5 noted as a deferred one-off in
> [`README.md`](README.md) (banner surgery held back — a concurrent DM agent was live on
> `game-state.md`). **Part A: APP-1 + APP-2 DONE 2026-07-09** (together they retire the
> discard/override/DB-repair dance); APP-3…8 still open for the product track.

### DMH-1 (P1) — Verify slash-command syntax **before** prompting players — ✅ DONE (encoded in [`dm-rules.md`](dm-rules.md), "At the table" → *Verify every slash command's syntax BEFORE you put it in a coda*)
- **What went wrong:** the Beat-18 initiative prompt told players `/roll d20+2` — the exact
  broken form. A player who tried it got a private "Couldn't read" error.
- **Fix:** before putting any exact slash command in an OOC coda, grep
  `internal/discord/commands.go` for the command + its option names and confirm the argument
  shape. (This was done correctly for `/move` / `/bonus` / `/action` later this cycle — make
  it the standard, not the exception.)
- **Encode in:** [`dm-rules.md`](dm-rules.md) ("Nudging with OOC hints" / command-menu rule).

### DMH-2 (P1) — Prompt initiative with exact, unambiguous syntax — ✅ DONE (encoded in [`runbook.md`](runbook.md) §4 "Starting a live combat", step 2)
- **What went wrong:** Vale and Forge rolled bare `1d20` (no modifier); Windreth rolled
  `1d20+4`. The inconsistency came from an ambiguous prompt (and the roll bug). The DM then
  had to add the missing modifiers by hand.
- **Fix:** give each player their literal string including their modifier — e.g. "Forge:
  `/roll 1d20+2 reason:initiative`" — and state "include your +N; I read the total." Once
  APP-5 lands, prompt `/initiative` instead. Adding a player's **fixed, known** modifier to
  a die they reported is legitimate adjudication (a deterministic stat, not a re-roll) and
  stays within "never roll a PC's die" — but avoid needing to by prompting cleanly.
- **Encode in:** [`runbook.md`](runbook.md) (combat-start how-to).

### DMH-3 (P1) — Follow the documented combat-start procedure; do not re-investigate — ✅ DONE (encoded in [`runbook.md`](runbook.md) §4 "Starting a live combat" checklist, linking the Appendix + memory)
- **What went wrong (cost, not error):** the start body shape, `Position.Col` letter format,
  the override contract, and the turn-ordering rules were all re-derived live via subagents
  and file reads.
- **Fix:** the reusable facts now live in memory [[project_combat_start_pc_init_seat_repair]]
  and this doc's **Appendix**. Read those first, then execute. Only investigate if the code
  has changed under them.
- **Encode in:** [`runbook.md`](runbook.md) — add a "Start a live combat" checklist that
  links the Appendix.

### DMH-4 (P2) — Minimize live DB writes; non-destructive UPDATE, never a blind DELETE — ✅ DONE (encoded in [`dm-rules.md`](dm-rules.md), "How DM actions must be performed" → *DB-repair exception*)
- **What went right (keep it):** the seat repair was a single, tightly WHERE-scoped
  `UPDATE` on a fresh, un-acted turn row — no `DELETE`, verified after. That is the correct
  bar for the standing "guarded one-off repair when no endpoint exists" allowance.
- **Fix / standard:** never `DELETE` a live game row without explicit user confirmation;
  scope every write with a tight `WHERE` on ids; read-verify immediately after. When
  APP-1/APP-2 ship, drop the repair entirely.
- **Encode in:** [`dm-rules.md`](dm-rules.md) (the DB-repair exception clause).

### DMH-5 (P2) — Refactor the `game-state.md` banner (concurrent-edit hazard) — ◻ NOTED, refactor DEFERRED (cleanup task recorded in [`README.md`](README.md) resume item 3; the actual banner surgery is left for a quiet moment — a concurrent DM agent was holding `game-state.md` when this was picked up, and blind-editing that live file is the exact clobber hazard this item warns about)
- **Problem:** the `_Last updated:` banner is a single ~5 KB line that accretes every beat.
  It is risky to edit surgically and clobber-prone when a concurrent agent is also writing
  the live-play docs; this session the banner's current-state pointer was nearly edited
  blind because a preview truncated before reaching it.
- **Fix:** cap the banner at a short current-state pointer (one or two sentences) and keep
  beat history in the numbered next-action bullets / [`sessions/`](sessions/) (already the
  pattern). A structural cleanup a future agent can do in isolation.
- **Encode in:** a one-off cleanup task; note in [`README.md`](README.md) resume order.

### DMH-6 (P3) — Package the render-verification + stat-leak check — ✅ DONE (reusable snippet + pass criteria in [`runbook.md`](runbook.md) §8 "Render + stat-leak check"; pointer added in [`dm-rules.md`](dm-rules.md) "Enemy HP and AC are secret")
- **Problem:** after every narration the DM hand-writes a JS structural check (embed follows
  content, reveal present, no `AC/HP/CR/id` leak). Re-authoring it each time is wasteful and
  drift-prone.
- **Fix:** capture the exact snippet + the pass criteria as a reusable checklist in
  [`dm-rules.md`](dm-rules.md) "Keep the record straight" / a `runbook.md` recipe.

### DMH-7 (P3) — Keep a DB schema cheat-sheet for DM reads — ✅ DONE (cheat-sheet added to [`runbook.md`](runbook.md) §6; the Appendix below remains the fuller reference)
- **Problem:** several read queries failed on column-name guesses (`is_complete`,
  `speed_ft`, `speed`), costing round-trips.
- **Fix:** the corrected names are in the Appendix; reference it before querying.

---

## What went well — preserve these

- **Red/green TDD** on the `/roll` fix (failing bare-`d` tests first, then the minimal regex
  change), plus committing only the two dice files (live-play docs left unstaged).
- **Verify-before-prompt** for `/move` / `/bonus` / `/action` (grepped `commands.go` first).
- **Faithful initiative:** discarded the app's PC rolls, adjudicated from the players' own
  dice, added only fixed modifiers, and honored the DEX tie-break — no fudging.
- **Non-destructive** seat repair (UPDATE, not DELETE), tightly scoped and verified.
- **Seal discipline:** narration was a partial reveal only; no `AC/HP/CR/creature-id` leak,
  render verified structurally each post.
- **Docs in lockstep** and a **memory** captured for the recurring pattern. The concurrent
  agent's `.claude/scheduled_tasks.lock` and its doc edits were left untouched.

---

## Appendix — verified facts (execute, don't re-investigate)

Verified against the code and DB on 2026-07-09. Re-check only if the source has changed.

### Combat-start request (`POST /api/combat/start`)
```jsonc
{
  "template_id": "<uuid>",
  "character_ids": ["<uuid>", ...],
  "character_positions": {
    "<charID>": { "col": "D", "row": 5 }   // col is a LETTER (A=0,B=1,…); row is 1-based
  },
  "surprised_combatant_short_ids": []       // omit/empty for a mutual reveal
}
```
- Response: `{ encounter, combatants[], initiative_tracker, first_turn }`. Each combatant
  carries `id, short_id, display_name, initiative_roll, initiative_order, hp_max, hp_current,
  ac, is_npc, is_alive`. `first_turn.combatant_id` is the auto-seated first actor.
- **Auto-rolls initiative for everyone, including PCs — no opt-out** (this is APP-1).

### Initiative override (`POST /api/combat/{enc}/override/combatant/{cb}/initiative`)
```jsonc
{ "initiative_roll": 19, "initiative_order": 1, "reason": "…" }
```
- Always send **both** numeric fields (omitting `initiative_order` writes `0` → jumps to
  front). Requires an **active turn** to exist (else 404). Writes only that one combatant;
  does not renumber others and does not move the current-turn pointer.

### Turn engine (how order + seat work)
- Turn order is the stored **`combatants.initiative_order` ASC** (1 acts first). Changing
  `initiative_roll` never re-sorts.
- `advance-turn` completes the current turn, then picks the lowest-order combatant who is
  alive and has **no** Turn row this round. Ties were broken **only once**, at roll time
  (roll desc → DEX mod desc → name asc → uuid asc), and frozen into `initiative_order`.
- The first turn is seated by `StartCombat` on the auto-order-1 combatant; the override does
  not re-seat it (this is APP-2). To re-seat by hand: `UPDATE turns SET combatant_id=<real
  order-1>, movement_remaining_ft=<their characters.speed_ft>, started_at=now(),
  timeout_at=NULL, <reset the boolean/int resource flags> WHERE id=<the seated turn row>` —
  the displaced combatant then has no turn row and is picked at its correct order next.

### DB schema cheat-sheet (DM read paths)
- `combatants`: **no** speed column — speed lives on `characters.speed_ft`. Ability scores
  are `characters.ability_scores` as **scores** (`{"dex":18,...}`), not modifiers.
  `position_col` is a letter, `position_row` is 1-based.
- `turns`: status column is **`status`** (not `is_complete`); has `movement_remaining_ft`,
  `attacks_remaining`, `action_used`, etc.; `completed_at` nullable.
- `encounters.current_turn_id` points at the active `turns.id`.
- `narration_posts` timestamp column is **`posted_at`** (not `created_at`).
- `dm_queue_items`: `status` in `pending|resolved|cancelled`.

### Live IDs from this session (for continuity)
- Campaign `532b4774-47ff-4f83-b591-632ce3509e40`; encounter `30baba5f`; template
  `807d2773`; map `ddb542d3`; creature `hb_9b87c216b7cf` (The Follower, `F1`).
- PCs: Forge `d2d98745…` (speed 25, DEX 14), Vale `b6ca7f49…` (DEX 10), Windreth
  `b2c436da…` (DEX 18).

---

_Opened by the DM agent after the 2026-07-09 combat-start cycle. Pick an item, do it, and
graduate app items into [`issues.md`](issues.md) / encode harness items into
[`dm-rules.md`](dm-rules.md) or [`runbook.md`](runbook.md), then check it off here._
