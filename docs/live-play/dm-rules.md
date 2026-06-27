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
- **Claude cannot see Discord directly.** Observe game state via the dashboard
  (browser) or by querying Postgres (see [`runbook.md`](runbook.md) §6). The user
  can also paste bot output back.
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

## Keep the record straight

- **Keep narration AND the state docs in lockstep with the mechanics — never let
  them fall behind.** The combat engine advances in Discord on its own whenever a
  player runs a slash command (`/attack`, `/done`, even a paralyzed enemy's
  auto-skipped turn) — *without* the DM doing anything. So after **every**
  mechanical beat, before you stop, you must: **(1) narrate** the beat to #the-story
  (read-aloud block), and **(2) update** [`game-state.md`](game-state.md) +
  [`party/roster.md`](party/roster.md) (HP/position/conditions) + the current
  [`sessions/`](sessions/) log to match the live DB.
  - The worst failure mode this folder has actually hit is *mechanics racing ahead
    of the story*: a fight resolved over Rounds 2–3 in #combat-log (Forge
    auto-critting a paralyzed wretch to death) while #the-story and the save file
    were both frozen two rounds back at "Round 1, wretch alive."
  - **On resume, treat the DB + #combat-log as the source of truth** (see the DM
    Console note in [`README.md`](README.md)): if the DB's round/HP is ahead of the
    docs, **stop and reconcile** — narrate the un-narrated beats, correct the docs —
    *before* taking any new action.

## Bugs found while playing

- **Fix-now TDD (standing policy).** A bug found in live play gets a red/green TDD
  fix + redeploy, logged in [`issues.md`](issues.md). With a bigger party waiting,
  minimize table idle: apply a fast workaround / unblock the player first, then run
  the TDD fix (delegate or background it where possible) so 5-6 people aren't
  blocked on a full red/green cycle. See [`big-party.md`](big-party.md) "Bugs mid-session."
