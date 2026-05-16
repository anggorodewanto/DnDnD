package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// autoResolvablePatterns lists keyword prefixes that indicate an interaction
// can be resolved immediately without DM adjudication.
var autoResolvablePatterns = []string{
	"draw",
	"sheathe",
	"sheath",
	"stow",
	"open",
	"close",
	"pick up",
	"grab",
	"pull out",
	"put away",
}

// InteractCommand holds the inputs for an object interaction.
type InteractCommand struct {
	Combatant   refdata.Combatant
	Turn        refdata.Turn
	Description string
}

// InteractResult holds the outputs of an object interaction.
type InteractResult struct {
	Turn           refdata.Turn
	PendingAction  refdata.PendingAction
	CombatLog      string
	DMQueueMessage string
	AutoResolved   bool
}

// dmRequiredKeywords are words that indicate DM adjudication is needed,
// even if the description matches an auto-resolvable prefix.
var dmRequiredKeywords = []string{"lock", "trap", "stuck", "barred", "sealed"}

// isAutoResolvable checks whether the interaction description matches
// a pattern that can be resolved without DM intervention.
func isAutoResolvable(description string) bool {
	lower := strings.ToLower(description)
	for _, pattern := range autoResolvablePatterns {
		if strings.HasPrefix(lower, pattern) {
			for _, kw := range dmRequiredKeywords {
				if strings.Contains(lower, kw) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// Interact handles the /interact command.
// First interaction per turn is free (uses free_interact_used).
// Second interaction costs the action. If action is spent, rejected.
// Auto-resolvable interactions resolve immediately; others go to DM queue.
func (s *Service) Interact(ctx context.Context, cmd InteractCommand) (InteractResult, error) {
	if ok, reason := CanActRaw(cmd.Combatant.Conditions); !ok {
		return InteractResult{}, fmt.Errorf("%s", reason)
	}

	resource := ResourceFreeInteract
	if cmd.Turn.FreeInteractUsed {
		if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
			return InteractResult{}, fmt.Errorf("Free interaction already used and action is spent")
		}
		resource = ResourceAction
	}

	updatedTurn, err := UseResource(cmd.Turn, resource)
	if err != nil {
		return InteractResult{}, err
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return InteractResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	if isAutoResolvable(cmd.Description) {
		combatLog := fmt.Sprintf("🤚 %s: %s", cmd.Combatant.DisplayName, cmd.Description)
		return InteractResult{
			Turn:         updatedTurn,
			CombatLog:    combatLog,
			AutoResolved: true,
		}, nil
	}

	pendingAction, err := s.store.CreatePendingAction(ctx, refdata.CreatePendingActionParams{
		EncounterID: cmd.Turn.EncounterID,
		CombatantID: cmd.Combatant.ID,
		ActionText:  cmd.Description,
	})
	if err != nil {
		return InteractResult{}, fmt.Errorf("creating pending action: %w", err)
	}

	dmQueueMsg := fmt.Sprintf("🤚 **Interact** — %s: \"%s\"", cmd.Combatant.DisplayName, cmd.Description)
	combatLog := fmt.Sprintf("🤚 %s: \"%s\" — sent to DM queue", cmd.Combatant.DisplayName, cmd.Description)

	return InteractResult{
		Turn:           updatedTurn,
		PendingAction:  pendingAction,
		CombatLog:      combatLog,
		DMQueueMessage: dmQueueMsg,
	}, nil
}
