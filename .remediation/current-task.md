finding_id: G-H06
severity: High
title: Item picker only searches weapons/armor/magic items
location: internal/itempicker/handler.go:57-156
spec_ref: spec §Item Picker line 2674 (Phase 86)
problem: |
  HandleSearch only iterates ListWeapons, ListArmor, ListMagicItems. Adventuring gear and potions/consumables are not searchable.
suggested_fix: |
  Add ListAdventuringGear / ListConsumables to the search and branch on category.
acceptance_criterion: |
  Searching for "rope" or "healing potion" returns results. A test demonstrates this.
