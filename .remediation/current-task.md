finding_id: H-C03
severity: Critical
title: Level-up does not auto-add new class/subclass features
location: internal/levelup/service.go:186 (ApplyLevelUp)
spec_ref: spec §"Leveling workflow" lines 2453, 2471
problem: |
  ClassRefData.FeaturesByLevel is loaded by the store adapter but never read in ApplyLevelUp. No code appends the class's level-N features to character.features on level-up.
suggested_fix: |
  After computing newClasses, iterate classRef.FeaturesByLevel for the new level and append to features (deduping by name+source).
acceptance_criterion: |
  A Fighter leveling from 4 to 5 gets "Extra Attack" added to their features. A test demonstrates the feature is appended.
