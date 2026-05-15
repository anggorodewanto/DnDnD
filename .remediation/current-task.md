finding_id: H-C05
severity: Critical
title: Levelup HTTP handler does not bound newLevel to 20
location: internal/levelup/handler.go:106
spec_ref: spec §Internal Character Format / 5e level cap
problem: |
  HandleLevelUp rejects newLevel < 1 but accepts any positive int. A DM can set Fighter to level 99 and the service will compute nonsense HP/proficiency/spell-slots.
suggested_fix: |
  Reject newLevel < 1 || newLevel > 20 at the handler.
acceptance_criterion: |
  HandleLevelUp returns 400 when newLevel > 20. A test demonstrates this.
