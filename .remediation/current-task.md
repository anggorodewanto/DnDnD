finding_id: G-C03
severity: Critical
title: destroy_on_zero roll happens at dawn, not when last charge is spent
location: internal/inventory/recharge.go:38-92, internal/inventory/active_ability.go:28-62
spec_ref: spec §Magic Items line 2707; PHB Wand of Fireballs
problem: |
  UseCharges decrements charges without any d20 destroy check at the moment the last charge is spent. The destroy roll is instead performed by DawnRecharge when it sees Charges == 0. RAW: "If you expend the wand's last charge, roll a d20. On a 1, the wand crumbles."
suggested_fix: |
  Move the d20 destroy roll into UseCharges when the deducted amount drains the item to 0 charges, and drop the check from DawnRecharge.
acceptance_criterion: |
  When UseCharges reduces an item to 0 charges and the item has destroy_on_zero=true, a d20 is rolled. On a 1, the item is destroyed. The dawn recharge path no longer performs this check. Tests demonstrate both.
