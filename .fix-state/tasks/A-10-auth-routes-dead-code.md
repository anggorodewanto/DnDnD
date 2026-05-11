---
id: A-10-auth-routes-dead-code
group: A
phase: 10
severity: MINOR
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Remove dead `auth.RegisterRoutes` helper

## Finding
`internal/auth/routes.go` defines `RegisterRoutes` but isn't called anywhere — the portal mounts the OAuth handlers inline. Dead code.

## Code paths cited
- `internal/auth/routes.go` — defines `RegisterRoutes`, not called anywhere.
- `internal/portal/routes.go:52-54` — portal mounts `/portal/auth/{login,callback,logout}` inline.

## Spec / phase-doc anchors
- `docs/phases.md` phase 10

## Acceptance criteria (test-checkable)
- [ ] `internal/auth/routes.go` is removed (or `RegisterRoutes` is deleted)
- [ ] `grep -R "auth.RegisterRoutes" internal cmd` returns no matches
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- None obvious.

## Notes
Low severity / cosmetic per the doc; no behavior change expected.
