---
id: C-43-stabilize-followup-wis-modifier
group: C
phase: 43
severity: LOW
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Follow-up: /action stabilize uses flat d20 (no WIS modifier)

## Finding
C-43-stabilize closure landed with a flat d20 vs DC 10 check. PHB Medicine is `d20 + WIS_mod + proficiency`. Currently a non-proficient stabilizer with WIS 8 has the same odds as a cleric with WIS 18 and Medicine proficiency.

## Code paths cited
- `internal/discord/action_handler.go` `dispatchStabilize` (or wherever the DC 10 gate lives) — flat d20 roll.
- `internal/character/` ability-score / proficiency lookups (existing).

## Spec / phase-doc anchors
- PHB Stabilizing a Creature, Medicine check rules.

## Acceptance criteria (test-checkable)
- [ ] `/action stabilize` adds the actor's WIS modifier + Medicine proficiency to the d20 roll
- [ ] Test in `internal/discord/action_handler_test.go` exercises a high-WIS proficient stabilizer (auto-pass) and a low-WIS amateur (failure on a 9 with -1)
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Touches `internal/discord/action_handler.go` — coordinate with other action-handler tasks.

## Notes
Surfaced by C-DISCORD reviewer; minor polish, not a critical functional gap.
