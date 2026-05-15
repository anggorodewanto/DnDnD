finding_id: H-C01
severity: Critical
title: Single-class half-caster (Paladin/Ranger) gets wrong slot count
location: internal/character/spellslots.go:108
spec_ref: spec §"Multiclass spell slots" line 2511
problem: |
  CalculateSpellSlots always routes through MulticastSpellSlots(casterLevel) with casterLevel = floor(classLevel/2) for half-casters even for single-class characters. The multiclass table at caster level N is not the same as the half-caster's own table. Example: Paladin 3 → own table gives 3×1st-level slots; current code returns MulticastSpellSlots(1) = {1:2} → only 2 slots.
suggested_fix: |
  Branch in CalculateSpellSlots: if len(classes)==1 && progression=="half", use a dedicated half-caster table (or compute as MulticastSpellSlots(ceil(level/2))). Add tests covering Paladin 3, 5, 9.
acceptance_criterion: |
  A single-class Paladin 3 gets {1:3} spell slots (3 first-level slots). A Paladin 5 gets {1:4, 2:2}. Tests demonstrate correct values.
