finding_id: H-C02
severity: Critical
title: Feat prerequisites and "already-has-feat" exclusion not enforced in live picker
location: cmd/dndnd/discord_handlers.go:1155 (asiFeatLister.ListEligibleFeats)
spec_ref: spec §"Feat path" lines 2486-2487
problem: |
  The production FeatLister returns the first 25 feats alphabetically with no prerequisite filtering and no exclusion of feats the character already has. CheckFeatPrerequisites exists but is never called in the player flow.
suggested_fix: |
  Implement ListEligibleFeats to load the character's scores/proficiencies, run CheckFeatPrerequisites per feat, and exclude IDs already present in Features (Source=="feat").
acceptance_criterion: |
  ListEligibleFeats excludes feats the character already has and feats whose prerequisites aren't met. A test demonstrates both exclusions.
