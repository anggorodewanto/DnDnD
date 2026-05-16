finding_id: A-H09
severity: High
title: Sessions middleware re-issues cookie even when slide TTL fails silently
location: internal/auth/middleware.go:62-77
spec_ref: spec §Session management (line 72); Phase 10
problem: |
  When SlideTTL fails the middleware logs and continues without re-issuing the cookie and without aborting the request. The session in the DB still has its old expires_at. This silently lets sessions expire mid-traffic.
suggested_fix: |
  Either fail the request on slide error (consistent with fail-closed auth) or, at minimum, always re-issue the cookie since the DB state already lets this request through.
acceptance_criterion: |
  When SlideTTL fails, the request is aborted with 500 (fail-closed). A test demonstrates this.
