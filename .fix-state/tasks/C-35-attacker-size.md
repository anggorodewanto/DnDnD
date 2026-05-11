---
id: C-35-attacker-size
group: C
phase: 35
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Heavy-weapon disadvantage for Small/Tiny attackers never fires

## Finding
`AdvantageInputs.AttackerSize` is never set by any discord handler. The heavy-weapon disadvantage when a Small or Tiny attacker wields a heavy weapon therefore never fires at runtime. Pure-function logic in `DetectAdvantage` is correct but unwired.

## Code paths cited
- `internal/combat/advantage.go` `DetectAdvantage` — consumer of `AttackerSize`
- `internal/combat/advantage_test.go` — only setter
- `internal/discord/attack_handler.go` / `buildAttackInput` — should populate the field from attacker's size category

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 35 advantage/disadvantage auto-detection + Phase 37 weapon properties)
- `.review-state/group-C-phases-29-43.md` Phase 35 findings

## Acceptance criteria (test-checkable)
- [ ] Attack pipeline populates `AttackerSize` from the attacking combatant's size category
- [ ] Small or Tiny attacker wielding a heavy weapon rolls attack at disadvantage end-to-end via `/attack`
- [ ] Medium or larger attacker is unaffected; non-heavy weapons unaffected
- [ ] Test in `internal/discord/attack_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-30-occupant-size — same size-resolution helper should serve both
- C-33-cover-on-attacks, C-35-hostile-near, C-35-dm-adv-flags — same `buildAttackInput` site

## Notes
Reuse the size lookup helper introduced for C-30 to avoid duplication.
