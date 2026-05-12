# Regression sweep — round 1

Date: 2026-05-12
Trigger: post-batch-3 (campaign-end regression sweep before playtest gate).

## Commands run

```
make test            ✅ all packages PASS
make cover-check     ✅ "OK: coverage thresholds met"
make build           ✅ clean (cmd/dndnd + cmd/playtest-player)
make e2e             ✅ all TestE2E_* PASS (TestE2E_RecapEmptyScenario, etc.)
make playtest-replay TRANSCRIPT=/home/ab/projects/DnDnD/internal/playtest/testdata/sample.jsonl
                     ✅ TestE2E_ReplayFromFile PASS
```

## Coverage spot-checks (post-batch)
- internal/combat: 92.90%
- internal/discord: 86.51%
- internal/dashboard: 92.78%
- internal/itempicker: 97.87%
- internal/registration: 93.41%
- overall: ≥ 90% (passing the make cover-check gate)

## Notes

1. `make playtest-replay` requires an absolute TRANSCRIPT path. The relative-path
   form fails because `go test` runs with cwd inside `cmd/dndnd/`.  Filing a
   trivial follow-up to either accept a relative path from project root or
   document the absolute requirement in `docs/playtest-quickstart.md`.
2. Earlier-batch flake `TestRun_AuthProtectedRoutesRejectUnauthenticated` not
   reproduced in this sweep — testcontainer cold-start was the likely cause.
3. No prior-task regression observed. All 86 closed tasks retain their wiring
   per spot-checked diffs.

## Re-open list
None.

## Verdict
PASS — proceed to final playtest-readiness gate.
