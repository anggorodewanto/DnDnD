finding_id: F-H06
severity: High
title: Legendary-action budget round-trips through the URL — no server persistence
location: internal/combat/legendary_handler.go:73-78,170-180
spec_ref: spec §Enemy / NPC Turns line 1916; Phase 78b
problem: |
  The dashboard sends budget_remaining as a query param and the server trusts it. Two dashboards can desync the budget.
suggested_fix: |
  Add a legendary_action_budget field persisted server-side. Decrement on ExecuteLegendaryAction, reset on creature's turn start.
acceptance_criterion: |
  ExecuteLegendaryAction decrements a server-side budget. A test demonstrates the budget decreases and rejects when exhausted.
