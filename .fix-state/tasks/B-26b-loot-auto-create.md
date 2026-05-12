---
id: B-26b-loot-auto-create
group: B
phase: 26b
severity: MEDIUM
status: done
owner: lifecycle
reviewer:
last_update: 2026-05-11

---

# Auto-create loot pool on EndCombat

## Finding
Loot pool availability: `internal/loot/Service.CreateLootPool` requires encounter
status = completed but is NOT auto-invoked from `EndCombat`. The dashboard
`/api/campaigns/{id}/encounters/{id}/loot` is mounted, so a DM can manually create a
pool, but the spec implies auto-availability post-end.

## Code paths cited
- `internal/loot/` — `Service.CreateLootPool` exists, requires status=completed
- `internal/combat/service.go` — `EndCombat` does not call `CreateLootPool`
- dashboard route `/api/campaigns/{id}/encounters/{id}/loot` — manual-only entry point

## Spec / phase-doc anchors
- Phase 26b: Combat Lifecycle — End Combat & Cleanup
- `.review-state/group-B-phases-18-28.md` line 115

## Acceptance criteria (test-checkable)
- [ ] `EndCombat` triggers `CreateLootPool` for the just-completed encounter
- [ ] Loot pool is available immediately after EndCombat without manual DM action
- [ ] Test in `internal/combat/service_test.go` (or `internal/loot/*_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- B-26b-all-hostiles-defeated-prompt — same lifecycle hook
- B-26b-ammo-recovery-prompt — same lifecycle hook
- B-26b-combat-log-announcement — same lifecycle hook

## Notes
Manual creation path remains valid; auto-creation should be idempotent or guarded
against duplicate pools per encounter.
