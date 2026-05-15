finding_id: J-C01
severity: Critical
title: WebSocket subscribes to any encounter without campaign-ownership check
location: internal/dashboard/ws.go:135
spec_ref: Phase 103 (WS state sync), spec §DM Dashboard
problem: |
  ServeWebSocket takes encounter_id from the query string and registers the client without verifying the authenticated DM owns that encounter's campaign. A DM of campaign A can stream full snapshots from campaign B's encounters.
suggested_fix: |
  In ServeWebSocket, parse the encounter_id UUID, load the encounter, resolve its campaign, and verify the authenticated user is that campaign's DM before registering the client. Reject mismatch with 403.
acceptance_criterion: |
  A DM connecting to a WebSocket with an encounter_id from another campaign gets rejected (403 or connection refused). A DM connecting to their own campaign's encounter succeeds. A test demonstrates both.
