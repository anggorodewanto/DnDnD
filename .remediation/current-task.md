finding_id: G-C04
severity: Critical
title: Antitoxin "advantage vs poison" is not actually tracked
location: internal/inventory/service.go:135-140
spec_ref: spec §Inventory Management lines 2647 (Phase 84)
problem: |
  UseConsumable consumes the antitoxin and posts a flavor message claiming advantage was granted, but no buff/condition is written to the character or combatant. The next poison save is a plain d20.
suggested_fix: |
  Apply a timed condition (e.g., "antitoxin" with duration 1 hour / 10 rounds) on the combatant that the save service consults to add advantage when the save is against poison.
acceptance_criterion: |
  After using an antitoxin, the character has an "antitoxin" condition applied. A test demonstrates the condition is written. (The save-side consumption of this condition is a separate concern for the save handler.)
