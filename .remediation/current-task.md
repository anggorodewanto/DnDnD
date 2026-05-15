finding_id: A-H03
severity: High
title: WebSocket origin verification defaults to InsecureSkipVerify: true
location: internal/dashboard/handler.go:117-170
spec_ref: spec §Authentication & Authorization (line 73); Phase 15
problem: |
  The dashboard WebSocket upgrade defaults to skipping origin checks. A forgotten config line lets any origin upgrade a session-cookie-authenticated WS connection (Cross-Site WebSocket Hijacking).
suggested_fix: |
  Flip the default to InsecureSkipVerify: false and require wsAllowedOrigins to be set explicitly.
acceptance_criterion: |
  The default wsInsecureSkipVerify is false. A test confirms the default rejects foreign origins.
