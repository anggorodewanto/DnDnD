# DnDnD — Agent Rules

## Development Process

- Always use red/green TDD: write a failing test first, then write the minimal code to make it pass, then refactor
- Aim for 90% code coverage (overall) and 85% per-package, enforced by `make cover-check`
- Run /simplify after coding to review for reuse, quality, and efficiency

## Testing

- See [docs/testing.md](docs/testing.md) for the three-tier test pyramid, fixture helpers (`internal/testutil`), and the coverage-exclusion list.

## Manual Playtest

- [docs/playtest-quickstart.md](docs/playtest-quickstart.md) — fresh checkout to live `/move`-ready encounter in <30 min.
- [docs/playtest-checklist.md](docs/playtest-checklist.md) — scenarios to walk every session.
- `cmd/playtest-player/` — REPL that observes Discord channels and validates / records player slash commands. `make playtest-replay TRANSCRIPT=path` replays a recording through the Phase 120 e2e harness.
