finding_id: H-H12
severity: High
title: Plus-2 ASI silently truncates at cap (loses 1 point) without warning
location: internal/levelup/asi.go:81 (applyPlus2)
spec_ref: spec §"ASI path" line 2484
problem: |
  The spec wants the bot to reject when a single score would exceed 20. The code accepts and silently caps at 20, so a STR-19 player picking +2 STR ends up with STR 20 and loses the second point.
suggested_fix: |
  Reject the choice if current + bonus > 20 (return error so user picks again).
acceptance_criterion: |
  applyPlus2 returns an error when the chosen ability is at 19 (19+2=21 > 20). A test demonstrates this.
