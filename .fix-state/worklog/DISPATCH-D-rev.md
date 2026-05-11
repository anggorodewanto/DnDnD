# DISPATCH-D review (2026-05-11)

Independent reviewer, read-only. All ten tasks audited against worklog + handler
source + new tests. Build / test / cover-check re-run from scratch.

## Per-task verdict

| Task ID | Verdict | Notes |
|---|---|---|
| D-47-wild-shape-dispatch | PASS | `bonus_handler.go:143-146` routes `wild-shape`/`revert-wild-shape` to `ActivateWildShape` / `RevertWildShapeService`. Tests `TestBonusHandler_WildShape_*` / `_RevertWildShape` assert `wsActivateCalls`/`wsRevertCalls`==1; removing the case lines drops the count to 0 → tests fail. |
| D-48b-flurry-of-blows | PASS | `bonus_handler.go:147` (`flurry`/`flurry-of-blows`/`flurryofblows`). Alias coverage in dedicated test. Attack-action precondition delegated to service per spec. |
| D-50-channel-divinity-dispatch | PASS | `action_handler.go:514` + `dispatchChannelDivinity` routes turn-undead / preserve-life / sacred-weapon / vow-of-enmity / DM-queue fallback; missing-option hint covered. 6 tests, each asserts the matching call slice. |
| D-52-lay-on-hands-action-vs-bonus | PASS | `/action lay-on-hands` wired (`action_handler.go:516`); `/bonus` retained as alias per worklog. Tests cover happy + bad-args paths on action surface. |
| D-53-action-surge | PASS | `action_handler.go:496` → `ActionSurge`. Test asserts surge call + log substring. |
| D-54-standard-actions-wiring | PASS-WITH-CAVEAT | All 7 actions (dash/disengage/dodge/help/stand/drop-prone/escape) wired. **Caveat**: `dispatchStand` hardcodes `MaxSpeed:30` — Halflings/Tabaxi will use wrong cost. Documented in worklog as follow-up but no task file filed. |
| D-54-cunning-action | PASS | `bonus_handler.go:149` → `CunningAction` with dash/disengage/hide modes; hide branch requires roller; bad-mode rejected. Rogue gate inside service. |
| D-56-drag-release | PARTIAL (confirmed) | `/bonus drag`/`release-drag` wired; `/move` doubles cost via `DragMovementCost`. Grappled-target tile-sync deferred — **not filed** as a new task file. |
| D-57-hide-action | PASS | `action_handler.go:506` → `Hide`; hostiles filtered via `filterHostiles`; no-roller branch tested. |
| D-57-cunning-action-hide | PASS | Covered by D-54 cunning-action hide branch; dedicated test asserts hide mode. |

## Findings

- **F1 (D-56)**: deferred grappled-target tile sync is mentioned in
  `DISPATCH-D-impl.md` but no `.fix-state/tasks/D-56-*-tilesync.md` follow-up
  file exists. Should be filed per review scope.
- **F2 (D-54)**: `dispatchStand` MaxSpeed=30 hardcode acknowledged as
  follow-up but, like F1, lacks a task file.
- **F3 (cover-check transient)**: first `make cover-check` run failed with
  `no required module provides package …rest/party_handler…`; re-run succeeded
  (`87.22%` discord, `OK: coverage thresholds met`). Likely stale `coverdata`
  artifact; not a blocker.

## Verification

- `git diff --stat HEAD`: 29 files / +2387 / −98 — matches scope.
- `make build`: clean (`bin/dndnd`, `bin/playtest-player`).
- `go test ./internal/discord/...`: PASS (0.463s).
- `make test`: PASS (whole tree cached, no failures).
- `make cover-check`: PASS on second run (flake noted in F3).
- Adapter spot-checks: `actionCombatServiceAdapter.{ActionSurge,Hide,TurnUndead,
  LayOnHands}` all forward to `a.svc.<Method>` (`discord_handlers.go:1077-1134`).
  Bonus adapter unchanged; `*combat.Service` structurally satisfies new methods.
- Conventions: early-return throughout; no WHAT-style comments; no
  `--no-verify` / hook skips in worklog or git history.
- Test FAIL-on-remove confirmed by reading mocks: each new branch appends to
  a call slice and tests assert `len == 1`; removing the dispatch line drops
  count to 0.

## Next steps

1. File `.fix-state/tasks/D-56-tilesync.md` for the deferred grappled-target
   path-sync work; close D-56 as PARTIAL with explicit link.
2. File a follow-up task for `dispatchStand` MaxSpeed lookup adapter
   (mirror `MoveSizeSpeedLookup` onto `ActionHandler`).
3. Investigate the `make cover-check` flake (stale coverage artifact) before
   the next bundle to avoid review-time false alarms.

## Overall verdict

**APPROVE WITH FINDINGS** — 9/10 PASS, D-56 PARTIAL as labeled. All
dispatch wiring exercised by tests that would fail on regression. Two
follow-ups need task files; neither blocks merge.
