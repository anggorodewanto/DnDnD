finding_id: H-H03
severity: High
title: ASI ApproveASI silently rejects feat type instead of routing
location: internal/levelup/asi.go:35 (ApplyASI)
spec_ref: spec §"Feat path" line 2499
problem: |
  ApplyASI returns "unsupported ASI type" error when choice.Type == ASIFeat. The HTTP handler's /api/levelup/asi/approve passes feat-typed payloads straight into ApplyASI which errors.
suggested_fix: |
  In Service.ApproveASI, branch on choice.Type == ASIFeat and route to ApplyFeat.
acceptance_criterion: |
  ApproveASI with a feat-type choice routes to ApplyFeat instead of erroring. A test demonstrates this.
