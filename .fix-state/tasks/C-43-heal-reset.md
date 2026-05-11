---
id: C-43-heal-reset
group: C
phase: 43
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Healing-from-0 does not reset death-save tallies / unconscious / prone

## Finding
`HealFromZeroHP` is test-only. The actual healing code paths (rest / lay-on-hands / spells) do not call it; whether death-save tallies and dying conditions are reset on heal depends on each individual healing handler — and is therefore inconsistent at best.

## Code paths cited
- `internal/combat/deathsave.go` `HealFromZeroHP` — defined
- `internal/combat/deathsave_test.go` — only caller
- Healing handlers: `internal/combat/...` rest, lay-on-hands, cure-wounds resolvers — should route through `HealFromZeroHP`

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 43 death saves & unconsciousness, healing from 0 HP)
- `.review-state/group-C-phases-29-43.md` Phase 43 findings

## Acceptance criteria (test-checkable)
- [ ] Any healing event that raises HP from 0 to >0 routes through `HealFromZeroHP`
- [ ] On heal-from-0: death-save successes/failures reset to 0; `unconscious` and `prone` (per `ConditionsForDying`) are removed
- [ ] Healing a non-dying combatant is unaffected
- [ ] Stable-but-at-0 combatants regain consciousness per the helper's contract
- [ ] Test in `internal/combat/...heal_test.go` (or a shared healing test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-43-prone-on-drop — symmetric on the damage path; both should iterate `ConditionsForDying`
- C-43-stabilize — both transition dying-state
- C-43-damage-at-0HP — same tally state machine

## Notes
Centralize through `HealFromZeroHP` so all healing handlers share one source of truth — avoid copy-pasting reset logic into each handler.
