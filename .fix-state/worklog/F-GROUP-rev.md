# F-GROUP bundle — review

Reviewer: opus-4.7 (read-only). Date: 2026-05-12.

## Per-task verdict

| Task | Verdict |
| --- | --- |
| F-78c-bonus-actions-schema | PASS |
| F-81-targeted-check-handler | PASS |
| F-81-group-check-handler | PASS |
| F-81-dm-prompted-checks | PASS |
| F-86-item-picker-homebrew-flag | PASS |
| F-86-item-picker-custom-entry | PASS |
| F-86-item-picker-narrative-price | PASS |
| F-88c-detect-magic-environment | PASS |
| F-89d-asi-restart-persistence | PASS |

## Verification

- `make build` clean.
- `make test` green across all packages (including `internal/discord`,
  `internal/dashboard`, `internal/itempicker`, `internal/combat`).
- All worklog-cited tests present and run:
  `TestResolveBonusActions_*`, `TestBuildTurnPlan_UsesStructuredBonusActions`,
  `TestCheckHandler_TargetedCheck_*` (5), `TestDashboardCheckHandler_GroupCheck_*`
  (5) + `_PromptCheck_*` (4), `TestHandleSearch_HomebrewFilter*` +
  `TestHandleCustomEntry_*`, `TestCastHandler_DetectMagic_*` (4),
  `TestASIHandler_PersistsPendingChoiceToStore` + `_HydratePending_*`.
- `sqlc generate` reproduces exactly the diff present in the working
  tree (`internal/refdata/creatures.sql.go`, `models.go`, new
  `pending_asi.sql.go`, `pending_checks.sql.go`). No undeclared drift.
- Migrations: only 3 additions
  (`20260511130000_add_bonus_actions_to_creatures.sql`,
  `20260511130001_create_pending_asi.sql`,
  `20260511130002_create_pending_checks.sql`); no historical migrations
  edited. `integration_test.go` MigrateDown prepends the three new names.
  `testutil/testdb.go` MutableTables adds `pending_asi`/`pending_checks`.

## Findings

- F-78c: `ResolveBonusActions` correctly prefers the structured column
  (Valid + non-empty RawMessage + successful Unmarshal yielding ≥1 entry)
  and falls back to `ParseBonusActions` on all failure modes; tests cover
  each fallback branch.
- F-81 targeted: adjacency uses Chebyshev ≤1, action deducted via
  `combat.UseResource`/`UpdateTurnActions`; persistence failure swallowed
  (DM correction note).
- F-81 group + DM-prompt: dashboard route only (allowed by spec OR clause).
  Endpoint persists `pending_checks` row.
- F-86: search now carries `homebrew` per result and accepts
  `?homebrew=true|false`; armor refdata has no column so always false (called
  out in worklog). Custom-entry endpoint returns generated `custom-<uuid>`
  with all narrative/price fields echoed.
- F-88c: `DetectMagicRadiusFt=30` (PHB). Self + nearby aggregated; scanner
  errors degrade silently to self-only.
- F-89d: store wired via `asiPendingStoreAdapter` in
  `cmd/dndnd/discord_handlers.go`; `HydratePending` invoked once at boot.

## Caveats (not F-GROUP-caused)

- `make cover-check` currently fails: overall 92.22%, but `cmd/dndnd`
  package falls to 28.12% because `cmd/dndnd/lifecycle_adapters.go`
  (untracked, owned by **LIFECYCLE** bundle, not F-GROUP) is unexcluded
  and low-covered. F-GROUP's only `cmd/dndnd` touch is in
  `discord_handlers.go`, already on `COVER_EXCLUDE`. F-GROUP-touched
  packages all clear the 85% gate (discord 86.51%, dashboard 92.78%,
  itempicker 97.87%, combat 92.90%).
- `go tool cover -func` errors with `bufio.Scanner: token too long` —
  pre-existing tooling issue independent of F-GROUP.

## Conclusion

F-GROUP bundle accepted. All 9 tasks meet acceptance criteria.
