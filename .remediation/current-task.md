finding_id: I-C01
severity: Critical
title: DM-created characters never inherit class or racial features
location: internal/dashboard/feature_provider.go:38, 49, 63-65; internal/dashboard/charcreate_handler.go:583, 552
spec_ref: Phase 93b — "features auto-populated from SRD class data by level"
problem: |
  RefDataFeatureProvider indexes class features by cls.ID (slug, e.g. "wizard") and races by r.ID. But the dashboard wizard form sets option value to display names (e.g. "Wizard", "Mountain Dwarf"). CollectFeatures then does classFeatures[c.Class] with the name and gets nothing.
suggested_fix: |
  Either store the class/race id (slug) as the option's value, or have the feature provider do a normalized lookup (case-fold + slugify) in CollectFeatures/RacialTraits.
acceptance_criterion: |
  CollectFeatures("Barbarian", 1) returns the same features as CollectFeatures("barbarian", 1). A test demonstrates the case-insensitive lookup works.
