package combat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// CharacterClass represents a single class entry in a character's classes JSON array.
type CharacterClass struct {
	Class string `json:"class"`
	Level int    `json:"level"`
}

// ResourceType represents a combat resource that can be used during a turn.
type ResourceType string

const (
	ResourceAction      ResourceType = "action"
	ResourceBonusAction ResourceType = "bonus action"
	ResourceReaction    ResourceType = "reaction"
	ResourceMovement    ResourceType = "movement"
	ResourceFreeInteract ResourceType = "free object interaction"
	ResourceAttack      ResourceType = "attack"
)

// ErrResourceSpent is the base error for when a resource has already been used.
var ErrResourceSpent = errors.New("resource already spent")

// ValidateResource checks if the given resource is still available on the turn.
// Returns ErrResourceSpent (wrapped with resource name) if the resource is used.
func ValidateResource(turn refdata.Turn, resource ResourceType) error {
	switch resource {
	case ResourceAction:
		if turn.ActionUsed {
			return fmt.Errorf("%s: %w", resource, ErrResourceSpent)
		}
	case ResourceBonusAction:
		if turn.BonusActionUsed {
			return fmt.Errorf("%s: %w", resource, ErrResourceSpent)
		}
	case ResourceReaction:
		if turn.ReactionUsed {
			return fmt.Errorf("%s: %w", resource, ErrResourceSpent)
		}
	case ResourceFreeInteract:
		if turn.FreeInteractUsed {
			return fmt.Errorf("%s: %w", resource, ErrResourceSpent)
		}
	case ResourceMovement:
		if turn.MovementRemainingFt <= 0 {
			return fmt.Errorf("%s: %w", resource, ErrResourceSpent)
		}
	case ResourceAttack:
		if turn.AttacksRemaining <= 0 {
			return fmt.Errorf("%s: %w", resource, ErrResourceSpent)
		}
	default:
		return fmt.Errorf("unknown resource type: %s", resource)
	}
	return nil
}

// UseResource marks a boolean resource (action, bonus action, reaction, free interact) as used.
// For movement and attacks, use UseMovement and UseAttack instead.
// Returns a copy of the turn with the resource marked as used.
func UseResource(turn refdata.Turn, resource ResourceType) (refdata.Turn, error) {
	switch resource {
	case ResourceMovement:
		return turn, fmt.Errorf("use UseMovement for movement resources")
	case ResourceAttack:
		return turn, fmt.Errorf("use UseAttack for attack resources")
	default:
		// handled below
	}
	if err := ValidateResource(turn, resource); err != nil {
		return turn, err
	}
	switch resource {
	case ResourceAction:
		turn.ActionUsed = true
	case ResourceBonusAction:
		turn.BonusActionUsed = true
	case ResourceReaction:
		turn.ReactionUsed = true
	case ResourceFreeInteract:
		turn.FreeInteractUsed = true
	}
	return turn, nil
}

// RefundResource marks a boolean resource as available again (sets it back to false/unused).
// Used when a pending action is cancelled and the resource should be returned.
// For movement and attacks, use dedicated refund functions instead.
func RefundResource(turn refdata.Turn, resource ResourceType) refdata.Turn {
	switch resource {
	case ResourceAction:
		turn.ActionUsed = false
	case ResourceBonusAction:
		turn.BonusActionUsed = false
	case ResourceReaction:
		turn.ReactionUsed = false
	case ResourceFreeInteract:
		turn.FreeInteractUsed = false
	case ResourceMovement, ResourceAttack:
		// Movement and attacks are not boolean resources; no-op here.
	}
	return turn
}

// UseMovement deducts the given feet from movement remaining.
// Returns an error if feet is not positive, ErrResourceSpent if no movement remains,
// or an error if requesting more movement than available.
func UseMovement(turn refdata.Turn, feet int32) (refdata.Turn, error) {
	if feet <= 0 {
		return turn, fmt.Errorf("movement must be positive, got %d", feet)
	}
	if turn.MovementRemainingFt <= 0 {
		return turn, fmt.Errorf("%s: %w", ResourceMovement, ErrResourceSpent)
	}
	if feet > turn.MovementRemainingFt {
		return turn, fmt.Errorf("not enough movement: %dft requested, %dft remaining", feet, turn.MovementRemainingFt)
	}
	turn.MovementRemainingFt -= feet
	return turn, nil
}

// AttacksPerActionForLevel determines the number of attacks per action based on
// a class's attacks_per_action map (e.g., {"1": 1, "5": 2}) and the character level.
// It finds the highest level threshold that the character meets or exceeds.
// Returns 1 as a default if no thresholds match.
func AttacksPerActionForLevel(attacks map[string]int, level int) int {
	if len(attacks) == 0 {
		return 1
	}
	best := 1
	bestThreshold := 0
	for k, v := range attacks {
		threshold, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		if threshold <= level && threshold > bestThreshold {
			bestThreshold = threshold
			best = v
		}
	}
	return best
}

// UseAttack deducts one attack from attacks remaining.
// Returns ErrResourceSpent if no attacks remain.
func UseAttack(turn refdata.Turn) (refdata.Turn, error) {
	if turn.AttacksRemaining <= 0 {
		return turn, fmt.Errorf("%s: %w", ResourceAttack, ErrResourceSpent)
	}
	turn.AttacksRemaining--
	return turn, nil
}

// TurnToUpdateParams converts a Turn struct to the UpdateTurnActionsParams needed
// to persist resource state to the database.
func TurnToUpdateParams(turn refdata.Turn) refdata.UpdateTurnActionsParams {
	return refdata.UpdateTurnActionsParams{
		ID:                   turn.ID,
		MovementRemainingFt:  turn.MovementRemainingFt,
		ActionUsed:           turn.ActionUsed,
		BonusActionUsed:      turn.BonusActionUsed,
		BonusActionSpellCast: turn.BonusActionSpellCast,
		ActionSpellCast:      turn.ActionSpellCast,
		ReactionUsed:         turn.ReactionUsed,
		FreeInteractUsed:     turn.FreeInteractUsed,
		AttacksRemaining:     turn.AttacksRemaining,
		HasDisengaged:        turn.HasDisengaged,
		ActionSurged:         turn.ActionSurged,
		HasStoodThisTurn:     turn.HasStoodThisTurn,
	}
}

// buildResourceList returns the list of available resource display strings for a turn.
func buildResourceList(turn refdata.Turn) []string {
	var parts []string
	if turn.MovementRemainingFt > 0 {
		parts = append(parts, fmt.Sprintf("\U0001f3c3 %dft move", turn.MovementRemainingFt))
	}
	if turn.AttacksRemaining > 0 {
		if turn.AttacksRemaining == 1 {
			parts = append(parts, "\u2694\ufe0f 1 attack")
		} else {
			parts = append(parts, fmt.Sprintf("\u2694\ufe0f %d attacks", turn.AttacksRemaining))
		}
	}
	if !turn.BonusActionUsed {
		parts = append(parts, "\U0001f381 Bonus action")
	}
	if !turn.FreeInteractUsed {
		parts = append(parts, "\u270b Free interact")
	}
	if !turn.ReactionUsed {
		parts = append(parts, "\U0001f6e1\ufe0f Reaction")
	}
	return parts
}

// ResolveTurnResources determines the starting movement (ft) and attacks remaining
// for a combatant at the start of their turn. For PCs, it looks up character speed
// and class attacks_per_action. For NPCs, defaults are 30ft and 1 attack.
// Condition effects (grappled/restrained → 0 speed) are applied.
func (s *Service) ResolveTurnResources(ctx context.Context, combatant refdata.Combatant) (speedFt int32, attacksRemaining int32, err error) {
	conds, _ := parseConditions(combatant.Conditions)

	if combatant.IsNpc || !combatant.CharacterID.Valid {
		return int32(EffectiveSpeed(30, conds)), 1, nil
	}

	char, err := s.store.GetCharacter(ctx, combatant.CharacterID.UUID)
	if err != nil {
		return 0, 0, fmt.Errorf("getting character for turn resources: %w", err)
	}

	speedFt = char.SpeedFt
	if speedFt <= 0 {
		speedFt = 30
	}

	return int32(EffectiveSpeed(int(speedFt), conds)), int32(s.resolveAttacksPerAction(ctx, char)), nil
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

// FormatTurnStartPrompt produces the turn start notification shown in #your-turn.
// An optional combatant may be passed to include Bardic Inspiration status.
func FormatTurnStartPrompt(encounterName string, roundNumber int32, combatantName string, turn refdata.Turn, combatant *refdata.Combatant) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\u2694\ufe0f %s \u2014 Round %d\n", encounterName, roundNumber)
	fmt.Fprintf(&b, "\U0001f514 @%s \u2014 it's your turn!\n", combatantName)

	var parts []string
	if combatant != nil {
		parts = BuildResourceListWithInspiration(turn, *combatant)
	} else {
		parts = buildResourceList(turn)
	}
	if len(parts) > 0 {
		fmt.Fprintf(&b, "\U0001f4cb Available: %s", strings.Join(parts, " | "))
	} else {
		b.WriteString("\U0001f4cb All actions spent \u2014 type /done to end your turn.")
	}
	return b.String()
}

// FormatTurnStartPromptWithExpiry produces the turn start notification with optional
// readied action expiry notices prepended.
func FormatTurnStartPromptWithExpiry(encounterName string, roundNumber int32, combatantName string, turn refdata.Turn, combatant *refdata.Combatant, expiryNotices []string) string {
	prompt := FormatTurnStartPrompt(encounterName, roundNumber, combatantName, turn, combatant)
	if len(expiryNotices) == 0 {
		return prompt
	}
	return strings.Join(expiryNotices, "\n") + "\n" + prompt
}

// BuildResourceListWithInspiration returns buildResourceList plus Bardic Inspiration status if present.
func BuildResourceListWithInspiration(turn refdata.Turn, combatant refdata.Combatant) []string {
	parts := buildResourceList(turn)
	if CombatantHasBardicInspiration(combatant) {
		parts = append(parts, FormatBardicInspirationStatus(combatant.BardicInspirationDie.String))
	}
	return parts
}

// FormatRemainingResources produces the status line appended after each command in #combat-log.
// An optional combatant may be passed to include Bardic Inspiration status.
func FormatRemainingResources(turn refdata.Turn, combatant *refdata.Combatant) string {
	var parts []string
	if combatant != nil {
		parts = BuildResourceListWithInspiration(turn, *combatant)
	} else {
		parts = buildResourceList(turn)
	}
	if len(parts) == 0 {
		return "\U0001f4cb All actions spent \u2014 type /done to end your turn."
	}
	return fmt.Sprintf("\U0001f4cb Remaining: %s", strings.Join(parts, " | "))
}
