finding_id: D-H04
severity: High
title: Monk Unarmored Defense not invalidated by shield
location: internal/combat/equip.go:416; internal/character/stats.go:63
spec_ref: Phase 48a; PHB Monk "Unarmored Defense"
problem: |
  Monk's Unarmored Defense (AC = 10 + DEX + WIS) requires no armor AND no shield. The code adds +2 for shield on top of any ac_formula-derived AC with no class check.
suggested_fix: |
  When ac_formula is the monk variant (contains WIS), skip the shield bonus.
acceptance_criterion: |
  A monk with a shield does NOT get the shield +2 added to their Unarmored Defense AC. A test demonstrates this.
