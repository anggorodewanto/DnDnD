finding_id: C-C01
severity: Critical
title: Multi-letter column labels truncated by colToIndex
location: internal/combat/attack.go:1571-1577
spec_ref: spec §Grid Movement (line 285-300, "AA12 valid on maps wider than 26 columns")
problem: |
  `colToIndex` takes only `strings.ToUpper(col)[0]`, so "AA" → 0 (same as "A"). Used by combatantDistance, resolveAttackCover, detectHostileNear, and creatureCoverOccupants. On maps with > 26 columns every attacker/target in the AA+ block resolves to column 0 — distances, cover lines, OA reach checks all collapse onto column A.
suggested_fix: |
  Reuse renderer.ParseCoordinate (or extract the column-letter loop from ParseCoordinate into a shared helper). The renderer code already handles AA/AB/... correctly. Alternatively, implement the standard base-26 conversion: iterate all characters, accumulating `result = result*26 + (ch - 'A' + 1)`, then subtract 1 for 0-based index.
acceptance_criterion: |
  colToIndex("A") == 0, colToIndex("Z") == 25, colToIndex("AA") == 26, colToIndex("AB") == 27, colToIndex("AZ") == 51, colToIndex("BA") == 52. Existing tests still pass.
