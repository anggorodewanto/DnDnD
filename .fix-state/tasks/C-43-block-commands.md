---
id: C-43-block-commands
group: C
phase: 43
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Off-turn commands not explicitly blocked for dying combatants

## Finding
No discord handler explicitly rejects `/move`, `/attack`, `/cast` from a dying combatant. Initiative auto-skips incapacitated combatants (Phase 40), so dying PCs never reach their own turn — but during another's turn, off-turn commands like `/distance` (which is exempt from turn lock) would not be blocked. The contract is mostly satisfied via turn-skip but is incomplete for off-turn paths.

## Code paths cited
- `internal/discord/distance_handler.go` — exempt from turn lock, no dying check
- `internal/discord/move_handler.go`, `attack_handler.go`, cast handler — rely on turn-lock + auto-skip
- `internal/combat/deathsave.go` `IsDying` — gating helper

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 43 death saves & unconsciousness, "block all commands except /deathsave")
- `.review-state/group-C-phases-29-43.md` Phase 43 findings

## Acceptance criteria (test-checkable)
- [ ] Any slash command issued by a dying combatant (other than `/deathsave` and any spec-permitted utilities) returns an explicit rejection
- [ ] Off-turn-exempt commands (`/distance` etc.) honor the same rejection when issued by a dying combatant
- [ ] `/deathsave` remains usable from a dying combatant
- [ ] Non-dying combatants unaffected
- [ ] Test in `internal/discord/distance_handler_test.go` (or shared handler-gate test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-43-stabilize — `/action stabilize` should be permitted from non-dying actors targeting a dying actor; this gate is asymmetric
- C-40-charmed-attack, C-40-frightened-move — same handler-gate pattern

## Notes
Prefer a single shared "is-actor-permitted-to-issue-command" gate that all slash handlers consult.
