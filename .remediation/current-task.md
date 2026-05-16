finding_id: C-H07
severity: High
title: Pre-clamp HP overflow excludes temp-HP absorbed damage from instant-death check
location: internal/combat/damage.go:226-247, 330-373
spec_ref: Phase 43; spec line 2096
problem: |
  The finding says the code is actually correct but needs a test to lock the invariant: "PC at 0 HP with 5 temp HP takes 25 damage; max HP 18" should result in instant death (25-5=20 >= 18).
suggested_fix: |
  Add an explicit test for "damage-at-0 with temp HP" to lock the invariant.
acceptance_criterion: |
  A test demonstrates that a PC at 0 HP with 5 temp HP taking 25 damage (max HP 18) results in instant death.
