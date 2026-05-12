---
id: B-26b-ammo-recovery-prompt
group: B
phase: 26b
severity: MEDIUM
status: done
owner: lifecycle
reviewer:
last_update: 2026-05-11

---

# Wire ammunition recovery prompt into EndCombat

## Finding
Ammunition recovery is NOT integrated. `RecoverAmmunition` helper exists at
`internal/combat/attack.go:212` but `EndCombat` explicitly notes it is deferred
("deferred as a separate schema migration. The helper exists at attack.go:212 ready
to call."). No per-encounter spent counter and no recovery prompt path.

## Code paths cited
- `internal/combat/attack.go:212` — `RecoverAmmunition` helper exists, unused
- `internal/combat/service.go:841-844` — comment notes ammo recovery deferred

## Spec / phase-doc anchors
- Phase 26b: Combat Lifecycle — End Combat & Cleanup
- `.review-state/group-B-phases-18-28.md` line 114

## Acceptance criteria (test-checkable)
- [ ] EndCombat invokes `RecoverAmmunition` (or surfaces a recovery prompt to players)
- [ ] Per-encounter spent-ammo counter is tracked and consulted
- [ ] Test in `internal/combat/attack_test.go` or `service_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- B-26b-all-hostiles-defeated-prompt — same lifecycle hook
- B-26b-loot-auto-create — same lifecycle hook
- B-26b-combat-log-announcement — same lifecycle hook

## Notes
Per the doc, this likely requires a separate schema migration to persist spent-ammo
counts; do not start coding before confirming migration scope.
