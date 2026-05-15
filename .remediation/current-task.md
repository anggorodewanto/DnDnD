finding_id: J-C03
severity: Critical
title: Open5e HTTP client has no timeout — upstream stall can hang any /search
location: internal/open5e/client.go:43
spec_ref: Phase 111
problem: |
  NewClient defaults httpClient to http.DefaultClient, which has zero timeout. A slow Open5e API call wedges the goroutine indefinitely.
suggested_fix: |
  Construct a default &http.Client{Timeout: 10*time.Second} instead of using http.DefaultClient.
acceptance_criterion: |
  The Open5e client uses a non-zero timeout by default. A test verifies the client's timeout is set.
