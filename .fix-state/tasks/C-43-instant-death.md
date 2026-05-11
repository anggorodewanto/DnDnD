---
id: C-43-instant-death
group: C
phase: 43
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Instant-death overflow check never wired into damage pipeline

## Finding
`CheckInstantDeath` and `ProcessDropToZeroHP` are test-only — they are never invoked in production. The damage pipeline never compares overflow vs max HP; massive overkill damage currently produces a normal "drop to 0, dying" outcome instead of instant death.

## Code paths cited
- `internal/combat/deathsave.go` `CheckInstantDeath`, `ProcessDropToZeroHP` — defined
- `internal/combat/deathsave_test.go` — only callers
- `internal/combat/concentration.go:385` `applyDamageHP` — should consult `CheckInstantDeath` on drop-to-0

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 43 death saves & unconsciousness, instant death rule)
- `.review-state/group-C-phases-29-43.md` Phase 43 findings

## Acceptance criteria (test-checkable)
- [ ] On any damage event that reduces HP to 0 or below, the pipeline computes the overflow (`damage - prevHP`) and routes through `CheckInstantDeath` / `ProcessDropToZeroHP`
- [ ] If overflow >= combatant's max HP, the combatant is marked dead immediately (no death saves)
- [ ] If overflow < max HP, the existing drop-to-0/dying flow runs
- [ ] Test in `internal/combat/concentration_test.go` (or `deathsave_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-43-prone-on-drop — same site; instant death must check BEFORE applying dying conditions
- C-43-damage-at-0HP — same site
- C-31-fall-damage-unwired — fall damage may trigger this path

## Notes
Order in `applyDamageHP`: compute newHP -> if newHP <= 0 then instant-death check first -> if not instant-death, apply dying conditions (Phase 43 prone-on-drop). This ordering is load-bearing.
