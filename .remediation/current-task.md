finding_id: C-H05
severity: High
title: Fall damage missing 20d6 cap
location: internal/combat/altitude.go:101-123 (FallDamage)
spec_ref: Phase 31; PHB p183
problem: |
  numDice := int(altitudeFt) / 10 with no cap. A 500ft fall rolls 50d6.
suggested_fix: |
  if numDice > 20 { numDice = 20 } before building the expression.
acceptance_criterion: |
  FallDamage for 500ft returns a 20d6 expression, not 50d6. A test demonstrates the cap.
