finding_id: F-C02
severity: Critical
title: Heavy-armor STR speed penalty computed but never applied to combat speed
location: internal/combat/equip.go:237,478-487; internal/combat/turnresources.go:217-254
spec_ref: spec §Equipment Enforcement lines 1483-1487; phases §Phase 75b
problem: |
  CheckHeavyArmorPenalty only emits a string in the equipArmor combat log; the returned penalty is discarded. ResolveTurnResources starts every turn from char.SpeedFt and never consults the equipped-armor STR requirement, so an underqualified PC moves at full speed every turn.
suggested_fix: |
  In ResolveTurnResources, look up the equipped armor and subtract CheckHeavyArmorPenalty from speedFt before exhaustion/condition handling.
acceptance_criterion: |
  A character wearing heavy armor with STR below the requirement has their combat speed reduced by 10ft at turn start. A character meeting the STR requirement has no penalty. Tests demonstrate both.
