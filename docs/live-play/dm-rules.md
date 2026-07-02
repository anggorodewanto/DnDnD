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
  12` to the player — exactly the kind of slip this rule prevents.)
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

## Bugs found while playing

- **Fix-now TDD (standing policy).** A bug found in live play gets a red/green TDD
  fix + redeploy, logged in [`issues.md`](issues.md). With a bigger party waiting,
  minimize table idle: apply a fast workaround / unblock the player first, then run
  the TDD fix (delegate or background it where possible) so 5-6 people aren't
  blocked on a full red/green cycle. See [`big-party.md`](big-party.md) "Bugs mid-session."
