package combat

import (
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// AdvantageInput holds all context needed to detect advantage/disadvantage sources.
type AdvantageInput struct {
	AttackerConditions []CombatCondition
	TargetConditions   []CombatCondition
	Weapon             refdata.Weapon
	DistanceFt         int
	HostileNearAttacker bool
	AttackerSize       string
	DMAdvantage        bool
	DMDisadvantage     bool
}

// DetectAdvantage examines attacker/target conditions, weapon properties, and combat
// context to determine the final roll mode. Returns the mode plus lists of reasons.
func DetectAdvantage(input AdvantageInput) (dice.RollMode, []string, []string) {
	var advReasons []string
	var disadvReasons []string

	// DM overrides
	if input.DMAdvantage {
		advReasons = append(advReasons, "DM override")
	}
	if input.DMDisadvantage {
		disadvReasons = append(disadvReasons, "DM override")
	}

	// Attacker conditions
	for _, c := range input.AttackerConditions {
		switch c.Condition {
		case "blinded":
			disadvReasons = append(disadvReasons, "attacker blinded")
		case "invisible":
			advReasons = append(advReasons, "attacker invisible")
		case "poisoned":
			disadvReasons = append(disadvReasons, "attacker poisoned")
		case "prone":
			disadvReasons = append(disadvReasons, "attacker prone")
		case "restrained":
			disadvReasons = append(disadvReasons, "attacker restrained")
		}
	}

	// Combat context: ranged attack with hostile within 5ft
	if input.HostileNearAttacker && IsRangedWeapon(input.Weapon) {
		disadvReasons = append(disadvReasons, "hostile within 5ft")
	}

	// Combat context: long range
	if IsInLongRange(input.Weapon, input.DistanceFt) {
		disadvReasons = append(disadvReasons, "long range")
	}

	// Combat context: heavy weapon + Small/Tiny creature
	if HasProperty(input.Weapon, "heavy") && (input.AttackerSize == "Small" || input.AttackerSize == "Tiny") {
		disadvReasons = append(disadvReasons, fmt.Sprintf("heavy weapon, %s creature", input.AttackerSize))
	}

	// Target conditions
	for _, c := range input.TargetConditions {
		switch c.Condition {
		case "blinded":
			advReasons = append(advReasons, "target blinded")
		case "invisible":
			disadvReasons = append(disadvReasons, "target invisible")
		case "restrained":
			advReasons = append(advReasons, "target restrained")
		case "stunned":
			advReasons = append(advReasons, "target stunned")
		case "paralyzed":
			advReasons = append(advReasons, "target paralyzed")
		case "unconscious":
			advReasons = append(advReasons, "target unconscious")
		case "petrified":
			advReasons = append(advReasons, "target petrified")
		case "prone":
			if input.DistanceFt <= 5 {
				advReasons = append(advReasons, "target prone within 5ft")
			} else {
				disadvReasons = append(disadvReasons, "target prone beyond 5ft")
			}
		}
	}

	return resolveMode(advReasons, disadvReasons), advReasons, disadvReasons
}

// resolveMode applies 5e cancellation: any advantage + any disadvantage = normal.
func resolveMode(adv, disadv []string) dice.RollMode {
	hasAdv := len(adv) > 0
	hasDisadv := len(disadv) > 0
	if hasAdv && hasDisadv {
		return dice.AdvantageAndDisadvantage
	}
	if hasAdv {
		return dice.Advantage
	}
	if hasDisadv {
		return dice.Disadvantage
	}
	return dice.Normal
}
