---
id: C-43-prone-on-drop
group: C
phase: 43
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Prone condition not applied at drop-to-0 HP

## Finding
`ConditionsForDying()` returns both `unconscious` and `prone`, but `applyDamageHP:385` only applies `unconscious` when `prevHP > 0 && newHP <= 0 && isAlive`. The `prone` half of the function's contract is dead code in production.

## Code paths cited
- `internal/combat/deathsave.go` `ConditionsForDying` — returns `unconscious` + `prone`
- `internal/combat/concentration.go:385` `applyDamageHP` — only applies `unconscious`

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 43 death saves & unconsciousness)
- `.review-state/group-C-phases-29-43.md` Phase 43 findings

## Acceptance criteria (test-checkable)
- [ ] When a combatant transitions from HP > 0 to HP <= 0 (and is still alive), both `unconscious` and `prone` conditions are applied via the existing condition pipeline
- [ ] Iterating over `ConditionsForDying()` (not duplicating its list) — single source of truth
- [ ] Existing unconscious application behavior unchanged
- [ ] Test in `internal/combat/concentration_test.go` (or `deathsave_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-31-fall-damage-unwired — airborne drop-to-0 should still trigger fall damage in addition to prone
- C-43-instant-death — same `applyDamageHP` site; coordinate ordering (check instant death before applying dying conditions)
- C-43-damage-at-0HP — same site
- C-43-heal-reset — symmetric on heal path

## Notes
Iterate `ConditionsForDying()` so future additions to the list flow automatically.
