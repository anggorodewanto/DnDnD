# DnDnD — Agent Rules

## What This Is

Discord-native D&D 5e play assistant: a Go service (bot + web dashboard) backed by
Postgres that runs encounters, combat, exploration, and map rendering. See
[README.md](README.md) and [docs/phases.md](docs/phases.md) for scope.

- **Entrypoints:** `cmd/dndnd/` (main service — bot gateway + dashboard), `cmd/playtest-player/` (player-sim REPL).
- **Domain logic:** `internal/<domain>/` (e.g. `combat`, `encounter`, `gamemap`, `character`, `discord`, `refdata`, `database`). Web UIs in `dashboard/`, `portal/`.
- **Module:** `github.com/ab/dndnd`, Go 1.26.

## Commands

```sh
make build        # build bin/dndnd + bin/playtest-player
make test         # go test ./... -v
make cover-check  # tests + coverage gates (90% overall / 85% per-pkg)
make run          # go run ./cmd/dndnd/ (bare binary)
make local-up     # docker compose: app + Postgres (production-shaped path)
make e2e          # scenario tests behind the `e2e` build tag
make sqlc-check   # fail if sqlc-generated code drifts from schema/queries
```

## Development Process

- Always act as an orchestrator: divide work into independent sub-tasks and delegate them to subagents (run in parallel where there are no dependencies), reserving the main thread for planning, synthesis, and review.
- Always use red/green TDD: write a failing test first, then write the minimal code to make it pass, then refactor
- Aim for 90% code coverage (overall) and 85% per-package, enforced by `make cover-check`
- Run /simplify after coding to review for reuse, quality, and efficiency

## Gotchas

- **sqlc-generated code:** `internal/refdata/*.sql.go` is generated — edit `.sql` queries + run `make sqlc-check`, never hand-edit the `*.sql.go`.
- **e2e tests are build-tag-gated** (`//go:build e2e`): they run only via `make e2e` / `make playtest-replay` and are excluded from coverage on purpose.
- **`make run` without `DATABASE_URL`** boots a half-dead dashboard-only mode (DB features skipped, bot gateway never opens). Use `make local-up` for a full stack.

## Testing

- See [docs/testing.md](docs/testing.md) for the three-tier test pyramid, fixture helpers (`internal/testutil`), and the coverage-exclusion list.

## Manual Playtest

- [docs/playtest-quickstart.md](docs/playtest-quickstart.md) — fresh checkout to live `/move`-ready encounter in <30 min.
- [docs/playtest-checklist.md](docs/playtest-checklist.md) — scenarios to walk every session.
- `cmd/playtest-player/` — REPL that observes Discord channels and validates / records player slash commands. `make playtest-replay TRANSCRIPT=path` replays a recording through the Phase 120 e2e harness.
