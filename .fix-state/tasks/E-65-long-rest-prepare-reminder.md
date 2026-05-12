---
id: E-65-long-rest-prepare-reminder
group: E
phase: 65
severity: MEDIUM
status: done
owner: lifecycle
reviewer:
last_update: 2026-05-11
---

# Long-rest prepare reminder not posted

## Finding
`combat.LongRestPrepareReminder` exists but has no non-test callers. The long-rest flow does not surface the reminder. Spec: "Post long-rest reminder." MVP UX (paginated select menus) is acknowledged as deferred in `prepare_handler.go:48`; this task is only about wiring the reminder.

## Code paths cited
- internal/combat/preparation.go — `LongRestPrepareReminder` (no production caller)
- internal/discord/prepare_handler.go:48 — MVP UX note (informational; not part of this task)
- internal/discord/commands.go:462 — `/prepare` command registration
- internal/rest/rest.go — long-rest flow (missing reminder invocation)

## Spec / phase-doc anchors
- docs/phases.md — Phase 65 ("Spell Preparation"); "Post long-rest reminder"

## Acceptance criteria (test-checkable)
- [ ] Long-rest flow invokes `LongRestPrepareReminder` for affected casters
- [ ] Reminder is posted to Discord for each caster who can re-prepare
- [ ] Test in `internal/rest/rest_test.go` (or prepare reminder test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None expected; rest path is independent.

## Notes
Paginated select-menu UX is explicitly deferred and out of scope here.
