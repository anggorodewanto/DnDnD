finding_id: C-H06
severity: High
title: Resistance/vulnerability halving allows damage to go to 0 (RAW says min 1)
location: internal/combat/damage.go:38-43 (ApplyDamageResistances)
spec_ref: Phase 42; PHB p197
problem: |
  1 fire damage to a fire-resistant target returns 1/2 = 0. Per RAW it should still be 1.
suggested_fix: |
  After the resistance branch, clamp to max(1, halved) when the raw input was >= 1 (preserve 0 → 0 for immunity).
acceptance_criterion: |
  1 damage with resistance returns 1 (not 0). 0 damage stays 0. A test demonstrates both.
