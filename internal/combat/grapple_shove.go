package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/pathfinding"
	"github.com/ab/dndnd/internal/refdata"
)

// ShoveMode represents the shove variant.
type ShoveMode string

const (
	ShoveProne ShoveMode = "prone"
	ShovePush  ShoveMode = "push"
)

// --- Grapple ---

// GrappleCommand holds the inputs for a Grapple action.
type GrappleCommand struct {
	Grappler  refdata.Combatant
	Target    refdata.Combatant
	Turn      refdata.Turn
	Encounter refdata.Encounter
}

// GrappleResult holds the outputs of a Grapple action.
type GrappleResult struct {
	Turn         refdata.Turn
	Target       refdata.Combatant
	CombatLog    string
	Success      bool
	GrapplerRoll int
	TargetRoll   int
}

// Grapple handles the /action grapple command.
// Costs an action. Contested Athletics (STR) vs target's Athletics or Acrobatics.
// On success, target gains the grappled condition.
func (s *Service) Grapple(ctx context.Context, cmd GrappleCommand, roller *dice.Roller) (GrappleResult, error) {
	if ok, reason := CanActRaw(cmd.Grappler.Conditions); !ok {
		return GrappleResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return GrappleResult{}, err
	}

	// Free hand check: PCs need at least one hand free; NPCs assumed free
	if err := s.checkFreeHand(ctx, cmd.Grappler); err != nil {
		return GrappleResult{}, err
	}

	// Size check: target no more than 1 size larger
	targetSize, err := s.resolveCombatantSize(ctx, cmd.Target)
	if err != nil {
		return GrappleResult{}, err
	}
	grapplerSize, err := s.resolveCombatantSize(ctx, cmd.Grappler)
	if err != nil {
		return GrappleResult{}, err
	}
	if targetSize > grapplerSize+1 {
		return GrappleResult{}, fmt.Errorf("target is too large to grapple (size difference exceeds 1)")
	}

	// Adjacency check: must be within 5ft
	dist := GridDistanceFt(
		cmd.Grappler.PositionCol, int(cmd.Grappler.PositionRow),
		cmd.Target.PositionCol, int(cmd.Target.PositionRow),
	)
	if dist > 5 {
		return GrappleResult{}, fmt.Errorf("grapple requires being within 5ft of %s (currently %dft away)",
			cmd.Target.DisplayName, dist)
	}

	// Get ability modifiers
	grapplerStrMod, err := s.getAbilityMod(ctx, cmd.Grappler, "str")
	if err != nil {
		return GrappleResult{}, err
	}
	targetDef, err := s.resolveTargetDefense(ctx, cmd.Target)
	if err != nil {
		return GrappleResult{}, err
	}

	// Use action
	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return GrappleResult{}, err
	}

	// Contested rolls
	grapplerRoll, err := roller.RollD20(grapplerStrMod, dice.Normal)
	if err != nil {
		return GrappleResult{}, fmt.Errorf("rolling grapple check: %w", err)
	}
	targetRoll, err := roller.RollD20(targetDef.Mod, dice.Normal)
	if err != nil {
		return GrappleResult{}, fmt.Errorf("rolling target check: %w", err)
	}

	success := grapplerRoll.Total >= targetRoll.Total
	updatedTarget := cmd.Target

	if success {
		grappleCond := CombatCondition{
			Condition:         "grappled",
			DurationRounds:    0, // indefinite
			SourceCombatantID: cmd.Grappler.ID.String(),
		}
		newConds, err := AddCondition(cmd.Target.Conditions, grappleCond)
		if err != nil {
			return GrappleResult{}, fmt.Errorf("adding grappled condition: %w", err)
		}
		updatedTarget, err = s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
			ID:              cmd.Target.ID,
			Conditions:      newConds,
			ExhaustionLevel: cmd.Target.ExhaustionLevel,
		})
		if err != nil {
			return GrappleResult{}, fmt.Errorf("updating target conditions: %w", err)
		}
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return GrappleResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	var log string
	if success {
		log = fmt.Sprintf("\U0001f93c  %s grapples %s — \U0001f3b2 Athletics: %d vs %s: %d — Grappled!",
			cmd.Grappler.DisplayName, cmd.Target.DisplayName, grapplerRoll.Total, targetDef.Skill, targetRoll.Total)
	} else {
		log = fmt.Sprintf("\U0001f93c  %s attempts to grapple %s — \U0001f3b2 Athletics: %d vs %s: %d — Failed",
			cmd.Grappler.DisplayName, cmd.Target.DisplayName, grapplerRoll.Total, targetDef.Skill, targetRoll.Total)
	}

	return GrappleResult{
		Turn:         updatedTurn,
		Target:       updatedTarget,
		CombatLog:    log,
		Success:      success,
		GrapplerRoll: grapplerRoll.Total,
		TargetRoll:   targetRoll.Total,
	}, nil
}

// --- Shove ---

// ShoveCommand holds the inputs for a Shove action.
type ShoveCommand struct {
	Shover    refdata.Combatant
	Target    refdata.Combatant
	Turn      refdata.Turn
	Encounter refdata.Encounter
	Mode      ShoveMode // "prone" or "push"
}

// ShoveResult holds the outputs of a Shove action.
type ShoveResult struct {
	Turn       refdata.Turn
	Target     refdata.Combatant
	CombatLog  string
	Success    bool
	ShoverRoll int
	TargetRoll int
	PushDest   string // destination label for push mode
}

// Shove handles the /shove command.
// Costs an action. Contested Athletics (STR) vs target's Athletics or Acrobatics.
// On success: --prone knocks target prone, --push pushes 5ft away.
func (s *Service) Shove(ctx context.Context, cmd ShoveCommand, roller *dice.Roller) (ShoveResult, error) {
	if ok, reason := CanActRaw(cmd.Shover.Conditions); !ok {
		return ShoveResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return ShoveResult{}, err
	}

	// Size check
	targetSize, err := s.resolveCombatantSize(ctx, cmd.Target)
	if err != nil {
		return ShoveResult{}, err
	}
	shoverSize, err := s.resolveCombatantSize(ctx, cmd.Shover)
	if err != nil {
		return ShoveResult{}, err
	}
	if targetSize > shoverSize+1 {
		return ShoveResult{}, fmt.Errorf("target is too large to shove (size difference exceeds 1)")
	}

	// Adjacency check
	dist := GridDistanceFt(
		cmd.Shover.PositionCol, int(cmd.Shover.PositionRow),
		cmd.Target.PositionCol, int(cmd.Target.PositionRow),
	)
	if dist > 5 {
		return ShoveResult{}, fmt.Errorf("shove requires being within 5ft of %s (currently %dft away)",
			cmd.Target.DisplayName, dist)
	}

	// For push mode, validate destination is unoccupied BEFORE rolling
	var pushCol, pushRow int
	var pushLabel string
	if cmd.Mode == ShovePush {
		pushCol, pushRow = calculatePushDestination(
			cmd.Shover.PositionCol, int(cmd.Shover.PositionRow),
			cmd.Target.PositionCol, int(cmd.Target.PositionRow),
		)
		pushColLabel := colIntToLabel(pushCol)
		pushLabel = fmt.Sprintf("%s%d", pushColLabel, pushRow)

		// Check occupancy
		combatants, err := s.store.ListCombatantsByEncounterID(ctx, cmd.Encounter.ID)
		if err != nil {
			return ShoveResult{}, fmt.Errorf("listing combatants: %w", err)
		}
		for _, c := range combatants {
			if c.ID == cmd.Target.ID || !c.IsAlive {
				continue
			}
			if c.PositionCol == pushColLabel && int(c.PositionRow) == pushRow {
				return ShoveResult{}, fmt.Errorf("push destination %s is occupied by %s", pushLabel, c.DisplayName)
			}
		}
	}

	// Get ability modifiers
	shoverStrMod, err := s.getAbilityMod(ctx, cmd.Shover, "str")
	if err != nil {
		return ShoveResult{}, err
	}
	targetDef, err := s.resolveTargetDefense(ctx, cmd.Target)
	if err != nil {
		return ShoveResult{}, err
	}

	// Use action
	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return ShoveResult{}, err
	}

	// Contested rolls
	shoverRoll, err := roller.RollD20(shoverStrMod, dice.Normal)
	if err != nil {
		return ShoveResult{}, fmt.Errorf("rolling shove check: %w", err)
	}
	targetRoll, err := roller.RollD20(targetDef.Mod, dice.Normal)
	if err != nil {
		return ShoveResult{}, fmt.Errorf("rolling target check: %w", err)
	}

	success := shoverRoll.Total >= targetRoll.Total
	updatedTarget := cmd.Target
	modeStr := string(cmd.Mode)

	if success {
		switch cmd.Mode {
		case ShoveProne:
			proneCond := CombatCondition{
				Condition:      "prone",
				DurationRounds: 0,
			}
			newConds, err := AddCondition(cmd.Target.Conditions, proneCond)
			if err != nil {
				return ShoveResult{}, fmt.Errorf("adding prone condition: %w", err)
			}
			updatedTarget, err = s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
				ID:              cmd.Target.ID,
				Conditions:      newConds,
				ExhaustionLevel: cmd.Target.ExhaustionLevel,
			})
			if err != nil {
				return ShoveResult{}, fmt.Errorf("updating target conditions: %w", err)
			}

		case ShovePush:
			updatedTarget, err = s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
				ID:          cmd.Target.ID,
				PositionCol: colIntToLabel(pushCol),
				PositionRow: int32(pushRow),
				AltitudeFt:  cmd.Target.AltitudeFt,
			})
			if err != nil {
				return ShoveResult{}, fmt.Errorf("updating target position: %w", err)
			}
		}
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return ShoveResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	var log string
	if success {
		switch cmd.Mode {
		case ShoveProne:
			log = fmt.Sprintf("\U0001f4cc  %s shoves %s (%s) — \U0001f3b2 Athletics: %d vs %s: %d — Knocked prone!",
				cmd.Shover.DisplayName, cmd.Target.DisplayName, modeStr, shoverRoll.Total, targetDef.Skill, targetRoll.Total)
		case ShovePush:
			log = fmt.Sprintf("\U0001f4cc  %s shoves %s (%s) — \U0001f3b2 Athletics: %d vs %s: %d — Pushed to %s",
				cmd.Shover.DisplayName, cmd.Target.DisplayName, modeStr, shoverRoll.Total, targetDef.Skill, targetRoll.Total, pushLabel)
		}
	} else {
		log = fmt.Sprintf("\U0001f4cc  %s attempts to shove %s (%s) — \U0001f3b2 Athletics: %d vs %s: %d — Failed",
			cmd.Shover.DisplayName, cmd.Target.DisplayName, modeStr, shoverRoll.Total, targetDef.Skill, targetRoll.Total)
	}

	return ShoveResult{
		Turn:       updatedTurn,
		Target:     updatedTarget,
		CombatLog:  log,
		Success:    success,
		ShoverRoll: shoverRoll.Total,
		TargetRoll: targetRoll.Total,
		PushDest:   pushLabel,
	}, nil
}

// --- Dragging ---

// DragCheckResult holds info about grappled targets the mover is holding.
type DragCheckResult struct {
	HasTargets      bool
	GrappledTargets []refdata.Combatant
}

// CheckDragTargets detects if a combatant is grappling any other combatants.
func (s *Service) CheckDragTargets(ctx context.Context, encounterID uuid.UUID, mover refdata.Combatant) (DragCheckResult, error) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return DragCheckResult{}, fmt.Errorf("listing combatants: %w", err)
	}

	moverID := mover.ID.String()
	var grappled []refdata.Combatant
	for _, c := range combatants {
		if c.ID == mover.ID {
			continue
		}
		cond, found := GetCondition(c.Conditions, "grappled")
		if found && cond.SourceCombatantID == moverID {
			grappled = append(grappled, c)
		}
	}

	return DragCheckResult{
		HasTargets:      len(grappled) > 0,
		GrappledTargets: grappled,
	}, nil
}

// FormatDragPrompt produces the drag confirmation prompt listing all grappled creatures.
func FormatDragPrompt(targets []refdata.Combatant) string {
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.DisplayName
	}
	return fmt.Sprintf("\U0001f93c You are grappling: %s. Drag them with you? [\u2705 Drag] [\u274c Release & Move]",
		strings.Join(names, ", "))
}

// DragMovementCost returns the movement cost when dragging (always x2).
func DragMovementCost(baseCost int) int {
	return baseCost * 2
}

// ReleaseDragResult holds the outputs of releasing grappled targets.
type ReleaseDragResult struct {
	Released  []refdata.Combatant
	CombatLogs []string
}

// ReleaseDrag removes the grappled condition from all targets grappled by the mover.
func (s *Service) ReleaseDrag(ctx context.Context, mover refdata.Combatant, targets []refdata.Combatant) (ReleaseDragResult, error) {
	var released []refdata.Combatant
	var logs []string

	for _, t := range targets {
		newConds, err := RemoveCondition(t.Conditions, "grappled")
		if err != nil {
			return ReleaseDragResult{}, fmt.Errorf("removing grappled from %s: %w", t.DisplayName, err)
		}
		updated, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
			ID:              t.ID,
			Conditions:      newConds,
			ExhaustionLevel: t.ExhaustionLevel,
		})
		if err != nil {
			return ReleaseDragResult{}, fmt.Errorf("updating conditions for %s: %w", t.DisplayName, err)
		}
		released = append(released, updated)
		logs = append(logs, fmt.Sprintf("\U0001f93c  %s releases %s (grapple ended)",
			mover.DisplayName, t.DisplayName))
	}

	return ReleaseDragResult{
		Released:   released,
		CombatLogs: logs,
	}, nil
}

// --- Helpers ---

// targetDefenseResult holds the best defensive skill for a contested check target.
type targetDefenseResult struct {
	Mod   int
	Skill string // "Athletics" or "Acrobatics"
}

// resolveTargetDefense returns the higher of the target's Athletics (STR) or Acrobatics (DEX).
func (s *Service) resolveTargetDefense(ctx context.Context, target refdata.Combatant) (targetDefenseResult, error) {
	strMod, err := s.getAbilityMod(ctx, target, "str")
	if err != nil {
		return targetDefenseResult{}, err
	}
	dexMod, err := s.getAbilityMod(ctx, target, "dex")
	if err != nil {
		return targetDefenseResult{}, err
	}
	if dexMod > strMod {
		return targetDefenseResult{Mod: dexMod, Skill: "Acrobatics"}, nil
	}
	return targetDefenseResult{Mod: strMod, Skill: "Athletics"}, nil
}

// checkFreeHand validates that a combatant has a free hand for grappling.
// NPCs are assumed to have a free hand.
func (s *Service) checkFreeHand(ctx context.Context, combatant refdata.Combatant) error {
	if combatant.IsNpc || !combatant.CharacterID.Valid {
		return nil
	}
	char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
	if err != nil {
		return fmt.Errorf("getting character for free hand check: %w", err)
	}
	mainOccupied := char.EquippedMainHand.Valid && char.EquippedMainHand.String != ""
	offOccupied := char.EquippedOffHand.Valid && char.EquippedOffHand.String != ""
	if mainOccupied && offOccupied {
		return fmt.Errorf("grapple requires a free hand (both hands are occupied)")
	}
	return nil
}

// resolveCombatantSize returns the pathfinding size category for a combatant.
// PCs default to Medium; NPCs look up creature size.
func (s *Service) resolveCombatantSize(ctx context.Context, combatant refdata.Combatant) (int, error) {
	if combatant.CreatureRefID.Valid && combatant.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, combatant.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature for size: %w", err)
		}
		return pathfinding.ParseSizeCategory(creature.Size), nil
	}
	return pathfinding.ParseSizeCategory("Medium"), nil // PCs default to Medium
}

// calculatePushDestination calculates the tile 5ft away from the attacker through the target.
// Returns (col, row) as integers. The push direction is from attacker toward target.
func calculatePushDestination(attackerCol string, attackerRow int, targetCol string, targetRow int) (int, int) {
	ac := colToInt(attackerCol)
	tc := colToInt(targetCol)

	// Direction from attacker to target
	dc := sign(tc - ac)
	dr := sign(targetRow - attackerRow)

	return tc + dc, targetRow + dr
}

func sign(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

// colIntToLabel converts column int (1-based) to letter label.
func colIntToLabel(col int) string {
	col-- // convert to 0-based
	label := ""
	for {
		label = string(rune('A'+col%26)) + label
		col = col/26 - 1
		if col < 0 {
			break
		}
	}
	return label
}
