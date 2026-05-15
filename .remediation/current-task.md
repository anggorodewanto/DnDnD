finding_id: C-C04
severity: Critical
title: /fly performs no fly-speed validation
location: internal/combat/altitude.go:52-81; internal/discord/fly_handler.go:98-109
spec_ref: Phase 31 (Altitude & Flying); spec §Altitude & Elevation
problem: |
  ValidateFly rejects only negative altitudes, same-altitude moves, and insufficient movement. There is no check for whether the combatant actually possesses a fly speed (character speed_fly, beast Wild Shape with fly speed, Fly spell-applied fly_speed condition, etc.). Any character can /fly 30 despite having no flight source.
suggested_fix: |
  Have Service.Fly (or ValidateFly) consult the character/creature speed and active conditions (e.g., fly_speed, wild_shape w/ beast fly speed) and reject with "You don't have a fly speed" when none apply. The FlySpeedCondition constant already exists but is only used on the cleanup side.
acceptance_criterion: |
  A combatant without a fly speed (no speed_fly on character, no fly_speed condition) gets an error when attempting /fly. A combatant WITH a fly speed (either innate or from a condition) can fly successfully.
