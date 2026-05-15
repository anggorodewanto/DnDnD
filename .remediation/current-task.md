finding_id: J-C02
severity: Critical
title: Open5e public search endpoint bypasses per-campaign source gating
location: internal/open5e/handler.go:37 (RegisterPublicRoutes); main.go:848
spec_ref: spec §Extended Content (lines 2541-2546), Phase 111
problem: |
  /api/open5e/monsters and /api/open5e/spells are mounted with no auth and no campaign_id filter. The spec says "DM enables/disables third-party sources per campaign", but the live search returns the full upstream catalog.
suggested_fix: |
  Gate the search GETs behind DM auth so only the DM proxies Open5e, or require ?campaign_id= and filter results through CampaignSourceLookup.
acceptance_criterion: |
  The Open5e search endpoints require authentication (DM auth). Unauthenticated requests get 401. A test demonstrates this.
