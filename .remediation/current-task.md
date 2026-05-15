finding_id: C-C03
severity: Critical
title: Off-hand (TWF) attack lacks Attack-action prerequisite and melee weapon check
location: internal/combat/attack.go:1147-1200
spec_ref: Phase 36; spec line 463 ("when a character attacks with a light melee weapon in their main hand... use bonus action to attack with a different light melee weapon")
problem: |
  OffhandAttack only validates ResourceBonusAction available, that both weapons exist and that both have the "light" property. It does NOT (a) verify the Attack action has been taken this turn (no AttacksRemaining < maxAttacks check), and (b) does not check the "melee" weapon type — a "light crossbow" or "dart" (light ranged) off-hand currently passes.
suggested_fix: |
  Reject the command if no attack has been made this turn (e.g., cmd.Turn.AttacksRemaining == initialMaxAttacks or check that ActionUsed is true). Also gate on !IsRangedWeapon(mainWeapon) && !IsRangedWeapon(offWeapon).
acceptance_criterion: |
  1. OffhandAttack fails with an error when no attack has been made this turn (AttacksRemaining == max).
  2. OffhandAttack fails when either weapon is ranged (e.g., light crossbow).
  3. OffhandAttack succeeds when both weapons are light melee and an attack has been made.
