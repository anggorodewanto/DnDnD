package combat

import (
	"context"
	"fmt"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// handCrossbowID is the seeded weapon id for the hand crossbow — the only
// weapon Crossbow Expert's bonus-action attack may fire.
const handCrossbowID = "hand-crossbow"

// IsHandCrossbow reports whether the weapon is a hand crossbow, the sole weapon
// eligible for the Crossbow Expert bonus-action attack.
func IsHandCrossbow(weapon refdata.Weapon) bool {
	return weapon.ID == handCrossbowID
}

// CrossbowExpertBonusAttackCommand holds the inputs for the Crossbow Expert
// bonus-action hand-crossbow attack. It mirrors GWMBonusAttackCommand: Walls and
// the vision fields let the shared full-attack path resolve cover and
// obscurement for this ranged swing.
type CrossbowExpertBonusAttackCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
	AttackerVision      VisionCapabilities
	TargetVision        VisionCapabilities
	Walls               []renderer.WallSegment
}

// CrossbowExpertBonusAttack handles /bonus crossbow (COV-9). After attacking with
// a one-handed weapon as part of the Attack action, a character with the Crossbow
// Expert feat may fire a hand crossbow they are holding (main or off hand) as a
// bonus action.
//
// Unlike the fixed-die monk/Polearm bonus strikes, this is a full ranged weapon
// attack, so it takes the same path as GWMBonusAttack — cover, obscurement, the
// Feature Effect System (Sneak Attack, magic-crossbow bonuses, Sharpshooter), and
// weapon mastery all apply — plus it spends a bolt from inventory (the one thing
// no other bonus-attack path does, since they are all melee). input.HasCrossbowExpert
// is set so the feat's own "no disadvantage firing within 5 ft of a hostile" rider
// carries onto this swing too.
func (s *Service) CrossbowExpertBonusAttack(ctx context.Context, cmd CrossbowExpertBonusAttackCommand, roller *dice.Roller) (AttackResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return AttackResult{}, err
	}

	if err := validateCharmedAttack(cmd.Attacker, cmd.Target); err != nil {
		return AttackResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return AttackResult{}, fmt.Errorf("Crossbow Expert bonus attack requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return AttackResult{}, fmt.Errorf("getting character: %w", err)
	}

	if !HasFeatureByName(char.Features.RawMessage, "Crossbow Expert") {
		return AttackResult{}, fmt.Errorf("Crossbow Expert bonus attack requires the Crossbow Expert feat")
	}

	// The feat triggers off the Attack action: an attack must already have been
	// made this turn (mirrors OffhandAttack). We do not track which weapon the
	// Attack action used, so the "one-handed weapon" clause is not enforced.
	maxAttacks := int32(s.resolveAttacksPerAction(ctx, char))
	if cmd.Turn.AttacksRemaining >= maxAttacks {
		return AttackResult{}, fmt.Errorf("Crossbow Expert bonus attack requires you to have attacked with a one-handed weapon this turn first")
	}

	weapon, ok, err := s.equippedHandCrossbow(ctx, char)
	if err != nil {
		return AttackResult{}, err
	}
	if !ok {
		return AttackResult{}, fmt.Errorf("Crossbow Expert bonus attack requires a hand crossbow in your main or off hand")
	}

	parsed, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return AttackResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}
	// SR-022 parity: wild-shaped attackers use the beast's merged scores.
	scores := ResolveAttackerScores(ctx, s.store, cmd.Attacker, parsed)

	// Cover gate runs BEFORE spending the bolt or the bonus action so a
	// total-cover shot burns neither.
	coverLevel, err := s.resolveAttackCover(ctx, cmd.Attacker, cmd.Target, cmd.Walls)
	if err != nil {
		return AttackResult{}, err
	}

	// Spend a bolt (and track it for post-combat recovery) before building the
	// swing. An empty quiver returns NoAmmunitionError here, before UseResource;
	// the turn is only persisted later in resolveAndPersistAttack, so no resource
	// is durably burned on that early return.
	if err := s.deductWeaponAmmunition(ctx, &char, weapon, cmd.Attacker); err != nil {
		return AttackResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return AttackResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	distFt := combatantDistance(cmd.Attacker, cmd.Target)
	dmAdv, dmDisadv := s.consumeDMAdvOverride(ctx, cmd.Attacker, cmd.DMAdvantage, cmd.DMDisadvantage)
	input := buildAttackInput(
		cmd.Attacker, cmd.Target, weapon, scores, int(char.ProficiencyBonus), distFt,
		cmd.HostileNearAttacker, cmd.AttackerSize,
		dmAdv, dmDisadv, nil,
	)
	s.populateAttackContext(ctx, &input, cmd.Attacker)
	input.Cover = coverLevel
	// The feat removes the "attacking at range within 5 ft of a hostile" penalty
	// on this swing too (mirrors the main Attack path's HasCrossbowExpert flag).
	input.HasCrossbowExpert = true
	if char.CharacterData.Valid {
		input.WeaponMasteries = parseWeaponMasteries(char.CharacterData.RawMessage)
	}

	attackerObs, targetObs, err := s.resolveObscurement(ctx, cmd.Attacker.EncounterID, cmd.Attacker, cmd.Target, cmd.AttackerVision, cmd.TargetVision)
	if err != nil {
		return AttackResult{}, err
	}
	input.AttackerObscurement = attackerObs
	input.TargetObscurement = targetObs

	// Wire FES so magic-crossbow bonuses, Sneak Attack, and the once-per-turn
	// tracker apply to the bonus shot too.
	fesCmd := AttackCommand{Attacker: cmd.Attacker, Target: cmd.Target, Turn: updatedTurn}
	if err := s.populateAttackFES(ctx, &input, fesCmd, &char, weapon, scores); err != nil {
		return AttackResult{}, err
	}

	result, err := s.resolveAndPersistAttack(ctx, input, updatedTurn, cmd.Attacker, roller)
	if err != nil {
		return result, err
	}
	if _, _, err := s.applyHitDamage(ctx, cmd.Attacker.EncounterID, cmd.Target, &result, true); err != nil {
		return result, err
	}

	// 2024 Weapon Mastery — apply any on-hit effect that fired (Vex on the hand
	// crossbow), mirroring the main Attack / off-hand paths.
	if err := s.applyMasteryEffects(ctx, cmd.Attacker, cmd.Target, &result, roller); err != nil {
		return result, err
	}
	s.markUsedEffects(cmd.Attacker.EncounterID, cmd.Attacker.ID, result.OncePerTurnEffectsFired)
	s.markRageAttacked(ctx, cmd.Attacker)

	// ISSUE-014: persist to action_log for the DM Console timeline.
	s.recordAttackAction(ctx, cmd.Turn.ID, cmd.Attacker.EncounterID, cmd.Attacker.ID,
		nullableCombatantID(cmd.Target.ID), result)

	s.populatePostHitPrompts(ctx, &result, cmd.Attacker, &char)
	// The shot counts as an attack vs the target, so it spends any help/vex
	// advantage scoped to that target and any sap disadvantage on the attacker.
	s.consumeHelpAdvantage(ctx, cmd.Attacker, cmd.Target)
	s.consumeSapDisadvantage(ctx, cmd.Attacker)
	return result, nil
}

// equippedHandCrossbow returns the character's hand crossbow, checking the main
// hand first, then the off hand. ok is false when neither hand holds one.
func (s *Service) equippedHandCrossbow(ctx context.Context, char refdata.Character) (refdata.Weapon, bool, error) {
	for _, slot := range []string{char.EquippedMainHand.String, char.EquippedOffHand.String} {
		if slot == "" {
			continue
		}
		weapon, err := s.store.GetWeapon(ctx, slot)
		if err != nil {
			return refdata.Weapon{}, false, fmt.Errorf("getting equipped weapon: %w", err)
		}
		if IsHandCrossbow(weapon) {
			return weapon, true, nil
		}
	}
	return refdata.Weapon{}, false, nil
}
