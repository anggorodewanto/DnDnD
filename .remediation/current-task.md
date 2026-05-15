finding_id: D-H01
severity: High
title: Step of the Wind dash adds remaining movement, not base speed
location: internal/combat/monk.go:444
spec_ref: Phase 48b; PHB Monk "Step of the Wind"
problem: |
  case "dash": updatedTurn.MovementRemainingFt += cmd.Turn.MovementRemainingFt adds whatever is currently left, not the monk's speed. A monk who already moved half their speed gets only half the dash bonus.
suggested_fix: |
  Replace with speed, _ := s.resolveBaseSpeed(ctx, cmd.Combatant); updatedTurn.MovementRemainingFt += speed.
acceptance_criterion: |
  Step of the Wind dash adds the monk's base speed regardless of how much movement was already used. A test demonstrates this.
