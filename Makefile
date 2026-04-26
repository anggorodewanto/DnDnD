.PHONY: build test cover cover-check cover-html run docker-build clean

# Excludes sqlc-generated query files, the cmd/dndnd main wiring, the thin
# discordgo *Adapter delegations, and the coverage_check tool itself —
# all of which are structurally untestable or tested via integration. See
# docs/testing.md for the rationale.
COVER_EXCLUDE := (internal/refdata/.*\.sql\.go|cmd/dndnd/main\.go|cmd/dndnd/discord_handlers\.go|cmd/dndnd/discord_adapters\.go|cmd/dndnd/notifier\.go|internal/discord/adapter\.go|scripts/coverage_check/main\.go|scripts/sqlc_drift_check/main\.go|internal/testutil/.*\.go)
COVER_MIN_OVERALL ?= 90
COVER_MIN_PER_PACKAGE ?= 85

build:
	go build -o bin/dndnd ./cmd/dndnd/

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
