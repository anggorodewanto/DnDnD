---
id: PLAYTEST-REPLAY-followup-path-handling
group: H
phase: 121.3
severity: LOW
status: deferred
owner:
reviewer:
last_update: 2026-05-12
---

# Follow-up: make playtest-replay TRANSCRIPT relative-path handling

## Finding
Regression sweep round 1 surfaced that `make playtest-replay TRANSCRIPT=<rel>` fails because `go test` cwd is `cmd/dndnd/` while the user provides a project-relative path. Absolute path works.

## Acceptance criteria
- [ ] Either: `make playtest-replay` resolves the relative TRANSCRIPT to repo root before passing to go test, OR
- [ ] `docs/playtest-quickstart.md` documents the absolute-path requirement.

## Notes
Trivial; deferring as it does not affect playtest readiness (absolute path works today).
