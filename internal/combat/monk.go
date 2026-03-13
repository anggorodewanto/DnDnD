package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

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

// FlurryOfBlowsCommand holds the inputs for Flurry of Blows.
type FlurryOfBlowsCommand struct {
	Attacker            refdata.Combatant
	Target              refdata.Combatant
	Turn                refdata.Turn
	HostileNearAttacker bool
	AttackerSize        string
	DMAdvantage         bool
	DMDisadvantage      bool
}

// FlurryOfBlowsResult holds the results of Flurry of Blows (2 unarmed strikes).
type FlurryOfBlowsResult struct {
	Attacks     []AttackResult
	KiRemaining int
	CombatLog   string
}

// FlurryOfBlows handles the /bonus flurry-of-blows command.
// Costs 1 ki point + bonus action. Makes 2 unarmed strikes.
func (s *Service) FlurryOfBlows(ctx context.Context, cmd FlurryOfBlowsCommand, roller *dice.Roller) (FlurryOfBlowsResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return FlurryOfBlowsResult{}, err
	}

	if !cmd.Attacker.CharacterID.Valid {
		return FlurryOfBlowsResult{}, fmt.Errorf("flurry of blows requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return FlurryOfBlowsResult{}, fmt.Errorf("getting character: %w", err)
	}

	ml := monkLevelFromJSON(char.Classes)
	if err := ValidateMartialArtsBonusAttack(ml, cmd.Turn.ActionUsed); err != nil {
		return FlurryOfBlowsResult{}, err
	}

	featureUses, kiRemaining, err := parseKiUses(char)
	if err != nil {
		return FlurryOfBlowsResult{}, err
	}

	if err := ValidateKiAbility(ml, kiRemaining, "flurry-of-blows"); err != nil {
		return FlurryOfBlowsResult{}, err
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return FlurryOfBlowsResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	// Deduct 1 ki point
	newKi := kiRemaining - 1
	featureUses["ki"] = newKi
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return FlurryOfBlowsResult{}, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return FlurryOfBlowsResult{}, fmt.Errorf("updating feature_uses: %w", err)
	}

	// Use bonus action
	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return FlurryOfBlowsResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	weapon := UnarmedStrike()
	distFt := combatantDistance(cmd.Attacker, cmd.Target)

	// Make 2 unarmed strikes
	var attacks []AttackResult
	for i := 0; i < 2; i++ {
		input := buildAttackInput(
			cmd.Attacker, cmd.Target, weapon, scores, int(char.ProficiencyBonus), distFt,
			cmd.HostileNearAttacker, cmd.AttackerSize,
			cmd.DMAdvantage, cmd.DMDisadvantage, nil,
		)
		input.MonkLevel = ml

		result, err := ResolveAttack(input, roller)
		if err != nil {
			return FlurryOfBlowsResult{}, fmt.Errorf("resolving flurry attack %d: %w", i+1, err)
		}
		attacks = append(attacks, result)
	}

	// Persist turn
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return FlurryOfBlowsResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	combatLog := fmt.Sprintf("\U0001f44a\U0001f44a %s uses Flurry of Blows! (1 ki spent, %d remaining)", cmd.Attacker.DisplayName, newKi)

	return FlurryOfBlowsResult{
		Attacks:     attacks,
		KiRemaining: newKi,
		CombatLog:   combatLog,
	}, nil
}

// KiAbilityCommand holds the inputs for non-attack ki abilities (patient defense, step of the wind).
type KiAbilityCommand struct {
	Combatant refdata.Combatant
	Turn      refdata.Turn
}

// KiAbilityResult holds the result of a ki ability activation.
type KiAbilityResult struct {
	KiRemaining int
	CombatLog   string
	Turn        refdata.Turn
}

// spendKi is a shared helper that validates monk class, ki points, and bonus action,
// then deducts 1 ki point and uses the bonus action. Returns the updated turn, new ki remaining, and char.
func (s *Service) spendKi(ctx context.Context, charID uuid.UUID, turn refdata.Turn, abilityName string) (refdata.Turn, int, refdata.Character, error) {
	char, err := s.store.GetCharacter(ctx, charID)
	if err != nil {
		return turn, 0, refdata.Character{}, fmt.Errorf("getting character: %w", err)
	}

	ml := monkLevelFromJSON(char.Classes)
	featureUses, kiRemaining, err := parseKiUses(char)
	if err != nil {
		return turn, 0, refdata.Character{}, err
	}

	if err := ValidateKiAbility(ml, kiRemaining, abilityName); err != nil {
		return turn, 0, refdata.Character{}, err
	}

	newKi := kiRemaining - 1
	featureUses["ki"] = newKi
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return turn, 0, refdata.Character{}, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return turn, 0, refdata.Character{}, fmt.Errorf("updating feature_uses: %w", err)
	}

	updatedTurn, err := UseResource(turn, ResourceBonusAction)
	if err != nil {
		return turn, 0, refdata.Character{}, fmt.Errorf("using bonus action: %w", err)
	}

	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return turn, 0, refdata.Character{}, fmt.Errorf("updating turn actions: %w", err)
	}

	return updatedTurn, newKi, char, nil
}

// PatientDefense handles the /bonus patient-defense command.
// Costs 1 ki point + bonus action. Applies the "dodge" condition.
func (s *Service) PatientDefense(ctx context.Context, cmd KiAbilityCommand) (KiAbilityResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return KiAbilityResult{}, err
	}

	if !cmd.Combatant.CharacterID.Valid {
		return KiAbilityResult{}, fmt.Errorf("patient defense requires a character (not NPC)")
	}

	updatedTurn, newKi, _, err := s.spendKi(ctx, cmd.Combatant.CharacterID.UUID, cmd.Turn, "patient-defense")
	if err != nil {
		return KiAbilityResult{}, err
	}

	// Apply dodge condition (lasts until start of next turn — indefinite, cleared on turn start)
	dodgeCond := CombatCondition{
		Condition:         "dodge",
		DurationRounds:    1,
		StartedRound:      0, // Will be set by caller if needed
		SourceCombatantID: cmd.Combatant.ID.String(),
		ExpiresOn:         "start_of_turn",
	}

	if _, _, err := s.ApplyCondition(ctx, cmd.Combatant.ID, dodgeCond); err != nil {
		return KiAbilityResult{}, fmt.Errorf("applying dodge condition: %w", err)
	}

	combatLog := fmt.Sprintf("\U0001f6e1\ufe0f %s uses Patient Defense! (1 ki spent, %d remaining)", cmd.Combatant.DisplayName, newKi)

	return KiAbilityResult{
		KiRemaining: newKi,
		CombatLog:   combatLog,
		Turn:        updatedTurn,
	}, nil
}

// StunningStrikeDC calculates the DC for Stunning Strike.
// DC = 8 + proficiency bonus + WIS modifier.
func StunningStrikeDC(profBonus int, wisScore int) int {
	return 8 + profBonus + AbilityModifier(wisScore)
}

// StunningStrikeCommand holds the inputs for Stunning Strike.
type StunningStrikeCommand struct {
	Attacker     refdata.Combatant
	Target       refdata.Combatant
	CurrentRound int
}

// StunningStrikeResult holds the result of a Stunning Strike attempt.
type StunningStrikeResult struct {
	SaveRoll      int
	SaveTotal     int
	DC            int
	SaveSucceeded bool
	Stunned       bool
	KiRemaining   int
	CombatLog     string
}

// resolveTargetConSave resolves the target's CON save bonus.
// For creatures, checks saving_throws JSON for "con" bonus, else uses ability scores CON modifier.
// For PCs, uses ability scores CON modifier.
func (s *Service) resolveTargetConSave(ctx context.Context, target refdata.Combatant) (int, error) {
	if target.CreatureRefID.Valid && target.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, target.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature for save: %w", err)
		}
		// Check for explicit save bonus
		if creature.SavingThrows.Valid && len(creature.SavingThrows.RawMessage) > 0 {
			var saves map[string]int
			if err := json.Unmarshal(creature.SavingThrows.RawMessage, &saves); err == nil {
				if conSave, ok := saves["con"]; ok {
					return conSave, nil
				}
			}
		}
		// Fall back to ability score
		scores, err := ParseAbilityScores(creature.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing creature ability scores: %w", err)
		}
		return AbilityModifier(scores.Con), nil
	}

	// PC target
	if target.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
		if err != nil {
			return 0, fmt.Errorf("getting target character: %w", err)
		}
		scores, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing target ability scores: %w", err)
		}
		return AbilityModifier(scores.Con), nil
	}

	// NPC without creature ref — default to +0
	return 0, nil
}

// StunningStrike handles the stunning strike prompt after a melee hit.
// Costs 1 ki point. Target makes CON save vs DC = 8 + proficiency + WIS mod.
// On fail: stunned until end of monk's next turn.
func (s *Service) StunningStrike(ctx context.Context, cmd StunningStrikeCommand, roller *dice.Roller) (StunningStrikeResult, error) {
	if !cmd.Attacker.CharacterID.Valid {
		return StunningStrikeResult{}, fmt.Errorf("stunning strike requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Attacker.CharacterID.UUID)
	if err != nil {
		return StunningStrikeResult{}, fmt.Errorf("getting character: %w", err)
	}

	ml := monkLevelFromJSON(char.Classes)
	featureUses, kiRemaining, err := parseKiUses(char)
	if err != nil {
		return StunningStrikeResult{}, err
	}

	if err := ValidateKiAbility(ml, kiRemaining, "stunning-strike"); err != nil {
		return StunningStrikeResult{}, err
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return StunningStrikeResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	// Deduct 1 ki point
	newKi := kiRemaining - 1
	featureUses["ki"] = newKi
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return StunningStrikeResult{}, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return StunningStrikeResult{}, fmt.Errorf("updating feature_uses: %w", err)
	}

	dc := StunningStrikeDC(int(char.ProficiencyBonus), scores.Wis)

	// Resolve target's CON save bonus
	conSaveBonus, err := s.resolveTargetConSave(ctx, cmd.Target)
	if err != nil {
		return StunningStrikeResult{}, err
	}

	// Roll the save
	d20Result, err := roller.RollD20(conSaveBonus, dice.Normal)
	if err != nil {
		return StunningStrikeResult{}, fmt.Errorf("rolling CON save: %w", err)
	}
	saveRoll := d20Result.Chosen
	saveTotal := d20Result.Total
	saveSucceeded := saveTotal >= dc

	result := StunningStrikeResult{
		SaveRoll:      saveRoll,
		SaveTotal:     saveTotal,
		DC:            dc,
		SaveSucceeded: saveSucceeded,
		KiRemaining:   newKi,
	}

	if saveSucceeded {
		result.Stunned = false
		result.CombatLog = fmt.Sprintf("\u26a1 %s attempts Stunning Strike on %s! CON save: %d (roll %d + %d) vs DC %d — %s resists!",
			cmd.Attacker.DisplayName, cmd.Target.DisplayName, saveTotal, saveRoll, conSaveBonus, dc, cmd.Target.DisplayName)
		return result, nil
	}

	// Apply stunned condition: lasts until end of monk's next turn
	stunnedCond := CombatCondition{
		Condition:         "stunned",
		DurationRounds:    1,
		StartedRound:      cmd.CurrentRound,
		SourceCombatantID: cmd.Attacker.ID.String(),
		ExpiresOn:         "end_of_turn",
	}

	if _, _, err := s.ApplyCondition(ctx, cmd.Target.ID, stunnedCond); err != nil {
		return StunningStrikeResult{}, fmt.Errorf("applying stunned condition: %w", err)
	}

	result.Stunned = true
	result.CombatLog = fmt.Sprintf("\u26a1 %s attempts Stunning Strike on %s! CON save: %d (roll %d + %d) vs DC %d — %s is stunned!",
		cmd.Attacker.DisplayName, cmd.Target.DisplayName, saveTotal, saveRoll, conSaveBonus, dc, cmd.Target.DisplayName)

	return result, nil
}

// StepOfTheWindCommand holds the inputs for Step of the Wind.
type StepOfTheWindCommand struct {
	KiAbilityCommand
	Mode string // "dash" or "disengage"
}

// StepOfTheWind handles the /bonus step-of-the-wind command.
// Costs 1 ki point + bonus action. Dash doubles speed; disengage allows free movement.
func (s *Service) StepOfTheWind(ctx context.Context, cmd StepOfTheWindCommand) (KiAbilityResult, error) {
	if cmd.Mode != "dash" && cmd.Mode != "disengage" {
		return KiAbilityResult{}, fmt.Errorf("invalid mode %q: must be 'dash' or 'disengage'", cmd.Mode)
	}

	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return KiAbilityResult{}, err
	}

	if !cmd.Combatant.CharacterID.Valid {
		return KiAbilityResult{}, fmt.Errorf("step of the wind requires a character (not NPC)")
	}

	updatedTurn, newKi, _, err := s.spendKi(ctx, cmd.Combatant.CharacterID.UUID, cmd.Turn, "step-of-the-wind")
	if err != nil {
		return KiAbilityResult{}, err
	}

	switch cmd.Mode {
	case "dash":
		updatedTurn.MovementRemainingFt += cmd.Turn.MovementRemainingFt
	case "disengage":
		updatedTurn.HasDisengaged = true
	}

	// Re-persist the turn with dash/disengage updates
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return KiAbilityResult{}, fmt.Errorf("updating turn actions for step-of-the-wind: %w", err)
	}

	combatLog := fmt.Sprintf("\U0001f4a8 %s uses Step of the Wind (%s)! (1 ki spent, %d remaining)", cmd.Combatant.DisplayName, cmd.Mode, newKi)

	return KiAbilityResult{
		KiRemaining: newKi,
		CombatLog:   combatLog,
		Turn:        updatedTurn,
	}, nil
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

// MartialArtsDieSides returns the die sides for a given monk level.
// 4 (1-4), 6 (5-10), 8 (11-16), 10 (17+).
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

// MartialArtsDie returns the martial arts die string for a given monk level.
// 1d4 (1-4), 1d6 (5-10), 1d8 (11-16), 1d10 (17+).
func MartialArtsDie(monkLevel int) string {
	return fmt.Sprintf("1d%d", MartialArtsDieSides(monkLevel))
}

// MonkDamageExpression returns the effective damage die for a monk weapon or unarmed strike.
// For unarmed strikes, always uses the martial arts die.
// For monk weapons, uses whichever is higher: weapon die or martial arts die.
func MonkDamageExpression(weapon refdata.Weapon, monkLevel int) string {
	maSides := MartialArtsDieSides(monkLevel)
	maDie := fmt.Sprintf("1d%d", maSides)

	// Unarmed strikes always use martial arts die
	if weapon.ID == "unarmed-strike" {
		return maDie
	}

	// Parse the weapon's base damage die to compare
	expr, err := dice.ParseExpression(weapon.Damage)
	if err != nil || len(expr.Groups) == 0 {
		return maDie
	}

	if maSides > expr.Groups[0].Sides {
		return maDie
	}
	return weapon.Damage
}

// ValidateKiAbility checks preconditions for any ki ability.
func ValidateKiAbility(monkLevel int, kiRemaining int, abilityName string) error {
	if monkLevel <= 0 {
		return fmt.Errorf("%s requires Monk class", abilityName)
	}
	if kiRemaining <= 0 {
		return fmt.Errorf("no ki points remaining for %s", abilityName)
	}
	return nil
}

// parseKiUses extracts ki point uses from character feature_uses JSON.
func parseKiUses(char refdata.Character) (map[string]int, int, error) {
	featureUses := make(map[string]int)
	if char.FeatureUses.Valid && len(char.FeatureUses.RawMessage) > 0 {
		if err := json.Unmarshal(char.FeatureUses.RawMessage, &featureUses); err != nil {
			return nil, 0, fmt.Errorf("parsing feature_uses: %w", err)
		}
	}
	kiRemaining, _ := featureUses["ki"]
	return featureUses, kiRemaining, nil
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
