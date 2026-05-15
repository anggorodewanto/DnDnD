finding_id: B-C01
severity: Critical
title: ParseExpression mangles modifiers with multiple +/- operators
location: internal/dice/dice.go:46-58
spec_ref: phases §Phase 18 ("parse dice expressions (NdM+K)", "modifier stacking")
problem: |
  The parser strips every dice group then strips every `+`, leaving the residue to Atoi. As a result:
  - "1d20+5+5" parses as modifier +55 (not +10).
  - "1d20-2+3" parses as modifier -23 (instead of +1).
  - "1d20-2-3" returns an error even though it is valid (meaning -5).
  - "1d4+1d6+2+3" would similarly collapse to +23.
suggested_fix: |
  Walk the post-dice residue token-by-token, summing each signed integer, or replace the regex hack with a proper expression tokenizer that accepts `\s*[+-]\s*\d+` repeatedly.
acceptance_criterion: |
  ParseExpression("1d20+5+5") returns Modifier=10. ParseExpression("1d20-2+3") returns Modifier=1. ParseExpression("1d20-2-3") returns Modifier=-5. ParseExpression("1d4+1d6+2+3") returns Modifier=5.
