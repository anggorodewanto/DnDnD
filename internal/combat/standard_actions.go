package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// --- Errors ---

var (
	ErrNotProne    = fmt.Errorf("not prone")
	ErrNotGrappled = fmt.Errorf("not grappled")
)

// =====================
// 1. DASH
// =====================

// DashCommand holds the inputs for a Dash action.
type DashCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	Encounter refdata.Encounter
}

// DashResult holds the outputs of a Dash action.
type DashResult struct {
	Turn          refdata.Turn
	CombatLog     string
	AddedMovement int32
}

// Dash handles the /action dash command.
// Costs an action. Adds the combatant's base speed to remaining movement.
func (s *Service) Dash(ctx context.Context, cmd DashCommand) (DashResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return DashResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return DashResult{}, err
	}

	speed, err := s.resolveBaseSpeed(ctx, cmd.Combatant)
	if err != nil {
		return DashResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return DashResult{}, err
	}
	updatedTurn.MovementRemainingFt += speed

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return DashResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	log := fmt.Sprintf("\U0001f3c3 %s uses Dash! (+%dft movement, %dft total remaining)",
		cmd.Combatant.DisplayName, speed, updatedTurn.MovementRemainingFt)

	return DashResult{
		Turn:          updatedTurn,
		CombatLog:     log,
		AddedMovement: speed,
	}, nil
}

// resolveBaseSpeed returns the base speed for a combatant.
// For PCs, looks up character speed. For NPCs, defaults to 30.
func (s *Service) resolveBaseSpeed(ctx context.Context, combatant refdata.Combatant) (int32, error) {
	if combatant.IsNpc || !combatant.CharacterID.Valid {
		return 30, nil
	}
	char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
	if err != nil {
		return 0, fmt.Errorf("getting character for speed: %w", err)
	}
	if char.SpeedFt <= 0 {
		return 30, nil
	}
	return char.SpeedFt, nil
}

// =====================
// 2. DISENGAGE
// =====================

// DisengageCommand holds the inputs for a Disengage action.
type DisengageCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
}

// DisengageResult holds the outputs of a Disengage action.
type DisengageResult struct {
	Turn      refdata.Turn
	CombatLog string
}

// Disengage handles the /action disengage command.
// Costs an action. Sets HasDisengaged = true.
func (s *Service) Disengage(ctx context.Context, cmd DisengageCommand) (DisengageResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return DisengageResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return DisengageResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return DisengageResult{}, err
	}
	updatedTurn.HasDisengaged = true

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return DisengageResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	log := fmt.Sprintf("\U0001f6ab %s uses Disengage (no opportunity attacks this turn)",
		cmd.Combatant.DisplayName)

	return DisengageResult{
		Turn:      updatedTurn,
		CombatLog: log,
	}, nil
}

// =====================
// 3. DODGE
// =====================

// DodgeCommand holds the inputs for a Dodge action.
type DodgeCommand struct {
	Combatant    refdata.Combatant
	Turn         refdata.Turn
	Encounter    refdata.Encounter
	CurrentRound int
}

// DodgeResult holds the outputs of a Dodge action.
type DodgeResult struct {
	Turn      refdata.Turn
	Combatant refdata.Combatant
	CombatLog string
}

// Dodge handles the /action dodge command.
// Costs an action. Applies the "dodge" condition (1 round, expires at start of actor's next turn).
func (s *Service) Dodge(ctx context.Context, cmd DodgeCommand) (DodgeResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return DodgeResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return DodgeResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return DodgeResult{}, err
	}

	dodgeCond := CombatCondition{
		Condition:         "dodge",
		DurationRounds:    1,
		StartedRound:      cmd.CurrentRound,
		SourceCombatantID: cmd.Combatant.ID.String(),
		ExpiresOn:         "start_of_turn",
	}

	newConds, err := AddCondition(cmd.Combatant.Conditions, dodgeCond)
	if err != nil {
		return DodgeResult{}, fmt.Errorf("adding dodge condition: %w", err)
	}

	updatedCombatant, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              cmd.Combatant.ID,
		Conditions:      newConds,
		ExhaustionLevel: cmd.Combatant.ExhaustionLevel,
	})
	if err != nil {
		return DodgeResult{}, fmt.Errorf("updating combatant conditions: %w", err)
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return DodgeResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	log := fmt.Sprintf("\U0001f6e1\ufe0f %s uses Dodge (disadvantage on attacks against, advantage on DEX saves)",
		cmd.Combatant.DisplayName)

	return DodgeResult{
		Turn:      updatedTurn,
		Combatant: updatedCombatant,
		CombatLog: log,
	}, nil
}

// =====================
// 4. HELP
// =====================

// HelpCommand holds the inputs for a Help action.
type HelpCommand struct {
	Helper    refdata.Combatant
	Ally      refdata.Combatant
	Target    refdata.Combatant
	Turn      refdata.Turn
	Encounter refdata.Encounter
}

// HelpResult holds the outputs of a Help action.
type HelpResult struct {
	Turn      refdata.Turn
	Ally      refdata.Combatant
	CombatLog string
}

// Help handles the /action help command.
// Costs an action. Grants the ally advantage on their next attack against the target.
// Requires helper to be within 5ft of the target.
func (s *Service) Help(ctx context.Context, cmd HelpCommand) (HelpResult, error) {
	if ok, reason := CanActRaw(cmd.Helper.Conditions); !ok {
		return HelpResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return HelpResult{}, err
	}

	// Adjacency check: helper must be within 5ft of target
	dist := GridDistanceFt(
		cmd.Helper.PositionCol, int(cmd.Helper.PositionRow),
		cmd.Target.PositionCol, int(cmd.Target.PositionRow),
	)
	if dist > 5 {
		return HelpResult{}, fmt.Errorf("Help requires being within 5ft of %s (currently %dft away)",
			cmd.Target.DisplayName, dist)
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return HelpResult{}, err
	}

	// Apply help_advantage condition on the ally
	helpCond := CombatCondition{
		Condition:         "help_advantage",
		DurationRounds:    1,
		StartedRound:      int(cmd.Encounter.RoundNumber),
		SourceCombatantID: cmd.Helper.ID.String(),
		ExpiresOn:         "start_of_turn",
	}

	newConds, err := AddCondition(cmd.Ally.Conditions, helpCond)
	if err != nil {
		return HelpResult{}, fmt.Errorf("adding help condition: %w", err)
	}

	updatedAlly, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              cmd.Ally.ID,
		Conditions:      newConds,
		ExhaustionLevel: cmd.Ally.ExhaustionLevel,
	})
	if err != nil {
		return HelpResult{}, fmt.Errorf("updating ally conditions: %w", err)
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return HelpResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	log := fmt.Sprintf("\U0001f91d %s uses Help! %s gains advantage on next attack against %s",
		cmd.Helper.DisplayName, cmd.Ally.DisplayName, cmd.Target.DisplayName)

	return HelpResult{
		Turn:      updatedTurn,
		Ally:      updatedAlly,
		CombatLog: log,
	}, nil
}

// =====================
// 5. HIDE
// =====================

// HideCommand holds the inputs for a Hide action.
type HideCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	Encounter refdata.Encounter
	Hostiles  []refdata.Combatant
}

// HideResult holds the outputs of a Hide action.
type HideResult struct {
	Turn             refdata.Turn
	Combatant        refdata.Combatant
	CombatLog        string
	Success          bool
	StealthRoll      int
	HighestPerception int
}

// Hide handles the /action hide command.
// Costs an action. Stealth check vs highest passive Perception among hostiles.
// On success, sets IsVisible = false.
func (s *Service) Hide(ctx context.Context, cmd HideCommand, roller *dice.Roller) (HideResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return HideResult{}, fmt.Errorf("%s", reason)
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return HideResult{}, err
	}

	// Get DEX modifier for stealth
	dexMod, err := s.getAbilityMod(ctx, cmd.Combatant, "dex")
	if err != nil {
		return HideResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return HideResult{}, err
	}

	// Roll stealth
	stealthResult, err := roller.RollD20(dexMod, dice.Normal)
	if err != nil {
		return HideResult{}, fmt.Errorf("rolling stealth: %w", err)
	}
	stealthTotal := stealthResult.Total

	// Calculate highest passive Perception among hostiles
	highestPP := 0
	for _, h := range cmd.Hostiles {
		pp, err := s.passivePerception(ctx, h)
		if err != nil {
			return HideResult{}, err
		}
		if pp > highestPP {
			highestPP = pp
		}
	}

	success := stealthTotal >= highestPP
	updatedCombatant := cmd.Combatant
	if success {
		updatedCombatant.IsVisible = false
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return HideResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	var log string
	if success {
		log = fmt.Sprintf("\U0001f575\ufe0f %s hides successfully! (Stealth %d vs Perception %d)",
			cmd.Combatant.DisplayName, stealthTotal, highestPP)
	} else {
		log = fmt.Sprintf("\U0001f575\ufe0f %s fails to hide (Stealth %d vs Perception %d)",
			cmd.Combatant.DisplayName, stealthTotal, highestPP)
	}

	return HideResult{
		Turn:              updatedTurn,
		Combatant:         updatedCombatant,
		CombatLog:         log,
		Success:           success,
		StealthRoll:       stealthTotal,
		HighestPerception: highestPP,
	}, nil
}

// getAbilityMod returns an ability modifier for a combatant.
func (s *Service) getAbilityMod(ctx context.Context, combatant refdata.Combatant, ability string) (int, error) {
	if combatant.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
		if err != nil {
			return 0, fmt.Errorf("getting character for ability: %w", err)
		}
		scores, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing ability scores: %w", err)
		}
		return abilityModFromScores(scores, ability), nil
	}
	if combatant.CreatureRefID.Valid && combatant.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, combatant.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature for ability: %w", err)
		}
		scores, err := ParseAbilityScores(creature.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing creature ability scores: %w", err)
		}
		return abilityModFromScores(scores, ability), nil
	}
	return 0, nil
}

// abilityModFromScores returns the modifier for the named ability.
func abilityModFromScores(scores AbilityScores, ability string) int {
	switch ability {
	case "str":
		return AbilityModifier(scores.Str)
	case "dex":
		return AbilityModifier(scores.Dex)
	case "con":
		return AbilityModifier(scores.Con)
	case "int":
		return AbilityModifier(scores.Int)
	case "wis":
		return AbilityModifier(scores.Wis)
	case "cha":
		return AbilityModifier(scores.Cha)
	default:
		return 0
	}
}

// passivePerception returns 10 + WIS modifier for a combatant.
func (s *Service) passivePerception(ctx context.Context, combatant refdata.Combatant) (int, error) {
	wisMod, err := s.getAbilityMod(ctx, combatant, "wis")
	if err != nil {
		return 0, err
	}
	return 10 + wisMod, nil
}

// =====================
// 6. STAND
// =====================

// StandCommand holds the inputs for standing from prone.
type StandCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	MaxSpeed  int
}

// StandResult holds the outputs of standing from prone.
type StandResult struct {
	Turn         refdata.Turn
	Combatant    refdata.Combatant
	CombatLog    string
	MovementCost int
}

// Stand handles the /action stand command.
// Does NOT cost an action. Costs half movement. Removes prone condition.
func (s *Service) Stand(ctx context.Context, cmd StandCommand) (StandResult, error) {
	if !HasCondition(cmd.Combatant.Conditions, "prone") {
		return StandResult{}, ErrNotProne
	}

	cost := StandFromProneCost(cmd.MaxSpeed)
	if int32(cost) > cmd.Turn.MovementRemainingFt {
		return StandResult{}, fmt.Errorf("not enough movement to stand: need %dft, have %dft",
			cost, cmd.Turn.MovementRemainingFt)
	}

	updatedTurn := cmd.Turn
	updatedTurn.MovementRemainingFt -= int32(cost)
	updatedTurn.HasStoodThisTurn = true

	newConds, err := RemoveCondition(cmd.Combatant.Conditions, "prone")
	if err != nil {
		return StandResult{}, fmt.Errorf("removing prone condition: %w", err)
	}

	updatedCombatant, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              cmd.Combatant.ID,
		Conditions:      newConds,
		ExhaustionLevel: cmd.Combatant.ExhaustionLevel,
	})
	if err != nil {
		return StandResult{}, fmt.Errorf("updating combatant conditions: %w", err)
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return StandResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	log := fmt.Sprintf("\U0001f9cd %s stands up (-%dft movement)",
		cmd.Combatant.DisplayName, cost)

	return StandResult{
		Turn:         updatedTurn,
		Combatant:    updatedCombatant,
		CombatLog:    log,
		MovementCost: cost,
	}, nil
}

// =====================
// 7. DROP PRONE
// =====================

// DropProneCommand holds the inputs for dropping prone.
type DropProneCommand struct {
	Combatant    refdata.Combatant
	Turn         refdata.Turn
	Encounter    refdata.Encounter
	CurrentRound int
}

// DropProneResult holds the outputs of dropping prone.
type DropProneResult struct {
	Combatant refdata.Combatant
	CombatLog string
}

// DropProne handles the /action drop-prone command.
// No resource cost. Applies prone condition.
func (s *Service) DropProne(ctx context.Context, cmd DropProneCommand) (DropProneResult, error) {
	if HasCondition(cmd.Combatant.Conditions, "prone") {
		return DropProneResult{}, fmt.Errorf("already prone")
	}

	proneCond := CombatCondition{
		Condition:      "prone",
		DurationRounds: 0, // indefinite - removed by Stand
	}

	newConds, err := AddCondition(cmd.Combatant.Conditions, proneCond)
	if err != nil {
		return DropProneResult{}, fmt.Errorf("adding prone condition: %w", err)
	}

	updatedCombatant, err := s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
		ID:              cmd.Combatant.ID,
		Conditions:      newConds,
		ExhaustionLevel: cmd.Combatant.ExhaustionLevel,
	})
	if err != nil {
		return DropProneResult{}, fmt.Errorf("updating combatant conditions: %w", err)
	}

	log := fmt.Sprintf("\u2B07\ufe0f %s drops prone", cmd.Combatant.DisplayName)

	return DropProneResult{
		Combatant: updatedCombatant,
		CombatLog: log,
	}, nil
}

// =====================
// 8. ESCAPE
// =====================

// EscapeCommand holds the inputs for an Escape action (break grapple).
type EscapeCommand struct {
	Escapee       refdata.Combatant
	Grappler      refdata.Combatant
	Turn          refdata.Turn
	Encounter     refdata.Encounter
	UseAcrobatics bool
}

// EscapeResult holds the outputs of an Escape action.
type EscapeResult struct {
	Turn         refdata.Turn
	Escapee      refdata.Combatant
	CombatLog    string
	Success      bool
	EscapeeRoll  int
	GrapplerRoll int
}

// Escape handles the /action escape command.
// Costs an action. Contested check: escapee's Athletics or Acrobatics vs grappler's Athletics.
// On success, removes grappled condition.
func (s *Service) Escape(ctx context.Context, cmd EscapeCommand, roller *dice.Roller) (EscapeResult, error) {
	if ok, reason := CanActRaw(cmd.Escapee.Conditions); !ok {
		return EscapeResult{}, fmt.Errorf("%s", reason)
	}
	if !HasCondition(cmd.Escapee.Conditions, "grappled") {
		return EscapeResult{}, ErrNotGrappled
	}
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return EscapeResult{}, err
	}

	// Determine escapee's ability
	escAbility := "str" // Athletics
	if cmd.UseAcrobatics {
		escAbility = "dex" // Acrobatics
	}
	escMod, err := s.getAbilityMod(ctx, cmd.Escapee, escAbility)
	if err != nil {
		return EscapeResult{}, err
	}

	// Grappler's Athletics (STR)
	grapMod, err := s.getAbilityMod(ctx, cmd.Grappler, "str")
	if err != nil {
		return EscapeResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return EscapeResult{}, err
	}

	// Contested rolls
	escRoll, err := roller.RollD20(escMod, dice.Normal)
	if err != nil {
		return EscapeResult{}, fmt.Errorf("rolling escape check: %w", err)
	}
	grapRoll, err := roller.RollD20(grapMod, dice.Normal)
	if err != nil {
		return EscapeResult{}, fmt.Errorf("rolling grappler check: %w", err)
	}

	success := escRoll.Total >= grapRoll.Total
	updatedEscapee := cmd.Escapee

	if success {
		newConds, err := RemoveCondition(cmd.Escapee.Conditions, "grappled")
		if err != nil {
			return EscapeResult{}, fmt.Errorf("removing grappled condition: %w", err)
		}
		updatedEscapee, err = s.store.UpdateCombatantConditions(ctx, refdata.UpdateCombatantConditionsParams{
			ID:              cmd.Escapee.ID,
			Conditions:      newConds,
			ExhaustionLevel: cmd.Escapee.ExhaustionLevel,
		})
		if err != nil {
			return EscapeResult{}, fmt.Errorf("updating escapee conditions: %w", err)
		}
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return EscapeResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	skillName := "Athletics"
	if cmd.UseAcrobatics {
		skillName = "Acrobatics"
	}

	var log string
	if success {
		log = fmt.Sprintf("\U0001f4aa %s escapes %s's grapple! (%s %d vs Athletics %d)",
			cmd.Escapee.DisplayName, cmd.Grappler.DisplayName, skillName, escRoll.Total, grapRoll.Total)
	} else {
		log = fmt.Sprintf("\U0001f4aa %s fails to escape %s's grapple (%s %d vs Athletics %d)",
			cmd.Escapee.DisplayName, cmd.Grappler.DisplayName, skillName, escRoll.Total, grapRoll.Total)
	}

	return EscapeResult{
		Turn:         updatedTurn,
		Escapee:      updatedEscapee,
		CombatLog:    log,
		Success:      success,
		EscapeeRoll:  escRoll.Total,
		GrapplerRoll: grapRoll.Total,
	}, nil
}

// =====================
// 9. CUNNING ACTION
// =====================

// CunningActionCommand holds the inputs for a Rogue Cunning Action.
type CunningActionCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
	Encounter refdata.Encounter
	Action    string // "dash" or "disengage"
}

// CunningActionResult holds the outputs of a Cunning Action.
type CunningActionResult struct {
	Turn          refdata.Turn
	CombatLog     string
	AddedMovement int32 // only for dash
}

// CunningAction handles /bonus cunning-action dash|disengage.
// Costs a bonus action instead of an action. Requires Rogue level 2+.
func (s *Service) CunningAction(ctx context.Context, cmd CunningActionCommand) (CunningActionResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return CunningActionResult{}, fmt.Errorf("%s", reason)
	}
	if cmd.Action != "dash" && cmd.Action != "disengage" {
		return CunningActionResult{}, fmt.Errorf("cunning action must be 'dash' or 'disengage', got %q", cmd.Action)
	}
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return CunningActionResult{}, err
	}

	if !cmd.Combatant.CharacterID.Valid {
		return CunningActionResult{}, fmt.Errorf("Cunning Action requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Combatant.CharacterID.UUID)
	if err != nil {
		return CunningActionResult{}, fmt.Errorf("getting character: %w", err)
	}

	rogueLevel := ClassLevelFromJSON(char.Classes, "Rogue")
	if rogueLevel < 2 {
		return CunningActionResult{}, fmt.Errorf("Cunning Action requires Rogue level 2+")
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return CunningActionResult{}, err
	}

	var addedMovement int32
	var actionLabel string

	switch cmd.Action {
	case "dash":
		speed, err := s.resolveBaseSpeed(ctx, cmd.Combatant)
		if err != nil {
			return CunningActionResult{}, err
		}
		updatedTurn.MovementRemainingFt += speed
		addedMovement = speed
		actionLabel = "Dash"
	case "disengage":
		updatedTurn.HasDisengaged = true
		actionLabel = "Disengage"
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return CunningActionResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	var log string
	if cmd.Action == "dash" {
		log = fmt.Sprintf("\u26a1 %s uses Cunning Action: %s! (+%dft movement, bonus action)",
			cmd.Combatant.DisplayName, actionLabel, addedMovement)
	} else {
		log = fmt.Sprintf("\u26a1 %s uses Cunning Action: %s (no opportunity attacks, bonus action)",
			cmd.Combatant.DisplayName, actionLabel)
	}

	return CunningActionResult{
		Turn:          updatedTurn,
		CombatLog:     log,
		AddedMovement: addedMovement,
	}, nil
}

// GridDistanceFt converts grid column letters and rows to Chebyshev distance in feet.
// Columns are letters (A=1, B=2, etc.), rows are integers.
func GridDistanceFt(col1 string, row1 int, col2 string, row2 int) int {
	c1 := colToInt(col1)
	c2 := colToInt(col2)
	return gridDistance(c1, row1, c2, row2) * 5
}

// colToInt converts a column letter(s) to an integer (A=1, B=2, ..., Z=26, AA=27, etc.).
func colToInt(col string) int {
	result := 0
	for _, ch := range col {
		result = result*26 + int(ch-'A') + 1
	}
	return result
}
