finding_id: G-C01
severity: Critical
title: Passive-effect vocabulary in spec does not match the code parser
location: internal/magicitem/effects.go:112-160
spec_ref: spec §Magic Items lines 2697-2701 (Phase 88a)
problem: |
  Spec uses "modify_save" and "grant_resistance" but code only recognizes "modify_saving_throw" and "resistance". Any DM authoring passive_effects JSON following the spec verbatim gets no effect.
suggested_fix: |
  Add "modify_save" and "grant_resistance" as accepted aliases in the switch statement.
acceptance_criterion: |
  Both "modify_save" and "modify_saving_throw" produce the same effect. Both "grant_resistance" and "resistance" produce the same effect. Tests demonstrate the aliases work.
