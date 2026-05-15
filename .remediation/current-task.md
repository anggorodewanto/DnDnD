finding_id: I-C02
severity: Critical
title: Pending #dm-queue badge count is campaign-wide, not per-encounter
location: internal/combat/workspace_handler.go:173-180, 271; db/queries/dm_queue_items.sql:56-63
spec_ref: Spec §DM Dashboard "Each tab shows a badge with the count of pending #dm-queue items for that encounter" (2792); Phase 94a
problem: |
  GetWorkspace calls CountPendingDMQueueItemsByCampaign once and assigns the same number to every encounter response. With two active encounters, both tabs display the same count.
suggested_fix: |
  Add encounter_id to dm_queue_items and write a per-encounter aggregation query, or filter the existing campaign-wide count by encounter_id at the handler level.
acceptance_criterion: |
  Each encounter in the workspace response has its own PendingQueueCount reflecting only items for that encounter. A test demonstrates different counts for different encounters.
