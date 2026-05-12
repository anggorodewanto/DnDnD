.PHONY: build test cover cover-check cover-html run docker-build clean e2e playtest-replay sqlc-check

# Excludes sqlc-generated query files, the cmd/dndnd main wiring, the thin
# discordgo *Adapter delegations, and the coverage_check tool itself —
# all of which are structurally untestable or tested via integration. See
# docs/testing.md for the rationale.
COVER_EXCLUDE := (internal/refdata/.*\.sql\.go|cmd/dndnd/main\.go|cmd/dndnd/discord_handlers\.go|cmd/dndnd/discord_adapters\.go|cmd/dndnd/dashboard_apis\.go|cmd/dndnd/notifier\.go|cmd/dndnd/lifecycle_adapters\.go|cmd/playtest-player/live_session\.go|internal/discord/adapter\.go|scripts/coverage_check/main\.go|scripts/sqlc_drift_check/main\.go|internal/testutil/.*\.go)
COVER_MIN_OVERALL ?= 90
COVER_MIN_PER_PACKAGE ?= 85

build:
	go build -o bin/dndnd ./cmd/dndnd/
	go build -o bin/playtest-player ./cmd/playtest-player/

test:
	go test ./... -v

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

cover-check: cover
	go run ./scripts/coverage_check \
		-profile coverage.out \
		-min-overall $(COVER_MIN_OVERALL) \
		-min-per-package $(COVER_MIN_PER_PACKAGE) \
		-exclude '$(COVER_EXCLUDE)'

cover-html: cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser"

run:
	go run ./cmd/dndnd/

docker-build:
	docker build -t dndnd .

clean:
	rm -rf bin/ coverage.out coverage.html

# Phase 118c local convenience target — same check the CI workflow runs.
# Re-runs `sqlc generate` and fails if any tracked file under internal/refdata
# changes. Requires the `sqlc` binary on PATH (or SQLC_BIN env override).
sqlc-check:
	go run ./scripts/sqlc_drift_check

# Phase 120: end-to-end test target. Runs only the scenario tests built
# under the `e2e` build tag, against a freshly-spun testcontainers Postgres.
# Kept off the default `make test` / `make cover-check` path so the existing
# 90%/85% coverage baseline stays at its current figure: every e2e file is
# build-tag-gated, so they cannot land in coverage.out unless invoked here.
e2e:
	go test -tags e2e ./cmd/dndnd/ -run TestE2E_ -count=1 -v

# Phase 121.3: replay a transcript captured by cmd/playtest-player
# through the Phase 120 harness. Override TRANSCRIPT to point at a
# specific JSONL file; the default is the checked-in sample.
TRANSCRIPT ?= $(CURDIR)/internal/playtest/testdata/sample.jsonl
playtest-replay:
	PLAYTEST_TRANSCRIPT=$(TRANSCRIPT) go test -tags e2e ./cmd/dndnd/ -run TestE2E_ReplayFromFile -count=1 -v
