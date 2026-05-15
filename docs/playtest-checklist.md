# Playtest Checklist

Scenarios to walk through every time a human DM and the
[player-agent CLI](../cmd/playtest-player) (or a human player) drive a
real playtest session. Pair this with
[`docs/playtest-quickstart.md`](playtest-quickstart.md) — that gets you
to a live `/move`-ready encounter; this list tells you what to actually
do once you're there.

Each scenario records a transcript via `playtest-player --record
internal/playtest/testdata/transcripts/<name>.jsonl`. After the
session, replay it through the Phase 120 harness:

```sh
make playtest-replay TRANSCRIPT=internal/playtest/testdata/transcripts/<name>.jsonl
```

> **Transcript status legend**
>
> - **captured** — transcript file exists in the repo and replays clean
>   under `make playtest-replay`.
> - **pending** — scenario is documented but no transcript has been
>   recorded yet. First real playtest session should record one.

## How to use this list

1. Walk top-to-bottom. Each scenario builds on the campaign state from
   the previous one, so resetting the DB between scenarios is not
   required (and is in fact a feature — multi-scenario transcripts
   exercise more interaction).
2. For each scenario: meet the pre-conditions, run the player-agent
   commands in order, take the listed DM actions, then verify every
   bullet under **Expect**.
3. If any **Expect** bullet fails, stop and file the failure. Do not
   continue to the next scenario; live state is now divergent from the
   transcript.

---

## 1. Combat round with opportunity attack

- **Status:** pending
- **Pre-conditions:**
  - One approved player (e.g. Aria, fighter) and one DM-controlled
    creature (goblin) on a 10×10 map.
  - Encounter is live; initiative rolled; Aria's turn.
- **Commands (player agent):**
  ```
  /move A2
  /done
  ```
  Then on the goblin's turn, the DM moves the goblin out of Aria's
  reach via the dashboard's Manual Move button.
- **DM actions:**
  - On the OA prompt, accept the attack (or `/attack target:goblin`
    via the player agent, depending on which side the OA fires on).
- **Expect:**
  - `#combat-log` posts: "Aria moves to A2." → "Aria's turn ends."
  - When the goblin moves: an OA prompt fires for Aria.
  - Resolving the OA appends a hit/miss line to `#combat-log`.
  - Database: `combatants.position_col` reflects the moves, `turns.movement_remaining_ft` decrements correctly, `actions_log` row exists per move + OA.

## 2. Spell with saving throw (DC vs. ability)

- **Status:** pending
- **Pre-conditions:** Aria's turn; one or more DM creatures within
  range of a save-vs-DEX spell (e.g. Burning Hands).
- **Commands:** `/cast spell:burning-hands target:cone-from-here`
- **DM actions:** Approve target list. Accept default DC = 8 + prof + spellcasting mod.
- **Expect:**
  - `#combat-log` posts: "Aria casts Burning Hands. DC <n> Dexterity save."
  - Each affected creature gets a save line (`(roll) vs DC <n> — saved/failed`).
  - Failed saves take full damage, succeeded saves take half.
  - Database: targets' `current_hp` reflects damage; `spell_slots` decremented one 1st-level slot.

## 3. Exploration → initiative transition

- **Status:** pending
- **Pre-conditions:** Campaign in `exploration` mode. At least two
  party members in `#the-story`.
- **Commands:** Players post free-form RP messages. DM narrates an
  enemy ambush via the Narrate Panel and clicks **Roll Initiative**.
- **Expect:**
  - `#combat-log` posts: "Initiative rolled. Order: …" with sorted
    list of combatants.
  - `encounters.mode` flips from `exploration` to `combat`.
  - `#the-story` posts the DM's ambush narration.

## 4. Death save sequence (fail → fail → succeed)

- **Status:** pending
- **Pre-conditions:** Player character at 0 HP from previous combat
  hit; player's turn.
- **Commands:** `/deathsave` three times across consecutive turns
  (rolls below 10 force failures; rolls 20 are crits and auto-revive).
- **Expect:**
  - Each `/deathsave` prints "(roll) — failure" or "success" in the
    player's DM and `#combat-log`.
  - `combatants.death_save_failures` increments; on third fail, the
    combatant's `status` flips to `dead` and the bot announces it in
    `#combat-log`.
  - On success path: failures stay where they are, successes
    increment; on third success, `status` → `stable`.

## 5. Short rest

- **Status:** pending
- **Pre-conditions:** Out of combat. Player has at least one Hit Die
  available.
- **Commands:** `/rest short`
- **Expect:**
  - Bot prompts in DM: "Spend Hit Dice? (1d10 + CON for fighter, …)"
  - Player chooses one HD. `current_hp` increases by the rolled
    amount + CON mod.
  - `hit_dice_remaining` decrements by one.
  - Class features that recharge on short rest reset (e.g. Second Wind).

## 6. Long rest

- **Status:** pending
- **Pre-conditions:** Out of combat in a safe location.
- **Commands:** `/rest long`
- **Expect:**
  - All players' `current_hp` → max.
  - Spell slots restored.
  - Half of expended HD restored (rounded down, minimum 1).
  - `#the-story` posts a long-rest narration line.

## 7. Loot claim

- **Status:** pending
- **Pre-conditions:** DM has placed a loot pool with at least one
  identifiable item (e.g. Potion of Healing).
- **Commands:** `/loot` then click the **Claim** button on the desired
  item.
- **Expect:**
  - `#the-story` posts: "Aria claims Potion of Healing."
  - `characters.inventory` JSONB gains the item entry.
  - Loot pool item count decrements; pool deletes itself when empty.

## 8. Item give between players

- **Status:** pending
- **Pre-conditions:** Two approved players in the campaign; player A
  holds an item.
- **Commands (as player A):** `/give item:potion-of-healing to:@PlayerB`
- **Expect:**
  - Bot prompts player B in DM to accept.
  - On accept: item moves from A's `characters.inventory` to B's.
  - `#the-story` posts: "Aria gives Potion of Healing to Bram."

## 9. Attune / unattune

- **Status:** pending
- **Pre-conditions:** Player owns an attunement-required magic item
  and has fewer than 3 currently attuned.
- **Commands:** `/attune item:cloak-of-protection` then later `/unattune item:cloak-of-protection`.
- **Expect:**
  - Attune adds the item ID to `characters.attuned_items`; AC/save
    bonuses recompute on the character card.
  - Unattune removes it; bonuses revert.
  - Attempting to attune a 4th item rejects with a clear error.

## 10. Equip swap

- **Status:** pending
- **Pre-conditions:** Player owns at least one weapon and one armor
  set in inventory.
- **Commands:** `/equip item:longsword`, `/equip item:chain-mail`, then
  `/equip item:dagger` to swap weapons.
- **Expect:**
  - Equipping armor recomputes AC.
  - Equipping a weapon makes it the default for `/attack` (no `weapon:` arg required).
  - Re-equipping a different weapon frees the prior main-hand slot. (No
    `/unequip` command exists today — slot release is implicit on the next
    `/equip`. Track an explicit unequip as a follow-up if scenarios need it.)

## 11. Dashboard-side encounter edit during live combat

- **Status:** pending
- **Pre-conditions:** Live encounter; player's turn.
- **DM actions:** Open the encounter on the dashboard; edit a
  combatant's `max_hp` or move them on the map; save.
- **Commands (player agent):** `/status` after the DM saves.
- **Expect:**
  - `#combat-log` posts the DM-side change announcement.
  - `/status` reflects the new state immediately.
  - No race / lock-failure errors in the bot logs (per Phase 27/103
    advisory locks the edit should serialize cleanly with the player
    command).

---

## Recorded transcripts

| Path | Scenario | Notes |
| --- | --- | --- |
| `internal/playtest/testdata/sample.jsonl` | smoke | `/recap` on empty campaign — used as the default for `make playtest-replay` |
| `internal/playtest/testdata/combat_round.jsonl` | combat round | `/move` → `/attack` → `/done` sequence — exercises a full combat turn |

> Add new rows here as scenarios get captured. The first real playtest
> session should aim to capture transcripts for #1 (combat OA) and #4
> (death save) — those exercise the most logic per command and so will
> catch the most regressions on replay.

## Player-agent CLI note

The `cmd/playtest-player` CLI is a **manual aid**, not an automated player.
It validates slash-command syntax, prints "PASTE THIS" instructions for the
human to copy into Discord, and records the resulting transcript. It does
**not** issue slash commands itself (Discord's interaction model requires a
real user session). This is by design: the transcript captures real
bot-to-Discord round-trips, not simulated ones.

## Transcript capture status (H-121.4)

All 11 scenarios above are intentionally `Status: pending` and the only
recorded transcript in the repo is `internal/playtest/testdata/sample.jsonl`
(smoke). Capture of real-session transcripts for the 11 scenarios is
**deferred-with-justification** under task
[`H-121.4-playtest-transcripts`](../.fix-state/tasks/H-121.4-playtest-transcripts.md):

- **Why deferred:** transcripts must be produced by the
  [`cmd/playtest-player`](../cmd/playtest-player) REPL against a live
  Discord bot + database during an actual playtest session. The 11
  scenarios cite cross-system flows (`#combat-log` echoes, DM dashboard
  edits, real death-save dice) that the offline test harness cannot
  fabricate without skipping the very integrations the transcripts are
  meant to exercise on replay.
- **Gating event (criterion to flip a row from `pending` to `captured`):**
  the scenario is walked end-to-end in a live playtest session under
  `playtest-player --record …`, the resulting JSONL file is committed
  under `internal/playtest/testdata/transcripts/<name>.jsonl`, and
  `make playtest-replay TRANSCRIPT=…` exits 0 against the recording.
- **Out of scope for this fix-state campaign:** scheduling the live
  session itself. The campaign rules treat this row as `deferred-with-
  justification` (see
  [`.fix-state/SUMMARY.md`](../.fix-state/SUMMARY.md)).
- **Already in place:** the recorder (`internal/playtest/recorder.go`),
  replay loader (`internal/playtest/replay.go`), and `make
  playtest-replay` target (`Makefile`) are all wired and green against
  the smoke transcript today, so adding a captured row is a docs +
  testdata change once a session runs.
