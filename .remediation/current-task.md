finding_id: C-H10
severity: High
title: Reach weapon OA detection — PC reach map relies on caller passing it
location: internal/combat/opportunity_attack.go:80-117, 148-164
spec_ref: Phase 39 / OA detection; spec line 1414-1416
problem: |
  resolveHostileReach returns 5ft for any PC by default. The override map pcReachByID must be supplied by the caller. If the move handler forgets, PCs with glaives don't threaten 10ft.
suggested_fix: |
  Move the PC reach computation into resolveHostileReach by looking up the PC's equipped_main_hand weapon properties.
acceptance_criterion: |
  A PC with a reach weapon (glaive) threatens 10ft without the caller needing to pass a reach map. A test demonstrates this.
