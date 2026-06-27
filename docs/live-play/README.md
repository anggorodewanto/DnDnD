# Live Play — DM Session Docs (index)

> **You are an AI agent picking this up as DM.** Read this file, load the small
> resume set below (not everything — pull detail on demand), pull the live picture
> from the DM Console, then continue from the "Next action" in
> [`game-state.md`](game-state.md). One agent DMs at a time; a fresh agent resumes
> from these docs (sequential handoff, no concurrent-DM mode).

This folder is the **memory for a live, human-in-the-loop D&D session** run through
the *running* DnDnD app against a *real* Discord server.

- **Claude = DM.** Sets up the campaign/encounter via the dashboard, narrates the
  story, adjudicates, reacts to player actions — and fixes bugs it hits (fix-now TDD).
- **Users = players.** They type the player slash commands in Discord; Claude
  narrates and drives the dashboard. The party is scaling to **5-6 PCs**.

This is **not** the AI-playtest test-authoring harness (that lives in
[`../ai-playtest/`](../ai-playtest/) and crystallizes automated test cases). This
folder is for *actually playing*.

---

## Resume protocol (fresh agent: load in this order)

**Always-load core** (small, every session):

1. **`README.md`** (this) — orientation, the play loop, the DM Console map.
2. **[`dm-rules.md`](dm-rules.md)** — the inviolable constraints. **Read before acting.**
3. **[`game-state.md`](game-state.md)** — the save file: live IDs, current scene,
   ops snapshot, **Next action**. *Where we are right now.*
4. **[`party/roster.md`](party/roster.md)** — the party at a glance (HP/AC/resources).
5. **[`sessions/`](sessions/)** — skim the **latest** session log for recent beats.
6. **[`issues.md`](issues.md)** — skim the triage table for known problems.

**Load on demand** (only when the moment needs it):

- **[`runbook.md`](runbook.md)** — how to operate (boot, auth, drive each DM action,
  observe, onboard players, the tunnel, teardown). Reference, not a cover-to-cover read.
- **[`big-party.md`](big-party.md)** — running 5-6 PCs (onboarding, spotlight,
  combat at scale, encounter scaling).
- **[`world.md`](world.md)** — Ashfall lore / NPCs / rulings — when narrating.
- **`party/<name>.md`** — a PC's full sheet — when you need their kit.
- **[`encounters/`](encounters/)** — pre-built encounters — when a fight starts.
- **`issues.md` "Details"** — a specific bug's deep dive — when fixing it.

Then continue as DM from the "Next action" line in [`game-state.md`](game-state.md).

> **Before acting, pull the live picture.** The hand-maintained docs drift. The **DM
> Console** (next section) is the *generated* single source of truth for what's
> pending, where the encounter stands, and what just happened. Consult it first each
> turn so you act on reality, not on a stale note.

---

## DM Console — the centralized situational view (read this each turn)

One endpoint aggregates **everything a DM needs to act**, so you don't reconstruct
it from six places (Discord #dm-queue, the approvals/level-up tabs,
#initiative-tracker, #combat-log, `action_log`, narration):

```
GET /api/dm/situation        # DM-only; resolves the active campaign from the session
```

Surface it the same way you observe anything else (see [`runbook.md`](runbook.md)):
open the **DM Console** tab in the dashboard (`#dm-console`), or fetch / mirror the
endpoint. It returns one JSON payload:

| Field | What it answers | Use it to |
| --- | --- | --- |
| `next_step` | "What should I do right now?" | A derived one-line suggestion (an NPC's live turn outranks pending requests). Start here. |
| `pending[]` | "What needs my action?" | Unified, priority-sorted worklist: dm-queue items (whispers, freeform actions, rests, enemy-turn-ready…) **+** character approvals **+** level-up requests. Each has a `resolve_url` when a one-click resolve exists. |
| `state` | "Where are we?" | Live encounter: `round`, `mode`, and every combatant's HP/AC/position/conditions, with the current-turn combatant flagged (`is_current`). Empty when out of combat. |
| `timeline[]` | "What just happened?" | Recent merged feed (combat actions + narration), newest first. |

**The DM Console is read-only situational awareness — it does not resolve
anything.** You still *act* through the existing tools: resolve a queue item via its
`resolve_url` / the dashboard resolver, apply damage through the combat workspace,
narrate via #the-story. The Console tells you *what* to do; the existing surfaces
are *how*. With 5-6 PCs this is not optional — you cannot hold the board in your
head (see [`big-party.md`](big-party.md)).

---

## The play loop

```
DM (Claude)                          Players (Users)
-----------                          ---------------
narrate a beat  ──gives text──►      pastes into #the-story
                                     decide what to do
observe result ◄─reads Discord/DB─   type a /command in Discord
                                     (bot responds in channels)
adjudicate, narrate next beat ──►    ...
```

Concretely each turn:
1. Claude writes narration → hands the user a ready-to-paste block (read-aloud).
2. User pastes it into the right Discord channel (usually #the-story).
3. A player decides and types a **player** slash command (`/move`, `/attack`, …).
4. The bot processes it and posts to the combat/turn channels.
5. Claude observes the outcome — reads Discord directly via Chrome (the only way to
   see #in-character roleplay), and the DM Console / DB for mechanical state — then
   narrates the consequence and updates the state docs in lockstep.

The hard constraints that govern every one of these steps are in
**[`dm-rules.md`](dm-rules.md)** — read it before acting.

---

## File map

| File | Role | Cadence |
| --- | --- | --- |
| `README.md` | Index, roles, play loop, resume order, DM Console. | Rarely. |
| [`dm-rules.md`](dm-rules.md) | Inviolable DM constraints. **Always load.** | Rarely. |
| [`game-state.md`](game-state.md) | Slim save file — IDs, current scene, ops, **Next action**. | **Every advance.** |
| [`party/roster.md`](party/roster.md) | Party at-a-glance + onboarding slots. | **Every advance.** |
| [`party/`](party/)`<name>.md` | Per-PC durable sheets. | On level-up / kit change. |
| [`world.md`](world.md) | Ashfall lore, NPCs, rulings. | When the world grows. |
| [`encounters/`](encounters/) | Pre-built encounter specs. | When prepping fights. |
| [`sessions/`](sessions/) | Per-session play-by-play (append-only). | **Every beat.** |
| [`runbook.md`](runbook.md) | Operations: boot, auth, actions, onboarding, tunnel. | Reference. |
| [`big-party.md`](big-party.md) | Technique for 5-6 PCs. | Reference. |
| [`issues.md`](issues.md) | Bugs / rough edges (fix-now TDD). | **Every issue.** |
