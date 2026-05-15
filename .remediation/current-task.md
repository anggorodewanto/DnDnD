finding_id: E-C03
severity: Critical
title: Dodge condition does not impose disadvantage on attackers
location: internal/combat/advantage.go:104-134 (no "dodge" case in target conditions loop)
spec_ref: Phase 54 "Dodge"; spec §1138 ("attacks against the character have disadvantage")
problem: |
  The "dodge" condition is applied to the dodging combatant and is consulted by CheckSaveConditionEffects for DEX-save advantage, but DetectAdvantage never checks for it. Attacks against a Dodging target proceed at normal advantage.
suggested_fix: |
  Add case "dodge" to the target-conditions switch in DetectAdvantage, emitting disadvReasons = append(disadvReasons, "target dodging").
acceptance_criterion: |
  An attack against a target with the "dodge" condition results in disadvantage being reported by DetectAdvantage. A test demonstrates this.
