finding_id: A-H01
severity: High
title: Player can never resubmit after changes_requested (broken status flow)
location: internal/registration/service.go:46-56 + internal/dashboard/approval_store.go:30-40
spec_ref: spec §Registration feedback (line 54); Phase 8 / Phase 14
problem: |
  validTransitions/validApprovalTransitions do not allow changes_requested -> pending (or anything else), and the partial unique index only excludes retired. So a player whose registration is in changes_requested is permanently stuck.
suggested_fix: |
  Allow changes_requested -> pending and rejected -> pending in both transition maps.
acceptance_criterion: |
  A player_character in changes_requested status can transition to pending. A test demonstrates this.
