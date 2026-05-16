finding_id: G-H01
severity: High
title: Gold split silently discards remainder
location: internal/loot/service.go:289-329
spec_ref: spec §Inventory Management line 2661 (Phase 85)
problem: |
  SplitGold computes share := pool.GoldTotal / len(pcs) then zeros the pool. For 7gp / 3 players, each gets 2gp and 1gp evaporates.
suggested_fix: |
  Leave GoldTotal % len(pcs) in the pool for the DM to dispense manually.
acceptance_criterion: |
  After splitting 7gp among 3 players, each gets 2gp and the pool retains 1gp. A test demonstrates this.
