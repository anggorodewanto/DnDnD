---
id: C-40-charmed-attack
group: C
phase: 40
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Charmed attack restriction never enforced at runtime

## Finding
`IsCharmedBy` in `condition_effects.go` is defined and unit-tested but is never called from `attack.go` or any discord handler. A charmed combatant can still target their charmer end-to-end.

## Code paths cited
- `internal/combat/condition_effects.go` — `IsCharmedBy` defined
- `internal/combat/condition_effects_test.go` — only consumer
- `internal/combat/attack.go` / `internal/discord/attack_handler.go` — should reject attacks where attacker is charmed by target

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 40 condition effects, Charmed)
- `.review-state/group-C-phases-29-43.md` Phase 40 findings

## Acceptance criteria (test-checkable)
- [ ] `/attack` rejects with a clear error message when the attacker has a `charmed` condition whose source is the target combatant
- [ ] Charmed attacker can still attack non-source targets
- [ ] Non-charmed attacker is unaffected
- [ ] Test in `internal/discord/attack_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-33, C-35, C-38 — all add validation/gating into the attack pipeline; coordinate ordering
- C-40-frightened-move — symmetric "source-aware condition validator" pattern

## Notes
Centralize the source-aware condition check so charmed/frightened (and future similar conditions) reuse the same helper.
