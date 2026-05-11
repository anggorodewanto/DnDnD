---
id: D-48b-stunning-strike-prompt
group: D
phase: 48b
severity: HIGH
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Stunning Strike prompt is not triggered post-hit

## Finding
`PromptStunningStrike` (`class_feature_prompt.go:49`) exists but is never invoked from `attack_handler.go` or the attack pipeline. Monks cannot decide whether to spend ki to stun a target after a hit.

## Code paths cited
- `internal/discord/class_feature_prompt.go:49` — `PromptStunningStrike` (orphaned)
- `internal/discord/attack_handler.go` — attack pipeline (missing prompt hook)
- `internal/combat/monk.go` — `StunningStrike` service

## Spec / phase-doc anchors
- `docs/phases.md` lines 246-320 (Phase 48b)
- `docs/dnd-async-discord-spec.md` Monk — Stunning Strike

## Acceptance criteria (test-checkable)
- [ ] After a monk's melee weapon attack hits a creature, `PromptStunningStrike` is invoked (gated on ki availability and class/level prerequisites)
- [ ] Declining the prompt deducts no ki; accepting deducts ki and triggers the CON save flow
- [ ] Test in `internal/discord/attack_handler_test.go` (or class_feature_prompt integration test) fails before the fix and passes after
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- Many group-D tasks touch `internal/discord/action_handler.go` — flag this for serial scheduling.
- `internal/discord/attack_handler.go` is also the hook site for Divine Smite prompt (D-51) — coordinate.

## Notes
Same class-feature-prompt pattern as Divine Smite; consider shared dispatch.
