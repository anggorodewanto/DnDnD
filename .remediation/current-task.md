finding_id: E-C02
severity: Critical
title: AoE damage path ignores upcasting and cantrip scaling
location: internal/combat/aoe.go:851 (ResolveAoEPendingSaves reads dmgInfo.Dice raw)
spec_ref: Phase 60 "Upcasting, Ritual, Cantrip Scaling"; spec §891-1072
problem: |
  CastAoE does compute effectiveSlotLevel but never calls ScaleSpellDice on the damage. The AoE damage pipeline uses the base dmgInfo.Dice string verbatim, so a 5th-level Fireball still rolls 8d6, and Thunderclap stays at 1d6 forever.
suggested_fix: |
  In the AoE cast path (or when creating pending saves), call ScaleSpellDice(dmgInfo, spellLevel, effectiveSlotLevel, charLevel) and pass the scaled result into the damage dice field. The charLevel must be looked up from the original caster.
acceptance_criterion: |
  A Fireball upcast to 5th level rolls 10d6 (base 8d6 + 2d6 for 2 levels above 3rd). A cantrip AoE at character level 5 rolls 2d6 instead of 1d6. Tests demonstrate both.
