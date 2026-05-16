finding_id: F-H03
severity: High
title: Hidden combatants (is_visible = false) still render on the map
location: internal/gamemap/renderer/fog.go:52-78; internal/gamemap/renderer/token.go:38
spec_ref: spec §Dynamic Fog of War lines 2202-2203; spec §Standard Actions (Hide)
problem: |
  filterCombatantsForFog only considers tile visibility state. The Combatant.IsVisible field (set to false by Hide) is never consulted. A hidden rogue still renders on every player map.
suggested_fix: |
  In filterCombatantsForFog, drop combatants whose IsVisible is false for the player audience. Hidden combatants should still render in the DM view.
acceptance_criterion: |
  A combatant with IsVisible=false is excluded from the filtered list when DMSeesAll=false. Included when DMSeesAll=true. A test demonstrates both.
