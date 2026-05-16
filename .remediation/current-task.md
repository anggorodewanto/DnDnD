finding_id: cross-cut-H01
severity: High
title: routePhase43DeathSave skips instant-death when rawNewHP == 0
location: internal/combat/damage.go:336-346
spec_ref: PHB p.197 Massive Damage
problem: |
  The helper computes overflow only when rawNewHP < 0. If the hit takes the PC from >0 directly to exactly 0 (rawNewHP == 0), overflow is 0 even when actual damage was massive. A 1-HP creature taking 200 damage enters dying instead of instant death.
suggested_fix: |
  Compute overflow := adjusted - int(target.HpCurrent) whenever target.HpCurrent > 0 (clamp to >= 0).
acceptance_criterion: |
  A 1-HP creature with maxHP 10 taking 15 damage (adjusted after temp HP) results in instant death (overflow 14 >= maxHP 10). A test demonstrates this.
