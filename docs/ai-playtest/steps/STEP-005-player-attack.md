# STEP-005 — Player `/attack` (one-shot, active combat)

First **combat-action** step. A player on their turn attacks an NPC; the bot
rolls, announces the result, spends the attack resource, and applies the damage.

## RESULT (2026-06-20) — `AUTOMATED` ✅

Authored the `/attack` happy path against a seeded active encounter. Found and
fixed a combat bug along the way (primary-hit damage was never applied), then
crystallized a deterministic NAT-20 crit as a replayable case + DB-lock scenario.

- Replay: `make playtest-replay TRANSCRIPT=../../internal/playtest/testdata/attack.jsonl` → PASS.
- DB lock: `TestE2E_AttackScenario` (in `make e2e`, 12/12 green).
- `make cover-check` clean; gofmt/vet clean.

## EXPLORE findings

- `/attack` → `AttackHandler.Handle` (`internal/discord/attack_handler.go:156`).
  Requires an **active encounter** + **active turn** (it is turn-gated). Resolves
  immediately (no button-confirm, unlike `/move`).
- Response content = `combat.FormatAttackLog(result)` (`internal/combat/attack.go:849`),
  posted to **#combat-log first**, then as the **ephemeral** reply (2 outbound
  entries, in that order).
- Target resolution = `combat.ResolveTarget` (`internal/combat/distance.go:29`):
  by ShortID (case-insensitive) **or grid coordinate**. Seeded combatant ShortIDs
  are randomised, so the transcript targets the NPC by **coordinate** (`B1`).
- Active combat is already seedable from STEP-002's `encounter` precondition +
  `PromoteEncounterToActive` (status=active, turn, current_turn_id).

## Bug found + fixed (combat damage)

**`/attack` announced the hit and damage and spent the attack resource, but never
applied the damage to the target's HP.** Only the *secondary* Graze
(`mastery.go:342`) and Cleave (`mastery.go:113`) damage routed through
`combat.ApplyDamage`; the *primary* hit did not (observed: goblin stayed 20/20
after a 19-damage crit). User confirmed this is a bug and chose **log + fix**.

- **Fix:** a shared `Service.applyHitDamage` helper (`internal/combat/attack.go`)
  routes primary-hit damage through `combat.ApplyDamage` — so resistances /
  immunities / vulnerabilities, temp HP, death-saves, concentration-on-damage,
  unconscious-at-0, and card refresh all fire exactly as for Graze/Cleave. Called
  after `resolveAndPersistAttack` in `Service.Attack`, `OffhandAttack`, and
  `attackImprovised`, gated on `Hit && DamageTotal > 0`.
- **Delegated** to a TDD fixing agent: red unit test
  (`TestServiceAttack_PrimaryHitAppliesDamageToTargetHP`) → green; one existing
  test flipped from the buggy "no primary HP write" invariant to assert 20→12;
  one mock store gained faithful `UpdateCombatantHP`/`GetCombatantConcentration`.
  `go test ./internal/combat/... ./internal/discord/...` → 3968 pass.

### NEW sibling bugs logged (NOT fixed — awaiting decision)

The /simplify altitude pass found the **same** damage-not-applied gap in two more
attack paths, both out of STEP-005 scope:
- `Service.MartialArtsBonusAttack` (`monk.go:88`) — a 4th caller of
  `resolveAndPersistAttack` the fix did not touch.
- `Service.FlurryOfBlows` (`monk.go:117`) — calls `ResolveAttack` directly.

Both resolve a hit, log it, spend resources, but apply 0 HP. Candidate STEP-006:
centralize primary-hit damage in `resolveAndPersistAttack` (fixes the monk bonus
attack for free) + handle FlurryOfBlows separately.

## Harness features added (reusable)

- **`withRoller` runOption** (`cmd/dndnd/main.go`) — injects a deterministic dice
  roller in place of crypto/rand. Production leaves it unset (`newRoller()` →
  `dice.NewRoller(nil)`). The e2e harness boots `withRoller(dice.NewRoller(e2eDefaultRoll))`,
  an **always-max** die → d20 nat 20, max damage dice. Makes every dice-driven
  command (`/attack`, future `/cast`, `/save`, ...) replayable.
- **`SeedNPCCombatant`** (`e2e_harness_test.go`) — a non-player combatant
  (null character, `is_npc=true`) as an attack target; target by coordinate.
- **`isNpc` precondition flag** (`combatantSeed` in `e2e_replay_test.go`) — seeds
  an NPC combatant in a transcript's `encounter` block.

## Crystallized assertions (confirmed correct with user)

Deterministic NAT-20 longsword crit by "Striker" vs NPC "Goblin":

```
⚔️  Striker attacks Goblin with Longsword
    → Roll to hit: 🎯 NAT 20 — CRITICAL HIT!
    → Damage: 19 slashing (doubled dice: 2d8+3)
```

- Ephemeral + #combat-log carry the crit line (`.jsonl` 2 observed lines).
- Attack resource spent: `AttacksRemaining` 1 → 0 (Go scenario).
- **Bug-fix lock:** target HP 20 → 1 (19 crit damage), still alive (Go scenario).

## Artifacts

- `internal/playtest/testdata/attack.jsonl` (+ `attack.preconditions.json`)
- `cmd/dndnd/e2e_scenarios_test.go` → `TestE2E_AttackScenario`
- `internal/combat/attack.go` → `applyHitDamage` + 3 call sites (the fix)
- `internal/combat/attack_test.go` → `TestServiceAttack_PrimaryHitAppliesDamageToTargetHP`

## Run it

```sh
make playtest-replay TRANSCRIPT=../../internal/playtest/testdata/attack.jsonl
make e2e            # includes TestE2E_AttackScenario
make cover-check
```

## Notes

- The `.jsonl` observer matches outbound entries **sequentially by index**, so
  `channel_id` in observed lines is documentation only; the NPC target produces
  no extra card-update entry (no character card), keeping the sequence
  `[combat-log, ephemeral]`.
- Roll outcome chosen = NAT-20 crit (harness default): robust (a nat 20 hits and
  crits regardless of future mod/AC/cover changes) and replays with zero extra
  infra. A non-crit "common hit" would need a per-transcript roll directive.
