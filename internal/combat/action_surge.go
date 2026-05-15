package combat

import (
	"context"
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
	if cmd.Turn.ActionSurged {
		return ActionSurgeResult{}, fmt.Errorf("Action Surge already used this turn")
	}
	if !cmd.Fighter.CharacterID.Valid {
		return ActionSurgeResult{}, fmt.Errorf("Action Surge requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Fighter.CharacterID.UUID)
	if err != nil {
		return ActionSurgeResult{}, fmt.Errorf("getting character: %w", err)
	}

	fighterLevel := ClassLevelFromJSON(char.Classes, "Fighter")
	if fighterLevel < 2 {
		return ActionSurgeResult{}, fmt.Errorf("Action Surge requires Fighter level 2+")
	}

	featureUses, remaining, err := ParseFeatureUses(char, FeatureKeyActionSurge)
	if err != nil {
		return ActionSurgeResult{}, err
	}
	if remaining <= 0 {
		return ActionSurgeResult{}, fmt.Errorf("no Action Surge uses remaining")
	}

	newRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyActionSurge, featureUses, remaining)
	if err != nil {
		return ActionSurgeResult{}, err
	}

	updatedTurn := cmd.Turn
	updatedTurn.ActionUsed = false
	updatedTurn.AttacksRemaining = int32(s.resolveAttacksPerAction(ctx, char))
	updatedTurn.ActionSurged = true

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

// ActionSurgeMaxUses returns the maximum number of Action Surge uses for a
// fighter of the given level. Returns 2 at level 17+, 1 otherwise.
func ActionSurgeMaxUses(fighterLevel int) int {
	if fighterLevel >= 17 {
		return 2
	}
	return 1
}
