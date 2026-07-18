# DM Rules — standing constraints (read before acting)

These are the **inviolable rules** for an agent acting as DM through the running
DnDnD app. They were each learned the hard way in live play; the parenthetical
"real correction" notes are actual mistakes this rule now prevents. **Load this
file every session** (it's step 2 of the resume order in
[`README.md`](README.md)). Operational *how-to* is in [`runbook.md`](runbook.md);
big-party technique is in [`big-party.md`](big-party.md).

---

## What the agent can and cannot do

- **Claude cannot invoke player slash commands.** Bot-to-bot slash invocation is
  forbidden by Discord; only the human types `/` commands. Claude *drives the
  dashboard* and *writes narration*, never the player's commands.
- **Claude reads Discord via Chrome (claude-in-chrome) — observation only.** Open
  the Discord web app in the DM's already-logged-in Chrome and read any channel
  directly (#in-character roleplay, #combat-log, #dm-queue, …). This is the *only*
  way to see player chatter the app never stores — **#in-character roleplay is
  Discord-only and appears in no dashboard / DB / DM-Console feed.** For
  *mechanical* state (encounter, HP, queue, turn order) still prefer the dashboard /
  DM Console / Postgres — that's the generated source of truth (see
  [`runbook.md`](runbook.md) §6); use Discord for the human/roleplay layer the
  generated views miss. The user pasting bot output back is now a fallback, not the
  only path.
- **Also read #roll-history — not every roll makes a `dm_queue_items` row.** When you
  pull the board, check the **#roll-history** channel in Discord alongside the queue
  and #in-character. A `/check` that carries a spotlight creates a queue item, but
  **supplementary / helper rolls post to #roll-history and generate NO queue row** —
  e.g. a *Guidance* `1d4` a player rolls to add to another PC's check, a damage die,
  an initiative roll. (Seen 07-08: Windreth's Survival 20 had Vale's Guidance d4
  posted in #roll-history; the queue only held the 20.) It rarely changes a clear
  success, but the total, crits, and near-DC calls depend on it — **read
  #roll-history before adjudicating so the reported number is the whole number.**
- **Reading is open; typing in Discord is not.** Claude observes any Discord channel
  through the browser, but **never types in Discord** — no slash commands (Discord
  forbids bot-to-bot invocation) and no messages. The human types `/` commands;
  narration reaches #the-story only through the dashboard Narrate editor (a
  mutation — next section). Read freely, mutate only through the dashboard.
- **Real OAuth is active.** To drive the dashboard, the user must be logged in at
  `http://localhost:8080` (or the tunnel URL) with Discord; Claude then takes over
  that browser tab.
- **Narration is the DM-agent's job; mechanics are the bot's.** The bot posts
  dice/combat results automatically; Claude supplies the *story* text the bot
  doesn't generate.

## How DM actions must be performed

- **Act as DM through the web dashboard, driven by Chrome (claude-in-chrome) —
  standing rule.** Every DM *mutation* — posting narration, resolving #dm-queue
  items, applying damage/conditions, advancing turns, building/starting encounters,
  approving characters — goes through the DnDnD dashboard tab (logged in as the DM),
  **never** raw SQL / curl. Mutation endpoints sit behind `dmAuthMw`, so curl can't
  auth anyway, and the dashboard tab is the authenticated DM session. *Reads /
  observation* via Postgres stay fine (see [`runbook.md`](runbook.md) §6); only
  *acting* must go through the dashboard.
- **DB-repair exception — a guarded one-off `UPDATE`, never a blind `DELETE`.** When the
  game reaches a correct state the app has **no endpoint** to express (this session: no
  way to re-seat the round-1 first actor after `start` mis-seated it), a single hand-run
  SQL write is the allowed last resort — but only under these guards, every time:
  1. **Never `DELETE` a live game row without explicit user confirmation.** Prefer a
     non-destructive `UPDATE` that reshapes the existing row; a `DELETE` (or a
     `TRUNCATE`/cascade) on encounter / combat / character / turn data is a destructive op
     and needs the user's go-ahead first (global rule: always ask before destructive DB ops).
  2. **Scope every write with a tight `WHERE` on ids** — target exactly one known row by its
     primary key, never a broad predicate that could sweep siblings.
  3. **Read-verify immediately after** (re-`SELECT` the touched row) to confirm the write did
     exactly what was intended, and record it in the beat notes.
  Prefer any real endpoint over SQL; reach for a write only when none exists. (Real
  correction 07-09: the seat repair was one tightly-`WHERE`d `UPDATE turns SET
  combatant_id=…, movement_remaining_ft=… WHERE id=<the one auto-seated turn row>` on a
  fresh, un-acted row — no `DELETE`, verified after. That is the correct bar. When APP-1 /
  APP-2 ship — see [`combat-ops-improvements.md`](combat-ops-improvements.md) — this repair
  is retired entirely.)
- **Wrap #the-story narration in a read-aloud block.** When posting DM story prose
  to #the-story via the Narrate editor, wrap the prose in a `:::read-aloud … :::`
  block (use the editor's **Insert Read-Aloud Block** button). See
  [`runbook.md`](runbook.md) §8.
- **Replying to players: Narrate by default; Whisper only for secrets.** Answering
  a player — a ruling, a clarification ("yes, roll a Charisma check"), a reaction,
  any table-visible reply — through the **Narrate editor → #the-story is fine**, so
  the whole table sees it (wrap in-world prose in a read-aloud block; a brief
  out-of-character DM aside for a pure rules answer is fine too). Use the
  **Message Player** whisper (a private Discord DM) **only when the content is
  genuinely secret between the DM and that one player** — a hidden result, private
  information, a solo scene, something the others must not see. Don't whisper
  table-public info. (Whisper delivery note: `Send DM` sends the Discord DM *before*
  it logs the row, so a dashboard error after the send does **not** mean the whisper
  failed — verify delivery before retrying, or you double-DM the player. See
  [`issues.md`](issues.md) ISSUE-053.)

## At the table

- **Players roll their own dice — never roll for them.** When a player's action
  needs an attack / damage / check / save roll, the *player* rolls it (via `/roll`,
  e.g. `/roll 1d6+2 reason:handaxe damage`) and reports the number. The DM
  adjudicates against that number (does 15 beat AC 12?), but **must not roll the
  player's dice**. Roll only for NPCs/monsters and DM-side checks. (This is the
  single most common correction the human DM gives — honor it.)
- **Enemy HP and AC are secret — never reveal exact numbers to players.** The bot
  already hides them (the #initiative-tracker masks NPC HP, NPCs get no character
  card, the damage endpoint posts nothing). The leak risk is *the DM's own prose* —
  narration, and especially **whisper/queue replies** (`dm_queue_items.outcome` is
  DM'd straight to the player). Describe enemy state, don't quote it: say *"it
  staggers, bloodied"* / *"barely scratched"* / *"reeling"*, **never** *"15/22 HP"*
  or *"AC is 12"*. Confirm hit/miss outcomes (a hit implies you cleared its AC)
  without stating the AC value, and never hand out the precise HP fraction. (Real
  correction: a damage whisper-reply once leaked a ghoul's `15/22` HP **and** `AC
  12` to the player — exactly the kind of slip this rule prevents.) Before you post a
  narration beat, run the **render + stat-leak check** recipe in
  [`runbook.md`](runbook.md) §8 — it confirms OOC-first / read-aloud-box-last and scans the
  rendered text for a leaked `AC` / HP fraction / `CR` / internal creature id, so you don't
  re-author the check each beat.
- **Don't narrate player choices.** Player-controlled PCs decide and speak for
  themselves; narrate their *arrival / the world's reaction*, not their decisions or
  dialogue. (See per-PC sheets in [`party/`](party/).)
- **Nudge with OOC hints — keep the spotlight moving.** After narrating a beat, add
  a brief **out-of-character coda** (plain text after the read-aloud block) that hands
  the spotlight back and lays out *concrete* options — the specific things each PC
  could do next and the exact rolls they could make (`/check history`,
  `/check perception`, "touch it / commune / hold your ground", etc.). **Do this
  often, and especially when an RP phase is dragging** — players hesitating, going
  quiet, or circling the same beat: surface 2-4 clear choices + the slash commands so
  nobody is stuck guessing what's possible. When the party is **split, or a PC is new
  or quiet**, make it **per-PC** (name each one + a tailored hook / suggested roll) so
  everyone gets an explicit invitation to act. Keep the coda a **menu, not a
  decision** — suggest, never pick; this pairs with "don't narrate player choices" and
  "players roll their own dice" (you list what they *could* do; they choose and roll).
  Keep it tight and non-spoilery: the bot renders the OOC coda **first** and the
  read-aloud box **last**, so the coda is the lead-in, not the reveal. (Player feedback
  07-02: *"I like the way you use OOC nudge and hint to players on what they can do and
  roll — do that more often, especially if an RP phase is dragging."*)
- **Verify every slash command's syntax BEFORE you put it in a coda.** Any exact `/command`
  you advertise to players must first be confirmed against the real command table —
  grep `internal/discord/commands.go` for the command name and its option names and
  match the argument shape exactly. A wrong form doesn't fail silently for the player: it
  throws a private "Couldn't read that" error and stalls them. (Real correction 07-09: the
  initiative coda told players `/roll d20+2` — the exact form the parser then rejected
  (bare `d` with no count); a player who typed it got the error. The parser was later fixed
  in `e967364`, but the rule stands: **confirm the syntax, then prompt.** The
  verify-before-prompt done for `/move` / `/bonus` / `/action` later that same cycle is the
  standard — make it the rule, not the exception.)
- **Write the story in plain, simple English.** The players asked for this directly. Keep
  #the-story narration easy to read: pick the common word over the fancy one, keep sentences
  short, favor concrete images over ornate phrasing, and drop the archaic / purple / clause-
  stacked style. Simpler is not flatter — stay vivid and atmospheric — but a player skimming on
  a phone should get the beat in one pass. Applies to the read-aloud prose **and** the OOC coda.
  (Player feedback 07-02: *"use simpler English for the DM story."*)
- **Adjudicating rolls & reveals — the patterns that hold this campaign's mystery
  together** (distilled from a full arc of live rulings; the blow-by-blow is in
  [`sessions/`](sessions/), the arc state in [`campaign-arc.md`](campaign-arc.md)):
  - **A failed roll withholds — it never *decides* against the player.** A miss means
    the answer doesn't come *yet*; it must not invent a hidden fact to "find," nor let a
    low roll settle an open DM lever. Keep the lever open for a better read (a daylight
    look, a higher check, the right source). Don't reward a fail with a lie or punish a
    vulnerable reveal with a slammed door.
  - **Gate the *ask*, not the gesture.** On a low social roll, the *extraction* fails but
    the roleplay still lands — a low Persuasion blocks the answer demanded, yet the PC's
    honesty / offered food / shown scrap is real and still moves the fiction (it just
    doesn't force the secret).
  - **Reward unforced RP over dice.** A player who gives up a real secret unprompted, or
    trades truth-for-truth at genuine cost, earns a real answer back with **no roll** —
    honesty is the currency. Don't make someone roll for what they paid for in the fiction.
  - **A high roll earns *trust and an open door*, not auto-exposition.** A big social
    success advances **one** earned layer and opens the next lever — it does not dump the
    sealed core. Reveal only what's earned; deepen the mystery rather than closing it.
  - **A strong check shows the *body*, not the *name*.** Perception / Survival / Insight /
    Investigation read ground, mood, method, and tracks — **never** a warded secret. The
    protected reveal (whose the name is, what the Order keeps) stays behind its proper
    channel (a true reading, a higher source), however good the roll.
- **Pre-declared non-lethal intent is honored — even with a ranged or spell attack
  (house rule, 2026-07-18).** RAW (2014 *and* 2024) lets you knock a creature to 0
  unconscious instead of killing it **only on a melee hit**. **This table waives that
  range restriction:** if a player **declares non-lethal intent before the blow** (to
  capture, interrogate, or spare), a hit that drops the target to 0 leaves it
  **unconscious and stable, not dead** — regardless of the attack's range or type
  (ranged weapon, Eldritch Blast, any damaging spell). The one condition: the
  declaration must come **before the roll** — no retroactive "actually I wanted them
  alive" after the kill lands (this mirrors the reactions-pre-declared rule: intent
  locks in before the dice). (Origin: Vale pre-declared *"we are going to defeat
  Sabinnet non-lethally, to interrogate afterwards,"* then dropped her to 0 with
  Eldritch Blast — a ranged cantrip RAW cannot subdue. Ruling honored the stated
  intent, so Sabinnet is captured alive and the sealed-name-reader lead survives
  rather than dying with her.)

## Keep the record straight

- **The DB / DM Console is the source of truth for mechanical state — never
  hand-track it.** Round, turn, HP, AC, positions, conditions, slots, the pending
  queue, and the action timeline are all *generated* and served live by the DM
  Console (`GET /api/dm/situation`). **Do NOT copy them into
  [`game-state.md`](game-state.md) or [`party/roster.md`](party/roster.md)** — a
  hand-kept copy drifts the moment a player runs a slash command, and an agent then
  acts on a stale board (this folder hit that failure repeatedly — see
  [`sessions/`](sessions/)). The docs hold only what the DB *can't* derive: rulings,
  lore, ops, technique, **DM intent** (the "Next action" + scene framing in
  game-state.md), the narrative session log, durable IDs, and per-PC durable kit.
  If the Console is missing something a DM needs — as the `action_log`
  player-action gap was (every player beat invisible in the timeline; fixed
  2026-06-28, ISSUE-025) — **fix the product**, don't paper over it with a manual doc.
- **Keep the STORY in lockstep with the mechanics — never let narration fall
  behind.** The combat engine advances in Discord on its own whenever a player runs
  a slash command (`/attack`, `/done`, even a paralyzed enemy's auto-skipped turn)
  — *without* the DM doing anything. So after **every** mechanical beat, before you
  stop, you must: **(1) narrate** the beat to #the-story (read-aloud block), and
  **(2) update the non-derivable docs** — the narrative [`sessions/`](sessions/) log
  + the **Next action** / scene in [`game-state.md`](game-state.md). Pull the
  numbers you narrate from the Console; don't transcribe them into a state table.
  - The worst failure mode this folder has actually hit is *mechanics racing ahead
    of the story*: a fight resolved over Rounds 2–3 in #combat-log (Forge
    auto-critting a paralyzed wretch to death) while #the-story and the save file
    were both frozen two rounds back at "Round 1, wretch alive."
  - **On resume, treat the DB + DM Console + #combat-log as the source of truth**
    (see [`README.md`](README.md)): if the live board is ahead of the docs, **stop
    and reconcile** — narrate the un-narrated beats — *before* taking any new action.
- **Write loot, found items, and awarded coin onto the character's sheet — never
  leave them only in narration.** The moment the party loots a body, searches a
  cache, or earns a reward, put it on the finding PC's sheet so it isn't forgotten:
  **Party → that PC's "Manage inventory" → "+ Add items"** (search the catalog for
  standard gear — rope, lantern, rations, etc.; each catalog item keeps its own row),
  the **"Add Custom"** tab for one-off story items, and the **Gold** field for coin.
  A line in #the-story is ephemeral; the sheet (`characters.inventory` / `.gold`) is
  the durable store the players actually carry forward, so anything that changes what
  a PC *owns* belongs there too — not just in the session log. Inventory/gold edits
  are **not** combat-blocked (unlike HP/slots), so you can hand out loot at any time.
  - **Gold is an absolute set, not additive** — read the current value first, then
    write `old + award` (the field replaces, it doesn't add).
  - **Custom-item gotcha:** every freeform "Add Custom" item persists with an empty
    `item_id`, and the add path stacks rows by `item_id` — so **two custom items in
    one character collapse into a single row**. Add at most one custom item per
    character, or map the rest to catalog slugs (each has a unique `item_id`).

## Bugs found while playing

- **Fix-now TDD (standing policy).** A bug found in live play gets a red/green TDD
  fix + redeploy, logged in [`issues.md`](issues.md). With a bigger party waiting,
  minimize table idle: apply a fast workaround / unblock the player first, then run
  the TDD fix (delegate or background it where possible) so 5-6 people aren't
  blocked on a full red/green cycle. See [`big-party.md`](big-party.md) "Bugs mid-session."
