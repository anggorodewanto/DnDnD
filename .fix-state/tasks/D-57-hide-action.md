---
id: D-57-hide-action
group: D
phase: 57
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Hide service is not wired into Discord

## Finding
The Hide service (`standard_actions.go:312-442`) is not wired into Discord — `/action hide` does not route to it. Same gap as Phase 54 standard actions, tracked separately because the stealth/perception machinery is its own subsystem.

## Code paths cited
- `internal/combat/standard_actions.go:312-442` — `Hide`, `resolveHide`, `stealthModAndMode`, `passivePerception` (orphaned)
- `internal/combat/attack.go:328-329, 497-498, 786-790, 1155-1156` — hidden flags (already integrated at attack resolution)
- `internal/combat/advantage.go:39-44` — advantage application (already integrated)
- `internal/discord/action_handler.go` — missing `hide` route

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 57)
- `docs/dnd-async-discord-spec.md` Stealth & Hiding

## Acceptance criteria (test-checkable)
- [ ] `/action hide` routes to the Hide service
- [ ] Stealth check vs highest passive Perception of hostiles resolves correctly
- [ ] On success, `IsVisible=false` is set on the actor
- [ ] Armor stealth_disadv applies (with Medium Armor Master feat negation)
- [ ] Auto-reveal on attacking still works (existing behavior preserved)
- [ ] Test in `internal/discord/action_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Overlaps with D-54 (standard actions dispatch refactor) and D-57-cunning-action-hide (rogue bonus variant).

## Notes
Hide is split out from the D-54 bundle because the stealth/perception calculations and hidden-flag plumbing are a distinct subsystem worth its own review.
