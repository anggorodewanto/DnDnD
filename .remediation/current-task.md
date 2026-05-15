finding_id: D-C02
severity: Critical
title: Feature uses never initialized at character creation
location: internal/portal/builder_store_adapter.go:125 (CreateCharacterParams omits FeatureUses)
spec_ref: spec §Channel Divinity recharge "short"; Phase 46/48b/49/50/52/53
problem: |
  Neither the portal builder nor the dashboard charcreate path writes any feature_uses JSON when creating a character. ParseFeatureUses then returns Current=0 for every key, so a freshly built barbarian fails with "no rage uses remaining (0/0)", monks have 0 ki, paladins can't lay on hands or channel divinity, and fighters can't action surge. Worse, when something does eventually write a value through SetFeaturePool, it preserves the existing (empty) Max and Recharge fields — so even after the value is set manually, short/long rests can never recharge them.
suggested_fix: |
  Populate FeatureUses in BuilderStoreAdapter.CreateCharacterRecord by walking the character's classes/level and seeding {Current, Max, Recharge} for every limited-use feature (rage, ki, channel-divinity, bardic-inspiration, lay-on-hands, action-surge, second-wind, sorcery-points, wild-shape). Create a helper function InitFeatureUses(classes []ClassEntry) map[string]FeatureUse that computes the correct values based on class and level.
acceptance_criterion: |
  A barbarian level 3 created via the portal builder has feature_uses containing {"rage": {Current: 3, Max: 3, Recharge: "long"}}. A fighter level 2 has {"action-surge": {Current: 1, Max: 1, Recharge: "short"}, "second-wind": {Current: 1, Max: 1, Recharge: "short"}}.
