finding_id: G-H05
severity: High
title: Items auto-populated from defeated NPCs are not removed from NPC inventory
location: internal/loot/service.go:67-142
spec_ref: spec §Inventory Management line 2654 (Phase 85)
problem: |
  CreateLootPool reads NPC gold+inventory into the pool but never zeros the NPC's values. Re-invoking CreateLootPool duplicates loot.
suggested_fix: |
  Inside the pool-create transaction, write UpdateCharacterGold(0) + clear inventory for each defeated NPC whose items moved into the pool.
acceptance_criterion: |
  After CreateLootPool, the defeated NPC's gold is 0 and inventory is empty. A test demonstrates this.
