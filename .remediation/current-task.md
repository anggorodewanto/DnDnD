finding_id: G-H04
severity: High
title: /check medicine target:AR does not validate target is dying and does not auto-stabilize
location: internal/discord/check_handler.go:286-320
spec_ref: spec §Death Saves line 2116 (Phase 81)
problem: |
  The check handler's TargetContext doesn't verify the target is at 0 HP, and on a successful Medicine roll it doesn't stabilize the target.
suggested_fix: |
  When skill == "medicine" and a target is supplied, require target.HpCurrent == 0, and on success (Total >= 10) persist stabilization.
acceptance_criterion: |
  A successful medicine check (DC 10) against a dying target stabilizes them. A medicine check against a non-dying target returns an error. Tests demonstrate both.
