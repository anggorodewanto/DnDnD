package combat

import (
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// AdvantageInput holds all context needed to detect advantage/disadvantage sources.
type AdvantageInput struct {
	AttackerConditions  []CombatCondition
	TargetConditions    []CombatCondition
	Weapon              refdata.Weapon
	DistanceFt          int
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
	Reckless            bool
	AttackerHidden      bool
	TargetHidden        bool
	AttackerObscurement ObscurementLevel
	TargetObscurement   ObscurementLevel
	// AbilityUsed is "str" or "dex" — which ability mod was chosen for this attack.
	AbilityUsed string
	// TargetCombatantID is the ID of the combatant currently being attacked.
	// SR-018: enables target-scoped condition checks (e.g. help_advantage).
	TargetCombatantID string
}

// DetectAdvantage examines attacker/target conditions, weapon properties, and combat
// context to determine the final roll mode. Returns the mode plus lists of reasons.
func DetectAdvantage(input AdvantageInput) (dice.RollMode, []string, []string) {
	var advReasons []string
	var disadvReasons []string

	// Reckless Attack
	if input.Reckless {
		advReasons = append(advReasons, "Reckless Attack")
	}

	// Hidden combatants
	if input.AttackerHidden {
		advReasons = append(advReasons, "attacker hidden")
	}
	if input.TargetHidden {
		disadvReasons = append(disadvReasons, "target hidden")
	}

	// DM overrides
	if input.DMAdvantage {
		advReasons = append(advReasons, "DM override")
	}
	if input.DMDisadvantage {
		disadvReasons = append(disadvReasons, "DM override")
	}

	// Obscurement effects (Blinded-like for heavily obscured)
	if input.AttackerObscurement == HeavilyObscured {
		disadvReasons = append(disadvReasons, "heavily obscured (blinded)")
	}
	if input.TargetObscurement == HeavilyObscured {
		advReasons = append(advReasons, "target heavily obscured (blinded)")
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
		case "help_advantage":
			// SR-018: Help action grants advantage on the helped creature's
			// next attack vs the named target only. Empty TargetCombatantID
			// is treated as no grant (defensive — never universal advantage).
			if c.TargetCombatantID != "" && c.TargetCombatantID == input.TargetCombatantID {
				advReasons = append(advReasons, "help advantage")
			}
		case "reckless":
			// C-C02: Reckless Attack's attacker-side half — the transient
			// condition grants advantage on melee STR attacks for the rest
			// of the turn (attacks 2+).
			if !IsRangedWeapon(input.Weapon) && input.AbilityUsed == "str" {
				advReasons = append(advReasons, "Reckless Attack (active)")
			}
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
		case "dodge":
			disadvReasons = append(disadvReasons, "target dodging")
		case "reckless":
			// C-38: Reckless Attack's target-side half — enemies have
			// advantage on attack rolls against the reckless attacker
			// until their next turn. The transient `reckless` condition
			// is applied to the attacker in Service.Attack and clears
			// at the start of their next turn.
			advReasons = append(advReasons, "target reckless")
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
