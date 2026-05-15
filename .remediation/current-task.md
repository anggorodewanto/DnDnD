finding_id: cross-cut-H04
severity: High
title: Paladin Channel Divinity max uses scale to 2 at level 15
location: internal/combat/channel_divinity.go:31-38
spec_ref: PHB p.85 Paladin class table
problem: |
  Paladin returns 2 at level >= 15. PHB says Paladin never gains a second CD use (only Cleric scales: 1@L2, 2@L6, 3@L18).
suggested_fix: |
  Drop the level >= 15 → 2 branch for Paladin (return 1 for level >= 3, 0 otherwise).
acceptance_criterion: |
  ChannelDivinityMaxUses("paladin", 15) returns 1, not 2. A test demonstrates this.
