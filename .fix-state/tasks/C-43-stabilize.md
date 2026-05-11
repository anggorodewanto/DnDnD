---
id: C-43-stabilize
group: C
phase: 43
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Stabilization paths (action + Spare the Dying) not wired

## Finding
No `/action stabilize` handler exists, and no Spare-the-Dying spell handler invokes `StabilizeTarget`. The seed for spare-the-dying exists (`seed_spells_cantrips.go:27`) but no resolution path calls the stabilize helper.

## Code paths cited
- `internal/combat/deathsave.go` `StabilizeTarget` — defined
- `internal/combat/deathsave_test.go` — only caller
- `internal/discord/standard_actions.go` (or equivalent action dispatcher) — missing stabilize action
- `internal/...seed_spells_cantrips.go:27` — Spare the Dying seed, no resolver

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 43 death saves & unconsciousness, stabilize action + Spare the Dying)
- `.review-state/group-C-phases-29-43.md` Phase 43 findings

## Acceptance criteria (test-checkable)
- [ ] `/action stabilize <target>` is available, requires the dying target to be within reach (5ft), consumes an action, and on a DC10 Medicine check (or per the phase doc's exact contract) calls `StabilizeTarget`
- [ ] Spare the Dying cantrip handler resolves to `StabilizeTarget` (auto-success, range per spell)
- [ ] On success, target gains the `stable` state via the helper; death-save tallies are handled per the helper's contract
- [ ] Test in `internal/discord/standard_actions_test.go` (and a spell handler test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-43-block-commands — stabilize action issued BY a non-dying actor TARGETING a dying actor; gate must not block the caster
- C-43-heal-reset — both touch dying-state transitions

## Notes
Verify the exact DC and ability for the action-based stabilize against the phase doc before implementing.
