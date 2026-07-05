package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

// polearmButtWeapons are the weapons whose opposite (blunt) end a Polearm Master
// may strike with as a bonus action (2014 PHB): glaive, halberd, quarterstaff,
// spear. Pike is intentionally absent — the seeded feat grants pike only the
// opportunity-attack-on-enter-reach half, not the bonus-action butt strike.
var polearmButtWeapons = map[string]bool{
	"glaive":       true,
	"halberd":      true,
	"quarterstaff": true,
	"spear":        true,
}

// IsPolearmButtWeapon reports whether the weapon qualifies for the Polearm
// Master bonus-action butt strike.
func IsPolearmButtWeapon(weapon refdata.Weapon) bool {
	return polearmButtWeapons[weapon.ID]
}

// PolearmMasterBonusAttackCommand holds the inputs for the Polearm Master
// bonus-action butt strike.
type PolearmMasterBonusAttackCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
}

// PolearmMasterBonusAttack handles /bonus polearm (COV-9). After taking the
// Attack action with a glaive, halberd, quarterstaff, or spear, a character with
// the Polearm Master feat can strike with the opposite end as a bonus action:
// one melee attack using the same ability modifier as the primary attack, but
// with a d4 bludgeoning damage die.
//
// It mirrors MartialArtsBonusAttack (the fixed-die bonus-strike template) rather
// than GWMBonusAttack, which reuses the weapon's own die. FES riders (Rage bonus
// damage, magic-weapon bonuses) are intentionally not wired onto the butt strike
// yet, matching the monk bonus strike; that parity is documented as a deferred
// follow-up.
func (s *Service) PolearmMasterBonusAttack(ctx context.Context, cmd PolearmMasterBonusAttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return AttackResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return AttackResult{}, fmt.Errorf("Polearm Master bonus attack requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting character: %w", err)
	}

	if !HasFeatureByName(char.Features.RawMessage, "Polearm Master") {
		return AttackResult{}, fmt.Errorf("Polearm Master bonus attack requires the Polearm Master feat")
	}

	if !cmd.Turn.ActionUsed {
		return AttackResult{}, fmt.Errorf("Polearm Master bonus attack requires the Attack action this turn")
	}

	if !char.EquippedMainHand.Valid || char.EquippedMainHand.String == "" {
		return AttackResult{}, fmt.Errorf("Polearm Master bonus attack requires a glaive, halberd, quarterstaff, or spear in your main hand")
	}
	weapon, err := s.store.GetWeapon(ctx, char.EquippedMainHand.String)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting main hand weapon: %w", err)
	}
	if !IsPolearmButtWeapon(weapon) {
		return AttackResult{}, fmt.Errorf("Polearm Master bonus attack requires a glaive, halberd, quarterstaff, or spear (%q is not one)", weapon.Name)
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	// The butt strike is still an attack with the same weapon, so proficiency,
	// the finesse/STR-vs-DEX ability choice, and the heavy-weapon small-creature
	// penalty all carry over — clone the equipped weapon and override only the
	// damage die (a d4) and type (bludgeoning), keeping the polearm's name for the
	// combat log. VersatileDamage rides along but stays inert: TwoHanded is never
	// set, so the versatile die is never consulted.
	buttWeapon := weapon
	buttWeapon.Damage = "1d4"
	buttWeapon.DamageType = "bludgeoning"

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	dmAdv, dmDisadv := s.consumeDMAdvOverride(ctx, cmd.Attacker, cmd.DMAdvantage, cmd.DMDisadvantage)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, buttWeapon, scores, int(char.ProficiencyBonus), distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		dmAdv, dmDisadv, nil,
	)

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}

	if _, _, err := s.applyHitDamage(ctx, cmd.Attacker.EncounterID, cmd.Target, result); err != nil {
		return result, err
	}

	s.markRageAttacked(ctx, cmd.Attacker)
	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, &char)
	return result, nil
}
