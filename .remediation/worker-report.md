# Worker Report: D-H04 — Monk Unarmored Defense not invalidated by shield

## Status: ✅ FIXED

## Summary

Monk's Unarmored Defense (AC = 10 + DEX + WIS) requires no armor AND no shield per PHB. The code was unconditionally adding +2 for shield in both `CalculateAC` (internal/character/stats.go) and `RecalculateAC` (internal/combat/equip.go).

## Changes

### 1. internal/character/stats.go (line ~63)
```go
// Before:
if hasShield {
    ac += 2
}

// After:
if hasShield && !strings.Contains(acFormula, "WIS") {
    ac += 2
}
```

### 2. internal/combat/equip.go (line ~415)
```go
// Before:
if hasShield {
    ac += 2
}

// After:
if hasShield && !(char.AcFormula.Valid && strings.Contains(char.AcFormula.String, "WIS")) {
    ac += 2
}
```

### 3. internal/character/stats_test.go
Updated `TestCalculateAC_UnarmoredDefense_WithShield` to expect AC 15 (no shield bonus) instead of 17.

### 4. internal/combat/equip_test.go
Updated `TestRecalculateAC_WithShield`, `TestEquip_Shield_WithACFormula`, and `TestEquip_DoffShield_WithACFormula` to reflect correct monk behavior (shield bonus skipped for WIS formula).

## Verification

- `make test` — all packages pass
- `make cover-check` — all coverage thresholds met ("OK: coverage thresholds met")
