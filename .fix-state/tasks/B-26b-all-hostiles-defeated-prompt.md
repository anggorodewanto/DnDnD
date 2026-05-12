---
id: B-26b-all-hostiles-defeated-prompt
group: B
phase: 26b
severity: HIGH
status: done
owner: lifecycle
reviewer:
last_update: 2026-05-11

---

# Auto-detect all hostiles at 0 HP and prompt DM to end combat

## Finding
Auto-detect "all hostiles at 0 HP -> DM prompt to end" is NOT wired. The check API
exists (`GET /api/combat/{encounterID}/hostiles-defeated`) and `AllHostilesDefeated`
is exposed, but no caller polls it; no Discord/dmqueue prompt is dispatched after a
hostile is killed.

## Code paths cited
- `internal/combat/service.go` — `AllHostilesDefeated` exposed via `GET /api/combat/{encounterID}/hostiles-defeated`
- `internal/combat/damage.go` — no references to `hostiles-defeated`/`AllHostilesDefeated`
- `internal/combat/deathsave.go` — no references to `hostiles-defeated`/`AllHostilesDefeated`
- dashboard svelte — no references to `hostiles-defeated`/`AllHostilesDefeated`

## Spec / phase-doc anchors
- Phase 26b: Combat Lifecycle — End Combat & Cleanup
- `.review-state/group-B-phases-18-28.md` lines 110-117

## Acceptance criteria (test-checkable)
- [ ] After a hostile combatant reaches 0 HP and all other hostiles are also at 0 HP, a DM-facing prompt is dispatched (Discord/dmqueue) suggesting end-of-combat
- [ ] Damage/death-save paths call (or trigger) the `AllHostilesDefeated` check
- [ ] Test in `internal/combat/*_test.go` (e.g. `service_test.go` or `damage_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- B-26b-loot-auto-create — same lifecycle hook (EndCombat)
- B-26b-combat-log-announcement — same lifecycle hook
- B-26b-ammo-recovery-prompt — same lifecycle hook

## Notes
Existing API endpoint and helper are present; the gap is the caller/notifier wiring
from damage/deathsave resolution to a DM prompt.
