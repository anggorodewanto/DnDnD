---
id: C-43-followup-timer-nat20-heal-reset
group: C
phase: 43
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: Nat-20 death-save timer path doesn't persist heal-reset

## Finding
DEATH-SAVE reviewer flagged: the death-save timer Nat-20 code path (likely in `internal/combat/timer_resolution.go` or similar) writes 1 HP back to the combatant but does NOT invoke `MaybeResetDeathSavesOnHeal`. Result: a PC who rolls a natural 20 on a death save returns to 1 HP with stale failure tallies still on record.

## Code paths cited
- `internal/combat/timer_resolution.go` — Nat-20 branch in the death-save resolver.
- `internal/combat/deathsave.go` — `MaybeResetDeathSavesOnHeal` already wired for `LayOnHands` and `PreserveLife`.

## Spec / phase-doc anchors
- docs/dnd-async-discord-spec.md (Phase 43 death-save Nat-20 rule)

## Acceptance criteria (test-checkable)
- [ ] Nat-20 death save resolves to 1 HP AND `death_save_failures` reset to 0
- [ ] Test in `internal/combat/deathsave_integration_test.go` exercises the Nat-20 → recovery path
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Touches `internal/combat/` — coordinate with any other combat-package task.

## Notes
Surfaced by DEATH-SAVE reviewer (`MAIN-WIRING-rev.md` peer). Not blocking — death saves still work end-to-end; this is a state-cleanup gap.
