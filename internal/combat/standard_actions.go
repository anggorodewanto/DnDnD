package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/character"
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

	// Apply help_advantage condition on the ally. SR-018: TargetCombatantID
	// scopes the grant to the named target — DetectAdvantage only fires the
	// "help advantage" reason when the attack is against this combatant, and
	// Service.Attack clears the condition after a single attack vs that target.
	helpCond := CombatCondition{
		Condition:         "help_advantage",
		DurationRounds:    1,
		StartedRound:      int(cmd.Encounter.RoundNumber),
		SourceCombatantID: cmd.Helper.ID.String(),
		TargetCombatantID: cmd.Target.ID.String(),
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

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return HideResult{}, err
	}

	logHeader := fmt.Sprintf("\U0001f648 %s attempts to Hide", cmd.Combatant.DisplayName)
	return s.resolveHide(ctx, cmd, updatedTurn, logHeader, roller)
}

// resolveHide performs the core hide logic (stealth check vs passive perception).
// Accepts an already-consumed turn and a log header (e.g. "🙈 Rogue attempts to Hide")
// so it can be reused by both Hide (action) and CunningAction hide (bonus action).
func (s *Service) resolveHide(ctx context.Context, cmd HideCommand, updatedTurn refdata.Turn, logHeader string, roller *dice.Roller) (HideResult, error) {
	stealthMod, rollMode, err := s.stealthModAndMode(ctx, cmd.Combatant)
	if err != nil {
		return HideResult{}, err
	}

	stealthResult, err := roller.RollD20(stealthMod, rollMode)
	if err != nil {
		return HideResult{}, fmt.Errorf("rolling stealth: %w", err)
	}
	stealthTotal := stealthResult.Total

	highestPP := 0
	var spottedBy string
	for _, h := range cmd.Hostiles {
		pp, err := s.passivePerception(ctx, h)
		if err != nil {
			return HideResult{}, err
		}
		if pp > highestPP {
			highestPP = pp
			spottedBy = h.DisplayName
		}
	}

	success := stealthTotal > highestPP
	updatedCombatant := cmd.Combatant
	if success {
		updatedCombatant, err = s.store.UpdateCombatantVisibility(ctx, refdata.UpdateCombatantVisibilityParams{
			ID:        cmd.Combatant.ID,
			IsVisible: false,
		})
		if err != nil {
			return HideResult{}, fmt.Errorf("persisting hide visibility: %w", err)
		}
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return HideResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	var log string
	if success {
		log = fmt.Sprintf("%s — \U0001f3b2 Stealth: %d — Hidden from all hostiles",
			logHeader, stealthTotal)
	} else {
		log = fmt.Sprintf("%s — \U0001f3b2 Stealth: %d — Failed (spotted by %s)",
			logHeader, stealthTotal, spottedBy)
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

// stealthModAndMode calculates the stealth modifier and roll mode for a combatant.
// For characters: uses full skill modifier and checks for armor stealth disadvantage.
// For creatures: uses pre-calculated stealth skill if available, else DEX mod.
func (s *Service) stealthModAndMode(ctx context.Context, combatant refdata.Combatant) (int, dice.RollMode, error) {
	if combatant.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
		if err != nil {
			return 0, dice.Normal, fmt.Errorf("getting character for ability: %w", err)
		}
		scores, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return 0, dice.Normal, fmt.Errorf("parsing ability scores: %w", err)
		}
		profSkills, expertiseSkills, jackOfAllTrades := parseProficiencies(char.Proficiencies.RawMessage)
		charScores := character.AbilityScores{
			STR: scores.Str, DEX: scores.Dex, CON: scores.Con,
			INT: scores.Int, WIS: scores.Wis, CHA: scores.Cha,
		}
		mod := character.SkillModifier(charScores, "stealth", profSkills, expertiseSkills, jackOfAllTrades, int(char.ProficiencyBonus))

		// Check armor stealth disadvantage
		rollMode := dice.Normal
		if char.EquippedArmor.Valid && char.EquippedArmor.String != "" {
			armor, err := s.store.GetArmor(ctx, char.EquippedArmor.String)
			if err == nil && armor.StealthDisadv.Valid && armor.StealthDisadv.Bool {
				// Check for Medium Armor Master feat negating stealth disadvantage
				if !hasFeatureEffect(char.Features, "no_stealth_disadvantage_medium_armor") || armor.ArmorType != "medium" {
					rollMode = dice.Disadvantage
				}
			}
		}
		return mod, rollMode, nil
	}

	if combatant.CreatureRefID.Valid && combatant.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, combatant.CreatureRefID.String)
		if err != nil {
			return 0, dice.Normal, fmt.Errorf("getting creature for ability: %w", err)
		}
		if mod, ok := creatureSkillMod(creature.Skills.RawMessage, "stealth"); ok {
			return mod, dice.Normal, nil
		}
		scores, err := ParseAbilityScores(creature.AbilityScores)
		if err != nil {
			return 0, dice.Normal, fmt.Errorf("parsing creature ability scores: %w", err)
		}
		return AbilityModifier(scores.Dex), dice.Normal, nil
	}

	return 0, dice.Normal, nil
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

// passivePerception returns 10 + Perception modifier (including proficiency) for a combatant.
func (s *Service) passivePerception(ctx context.Context, combatant refdata.Combatant) (int, error) {
	// Character: use full skill modifier (proficiency, expertise, jack of all trades)
	if combatant.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
		if err != nil {
			return 0, fmt.Errorf("getting character for perception: %w", err)
		}
		scores, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing ability scores: %w", err)
		}
		profSkills, expertiseSkills, jackOfAllTrades := parseProficiencies(char.Proficiencies.RawMessage)
		charScores := character.AbilityScores{
			STR: scores.Str, DEX: scores.Dex, CON: scores.Con,
			INT: scores.Int, WIS: scores.Wis, CHA: scores.Cha,
		}
		mod := character.SkillModifier(charScores, "perception", profSkills, expertiseSkills, jackOfAllTrades, int(char.ProficiencyBonus))
		return 10 + mod, nil
	}

	// Creature: use pre-calculated skill value if available, else fallback to WIS mod
	if combatant.CreatureRefID.Valid && combatant.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, combatant.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature for ability: %w", err)
		}
		if mod, ok := creatureSkillMod(creature.Skills.RawMessage, "perception"); ok {
			return 10 + mod, nil
		}
		scores, err := ParseAbilityScores(creature.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing creature ability scores: %w", err)
		}
		return 10 + AbilityModifier(scores.Wis), nil
	}

	// Fallback: no character or creature data
	return 10, nil
}

// parseProficiencies extracts skill proficiency data from the proficiencies JSON column.
func parseProficiencies(raw json.RawMessage) (skills []string, expertise []string, jackOfAllTrades bool) {
	if len(raw) == 0 {
		return nil, nil, false
	}
	var data struct {
		Skills          []string `json:"skills"`
		Expertise       []string `json:"expertise"`
		JackOfAllTrades bool     `json:"jack_of_all_trades"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, nil, false
	}
	return data.Skills, data.Expertise, data.JackOfAllTrades
}

// creatureSkillMod reads a pre-calculated skill modifier from a creature's Skills JSON.
// Returns the modifier and true if found, or 0 and false if not present.
func creatureSkillMod(skills []byte, skill string) (int, bool) {
	if len(skills) == 0 {
		return 0, false
	}
	var m map[string]int
	if err := json.Unmarshal(skills, &m); err != nil {
		return 0, false
	}
	v, ok := m[skill]
	return v, ok
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

	updatedCombatant, msgs, err := s.ApplyCondition(ctx, cmd.Combatant.ID, proneCond)
	if err != nil {
		return DropProneResult{}, fmt.Errorf("applying prone condition: %w", err)
	}

	log := fmt.Sprintf("\u2B07\ufe0f %s drops prone", cmd.Combatant.DisplayName)
	for _, m := range msgs {
		log += "\n" + m
	}

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
	UseAthletics  bool
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
	} else if !cmd.UseAthletics {
		// Auto-pick: use whichever mod is higher (spec default)
		strMod, err := s.getAbilityMod(ctx, cmd.Escapee, "str")
		if err != nil {
			return EscapeResult{}, err
		}
		dexMod, err := s.getAbilityMod(ctx, cmd.Escapee, "dex")
		if err != nil {
			return EscapeResult{}, err
		}
		if dexMod > strMod {
			escAbility = "dex"
		}
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
	if escAbility == "dex" {
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
	Action    string // "dash", "disengage", or "hide"
	Hostiles  []refdata.Combatant // only for hide
}

// CunningActionResult holds the outputs of a Cunning Action.
type CunningActionResult struct {
	Turn          refdata.Turn
	CombatLog     string
	AddedMovement int32       // only for dash
	HideResult    *HideResult // only for hide
}

// CunningAction handles /bonus cunning-action dash|disengage|hide.
// Costs a bonus action instead of an action. Requires Rogue level 2+.
func (s *Service) CunningAction(ctx context.Context, cmd CunningActionCommand, roller ...*dice.Roller) (CunningActionResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return CunningActionResult{}, fmt.Errorf("%s", reason)
	}
	if cmd.Action != "dash" && cmd.Action != "disengage" && cmd.Action != "hide" {
		return CunningActionResult{}, fmt.Errorf("cunning action must be 'dash', 'disengage', or 'hide', got %q", cmd.Action)
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

	// For hide, delegate to resolveHide using the bonus action resource
	if cmd.Action == "hide" {
		if len(roller) == 0 {
			return CunningActionResult{}, fmt.Errorf("roller required for hide")
		}
		updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
		if err != nil {
			return CunningActionResult{}, err
		}

		hideCmd := HideCommand{
			Combatant: cmd.Combatant,
			Turn:      cmd.Turn,
			Encounter: cmd.Encounter,
			Hostiles:  cmd.Hostiles,
		}
		logHeader := fmt.Sprintf("\u26a1 %s uses Cunning Action: Hide", cmd.Combatant.DisplayName)
		hr, err := s.resolveHide(ctx, hideCmd, updatedTurn, logHeader, roller[0])
		if err != nil {
			return CunningActionResult{}, err
		}

		return CunningActionResult{
			Turn:       hr.Turn,
			CombatLog:  hr.CombatLog,
			HideResult: &hr,
		}, nil
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
