package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// ValidateMartialArtsBonusAttack checks the preconditions for the martial arts
// bonus unarmed strike. monkLevel must be > 0 and the attack action must have
// been used this turn.
func ValidateMartialArtsBonusAttack(monkLevel int, attackActionUsed bool) error {
	if monkLevel <= 0 {
		return fmt.Errorf("Martial Arts bonus attack requires Monk class")
	}
	if !attackActionUsed {
		return fmt.Errorf("Martial Arts bonus attack requires the Attack action this turn")
	}
	return nil
}

// FormatMartialArtsBonusAttack returns the combat log header for a martial arts
// bonus unarmed strike.
func FormatMartialArtsBonusAttack(name string) string {
	return fmt.Sprintf("\U0001f44a  %s makes a Martial Arts bonus unarmed strike!", name)
}

// monkLevelFromJSON returns the monk level from character classes JSON.
func monkLevelFromJSON(classesJSON []byte) int {
	if len(classesJSON) == 0 {
		return 0
	}
	var classes []CharacterClass
	if err := json.Unmarshal(classesJSON, &classes); err != nil {
		return 0
	}
	return classLevel(classes, "Monk")
}

// MartialArtsBonusAttackCommand holds the inputs for a martial arts bonus unarmed strike.
type MartialArtsBonusAttackCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
}

// MartialArtsBonusAttack handles the /bonus martial-arts command.
// After taking the Attack action with an unarmed strike or monk weapon,
// the Monk can make one unarmed strike as a bonus action.
func (s *Service) MartialArtsBonusAttack(ctx context.Context, cmd MartialArtsBonusAttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return AttackResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return AttackResult{}, fmt.Errorf("martial arts bonus attack requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting character: %w", err)
	}

	ml := monkLevelFromJSON(char.Classes)
	if err := ValidateMartialArtsBonusAttack(ml, cmd.Turn.ActionUsed); err != nil {
		return AttackResult{}, err
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	weapon := UnarmedStrike()
	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, int(char.ProficiencyBonus), distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		cmd.DMAdvantage, cmd.DMDisadvantage, nil,
	)
	input.MonkLevel = ml

	return s.resolveAndPersistAttack(ctx, input, updatedTurn, roller)
}

// UnarmoredMovementBonus returns the speed bonus in feet for a monk of the given level.
// +10 (2), +15 (6), +20 (10), +25 (14), +30 (18).
func UnarmoredMovementBonus(monkLevel int) int {
	if monkLevel >= 18 {
		return 30
	}
	if monkLevel >= 14 {
		return 25
	}
	if monkLevel >= 10 {
		return 20
	}
	if monkLevel >= 6 {
		return 15
	}
	if monkLevel >= 2 {
		return 10
	}
	return 0
}

// UnarmoredMovementFeature returns the FeatureDefinition for Unarmored Movement
// at the given monk level. Only applies when not wearing armor.
func UnarmoredMovementFeature(monkLevel int) FeatureDefinition {
	return FeatureDefinition{
		Name:   "Unarmored Movement",
		Source: "monk",
		Effects: []Effect{
			{
				Type:     EffectModifySpeed,
				Trigger:  TriggerOnTurnStart,
				Modifier: UnarmoredMovementBonus(monkLevel),
				Conditions: EffectConditions{
					NotWearingArmor: true,
				},
			},
		},
	}
}

// MartialArtsDie returns the martial arts die string for a given monk level.
// 1d4 (1-4), 1d6 (5-10), 1d8 (11-16), 1d10 (17+).
func MartialArtsDie(monkLevel int) string {
	if monkLevel >= 17 {
		return "1d10"
	}
	if monkLevel >= 11 {
		return "1d8"
	}
	if monkLevel >= 5 {
		return "1d6"
	}
	return "1d4"
}

// MartialArtsDieSides returns the die sides for a given monk level.
func MartialArtsDieSides(monkLevel int) int {
	if monkLevel >= 17 {
		return 10
	}
	if monkLevel >= 11 {
		return 8
	}
	if monkLevel >= 5 {
		return 6
	}
	return 4
}

// MonkDamageExpression returns the effective damage die for a monk weapon or unarmed strike.
// For unarmed strikes, always uses the martial arts die.
// For monk weapons, uses whichever is higher: weapon die or martial arts die.
func MonkDamageExpression(weapon refdata.Weapon, monkLevel int) string {
	maDie := MartialArtsDie(monkLevel)
	maSides := MartialArtsDieSides(monkLevel)

	// Unarmed strikes always use martial arts die
	if weapon.ID == "unarmed-strike" {
		return maDie
	}

	// Parse the weapon's base damage die to compare
	expr, err := dice.ParseExpression(weapon.Damage)
	if err != nil || len(expr.Groups) == 0 {
		return maDie
	}

	weaponSides := expr.Groups[0].Sides
	if maSides > weaponSides {
		return maDie
	}
	return weapon.Damage
}

// IsMonkWeapon returns true if the weapon qualifies as a monk weapon:
// unarmed strikes, shortswords, or any simple melee weapon without the
// heavy or two-handed property.
func IsMonkWeapon(weapon refdata.Weapon) bool {
	if weapon.ID == "unarmed-strike" {
		return true
	}
	if weapon.ID == "shortsword" {
		return true
	}
	if weapon.WeaponType != "simple_melee" {
		return false
	}
	if HasProperty(weapon, "heavy") || HasProperty(weapon, "two-handed") {
		return false
	}
	return true
}
