---
id: C-33-cover-on-attacks
group: C
phase: 33
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Cover bonus to AC never applied on attack rolls

## Finding
`CalculateCover` is never called from production attack paths. `attack.go:429` uses `EffectiveAC(input.TargetAC, input.Cover)` but `input.Cover` is never set by `buildAttackInput` or any `Service.Attack` / `OffhandAttack` call site — it stays at the zero value `CoverNone`. Cover-on-attacks does not fire end-to-end. (Cover-on-saves IS wired via `aoe.go:188`.)

## Code paths cited
- `internal/combat/cover.go` — `CalculateCover`, `EffectiveAC` defined
- `internal/combat/cover_test.go` — only `CalculateCover` consumer
- `internal/combat/attack.go:429` — `EffectiveAC(input.TargetAC, input.Cover)` consumer with always-zero `Cover`
- `internal/combat/attack.go` `buildAttackInput` — never populates `input.Cover`
- `internal/combat/aoe.go:188` — working reference: `CalculateCoverFromOrigin` + `DEXSaveBonus`

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 33 cover calculation)
- `.review-state/group-C-phases-29-43.md` Phase 33 findings

## Acceptance criteria (test-checkable)
- [ ] `buildAttackInput` (or equivalent attack-pipeline assembly site) calls `CalculateCover` from attacker to target and stores the result on `AttackInput.Cover`
- [ ] Half cover (+2 AC) and three-quarters cover (+5 AC) measurably change attack hit results
- [ ] Full cover causes the attack to be rejected (cannot target)
- [ ] Cover-on-saves behavior (AoE) remains unchanged
- [ ] Test in `internal/combat/attack_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-35-hostile-near, C-35-attacker-size, C-35-dm-adv-flags — all touch `buildAttackInput`; coordinate to avoid merge churn
- C-38-reckless-target-side — also `attack.go` attack-pipeline territory

## Notes
Mirror the AoE-save wiring pattern in `aoe.go:188`. Use creature-cover via `creatureCover` for intervening occupants in addition to wall cover.
