finding_id: E-H03
severity: High
title: Pact-magic upcast respects pact level but silently ignores --slot requests
location: internal/combat/spellcasting.go:446-457
spec_ref: Phase 64 "Pact Magic (Warlock)"
problem: |
  If a multiclass warlock passes --slot 2 but their pact slot is level 3, the code uses pact slot at level 3 regardless. Players cannot intentionally downcast below pact level.
suggested_fix: |
  When cmd.SlotLevel > 0 and falling into the pact path, reject with error if cmd.SlotLevel > pactSlots.SlotLevel. If cmd.SlotLevel < pactSlots.SlotLevel, also reject (can't downcast pact slots).
acceptance_criterion: |
  A warlock with pact slot level 3 requesting --slot 5 gets an error. A test demonstrates this.
