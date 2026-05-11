---
id: D-51-divine-smite-prompt
group: D
phase: 51
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Divine Smite post-hit prompt is never fired

## Finding
`PromptDivineSmite` (`class_feature_prompt.go:79`) is not invoked from `attack_handler.go` or any attack pipeline. `ResourceTriggers` is populated by `ProcessEffects` but no consumer reads it. Tests cover the service and the prompt poster in isolation only.

## Code paths cited
- `internal/discord/class_feature_prompt.go:79` — `PromptDivineSmite` (orphaned)
- `internal/discord/attack_handler.go` — attack pipeline (missing hook)
- `internal/combat/divine_smite.go` — service (correct)
- `internal/combat/effect.go` — `ResourceTriggers` populated but unconsumed

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 51)
- `docs/dnd-async-discord-spec.md` Divine Smite section

## Acceptance criteria (test-checkable)
- [ ] After a paladin's melee weapon attack hits, `PromptDivineSmite` is invoked (gated on slot availability)
- [ ] Accepting the prompt enumerates available slots and applies smite damage per service
- [ ] Crit doubling and undead/fiend +1d8 are honored when invoked from the prompt
- [ ] Test in `internal/discord/attack_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- `attack_handler.go` is also the hook site for D-48b (Stunning Strike) and D-49 (Bardic Inspiration) prompts.

## Notes
Consider a unified post-hit prompt dispatcher reading `ResourceTriggers` so all class-feature prompts share one wiring point.
