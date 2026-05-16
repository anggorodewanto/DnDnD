finding_id: E-H04
severity: High
title: Multiclass spellcasting ability picks highest score, not class-of-spell
location: internal/combat/spellcasting.go:1542-1557 (resolveSpellcastingAbilityScore)
spec_ref: Phase 58; spec §988-989
problem: |
  resolveSpellcastingAbilityScore iterates every class and returns the maximum score. A Wizard/Cleric with INT 16 and WIS 18 casts fire bolt using WIS instead of INT.
suggested_fix: |
  Pass the spell's Classes slice into the resolver, intersect with the caster's classes, and use the first match's ability. Fall back to max only when the spell isn't on any of the caster's class lists.
acceptance_criterion: |
  A Wizard/Cleric casting a Wizard spell uses INT, not WIS. A test demonstrates this.
