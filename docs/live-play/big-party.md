# Running a big party (5-6 PCs)

> Technique for a full table. Load when the party is large or growing. The
> inviolable rules are in [`dm-rules.md`](dm-rules.md); ops in
> [`runbook.md`](runbook.md); this file is *how to run many players well*.

The Ashfall campaign is scaling from 2 PCs (Vale + Forge) to **5-6**. More players
change pacing, spotlight, combat length, and DM-queue load — not the rules. One
agent DMs at a time (sequential handoff — see the resume order in
[`README.md`](README.md)); there is no concurrent-DM mode.

## Onboarding the new players

Batch the new friends in. Per player: `/register` → build in the portal → **DM
approves** on the dashboard → add a [`party/roster.md`](party/roster.md) row + a
`party/<name>.md` sheet → fold into the fiction. Remote players reach the portal +
OAuth via the cloudflared tunnel. Full steps + the tunnel:
[`runbook.md`](runbook.md) "Onboarding players" and "Remote players."

- Approve as they come in — don't make an onboarded player wait on the others.
- Each new PC needs a starting position in the fiction: a fellow traveler who
  reached Ashfall, or was already inside. Narrate the arrival, not the choices.

## Spotlight — every PC gets screen time

With 5-6 players the failure mode is the loud two dominating while the quiet three
go a whole session without a beat.

- **Seed a per-PC hook on arrival** (a sensory cue tied to their class/background —
  see [`world.md`](world.md)). Gives each player an immediate thread.
- **Address PCs by name** in narration and #dm-queue replies so a specific player
  is invited to act, not the room.
- **Round-robin in exploration**, not just combat: "Vale, you smell it first —
  Forge, you hear the boards groan — Kael, your torch catches the claw-marks." Each
  player has something to react to.
- Watch #in-character for the players who *aren't* posting and prompt them directly.

## Combat at scale

More combatants = longer rounds and more bookkeeping. The engine still auto-advances
on each player's `/done`; the DM's job is to keep it moving and stay in lockstep.

- **Read the DM Console `state` every turn** (see [`README.md`](README.md)). With
  6 PCs + several foes you cannot hold initiative/HP/positions in your head —
  reload the live picture each turn rather than tracking it mentally.
- **Narrate in batches, not per-combatant paragraphs.** A round of 6 PCs + 4 foes
  is 10 turns; one tight paragraph per *meaningful* beat keeps #the-story readable.
  Don't write prose for an auto-skipped or whiffed turn.
- **The turn queue can stall** on an absent player (24h turn timeout). For a live
  session, nudge the player; the bot advances on their `/done`. Don't resolve a
  player's turn for them.
- **Keep [`party/roster.md`](party/roster.md) HP/position current** — at 6 PCs the
  roster table *is* your battle map summary.

## Encounter scaling

Encounters tuned for 2 L3 PCs are trivial for 5-6. See
[`encounters/cellar-brood.md`](encounters/cellar-brood.md) for the live example.

- Rough guide: **~1 SRD-Ghoul-class wretch per 1.5 PCs** (4 PCs → 3, 6 PCs → 4).
- Prefer **more foes over higher-HP foes** so multiple PCs have targets (action
  economy); a single high-HP boss with 5-6 PCs ends in one round of focus fire.
- Pre-build encounters in the dashboard so starting a fight is one click, not a
  setup scramble while the table waits. Use the **Reserve mechanic** (foes arriving
  mid-fight) to tune difficulty live.
- Spawn zones must seat the whole party — verify the map has a PC spawn zone big
  enough (the cellar map's stairs landing seats the party on Start Combat).

## DM-queue load

More players = more whispers, freeform actions, rests, and approvals landing in
#dm-queue. Resolve promptly so people aren't blocked.

- The **DM Console `pending[]`** is the unified, priority-sorted worklist (whispers
  + approvals + level-ups). Work it top-down each turn.
- Same secret-info rule applies to **every** whisper reply — at 6 players that's 6×
  the chances to leak an enemy's HP/AC. Describe, don't quote (see
  [`dm-rules.md`](dm-rules.md)).

## Bugs mid-session

Policy is **fix-now TDD** ([`dm-rules.md`](dm-rules.md)). With a full table waiting:

1. **Unblock first** — a fast workaround / DM-side data nudge so the affected
   player can keep playing (or move the spotlight to others).
2. **Then fix** — red/green TDD + redeploy, ideally delegated to a subagent /
   background run so the session continues while it builds.
3. **Log it** in [`issues.md`](issues.md) regardless.

The point is not to defer the fix — it's to keep 5-6 people from idling on a full
red/green cycle.
