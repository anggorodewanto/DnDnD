finding_id: J-H01
severity: High
title: Campaign Home cards show player-facing display_name, not the spoilery internal name
location: cmd/dndnd/main.go:243-246 and 261-265
spec_ref: spec lines 1694, 2840, 3094-3095
problem: |
  Both adapters prefer e.DisplayName over e.Name and surface the result into the DM's Campaign Home cards. Spec says the internal name is the dashboard-only spoiler-safe name.
suggested_fix: |
  Return e.Name unconditionally for the dashboard cards.
acceptance_criterion: |
  The encounter lister adapters return e.Name (internal name) for dashboard display. A test demonstrates this.
