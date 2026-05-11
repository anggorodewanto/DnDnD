---
id: C-35-hostile-near
group: C
phase: 35
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Ranged-with-hostile-within-5ft disadvantage never fires

## Finding
`AdvantageInputs.HostileNearAttacker` is never set by any discord handler. Grep returns zero hits in `internal/discord/`. The ranged-attack disadvantage for having a hostile creature within 5ft therefore never fires at runtime. Pure-function logic in `DetectAdvantage` is correct but unwired.

## Code paths cited
- `internal/combat/advantage.go` `DetectAdvantage` — consumer of `HostileNearAttacker`
- `internal/combat/advantage_test.go` — only setter
- `internal/discord/attack_handler.go` / `buildAttackInput` — should populate the flag from board state

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 35 advantage/disadvantage auto-detection)
- `.review-state/group-C-phases-29-43.md` Phase 35 findings

## Acceptance criteria (test-checkable)
- [ ] Attack pipeline populates `HostileNearAttacker` from board state (any hostile, non-incapacitated combatant within 5ft of attacker)
- [ ] Ranged attack with hostile within 5ft rolls at disadvantage end-to-end via `/attack`
- [ ] Melee attack and ranged-with-no-hostile-near are unaffected
- [ ] Test in `internal/discord/attack_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-33-cover-on-attacks, C-35-attacker-size, C-35-dm-adv-flags — all populate `AttackInput` in `buildAttackInput`
- C-38-reckless-target-side — same pipeline

## Notes
Definition of "hostile" should match the existing hostility model used elsewhere in combat (faction or DM-marked).
