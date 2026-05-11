---
id: C-43-damage-at-0hp
group: C
phase: 43
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Damage-at-0-HP death save failures never tallied

## Finding
`ApplyDamageAtZeroHP` is test-only. When a dying combatant takes another hit, no death-save failures are recorded (1 normal hit / 2 on crit per RAW). `applyDamageHP:379` short-circuits the unconscious application when `prevHP <= 0` but never increments `death_saves.failures`.

## Code paths cited
- `internal/combat/deathsave.go` `ApplyDamageAtZeroHP` — defined
- `internal/combat/deathsave_test.go` — only caller
- `internal/combat/concentration.go:379,385` `applyDamageHP` — short-circuit path missing the tally

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 43 death saves & unconsciousness, damage at 0 HP)
- `.review-state/group-C-phases-29-43.md` Phase 43 findings

## Acceptance criteria (test-checkable)
- [ ] When a combatant at 0 HP takes any damage, `death_saves.failures` increments by 1
- [ ] If the hit is a critical, failures increment by 2
- [ ] Reaching 3 failures triggers death via the existing path
- [ ] Instant-death overflow rule (C-43-instant-death) still takes precedence if applicable
- [ ] Test in `internal/combat/concentration_test.go` (or `deathsave_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-43-instant-death — same site; check instant death first
- C-43-prone-on-drop — same site
- C-43-heal-reset — symmetric reset on heal

## Notes
Route through `ApplyDamageAtZeroHP` to avoid duplicating failure-tally logic.
