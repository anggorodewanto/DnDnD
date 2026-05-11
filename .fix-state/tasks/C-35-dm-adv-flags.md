---
id: C-35-dm-adv-flags
group: C
phase: 35
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# DM override advantage/disadvantage flags wired only at data-model layer

## Finding
`DMAdvantage` and `DMDisadvantage` flags exist on `AttackCommand`, but no DM dashboard handler in `internal/discord/` ever sets them. The "DM override from dashboard" branch of Phase 35 is wired only at the data-model layer; there is no user-reachable way for the DM to flip these flags.

## Code paths cited
- `internal/combat/attack.go` (or wherever `AttackCommand`/`AdvantageInputs` define `DMAdvantage` / `DMDisadvantage`)
- `internal/discord/` — DM dashboard handlers, currently no setters

## Spec / phase-doc anchors
- `docs/phases.md` lines 170-244 (Phase 35 advantage/disadvantage auto-detection, DM override branch)
- `.review-state/group-C-phases-29-43.md` Phase 35 findings

## Acceptance criteria (test-checkable)
- [ ] DM dashboard exposes a control (slash command or button) to set `DMAdvantage` / `DMDisadvantage` for the next attack roll of a targeted combatant
- [ ] When set, the override propagates through the attack pipeline and cancels/combines per existing `resolveMode` rules
- [ ] Override is consumed exactly once (does not persist across multiple attacks unless that is the spec)
- [ ] Test in `internal/discord/` (DM dashboard handler test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- C-33-cover-on-attacks, C-35-hostile-near, C-35-attacker-size — all converge on `buildAttackInput`

## Notes
Verify against the phase doc whether the override is per-attack or persistent until cleared; default to per-attack if ambiguous.
