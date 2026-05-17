finding_id: I-H04
severity: High
title: Action Resolver move effect bypasses turn lock, walls, and concentration hooks
location: internal/combat/dm_dashboard_handler.go:215-313, 400-421
spec_ref: Spec §Undo & Corrections; Phase 94b
problem: |
  ResolvePendingAction is not wrapped in withTurnLock, and applyMoveEffect writes directly to store.UpdateCombatantPosition rather than going through the service.
suggested_fix: |
  Wrap ResolvePendingAction in withTurnLock, and replace the raw store call with svc.UpdateCombatantPosition.
acceptance_criterion: |
  ResolvePendingAction acquires the turn lock before mutating. A test demonstrates the lock is acquired.
