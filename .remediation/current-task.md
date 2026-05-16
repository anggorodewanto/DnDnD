finding_id: J-H03
severity: High
title: DM dashboard error panel cannot render stack trace — error_detail column never written
location: internal/errorlog/recorder.go:18-29; internal/errorlog/pgstore.go:79-83
spec_ref: spec §Error Log (lines 3176-3194); Phase 119
problem: |
  The migration defines error_detail JSONB but Entry has no Detail field and the INSERT only writes command, user_id, summary. Stack traces are discarded.
suggested_fix: |
  Add Detail json.RawMessage to Entry, populate from debug.Stack() in the panic middleware, include in INSERT.
acceptance_criterion: |
  Entry struct has a Detail field. The INSERT includes it. A test demonstrates Detail is stored.
