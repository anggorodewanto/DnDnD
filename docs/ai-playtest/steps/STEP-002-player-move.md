# STEP-002 — Player `/move` one tile (with button-confirm)

Phase: **AUTOMATED** ✅ (crystallized + green). See [../README.md](../README.md).

## RESULT (2026-06-20)

Authored in two increments (smallest-steps): **002a** = preconditions + confirm
ephemeral; **002b** = button-click model + move completion.

- **Artifacts:**
  - `internal/playtest/testdata/move.jsonl` — full case: dispatch → confirm
    ephemeral → click → combat-log → move result.
  - `internal/playtest/testdata/move.preconditions.json` — seeds campaign,
    approved player "Mover", a map, a combatant at A1, encounter promoted to active.
- **Run it:** `make playtest-replay TRANSCRIPT=$(pwd)/internal/playtest/testdata/move.jsonl` → PASS (~2.7s).

### Harness/engine features added (reusable)

1. **Preconditions: `encounter` block** (`cmd/dndnd/e2e_replay_test.go`) — manifest
   can now declare an active encounter: optional map + combatants (player, name,
   col, row, turnHolder). Applier seeds shell → map → combatants → promote-to-active.
2. **Transcript `click` direction** (`internal/playtest/recorder.go`
   `DirectionClick`) — a transcript step that clicks a button. The `command`
   field holds a **CustomID prefix** (e.g. `move_confirm:`) because the live
   CustomID embeds runtime UUIDs unknowable at authoring time.
3. **`Clicker` interface + `ReplayOptions.Clicker`** (`internal/playtest/replay.go`)
   — replay engine resolves+triggers the button. Nil Clicker + a click entry = error.
4. **`harnessClicker` + `latestButtonCustomID`** (`e2e_replay_test.go`) — finds the
   newest button matching the prefix in `fake.Transcript()` and clicks it.
5. **`ClickButton(playerID, customID)` helper** (`e2e_harness_test.go`) — injects a
   component interaction mirroring `PlayerCommand`'s wiring. Reusable for every
   future confirm-flow command (attack, cast, etc.).

### Confirmed actual output (from the live harness dump)

```
[1] interaction_response ephemeral  "🏃 Move to B1 — 5ft, 25ft remaining after."  (+ move_confirm button)
[2] channel_message  ch-combatlog   "🏃 Mover moves to B1."          ← posted BEFORE the result
[3] interaction_response            "🏃 Moved to B1. 📋 Remaining: 🏃 25ft move | ⚔️ 1 attack | …"
```

Note the combat-log line lands **before** the move-result interaction response —
discovered via the red/green replay dump, not guessed.

### Coverage notes / deferred

- DB position + movement-budget assertions stay in `TestE2E_MovementScenario`
  (`make e2e`); `.jsonl` replay is Discord-visible only.
- The `move_confirm:` flow + `Clicker` now generalize to other confirm-gated
  commands; next such step should reuse them.
