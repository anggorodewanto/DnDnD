finding_id: I-C02
status: done
files_changed:
  - internal/combat/workspace_handler.go
  - internal/combat/workspace_handler_test.go
  - cmd/dndnd/main.go
  - cmd/dndnd/main_wiring_test.go
test_command_that_validates: go test ./internal/combat/ -run TestWorkspaceHandler_GetWorkspace_PendingQueueCount_PerEncounter -v
acceptance_criterion_met: yes
notes: |
  The handler previously called CountPendingDMQueueItemsByCampaign once and assigned the same count to every encounter. Fixed by adding CountPendingDMQueueItemsByEncounter to the WorkspaceStore interface, calling it per-encounter in the loop, and implementing it in the production adapter via a JOIN through pending_actions (which links dm_queue_items to encounters). No DB migration required. The failing test confirmed both encounters received the campaign-wide count (5); after the fix, each gets its own count (3 and 1).
follow_ups:
  - Consider adding a sqlc query for CountPendingDMQueueItemsByEncounter to replace the raw SQL in workspaceStoreAdapter
  - The unused CountPendingDMQueueItemsByCampaign method remains in the interface; remove if no other consumer needs it
