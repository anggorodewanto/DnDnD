finding_id: G-C02
severity: Critical
title: /attune does not require a short rest
location: internal/inventory/attunement.go:33-67, internal/discord/attune_handler.go:68-159
spec_ref: spec §Magic Items lines 2710-2712 (Phase 88b)
problem: |
  Attune validates inventory presence, requires_attunement, slot cap (3), and class restriction, but never checks that the caster is currently in or has just completed a short rest. The Discord handler can be called at any time, immediately granting bonuses.
suggested_fix: |
  Add an OutOfCombat precondition check (reject if the character is in an active encounter) — this is the simplest enforcement that prevents mid-combat attunement. The spec says "can be done during /rest short flow" which implies it should be blocked during combat at minimum.
acceptance_criterion: |
  /attune returns an error when the character is in an active encounter. /attune succeeds when the character is not in combat. A test demonstrates both.
