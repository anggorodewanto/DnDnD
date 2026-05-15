finding_id: B-C02
severity: Critical
title: cryptoRand / RollD20 panic on degenerate dice (Nd0)
location: internal/dice/roller.go:48-54, internal/dice/dice.go:23
spec_ref: phases §Phase 18
problem: |
  ParseExpression accepts "1d0", "5d0", etc. (regex `\d+d\d+` does not exclude zero). rollGroups then calls r.randFn(0), which is cryptoRand(0), which calls rand.Int(rand.Reader, big.NewInt(0)) — that panics with "crypto/rand.Int argument must be > 0". Any user-supplied `/roll 1d0` crashes the request goroutine.
suggested_fix: |
  Validate Count >= 1 && Sides >= 1 in ParseExpression (reject with an error). Also add a defensive guard in rollGroups before calling randFn.
acceptance_criterion: |
  ParseExpression("1d0") returns an error. ParseExpression("0d6") returns an error. ParseExpression("0d0") returns an error. No panic occurs for any of these inputs.
