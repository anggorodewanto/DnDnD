package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/refdata"
)

// ActionSurgeCommand holds the inputs for an Action Surge command.
type ActionSurgeCommand struct {
	Fighter refdata.Combatant
	Turn    refdata.Turn
}

// ActionSurgeResult holds the output of an Action Surge command.
type ActionSurgeResult struct {
	Turn          refdata.Turn
	CombatLog     string
	UsesRemaining int
}

// ActionSurge handles the /action surge command.
// Validates fighter class, surge availability, and resets action/attacks.
func (s *Service) ActionSurge(ctx context.Context, cmd ActionSurgeCommand) (ActionSurgeResult, error) {
	// Must not have already surged this turn
	if cmd.Turn.ActionSurged {
		return ActionSurgeResult{}, fmt.Errorf("Action Surge already used this turn")
	}

	// Must be a character (not NPC)
	if !cmd.Fighter.CharacterID.Valid {
		return ActionSurgeResult{}, fmt.Errorf("Action Surge requires a character (not NPC)")
	}

	// Get character data
	char, err := s.store.GetCharacter(ctx, cmd.Fighter.CharacterID.UUID)
	if err != nil {
		return ActionSurgeResult{}, fmt.Errorf("getting character: %w", err)
	}

	// Validate fighter class (level 2+)
	fighterLevel := ClassLevelFromJSON(char.Classes, "Fighter")
	if fighterLevel < 2 {
		return ActionSurgeResult{}, fmt.Errorf("Action Surge requires Fighter level 2+")
	}

	// Parse feature uses and check remaining
	featureUses, remaining, err := ParseFeatureUses(char, FeatureKeyActionSurge)
	if err != nil {
		return ActionSurgeResult{}, err
	}
	if remaining <= 0 {
		return ActionSurgeResult{}, fmt.Errorf("no Action Surge uses remaining")
	}

	// Deduct one use
	newRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyActionSurge, featureUses, remaining)
	if err != nil {
		return ActionSurgeResult{}, err
	}

	// Determine attacks per action for the fighter
	attacksPerAction := s.resolveAttacksPerAction(ctx, char)

	// Reset action and attacks
	updatedTurn := cmd.Turn
	updatedTurn.ActionUsed = false
	updatedTurn.AttacksRemaining = int32(attacksPerAction)
	updatedTurn.ActionSurged = true

	// Persist turn state
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return ActionSurgeResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	combatLog := fmt.Sprintf("\u26a1  %s uses Action Surge! (%d use(s) remaining)",
		cmd.Fighter.DisplayName, newRemaining)

	return ActionSurgeResult{
		Turn:          updatedTurn,
		CombatLog:     combatLog,
		UsesRemaining: newRemaining,
	}, nil
}

// resolveAttacksPerAction determines the number of attacks per action for a character
// based on their class data.
func (s *Service) resolveAttacksPerAction(ctx context.Context, char refdata.Character) int {
	var classes []CharacterClass
	if err := json.Unmarshal(char.Classes, &classes); err != nil {
		return 1
	}

	bestAttacks := 1
	for _, cc := range classes {
		classInfo, err := s.store.GetClass(ctx, cc.Class)
		if err != nil {
			continue
		}
		var attacksMap map[string]int
		if err := json.Unmarshal(classInfo.AttacksPerAction, &attacksMap); err != nil {
			continue
		}
		attacks := AttacksPerActionForLevel(attacksMap, cc.Level)
		if attacks > bestAttacks {
			bestAttacks = attacks
		}
	}

	return bestAttacks
}
