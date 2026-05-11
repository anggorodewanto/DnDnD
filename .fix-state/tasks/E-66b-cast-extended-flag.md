---
id: E-66b-cast-extended-flag
group: E
phase: 66b
severity: MEDIUM
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Extended Spell metamagic missing from /cast slash command options

## Finding
All 8 SRD metamagic options have validators and effect computation in service. Discord `/cast` exposes only 7 of 8 — `extended` is missing as a slash-command option in `commands.go:115-147`, even though `cast_handler.go:331` already reads it from metamagicFlags. End users cannot trigger Extended Spell from Discord.

## Code paths cited
- internal/discord/commands.go:115-147 — `/cast` options (missing `extended`)
- internal/discord/cast_handler.go:331 — metamagicFlags reader (already includes extended)
- internal/combat/metamagic.go — `ApplyExtendedSpell`, `ValidateMetamagicOptions`

## Spec / phase-doc anchors
- docs/phases.md — Phase 66b ("Metamagic — Individual Options"); SRD metamagic list including Extended Spell

## Acceptance criteria (test-checkable)
- [ ] `/cast` exposes an `extended` boolean (or equivalent) option in commands.go
- [ ] When passed, Extended Spell is applied to eligible spells (doubles duration up to 24h)
- [ ] Sorcery-point cost is deducted via existing `MetamagicTotalCost`
- [ ] Test in `internal/discord/cast_handler_test.go` (or commands registration test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- E-59-aoe-pending-saves, E-63-material-component-prompt (same handler and commands files)

## Notes
Heightened-disadvantage downstream consumer is flagged separately in the review; this task is only the missing slash option for Extended.
