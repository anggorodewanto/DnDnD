# DnDnD — Agent Rules

## Development Process

- Always use red/green TDD: write a failing test first, then write the minimal code to make it pass, then refactor
- Aim for 90% code coverage (overall) and 85% per-package, enforced by `make cover-check`
- Run /simplify after coding to review for reuse, quality, and efficiency

## Testing

- See [docs/testing.md](docs/testing.md) for the three-tier test pyramid, fixture helpers (`internal/testutil`), and the coverage-exclusion list.
