---
id: E-69-obscurement-misc
group: E
phase: 69
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Obscurement: checks not modified, Hide not gated, lighting reason not in log

## Finding
Phase 69 wires obscurement into attacks (heavily-obscured attacker→disadvantage, target→advantage) but three downstream consumers are missing:
- `ObscurementCheckEffect` has no production callers. `/check` does not auto-apply Perception disadvantage from obscurement.
- `ObscurementAllowsHide` has no callers. `/action hide` does not consult obscurement.
- `ObscurementReasonString` has no callers — the combat log does not surface "lightly obscured: disadvantage on Perception" or similar.

## Code paths cited
- internal/combat/obscurement.go — `ObscurementCheckEffect`, `ObscurementAllowsHide`, `ObscurementReasonString`, `CombatantObscurement`
- internal/combat/attack.go:1162-1180 — attack-side integration (already wired)
- internal/combat/advantage.go:55-58 — attack-side integration (already wired)
- `/check` handler (consumer for `ObscurementCheckEffect`)
- `/action hide` handler (consumer for `ObscurementAllowsHide`)

## Spec / phase-doc anchors
- docs/phases.md — Phase 69 ("Obscurement & Lighting Zones"); Perception modifier, hide gating, log-line lighting reason

## Acceptance criteria (test-checkable)
- [ ] `/check` for Perception (or sight-based checks) inside relevant obscurement applies the disadvantage from `ObscurementCheckEffect`
- [ ] `/action hide` is blocked or allowed per `ObscurementAllowsHide` depending on the combatant's obscurement
- [ ] Combat log surfaces `ObscurementReasonString` when obscurement modifies a roll or check
- [ ] Test in `internal/combat/obscurement_test.go` (or check/hide handler tests) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-68-fov-minor (shared obscurement.go internals)
- E-67-zone-* (zone-derived obscurement source)

## Notes
Three independent consumers; can be implemented as one PR.
