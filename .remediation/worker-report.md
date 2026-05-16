# Worker Report: C-H02

## Finding
**PC creature size hard-coded to "Medium" — heavy-weapon disadvantage never fires**

`resolveAttackerSize` in `internal/combat/attack.go` returned `"Medium"` for all PCs regardless of race. Halfling/gnome PCs wielding heavy weapons never got disadvantage.

## Fix Applied

### Files Changed
1. **internal/combat/service.go** — Added `GetRace(ctx context.Context, id string) (refdata.Race, error)` to the `Store` interface (reference data lookups section).
2. **internal/combat/attack.go** — Updated `resolveAttackerSize` to look up the PC's character record, then resolve the race's size via `GetRace`. Falls back to `"Medium"` if either lookup fails (backward compat).
3. **internal/combat/service_test.go** — Added `getRaceFn` field to `mockStore` struct and `GetRace` method implementation.
4. **internal/combat/bundle3_test.go** — Added two tests:
   - `TestPopulateAttackContext_ResolvesPCRaceSize`: verifies a halfling PC resolves to "Small".
   - `TestSmallPC_HeavyWeapon_GetsDisadvantage`: verifies DetectAdvantage produces disadvantage for a Small creature with a heavy weapon.

### TDD Cycle
- **Red:** `TestPopulateAttackContext_ResolvesPCRaceSize` failed with `expected: "Small"`, `actual: "Medium"`.
- **Green:** After wiring `GetRace` through the Store interface and updating `resolveAttackerSize`, all tests pass.

### Verification
- `make test` — ✅ all tests pass
- `make cover-check` — ✅ coverage thresholds met

### Backward Compatibility
- If `GetRace` returns an error (e.g., race not in DB), falls back to `"Medium"`.
- If `GetCharacter` fails (no character linked), falls back to `"Medium"`.
- Existing `TestPopulateAttackContext_DefaultsPCSizeToMedium` still passes (default mock returns `sql.ErrNoRows` for character lookup).
- Existing `TestPopulateAttackContext_CommandOverrideWins` still passes (explicit `AttackerSize` is never overwritten).
- `storeAdapter` (wrapping `*refdata.Queries`) automatically satisfies the new interface method since `refdata.Queries` already has `GetRace`.
