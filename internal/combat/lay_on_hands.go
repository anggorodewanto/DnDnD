package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/refdata"
)

// LayOnHandsPoolMax returns the total Lay on Hands healing pool (5 x paladin level).
func LayOnHandsPoolMax(paladinLevel int) int {
	return 5 * paladinLevel
}

func isUndeadOrConstruct(creatureType string) bool {
	lower := strings.ToLower(creatureType)
	return lower == "undead" || lower == "construct"
}

// LayOnHandsCommand holds the inputs for a Lay on Hands action.
type LayOnHandsCommand struct {
	Paladin    refdata.Combatant
	Target     refdata.Combatant
	Turn       refdata.Turn
	HP         int  // HP to restore from the pool
	CurePoison bool // spend 5 HP to cure poison
	CureDisease bool // spend 5 HP to cure disease
}

// LayOnHandsResult holds the output of a Lay on Hands action.
type LayOnHandsResult struct {
	HPRestored    int32
	HPAfter       int32
	PoolRemaining int
	PoolMax       int
	CuredPoison   bool
	CuredDisease  bool
	CombatLog     string
	Turn          refdata.Turn
}

// LayOnHands handles the /action lay-on-hands command.
// Validates paladin class, action resource, adjacency, creature type, and pool.
// Heals the target and optionally cures poison/disease conditions.
func (s *Service) LayOnHands(ctx context.Context, cmd LayOnHandsCommand) (LayOnHandsResult, error) {
	// Validate action resource
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return LayOnHandsResult{}, err
	}

	// Must be a character (not NPC)
	if !cmd.Paladin.CharacterID.Valid {
		return LayOnHandsResult{}, fmt.Errorf("Lay on Hands requires a character (not NPC)")
	}

	// Reject undead/construct targets
	if cmd.Target.CreatureRefID.Valid && cmd.Target.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, cmd.Target.CreatureRefID.String)
		if err != nil {
			return LayOnHandsResult{}, fmt.Errorf("getting creature: %w", err)
		}
		if isUndeadOrConstruct(creature.Type) {
			return LayOnHandsResult{}, fmt.Errorf("Lay on Hands has no effect on undead or constructs")
		}
	}

	// Adjacency check (within 5ft) — skip if self-targeting
	if cmd.Paladin.ID != cmd.Target.ID {
		dist := combatantDistance(cmd.Paladin, cmd.Target)
		if dist > 5 {
			return LayOnHandsResult{}, fmt.Errorf("target is out of range — %dft away (max 5ft)", dist)
		}
	}

	// Get character data
	char, err := s.store.GetCharacter(ctx, cmd.Paladin.CharacterID.UUID)
	if err != nil {
		return LayOnHandsResult{}, fmt.Errorf("getting character: %w", err)
	}

	// Validate paladin class
	paladinLevel := ClassLevelFromJSON(char.Classes, "Paladin")
	if paladinLevel < 1 {
		return LayOnHandsResult{}, fmt.Errorf("Lay on Hands requires Paladin class")
	}

	// Parse feature uses and get pool
	featureUses, poolRemaining, err := ParseFeatureUses(char, FeatureKeyLayOnHands)
	if err != nil {
		return LayOnHandsResult{}, err
	}
	poolMax := LayOnHandsPoolMax(paladinLevel)

	// Calculate total cost
	totalCost := cmd.HP
	if cmd.CurePoison {
		totalCost += 5
	}
	if cmd.CureDisease {
		totalCost += 5
	}

	if totalCost <= 0 {
		return LayOnHandsResult{}, fmt.Errorf("must spend at least 1 HP from pool")
	}

	newPoolRemaining, err := s.DeductFeaturePool(ctx, char, FeatureKeyLayOnHands, featureUses, poolRemaining, totalCost)
	if err != nil {
		return LayOnHandsResult{}, err
	}

	// Use action
	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return LayOnHandsResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return LayOnHandsResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Apply healing
	var hpRestored int32
	hpAfter := cmd.Target.HpCurrent
	if cmd.HP > 0 {
		hpRestored = int32(cmd.HP)
		hpAfter = cmd.Target.HpCurrent + hpRestored
		if hpAfter > cmd.Target.HpMax {
			hpAfter = cmd.Target.HpMax
			hpRestored = hpAfter - cmd.Target.HpCurrent
		}
		if _, err := s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
			ID:        cmd.Target.ID,
			HpCurrent: hpAfter,
			TempHp:    cmd.Target.TempHp,
			IsAlive:   true,
		}); err != nil {
			return LayOnHandsResult{}, fmt.Errorf("updating target HP: %w", err)
		}
		// C-43 / Phase 43: when the heal lifts the target from 0 HP back to
		// >=1, reset death-save tallies and drop the dying-condition bundle.
		if _, err := s.MaybeResetDeathSavesOnHeal(ctx, cmd.Target, hpAfter); err != nil {
			return LayOnHandsResult{}, fmt.Errorf("resetting death state on heal: %w", err)
		}
	}

	// Cure poison/disease conditions
	curedPoison := cmd.CurePoison && HasCondition(cmd.Target.Conditions, "poisoned")
	curedDisease := cmd.CureDisease && HasCondition(cmd.Target.Conditions, "diseased")

	if curedPoison {
		if _, _, err := s.RemoveConditionFromCombatant(ctx, cmd.Target.ID, "poisoned"); err != nil {
			return LayOnHandsResult{}, fmt.Errorf("removing poisoned condition: %w", err)
		}
	}
	if curedDisease {
		if _, _, err := s.RemoveConditionFromCombatant(ctx, cmd.Target.ID, "diseased"); err != nil {
			return LayOnHandsResult{}, fmt.Errorf("removing diseased condition: %w", err)
		}
	}

	// Build combat log
	combatLog := formatLayOnHandsLog(cmd.Paladin.DisplayName, cmd.Target.DisplayName, hpRestored, newPoolRemaining, poolMax, curedPoison, curedDisease)

	return LayOnHandsResult{
		HPRestored:    hpRestored,
		HPAfter:       hpAfter,
		PoolRemaining: newPoolRemaining,
		PoolMax:       poolMax,
		CuredPoison:   curedPoison,
		CuredDisease:  curedDisease,
		CombatLog:     combatLog,
		Turn:          updatedTurn,
	}, nil
}

// formatLayOnHandsLog builds the combat log for Lay on Hands.
func formatLayOnHandsLog(paladinName, targetName string, hpRestored int32, poolRemaining, poolMax int, curedPoison, curedDisease bool) string {
	var parts []string

	if hpRestored > 0 {
		parts = append(parts, fmt.Sprintf("💛  %s uses Lay on Hands on %s — restores %d HP (pool: %d/%d remaining)",
			paladinName, targetName, hpRestored, poolRemaining, poolMax))
	}
	if curedPoison {
		parts = append(parts, fmt.Sprintf("💛  %s uses Lay on Hands — cures %s of Poison (pool: %d/%d remaining)",
			paladinName, targetName, poolRemaining, poolMax))
	}
	if curedDisease {
		parts = append(parts, fmt.Sprintf("💛  %s uses Lay on Hands — cures %s of Disease (pool: %d/%d remaining)",
			paladinName, targetName, poolRemaining, poolMax))
	}

	return strings.Join(parts, "\n")
}
