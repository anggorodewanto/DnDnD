finding_id: D-C01
severity: Critical
title: Rage damage resistance never fires for seed-created barbarians
location: internal/combat/feature_integration.go:347, internal/refdata/seed_classes.go:23
spec_ref: spec §Feature Effect System example "Rage (Barbarian)"; Phase 46
problem: |
  The Rage class feature is seeded with mechanical_effect: "advantage_str_checks_saves,resistance_bludgeoning_piercing_slashing,bonus_rage_damage", but BuildFeatureDefinitions only emits a RageFeature for the literal token "rage". The two never match, so a barbarian created from the standard class seeds gets IsRaging=true but no RageFeature enters the FES — resistance, +damage, and STR adv are all dropped. Tests use MechanicalEffect: "rage" (rage_test.go:235), masking the bug.
suggested_fix: |
  Either alias all three seed tokens (resistance_bludgeoning_piercing_slashing, advantage_str_checks_saves, bonus_rage_damage) to the RageFeature builder, or replace the seed mechanical_effect with "rage". The simplest fix: change the seed to use "rage" as the mechanical_effect for the Rage feature, since that's what the FES expects. Add an integration test that runs BuildFeatureDefinitions over real seed features.
acceptance_criterion: |
  A barbarian character created from the standard seed data (seed_classes.go) has its Rage feature recognized by BuildFeatureDefinitions and produces a RageFeature definition. An integration test confirms this.
