finding_id: cross-cut-H05
severity: High
title: Action Surge max uses never scales to 2 at fighter level 17
location: internal/combat/action_surge.go (no scaling function exists)
spec_ref: PHB p.72 Fighter class table
problem: |
  Action Surge has 2 uses at Fighter 17+. No code raises Max to 2 — stays at 1 forever.
suggested_fix: |
  Add ActionSurgeMaxUses(fighterLevel int) int (1 at L2-16, 2 at L17+) and have the level-up service bump feature_uses["action-surge"].Max when a Fighter crosses level 17.
acceptance_criterion: |
  ActionSurgeMaxUses(17) returns 2. ActionSurgeMaxUses(16) returns 1. A test demonstrates both.
