---
id: B-26b-combat-log-announcement
group: B
phase: 26b
severity: MEDIUM
status: done
owner: lifecycle
reviewer:
last_update: 2026-05-11

---

# Post "combat ended" announcement to #combat-log

## Finding
Bot announcement to `#combat-log` on end: only the persistent `#initiative-tracker`
is updated; no distinct "combat ended" message is posted to `#combat-log`. The
`EndCombatResult.Summary` is returned but no notifier consumes it. Verify against
`/recap` flow if that satisfies; otherwise gap.

## Code paths cited
- `internal/combat/service.go` — `EndCombat` returns `EndCombatResult.Summary`, no notifier consumes it
- `#initiative-tracker` notifier — only that channel is updated on end

## Spec / phase-doc anchors
- Phase 26b: Combat Lifecycle — End Combat & Cleanup
- `.review-state/group-B-phases-18-28.md` line 116

## Acceptance criteria (test-checkable)
- [ ] On `EndCombat`, a distinct "combat ended" message is posted to `#combat-log`
- [ ] The post consumes `EndCombatResult.Summary` (or equivalent content)
- [ ] Test in `internal/combat/service_test.go` (or relevant notifier test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- B-26b-all-hostiles-defeated-prompt — same lifecycle hook
- B-26b-ammo-recovery-prompt — same lifecycle hook
- B-26b-loot-auto-create — same lifecycle hook

## Notes
Confirm whether `/recap` flow already satisfies the announcement requirement before
adding a separate notifier; if it does, this task may be downgraded.
