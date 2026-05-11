# C-33-cover-on-attacks — review worklog

Reviewer: opus-4.7. Date: 2026-05-11. Read-only.

## Verdict: APPROVE

All acceptance criteria met. Wiring is rule-correct, resource-safe, well-tested,
and the deferred Discord-side work is captured in its own task file.

## Findings

- `internal/combat/attack.go` adds `Walls []renderer.WallSegment` to both
  `AttackCommand` and `OffhandAttackCommand`, a package sentinel
  `ErrTargetFullyCovered`, and two helpers `resolveAttackCover` /
  `creatureCoverOccupants` (uses `ListCombatantsByEncounterID`, filters
  attacker+target+dead).
- Cover gate runs **before** `UseAttack` / `UseResource` in all three
  pipelines (`Service.Attack` line ~857, `attackImprovised` line ~994,
  `OffhandAttack` line ~1085) — verified by diff inspection — so total
  cover does NOT consume action/bonus-action/ammo. Acceptance criterion
  satisfied.
- `input.Cover = coverLevel` set on `AttackInput` before `ResolveAttack`,
  so existing `EffectiveAC` pickup applies +2 / +5 transparently.
- `internal/combat/cover.go` and `internal/combat/aoe.go` UNCHANGED
  (`git diff` empty for both). AoE-save cover path intact.

## Test verification

- New tests in `attack_test.go` (lines 3119-3342) cover half / three-quarters /
  total cover end-to-end via service entry point.
- RED-before-GREEN confirmed: stashed `attack.go` only, recompile of the
  test file fails with `unknown field Walls` and `undefined: ErrTargetFullyCovered`.
  Restored cleanly post-check.
- `coverTestArcher` helper DRYs longbow + DEX-14 archer setup.

## Caller-impact sweep (`rg "AttackCommand\{"`)

- Production call sites: `internal/discord/attack_handler.go:144,178` —
  both omit `Walls` (nil); deferred via `C-33-followup-discord-walls`
  task file (verified present at the documented path). No other
  unexpected production caller.
- 28 existing test sites in `internal/combat/*_test.go` continue to
  compile and pass (nil Walls is the documented degraded mode).

## Build / test / coverage

- `make build` — clean.
- `make test` — green across all 44 packages.
- `make cover-check` — `OK: coverage thresholds met` (overall 92.81%,
  per-package floors satisfied; exit 0).

## Next steps

- Schedule `C-33-followup-discord-walls` (Discord `/attack` `loadWalls`
  mirror of `cast_handler.loadWalls`).
- Optional efficiency follow-up flagged by implementer:
  `populateAttackFES` repeats `ListCombatantsByEncounterID`; share the
  list with `resolveAttackCover`. Out of C-33 scope.
