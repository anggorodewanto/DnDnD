# Worker Report: C-H01

## Finding
**ID:** C-H01  
**Severity:** High  
**Title:** Auto-crit applies to ranged attacks within 5ft against paralyzed/unconscious

## Root Cause
`CheckAutoCrit` only gated on `distFt > 5` without considering weapon type. Any attack (including ranged) within 5ft against a paralyzed/unconscious target would auto-crit, violating RAW (only melee attacks auto-crit).

## Fix Applied

### `internal/combat/attack.go`
- Added `weapon refdata.Weapon` parameter to `CheckAutoCrit` signature.
- Added early return `false` when `IsRangedWeapon(weapon)` is true (before condition checks).
- Updated the call site in `buildAttackInput` to pass the `weapon` argument.

### `internal/combat/attack_test.go`
- Rewrote `TestCheckAutoCrit` table tests to include a `weapon` field.
- Changed existing "paralyzed ranged within 5ft" / "unconscious ranged within 5ft" cases from `expectCrit: true` → `expectCrit: false`.
- Added "thrown melee within 5ft paralyzed auto-crits" case to confirm thrown melee weapons (e.g., handaxe) still auto-crit correctly.
- Updated `TestCheckAutoCrit_BadJSON` to pass the new weapon parameter.

## Verification
- `make test` — PASS (all packages)
- `make cover-check` — PASS (all thresholds met)

## Notes
- Darts (`simple_ranged` + `thrown` property) are correctly excluded from auto-crit since `IsRangedWeapon` checks `WeaponType` suffix `_ranged`.
- Thrown melee weapons (handaxe, javelin — `simple_melee` + `thrown`) still auto-crit as expected since their `WeaponType` is `_melee`.
