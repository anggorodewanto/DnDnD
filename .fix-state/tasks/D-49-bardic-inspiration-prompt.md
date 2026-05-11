---
id: D-49-bardic-inspiration-prompt
group: D
phase: 49
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Bardic Inspiration usage prompt is not wired

## Finding
`PromptBardicInspiration` exists but no caller. Players cannot consume their granted die from a prompt on attack/check/save — only via the `UseBardicInspiration` service which has no Discord entry point.

## Code paths cited
- `internal/combat/bardic_inspiration.go` — `UseBardicInspiration` service
- `internal/discord/class_feature_prompt.go` — `PromptBardicInspiration` (orphaned)
- `internal/discord/attack_handler.go` / check / save handlers — missing invocation
- `internal/discord/bonus_handler.go:127` — granting handler (already wired)

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 49)
- `docs/dnd-async-discord-spec.md` Bardic Inspiration usage on attack/check/save

## Acceptance criteria (test-checkable)
- [ ] When a combatant holding an inspiration die makes an attack roll, ability check, or saving throw, `PromptBardicInspiration` is offered before the roll resolves
- [ ] Accepting consumes the die and adds it to the roll; declining leaves the die intact
- [ ] Test in `internal/discord/<roll>_handler_test.go` fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- Hooks into the same attack/check/save pipelines as Divine Smite (D-51), Stunning Strike (D-48b).

## Notes
10-min real-time expiry sweep already exists in `initiative.go:405`; prompt must verify die has not expired.
