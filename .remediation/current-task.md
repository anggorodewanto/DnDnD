finding_id: D-H06
severity: High
title: Wild Shape on-revert does not restore the druid's speed snapshot
location: internal/combat/wildshape.go:181 (RevertWildShape)
spec_ref: Phase 47
problem: |
  WildShapeSnapshot stores SpeedFt but RevertWildShape only writes HpMax/HpCurrent/Ac. The snapshot's speed field is never read on revert.
suggested_fix: |
  In RevertWildShape, also restore SpeedFt from the snapshot.
acceptance_criterion: |
  After reverting wild shape, the combatant's speed matches the snapshot. A test demonstrates this.
