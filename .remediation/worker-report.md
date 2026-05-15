finding_id: J-C03
status: done
files_changed:
  - internal/open5e/client.go
  - internal/open5e/client_test.go
test_command_that_validates: go test ./internal/open5e/ -run TestNewClient_DefaultHTTPClientHasTimeout -v
acceptance_criterion_met: yes
notes: Replaced http.DefaultClient (zero timeout) with &http.Client{Timeout: 10 * time.Second} as the default when no custom client is provided. Added TestNewClient_DefaultHTTPClientHasTimeout which asserts the default client's Timeout field is non-zero. Both make test and make cover-check pass cleanly. The open5e package coverage is 92.47%.
follow_ups: []
