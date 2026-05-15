finding_id: F-C04
severity: Critical
title: Lair Action placed at head of turn queue instead of "loses ties" with 20
location: internal/combat/legendary.go:304-348
spec_ref: spec §Enemy / NPC Turns lines 1916-1918; phases §Phase 78b
problem: |
  BuildTurnQueueEntries prepends the Lair Action entry at Initiative: 20 and then appends regular combatants. The function does not sort by initiative descending, and lair actions can fire before legitimate winners at 20. Per DMG p246, lair actions fire on initiative count 20, losing initiative ties.
suggested_fix: |
  After building all entries (including lair), sort the slice by initiative descending with a stable secondary key that pushes Lair Action entries after every combatant sharing the same initiative number. Add an IsLairAction bool field to the entry struct and use it as a tiebreaker.
acceptance_criterion: |
  When a combatant has initiative 20 and a lair action also has initiative 20, the combatant acts first (lair action loses ties). A test demonstrates this ordering.
