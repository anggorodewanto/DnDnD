---
id: G-90-ddb-importer-wiring
group: G
phase: 90
severity: CRITICAL
status: open
owner:
reviewer:
last_update: 2026-05-11
---

# Phase 90 — Wire DDB importer service, DM approval path, and warning surface

## Finding
The DDB import service is feature-complete (parse → fetch → validate → preview → diff → pending re-sync → Approve/Discard with 24h TTL), but it is never wired in `cmd/dndnd/main.go`. `WithDDBImporter` option exists on `ImportHandler` (`registration_handler.go:137`) but no caller in `cmd/` invokes it or constructs `ddbimport.NewService`, so at runtime `/import` falls through to `handlePlaceholderImport` (`registration_handler.go:201`) which only writes a stub `CreatePlaceholder` row. Neither `ApproveImport` nor `DiscardImport` is invoked from the dashboard approval handler or any Discord interaction; `ValidationWarning` surfaces only in the disabled ephemeral preview path with no DM-facing dashboard warning UI.

## Code paths cited
- `internal/ddbimport/{client,parser,validator,diff,preview,service}.go` — feature-complete service
- `internal/ddbimport/client.go:18` — exponential backoff on fetch
- `internal/discord/registration_handler.go:108-219` — `ImportHandler`
- `internal/discord/registration_handler.go:137` — `WithDDBImporter` option (never invoked from `cmd/`)
- `internal/discord/registration_handler.go:201` — `handlePlaceholderImport` fallback
- `cmd/dndnd/main.go` — no `WithDDBImporter` / `ddbimport.NewService` callers

## Spec / phase-doc anchors
- `.review-state/group-G-phases-90-103.md` — Phase 90: D&D Beyond Import

## Acceptance criteria (test-checkable)
- [ ] `cmd/dndnd/main.go` constructs `ddbimport.NewService` and passes it via `WithDDBImporter` so `/import` no longer falls through to `handlePlaceholderImport`
- [ ] DM approval handler (or equivalent Discord interaction) invokes `ApproveImport` / `DiscardImport` before the 24h TTL expires
- [ ] `ValidationWarning` results surface in the DM-facing dashboard approval UI (not just the disabled ephemeral preview path)
- [ ] Test in `cmd/dndnd` or wiring smoke test fails before the fix and passes after, exercising the new wiring
- [ ] `make test && make cover-check && make build` clean

## Related / overlap risks
- group-G tasks heavily overlap `cmd/dndnd/main.go`. This is a serialization hotspot — coordinate with G-94a, G-95, and G-97b which also edit `cmd/dndnd/main.go` wiring.

## Notes
The doc bundles three sub-findings (no wiring, no DM approval path, no warning UI) under one phase entry. They share the root cause that the importer service is never reachable end-to-end; keep them in this single task unless they need to be split during implementation.
