package rest

import (
	"fmt"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
)

// Service handles rest logic (short and long rests).
type Service struct {
	roller *dice.Roller
}

// NewService creates a new rest Service.
func NewService(roller *dice.Roller) *Service {
	return &Service{roller: roller}
}

// ShortRestInput holds parameters for a short rest.
type ShortRestInput struct {
	HPCurrent        int
	HPMax            int
	CONModifier      int
	HitDiceRemaining map[string]int            // e.g. {"d10": 5, "d8": 2}
	HitDiceSpend     map[string]int            // e.g. {"d10": 2} — how many to spend per die type
	FeatureUses      map[string]character.FeatureUse
	PactMagicSlots   *character.PactMagicSlots // nil if not a warlock
	Classes          []character.ClassEntry
}

// HitDieRoll records a single hit die roll result.
type HitDieRoll struct {
	DieType string // e.g. "d10"
	Rolled  int    // the die result
	CONMod  int    // CON modifier added
	Healed  int    // actual HP healed (may be less if capped)
}

// ShortRestResult holds the results of a short rest.
type ShortRestResult struct {
	HPBefore            int
	HPAfter             int
	HPHealed            int
	HitDieRolls         []HitDieRoll
	HitDiceRemaining    map[string]int
	FeaturesRecharged   []string
	PactSlotsRestored   bool
	PactSlotsCurrent    int
}

// ShortRest applies a short rest to a character.
func (s *Service) ShortRest(input ShortRestInput) (ShortRestResult, error) {
	result := ShortRestResult{
		HPBefore:         input.HPCurrent,
		HitDiceRemaining: copyMap(input.HitDiceRemaining),
	}

	hp := input.HPCurrent

	// Spend hit dice
	for dieType, count := range input.HitDiceSpend {
		remaining, ok := result.HitDiceRemaining[dieType]
		if !ok {
			return ShortRestResult{}, fmt.Errorf("no hit dice of type %s available", dieType)
		}
		if count > remaining {
			return ShortRestResult{}, fmt.Errorf("cannot spend %d %s hit dice, only %d remaining", count, dieType, remaining)
		}

		dieSize := character.HitDieValue(dieType)
		if dieSize == 0 {
			return ShortRestResult{}, fmt.Errorf("invalid hit die type: %s", dieType)
		}

		for i := 0; i < count; i++ {
			roll, err := s.roller.Roll(fmt.Sprintf("1%s", dieType))
			if err != nil {
				return ShortRestResult{}, fmt.Errorf("rolling hit die: %w", err)
			}

			healing := roll.Total + input.CONModifier
			if healing < 0 {
				healing = 0
			}

			actualHealed := healing
			if hp+actualHealed > input.HPMax {
				actualHealed = input.HPMax - hp
			}

			hp += actualHealed

			result.HitDieRolls = append(result.HitDieRolls, HitDieRoll{
				DieType: dieType,
				Rolled:  roll.Total,
				CONMod:  input.CONModifier,
				Healed:  actualHealed,
			})
		}

		result.HitDiceRemaining[dieType] = remaining - count
	}

	// Recharge short rest features
	for name, fu := range input.FeatureUses {
		if fu.Recharge == "short" {
			if fu.Current < fu.Max {
				fu.Current = fu.Max
				input.FeatureUses[name] = fu
				result.FeaturesRecharged = append(result.FeaturesRecharged, name)
			}
		}
	}

	// Restore pact magic slots
	if input.PactMagicSlots != nil && input.PactMagicSlots.Max > 0 {
		if input.PactMagicSlots.Current < input.PactMagicSlots.Max {
			input.PactMagicSlots.Current = input.PactMagicSlots.Max
			result.PactSlotsRestored = true
		}
		result.PactSlotsCurrent = input.PactMagicSlots.Current
	}

	result.HPAfter = hp
	result.HPHealed = hp - input.HPCurrent

	return result, nil
}

// LongRestInput holds parameters for a long rest.
type LongRestInput struct {
	HPCurrent          int
	HPMax              int
	HitDiceRemaining   map[string]int
	Classes            []character.ClassEntry
	FeatureUses        map[string]character.FeatureUse
	SpellSlots         map[string]character.SlotInfo
	PactMagicSlots     *character.PactMagicSlots
	DeathSaveSuccesses int
	DeathSaveFailures  int
}

// LongRestResult holds the results of a long rest.
type LongRestResult struct {
	HPBefore              int
	HPAfter               int
	HPHealed              int
	HitDiceRemaining      map[string]int
	HitDiceRestored       int
	FeaturesRecharged     []string
	SpellSlots            map[string]character.SlotInfo
	PactSlotsRestored     bool
	DeathSavesReset       bool
	PreparedCasterReminder bool
}

// preparedCasterClasses are classes that prepare spells and should be reminded.
var preparedCasterClasses = map[string]bool{
	"cleric":  true,
	"druid":   true,
	"paladin": true,
}

// LongRest applies a long rest to a character.
func (s *Service) LongRest(input LongRestInput) LongRestResult {
	result := LongRestResult{
		HPBefore:         input.HPCurrent,
		HPAfter:          input.HPMax,
		HPHealed:         input.HPMax - input.HPCurrent,
		HitDiceRemaining: copyMap(input.HitDiceRemaining),
	}

	// Restore spell slots
	result.SpellSlots = make(map[string]character.SlotInfo, len(input.SpellSlots))
	for level, slot := range input.SpellSlots {
		result.SpellSlots[level] = character.SlotInfo{Current: slot.Max, Max: slot.Max}
	}

	// Restore pact magic slots
	if input.PactMagicSlots != nil && input.PactMagicSlots.Max > 0 {
		if input.PactMagicSlots.Current < input.PactMagicSlots.Max {
			input.PactMagicSlots.Current = input.PactMagicSlots.Max
			result.PactSlotsRestored = true
		}
	}

	// Recharge all short and long rest features
	for name, fu := range input.FeatureUses {
		if (fu.Recharge == "short" || fu.Recharge == "long") && fu.Current < fu.Max {
			fu.Current = fu.Max
			input.FeatureUses[name] = fu
			result.FeaturesRecharged = append(result.FeaturesRecharged, name)
		}
	}

	// Restore hit dice: regain half total level (minimum 1)
	totalLevel := character.TotalLevel(input.Classes)
	toRestore := totalLevel / 2
	if toRestore < 1 {
		toRestore = 1
	}

	// Build max hit dice from classes
	maxHitDice := make(map[string]int)
	for _, c := range input.Classes {
		die := classHitDie(c.Class)
		maxHitDice[die] += c.Level
	}

	// Distribute restoration proportionally across die types
	restored := 0
	for die, maxCount := range maxHitDice {
		current := result.HitDiceRemaining[die]
		canRestore := maxCount - current
		if canRestore <= 0 {
			continue
		}
		restoreCount := toRestore - restored
		if restoreCount <= 0 {
			break
		}
		if restoreCount > canRestore {
			restoreCount = canRestore
		}
		result.HitDiceRemaining[die] = current + restoreCount
		restored += restoreCount
	}
	result.HitDiceRestored = restored

	// Death saves reset
	if input.DeathSaveSuccesses > 0 || input.DeathSaveFailures > 0 {
		result.DeathSavesReset = true
	}

	// Prepared caster reminder
	for _, c := range input.Classes {
		if preparedCasterClasses[c.Class] {
			result.PreparedCasterReminder = true
			break
		}
	}

	return result
}

// classHitDie returns the hit die string for a class.
func classHitDie(class string) string {
	switch class {
	case "barbarian":
		return "d12"
	case "fighter", "paladin", "ranger":
		return "d10"
	case "bard", "cleric", "druid", "monk", "rogue", "warlock":
		return "d8"
	case "sorcerer", "wizard":
		return "d6"
	default:
		return "d8"
	}
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
