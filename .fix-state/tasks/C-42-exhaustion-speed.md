---
id: C-42-exhaustion-speed
group: C
phase: 42
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Exhaustion speed effects (level 2+, level 5+) not applied at turn start

## Finding
`turnresources.go:218` and `:231` `ResolveTurnResources` call `EffectiveSpeed`, which considers only conditions. The exhaustion-aware variant `EffectiveSpeedWithExhaustion` in `condition_effects.go:202` exists but is never used. Exhaustion level 2+ (halve speed) and level 5+ (speed 0) therefore never apply at turn start.

## Code paths cited
- `internal/combat/turnresources.go:218,231` — `ResolveTurnResources` uses `EffectiveSpeed`
- `internal/combat/condition_effects.go:202` — `EffectiveSpeedWithExhaustion` defined
- `internal/combat/condition_effects_test.go` — only consumer of the exhaustion-aware variant

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 42 damage processing, exhaustion ladder)
- `.review-state/group-C-phases-29-43.md` Phase 42 findings

## Acceptance criteria (test-checkable)
- [ ] `ResolveTurnResources` consults exhaustion level when computing per-turn speed
- [ ] Exhaustion 2 halves max speed at turn start; exhaustion 5 sets speed to 0
- [ ] Exhaustion 0/1 unchanged from current behavior
- [ ] Existing exhaustion-6 instant-death and exhaustion-4+ HP halving remain intact
- [ ] Test in `internal/combat/turnresources_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-30-occupant-size — both touch movement-pipeline budget
- C-40-frightened-move — also movement validation, but separate concern

## Notes
Likely a one-line swap from `EffectiveSpeed` to `EffectiveSpeedWithExhaustion` plus an exhaustion-level lookup on the combatant. Confirm the helper's exact signature when wiring.
