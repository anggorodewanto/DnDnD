# C-33-cover-on-attacks — implementation worklog

Task: `C-33-cover-on-attacks` (HIGH). Date: 2026-05-11.
Implementer: opus-4.7.

## Task status — DONE

`CalculateCover` is now invoked from the player-driven attack pipeline.
`AttackInput.Cover` is populated end-to-end, so half cover applies +2 AC,
three-quarters cover applies +5 AC, and total cover rejects the attack with
`ErrTargetFullyCovered` before the attack resource / ammo are consumed.

## Implementation

`internal/combat/attack.go`:

- New field `Walls []renderer.WallSegment` on both `AttackCommand` and
  `OffhandAttackCommand`. Callers (e.g. the Discord attack handler — out of
  scope for this task) pass the encounter map's wall segments through. A
  nil slice degrades to "no wall cover"; creature-granted cover still
  applies via the encounter's combatant list.
- New package-level sentinel `ErrTargetFullyCovered` (PHB p.196: a creature
  behind total cover can't be targeted).
- New `Service.resolveAttackCover(ctx, attacker, target, walls)` — decodes
  positions, builds the occupant list via `creatureCoverOccupants`, calls
  `CalculateCover`, and short-circuits to `(CoverFull, ErrTargetFullyCovered)`
  on total cover so the caller can `return result, err` cleanly without a
  separate level check.
- New `Service.creatureCoverOccupants(ctx, attacker, target)` — lists living
  combatants in the encounter, filters out the attacker and target by ID,
  and converts to `[]CoverOccupant`. Combatants with `IsAlive == false`
  are skipped so corpses don't artificially block sightlines.
- Wired the cover gate into `Service.Attack`, `Service.attackImprovised`,
  and `Service.OffhandAttack` BEFORE the `UseAttack` / `UseResource`
  consumption step so a total-cover rejection does not burn the action /
  attack / ammo. `input.Cover = coverLevel` is set on the resulting
  `AttackInput` so `ResolveAttack`'s existing `EffectiveAC` pickup applies
  the +2 / +5 bonus.

The AoE-save cover path (`aoe.go:188`, `CalculateCoverFromOrigin` + DEX save
bonus) is untouched and continues to work — its tests still pass.

## Tests added (`internal/combat/attack_test.go`)

All three were RED before the fix and GREEN after.

- `TestServiceAttack_HalfCoverFromWalls_FlipsHitToMiss` — d20 13 + DEX 2 +
  prof 2 = 17 vs AC 17 hits with no cover but misses with the half-cover
  +2 (AC 19). Uses the proven `{X1:2,Y1:0.5,X2:2,Y2:1.5}` partial-wall
  geometry from `TestCalculateCover_HalfCover_Wall`.
- `TestServiceAttack_ThreeQuartersCoverFromWalls_FlipsHitToMiss` — same
  roll vs AC 17 → misses at AC 22 with three-quarters cover. Uses the
  L-shaped walls from `TestCalculateCover_ThreeQuartersCover`.
- `TestServiceAttack_FullCoverRejectsAttack` — tall vertical wall produces
  `ErrTargetFullyCovered`; verified via `assert.ErrorIs`.

New test helper `coverTestArcher(t)` returns `(*Service, uuid.UUID)` to
DRY the longbow/character/mock-store setup shared by the three tests.

## Verification

- `go test ./internal/combat/` — green (17s).
- `make test` — green.
- `make cover-check` — coverage thresholds met.
- `make build` — clean.

## /simplify pass

- Folded the "compute cover → if CoverFull return err" pattern into
  `resolveAttackCover` itself so the three call sites are now one
  `if err != nil { return AttackResult{}, err }` block each (was 7 lines
  × 3, now 4 lines × 3). The helper returns `(CoverFull,
  ErrTargetFullyCovered)` so callers don't need to re-check the level
  against `CoverFull`.
- Tightened `coverTestArcher`'s return signature from 4 values to 2
  (the tests only needed `*Service` and the character UUID).

## Out of scope (deferred follow-ups)

- The Discord attack handler (`internal/discord/attack_handler.go`) still
  passes `AttackCommand{}` with `Walls: nil`. The handler file is outside
  this task's file zone. Production-side wall loading needs a follow-up:
  mirror the `loadWalls(ctx, encounter)` helper from `cast_handler.go`
  and populate `cmd.Walls` before calling `Attack`/`OffhandAttack`. Until
  then the service layer applies creature cover but no wall cover from
  live encounters — the rule still degrades gracefully (no false
  positives, no false negatives at the service surface).
- `populateAttackFES` (later in `Service.Attack`) makes a second
  `ListCombatantsByEncounterID` call. Sharing the list with
  `resolveAttackCover` would shave one query per attack, but broadens the
  refactor beyond C-33. Flag for a separate efficiency follow-up.

## Files touched

- `internal/combat/attack.go` — added `Walls` field × 2 commands, added
  `ErrTargetFullyCovered`, added `resolveAttackCover` + `creatureCoverOccupants`,
  wired the cover gate into `Service.Attack`, `attackImprovised`,
  `OffhandAttack` (cover gate runs BEFORE attack/resource consumption).
- `internal/combat/attack_test.go` — added `coverTestArcher` helper and
  three Service-level integration tests covering half / three-quarters /
  total cover end-to-end.
