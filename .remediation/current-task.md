finding_id: J-H09
severity: High
title: Encounter snapshot publisher does NOT trigger on /move position writes
location: internal/discord/move_handler.go:686-735
spec_ref: Phase 104b
problem: |
  HandleMoveConfirm calls UpdateCombatantPosition directly via refdata.Queries, bypassing the service that has SetPublisher. The dashboard doesn't receive a fresh snapshot after a player moves.
suggested_fix: |
  Add an explicit publisher.PublishEncounterSnapshot call inside HandleMoveConfirm's post-write path.
acceptance_criterion: |
  After a move is confirmed, the encounter snapshot is published. A test demonstrates the publish call is made.
