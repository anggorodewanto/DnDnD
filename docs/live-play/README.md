# Live Play — DM Session Docs (index)

This folder is the **memory for a live, human-in-the-loop D&D session** run
through the *running* DnDnD app against a *real* Discord server.

- **Claude = DM.** Sets up the campaign/encounter via the dashboard, narrates the
  story, adjudicates, and reacts to player actions.
- **User = player.** Types the player slash commands in Discord and pastes the
  DM's narration text into the story channel.

This is **not** the AI-playtest test-authoring harness (that lives in
[`../ai-playtest/`](../ai-playtest/) and crystallizes automated test cases). This
folder is for *actually playing*. The two are cousins: the playtest harness docs
explain how the system is driven; this folder records a live game in progress.

---

## Resume protocol (fresh agent: read in this order)

1. **`README.md`** (this file) — what we're doing, the roles, the play loop.
2. **`game-state.md`** — the **save file**: live IDs (campaign, channels, map,
   character, encounter), current scene, initiative/HP. *Where we are right now.*
3. **`play-log.md`** — chronological play-by-play. *What has happened.*
4. **`runbook.md`** — how to operate: boot the stack, the auth model, and the
   exact way to drive each DM action (approve a character, build an encounter,
   start combat) + how to observe game state. *How to do things.*
5. **`issues.md`** — bugs / rough edges / surprises found while playing. Skim for
   known problems; **append any new issue you hit.**

Then continue as DM from the "Next action" line in `game-state.md`.

> **Before acting, pull the live picture.** `game-state.md` is a hand-maintained
> save file and drifts. The **DM Console** (next section) is the *generated*
> single source of truth for what's pending, where the encounter stands, and
> what just happened. Consult it first each turn so you act on reality, not on a
> stale note.

---

## DM Console — the centralized situational view (read this each turn)

There is one endpoint that aggregates **everything a DM needs to act**, so you
don't reconstruct it from six places (Discord #dm-queue, the approvals/level-up
tabs, #initiative-tracker, #combat-log, `action_log`, narration):

```
GET /api/dm/situation        # DM-only; resolves the active campaign from the session
```

Surface it the same way you observe anything else (see `runbook.md`): open the
**DM Console** tab in the dashboard (`#dm-console`), or fetch the endpoint /
mirror it from Postgres. It returns one JSON payload:

| Field | What it answers | Use it to |
| --- | --- | --- |
| `next_step` | "What should I do right now?" | A derived one-line suggestion (an NPC's live turn outranks pending requests). Start here. |
| `pending[]` | "What needs my action?" | Unified, priority-sorted worklist: dm-queue items (whispers, freeform actions, rests, enemy-turn-ready…) **+** character approvals **+** level-up requests. Each has a `resolve_url` when a one-click resolve exists. |
| `state` | "Where are we?" | Live encounter: `round`, `mode`, and every combatant's HP/AC/position/conditions, with the current-turn combatant flagged (`is_current`). Empty when out of combat. |
| `timeline[]` | "What just happened?" | Recent merged feed (combat actions + narration), newest first. |

**The DM Console is read-only situational awareness — it does not resolve
anything.** You still *act* through the existing tools: resolve a queue item via
its `resolve_url` / the dashboard resolver, apply damage through the combat
workspace, narrate via #the-story. The Console tells you *what* to do; the
existing surfaces are *how*.

Why this exists: acting as DM off a partial picture is the main cause of
mis-steps (resolving the wrong thing, missing a pending whisper, narrating
against stale HP). One read of the Console replaces the guesswork.

---

## File map

| File | Role |
| --- | --- |
| `README.md` | Index, roles, the play loop. Changes rarely. |
| `game-state.md` | Live save file — current IDs + scene + combat state. **Update as play advances.** |
| `play-log.md` | Append-only narrative + mechanical log. **Append every beat.** |
| `runbook.md` | Operations: boot, auth, dashboard actions, observation, teardown. |
| `issues.md` | Bugs / rough edges found while playing. **Append every issue.** |

---

## The play loop

```
DM (Claude)                          Player (User)
-----------                          -------------
narrate a beat  ──gives text──►      pastes into #the-story
                                     decides what to do
observe result ◄──relays / DB──      types a /command in Discord
                                     (bot responds in channels)
adjudicate, narrate next beat ──►    ...
```

Concretely each turn:
1. Claude writes narration → hands the user a ready-to-paste block.
2. User pastes it into the right Discord channel (usually `#the-story`).
3. User decides and types a **player** slash command (`/move`, `/attack`, …).
4. The bot processes it and posts to the combat/turn channels.
5. Claude observes the outcome (user relays the bot message, **or** Claude reads
   it from the dashboard / queries the DB) and narrates the consequence.

## Hard constraints (do not violate)

- **Claude cannot invoke player slash commands.** Bot-to-bot slash invocation is
  forbidden by Discord; only the human types `/` commands. Claude *drives the
  dashboard* and *writes narration*, never the player's commands.
- **Claude cannot see Discord directly.** Observe game state via the dashboard
  (browser) or by querying Postgres (see `runbook.md`). The user can also paste
  bot output back.
- **Real OAuth is active.** To drive the dashboard, the user must be logged in at
  `http://localhost:8080` with Discord; Claude then takes over that browser tab.
- **Narration is the human DM's job, mechanics are the bot's.** The bot posts
  dice/combat results automatically; Claude supplies the *story* text the bot
  doesn't generate.
- **Players roll their own dice — never roll for them.** When a player's action
  needs an attack / damage / check / save roll, the *player* rolls it (now via
  `/roll`, e.g. `/roll 1d6+2 reason:handaxe damage`) and reports the number. The
  DM adjudicates against that number (does 15 beat AC 12?), but **must not roll
  the player's dice**. Roll only for NPCs/monsters and DM-side checks. (This is
  the single most common correction the human DM gives — honor it.)
</content>
</invoke>
