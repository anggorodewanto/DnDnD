---
id: C-38-reckless-target-side
group: C
phase: 38
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Reckless Attack's "enemies get advantage against you" half is unmodeled

## Finding
`attack.go:438-454` applies the attacker-side half of Reckless Attack (advantage on the reckless attacker's STR-based melee attack), but the target-side half — "enemies have advantage on attack rolls against you until your next turn" — is not modeled. There is no per-target effect or condition applied to the reckless attacker that influences incoming attacks.

## Code paths cited
- `internal/combat/attack.go:438-454` — Reckless attacker-side wiring
- `internal/combat/advantage.go` `DetectAdvantage` — target-side path that should grant advantage when target is "reckless this round"
- `internal/combat/condition.go` — possible host for a transient `reckless` marker

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 38 attack modifier flags, Reckless Attack)
- `.review-state/group-C-phases-29-43.md` Phase 38 findings

## Acceptance criteria (test-checkable)
- [ ] When a combatant uses Reckless Attack, a transient marker (condition or per-turn flag) is set on them lasting until their next turn
- [ ] While the marker is active, `DetectAdvantage` grants advantage to attackers targeting them
- [ ] Marker clears on the reckless combatant's next turn start
- [ ] Test in `internal/combat/attack_test.go` (or `advantage_test.go`) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-33-cover-on-attacks, C-35-* — same `DetectAdvantage` / `buildAttackInput` territory
- Phase 39 condition-system hooks — may host the transient marker

## Notes
Prefer a condition with `source = "reckless"` and `DurationRounds` tuned to "until start of attacker's next turn" so it auto-expires through existing Phase 39 plumbing.
