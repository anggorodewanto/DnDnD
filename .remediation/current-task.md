finding_id: C-C02
severity: Critical
title: Reckless Attack advantage missing on attacks 2+
location: internal/combat/attack.go:887-901, internal/combat/advantage.go:36-39
spec_ref: Phase 38; spec line 217 ("advantage on melee STR attacks this turn")
problem: |
  Reckless Attack grants advantage to ALL melee STR attack rolls for the entire turn. The reckless gate rejects --reckless on any swing other than the first. The transient `reckless` condition applied to the attacker is only consulted on the target-side branch of DetectAdvantage (granting advantage to enemies attacking the reckless attacker). There is no attacker-side branch that re-applies "Reckless Attack" advantage on attack 2/3/4 of the same turn.
suggested_fix: |
  In DetectAdvantage, also check attackerConditions for "reckless" and, when present, add "Reckless Attack (active)" to advReasons for melee STR attacks. Keep the existing first-attack gate on the --reckless flag itself.
acceptance_criterion: |
  A barbarian with the "reckless" condition making a second melee STR attack in the same turn gets advantage from DetectAdvantage. A ranged attack or DEX-based attack does NOT get the advantage even with the reckless condition.
