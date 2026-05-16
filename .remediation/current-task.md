finding_id: F-H05
severity: High
title: Lair-action "no consecutive repeats" tracker is in-memory only
location: internal/combat/legendary.go:198-263
spec_ref: spec §Enemy / NPC Turns line 1918; Phase 78b
problem: |
  LairActionTracker is a value type with no DB persistence. After a bot restart the tracker resets and the "no repeats" rule lapses.
suggested_fix: |
  Persist last_used_lair_action on the encounter row and hydrate LairActionTracker from it.
acceptance_criterion: |
  After setting a lair action, the tracker persists the choice. A test demonstrates the value survives a "reload" (re-hydration from stored state).
