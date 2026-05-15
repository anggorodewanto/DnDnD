finding_id: C-H04
severity: High
title: Dash adds raw base speed, ignoring exhaustion/condition speed modifiers
location: internal/combat/standard_actions.go:38-71 (Dash), 73-87 (resolveBaseSpeed)
spec_ref: Phase 42 (exhaustion levels 2/5 modify speed); spec §Exhaustion
problem: |
  Dash does updatedTurn.MovementRemainingFt += speed where speed is char.SpeedFt (raw base). An exhaustion-2 PC (speed halved) gets a full base-speed Dash bonus, effectively recovering the halving.
suggested_fix: |
  Pipe Dash through EffectiveSpeedWithExhaustion/EffectiveSpeed so it adds the effective speed. Also reject Dash when effective speed = 0.
acceptance_criterion: |
  A Dash for an exhaustion-2 character (speed halved) adds half the base speed, not full. A test demonstrates this.
