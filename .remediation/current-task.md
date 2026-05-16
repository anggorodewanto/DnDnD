finding_id: C-H02
severity: High
title: PC creature size hard-coded to "Medium" — heavy-weapon disadvantage never fires
location: internal/combat/attack.go:1316-1326
spec_ref: Phase 35; spec line 687
problem: |
  resolveAttackerSize returns the creature row's size for NPCs but falls through to "Medium" for every PC. Halfling/gnome PCs wielding heavy weapons never get disadvantage.
suggested_fix: |
  Look up the PC's race and read races.size. Pass it through the attack input.
acceptance_criterion: |
  A Small PC (halfling/gnome) attacking with a heavy weapon gets disadvantage. A Medium PC does not. Tests demonstrate both.
