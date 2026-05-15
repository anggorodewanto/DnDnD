finding_id: J-C01
status: done
files_changed:
  - internal/dashboard/ws.go
  - internal/dashboard/ws_test.go
  - internal/dashboard/handler.go
test_command_that_validates: go test ./internal/dashboard/ -run "TestWebSocketEndpoint_RejectsEncounterFromOtherCampaign|TestWebSocketEndpoint_AcceptsEncounterFromOwnCampaign" -v -count=1
acceptance_criterion_met: yes
notes: Added EncounterCampaignResolver interface and ownership check in ServeWebSocket. When encounter_id is provided and both CampaignLookup and EncounterCampaignResolver are wired, the handler parses the encounter UUID, resolves its campaign, compares against the DM's active campaign, and rejects with HTTP 403 on mismatch. The check is a no-op when resolvers are not wired, preserving backward compatibility with existing tests and deployments.
follow_ups:
  - Wire EncounterCampaignResolver in cmd/dndnd/main.go (implement adapter over the existing store's GetEncounter method)
  - Consider adding a test for invalid encounter_id UUID format (400 response)
  - Consider adding a test for encounter not found (404 response)
