finding_id: D-C03
severity: Critical
title: Rage advantage on STR ability checks never wired
location: internal/check/check.go (no FES integration) and internal/combat/rage.go:71
spec_ref: spec §Feature Effect System "Rage … conditional_advantage on str_check on_check"; Phase 46
problem: |
  RageFeature emits a TriggerOnCheck conditional-advantage effect, but the check service never builds a FeatureDefinition list, never builds an EffectContext, and never calls ProcessEffects with TriggerOnCheck. A grep confirms no consumer of TriggerOnCheck anywhere in the repo. Result: a raging barbarian shoves an enemy with no rage advantage on the athletics check.
suggested_fix: |
  Mirror the save handler pattern — add FeatureEffects + EffectContext (with IsRaging, AbilityUsed) to check.SingleCheckInput, then call combat.ProcessEffects(_, TriggerOnCheck, _) and fold the resulting RollMode into the check's d20 mode. The simplest approach: add an optional RollMode override field to SingleCheckInput that callers can set when the character is raging and the ability is STR.
acceptance_criterion: |
  A raging barbarian making a STR ability check (e.g., Athletics for grapple) gets advantage applied to the roll. A non-STR check while raging does NOT get advantage.
