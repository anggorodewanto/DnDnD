---
id: C-37-ammo-recovery
group: C
phase: 37
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Post-combat half-ammunition recovery never invoked

## Finding
`service.go:841-844` has an explicit comment stating that the helper `RecoverAmmunition` (at `attack.go:212`) exists but is "deferred as a separate schema migration." The spec line "post-combat half recovery" of ammunition is unimplemented — `RecoverAmmunition` is dead code.

## Code paths cited
- `internal/combat/attack.go:212` — `RecoverAmmunition` defined
- `internal/combat/service.go:841-844` — explicit deferral comment
- Post-combat / encounter-end hook — should call `RecoverAmmunition`

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 37 weapon properties, ammunition recovery)
- `.review-state/group-C-phases-29-43.md` Phase 37 findings

## Acceptance criteria (test-checkable)
- [ ] On encounter end (or whichever post-combat hook the spec specifies), each combatant's expended ammunition is recovered at half (rounded down) per ammunition type
- [ ] Recovered ammunition is persisted to the underlying character/creature inventory (schema migration may be required)
- [ ] Recovery does not exceed the originally-expended count
- [ ] Test in `internal/combat/service_test.go` (or post-combat handler test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None directly; schema migration may touch inventory tables shared with other features

## Notes
Schema migration is the gating concern called out in the deferral comment — design the migration small and additive.
