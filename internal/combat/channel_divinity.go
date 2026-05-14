package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/dmqueue"
	"github.com/ab/dndnd/internal/refdata"
)

const FeatureKeyChannelDivinity = "channel-divinity"

// ChannelDivinityMaxUses returns the maximum Channel Divinity uses per short rest
// for the given class and level. Returns 0 if the class/level doesn't grant it.
func ChannelDivinityMaxUses(className string, level int) int {
	switch strings.ToLower(className) {
	case "cleric":
		if level >= 18 {
			return 3
		}
		if level >= 6 {
			return 2
		}
		if level >= 2 {
			return 1
		}
		return 0
	case "paladin":
		if level >= 15 {
			return 2
		}
		if level >= 3 {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// DestroyUndeadCRThreshold returns the CR threshold for Destroy Undead at the given
// cleric level. Undead failing Turn Undead with CR at or below this threshold are
// instantly destroyed. Returns (threshold, true) if active, or (0, false) if below level 5.
func DestroyUndeadCRThreshold(clericLevel int) (float64, bool) {
	if clericLevel >= 17 {
		return 4.0, true
	}
	if clericLevel >= 14 {
		return 3.0, true
	}
	if clericLevel >= 11 {
		return 2.0, true
	}
	if clericLevel >= 8 {
		return 1.0, true
	}
	if clericLevel >= 5 {
		return 0.5, true
	}
	return 0, false
}

// ValidateChannelDivinity checks all preconditions for using Channel Divinity.
func ValidateChannelDivinity(className string, classLevel int, usesRemaining int) error {
	lower := strings.ToLower(className)
	if lower != "cleric" && lower != "paladin" {
		return fmt.Errorf("Channel Divinity requires Cleric or Paladin class")
	}
	minLevel := 2
	if lower == "paladin" {
		minLevel = 3
	}
	if classLevel < minLevel {
		return fmt.Errorf("Channel Divinity requires %s level %d+", className, minLevel)
	}
	if usesRemaining <= 0 {
		return fmt.Errorf("no Channel Divinity uses remaining")
	}
	return nil
}

// SpellSaveDC calculates the spell save DC for a spellcaster.
// DC = 8 + proficiency bonus + spellcasting ability modifier.
func SpellSaveDC(profBonus int, abilityScore int) int {
	return 8 + profBonus + AbilityModifier(abilityScore)
}

// TurnUndeadCommand holds inputs for the Turn Undead Channel Divinity option.
type TurnUndeadCommand struct {
	Cleric       refdata.Combatant
	Turn         refdata.Turn
	CurrentRound int
}

// TurnUndeadTargetResult holds the result for a single undead target.
type TurnUndeadTargetResult struct {
	CombatantID   string
	DisplayName   string
	SaveRoll      int
	SaveTotal     int
	SaveBonus     int
	SaveSucceeded bool
	Turned        bool
	Destroyed     bool
	CR            string
}

// TurnUndeadResult holds the full result of Turn Undead.
type TurnUndeadResult struct {
	Targets      []TurnUndeadTargetResult
	DC           int
	UsesLeft     int
	CombatLog    string
	Turn         refdata.Turn
}

// resolveTargetWisSave resolves a creature's WIS save bonus.
func (s *Service) resolveTargetWisSave(ctx context.Context, target refdata.Combatant) (int, error) {
	if target.CreatureRefID.Valid && target.CreatureRefID.String != "" {
		creature, err := s.store.GetCreature(ctx, target.CreatureRefID.String)
		if err != nil {
			return 0, fmt.Errorf("getting creature for save: %w", err)
		}
		if creature.SavingThrows.Valid && len(creature.SavingThrows.RawMessage) > 0 {
			var saves map[string]int
			if err := json.Unmarshal(creature.SavingThrows.RawMessage, &saves); err == nil {
				if wisSave, ok := saves["wis"]; ok {
					return wisSave, nil
				}
			}
		}
		scores, err := ParseAbilityScores(creature.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing creature ability scores: %w", err)
		}
		return AbilityModifier(scores.Wis), nil
	}
	if target.CharacterID.Valid {
		char, err := s.store.GetCharacter(ctx, target.CharacterID.UUID)
		if err != nil {
			return 0, fmt.Errorf("getting target character: %w", err)
		}
		scores, err := ParseAbilityScores(char.AbilityScores)
		if err != nil {
			return 0, fmt.Errorf("parsing target ability scores: %w", err)
		}
		return AbilityModifier(scores.Wis), nil
	}
	return 0, nil
}

// TurnUndead handles the /action channel-divinity turn-undead command.
// All undead within 30ft make a WIS save vs the Cleric's spell save DC.
// On fail: Turned condition (10 rounds). If Cleric 5+ and undead CR below threshold,
// destroyed instead.
func (s *Service) TurnUndead(ctx context.Context, cmd TurnUndeadCommand, roller *dice.Roller) (TurnUndeadResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return TurnUndeadResult{}, err
	}
	if !cmd.Cleric.CharacterID.Valid {
		return TurnUndeadResult{}, fmt.Errorf("Turn Undead requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Cleric.CharacterID.UUID)
	if err != nil {
		return TurnUndeadResult{}, fmt.Errorf("getting character: %w", err)
	}

	clericLevel := ClassLevelFromJSON(char.Classes, "Cleric")
	featureUses, usesRemaining, err := ParseFeatureUses(char, FeatureKeyChannelDivinity)
	if err != nil {
		return TurnUndeadResult{}, err
	}
	if err := ValidateChannelDivinity("Cleric", clericLevel, usesRemaining); err != nil {
		return TurnUndeadResult{}, err
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return TurnUndeadResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}

	dc := SpellSaveDC(int(char.ProficiencyBonus), scores.Wis)
	destroyCR, destroyActive := DestroyUndeadCRThreshold(clericLevel)

	// Deduct use
	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyChannelDivinity, featureUses, usesRemaining)
	if err != nil {
		return TurnUndeadResult{}, err
	}

	// Use action
	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return TurnUndeadResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return TurnUndeadResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Find all undead within 30ft
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, cmd.Cleric.EncounterID)
	if err != nil {
		return TurnUndeadResult{}, fmt.Errorf("listing combatants: %w", err)
	}

	var targets []TurnUndeadTargetResult
	var logParts []string

	for _, c := range combatants {
		if c.ID == cmd.Cleric.ID {
			continue
		}
		if !c.CreatureRefID.Valid || c.CreatureRefID.String == "" {
			continue
		}
		if !c.IsAlive || c.HpCurrent <= 0 {
			continue
		}

		creature, err := s.store.GetCreature(ctx, c.CreatureRefID.String)
		if err != nil {
			continue
		}
		if strings.ToLower(creature.Type) != "undead" {
			continue
		}

		dist := combatantDistance(cmd.Cleric, c)
		if dist > 30 {
			continue
		}

		// Resolve WIS save
		wisBonus, err := s.resolveTargetWisSave(ctx, c)
		if err != nil {
			continue
		}

		d20Result, err := roller.RollD20(wisBonus, dice.Normal)
		if err != nil {
			continue
		}

		saveTotal := d20Result.Total
		saveSucceeded := saveTotal >= dc

		tr := TurnUndeadTargetResult{
			CombatantID:   c.ID.String(),
			DisplayName:   c.DisplayName,
			SaveRoll:      d20Result.Chosen,
			SaveTotal:     saveTotal,
			SaveBonus:     wisBonus,
			SaveSucceeded: saveSucceeded,
			CR:            creature.Cr,
		}

		if saveSucceeded {
			logParts = append(logParts, fmt.Sprintf("%s \U0001f3b2 WIS save: %d vs DC %d — Resists!", c.DisplayName, saveTotal, dc))
		} else {
			// Check Destroy Undead
			creatureCR := ParseCR(creature.Cr)
			if destroyActive && creatureCR <= destroyCR {
				// Destroy the undead. Routed through ApplyDamage with
				// Override=true so the Phase 118 hooks (concentration
				// save, unconscious-at-0) still fire, while skipping
				// R/I/V — Destroy Undead is an outright destruction
				// effect and ignores damage-type modifiers.
				tr.Destroyed = true
				if _, err := s.ApplyDamage(ctx, ApplyDamageInput{
					EncounterID: c.EncounterID,
					Target:      c,
					RawDamage:   int(c.HpCurrent),
					Override:    true,
				}); err != nil {
					return TurnUndeadResult{}, fmt.Errorf("destroying undead %s: %w", c.DisplayName, err)
				}
				logParts = append(logParts, fmt.Sprintf("✝️ %s (CR %s) is destroyed by Turn Undead!", c.DisplayName, creature.Cr))
			} else {
				// Apply Turned condition
				tr.Turned = true
				turnedCond := CombatCondition{
					Condition:         "turned",
					DurationRounds:    10,
					StartedRound:      cmd.CurrentRound,
					SourceCombatantID: cmd.Cleric.ID.String(),
					ExpiresOn:         "end_of_turn",
				}
				if _, _, err := s.ApplyCondition(ctx, c.ID, turnedCond); err != nil {
					return TurnUndeadResult{}, fmt.Errorf("applying turned condition to %s: %w", c.DisplayName, err)
				}
				logParts = append(logParts, fmt.Sprintf("%s \U0001f3b2 WIS save: %d vs DC %d — Turned!", c.DisplayName, saveTotal, dc))
			}
		}

		targets = append(targets, tr)
	}

	combatLog := fmt.Sprintf("✝️ %s channels Turn Undead", cmd.Cleric.DisplayName)
	if len(logParts) > 0 {
		combatLog += " — " + strings.Join(logParts, " | ")
	}

	return TurnUndeadResult{
		Targets:   targets,
		DC:        dc,
		UsesLeft:  newUsesRemaining,
		CombatLog: combatLog,
		Turn:      updatedTurn,
	}, nil
}

// PreserveLifeCommand holds inputs for the Preserve Life Channel Divinity option.
type PreserveLifeCommand struct {
	Cleric       refdata.Combatant
	Turn         refdata.Turn
	// TargetHealing maps combatant ID string to HP to restore.
	TargetHealing map[string]int32
}

// PreserveLifeResult holds the result of Preserve Life.
type PreserveLifeResult struct {
	HealedTargets []PreserveLifeHeal
	UsesLeft      int
	CombatLog     string
	Turn          refdata.Turn
}

// PreserveLifeHeal holds the healing for a single target.
type PreserveLifeHeal struct {
	CombatantID string
	DisplayName string
	HPRestored  int32
	HPAfter     int32
}

// DMQueueResult holds the result for a DM-resolved Channel Divinity option.
type DMQueueResult struct {
	UsesLeft      int
	CombatLog     string
	Turn          refdata.Turn
	OptionName    string
	DMQueueItemID string
}

// SacredWeaponCommand holds inputs for Sacred Weapon (Devotion Paladin).
type SacredWeaponCommand struct {
	Paladin      refdata.Combatant
	Turn         refdata.Turn
	CurrentRound int
}

// SacredWeaponResult holds the result of Sacred Weapon activation.
type SacredWeaponResult struct {
	UsesLeft    int
	CombatLog   string
	Turn        refdata.Turn
	CHAModifier int
}

// SacredWeapon handles the Sacred Weapon (Devotion Paladin) Channel Divinity option.
// Adds CHA modifier to attack rolls for 1 minute (10 rounds).
func (s *Service) SacredWeapon(ctx context.Context, cmd SacredWeaponCommand) (SacredWeaponResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return SacredWeaponResult{}, err
	}
	if !cmd.Paladin.CharacterID.Valid {
		return SacredWeaponResult{}, fmt.Errorf("Sacred Weapon requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Paladin.CharacterID.UUID)
	if err != nil {
		return SacredWeaponResult{}, fmt.Errorf("getting character: %w", err)
	}

	paladinLevel := ClassLevelFromJSON(char.Classes, "Paladin")
	featureUses, usesRemaining, err := ParseFeatureUses(char, FeatureKeyChannelDivinity)
	if err != nil {
		return SacredWeaponResult{}, err
	}
	if err := ValidateChannelDivinity("Paladin", paladinLevel, usesRemaining); err != nil {
		return SacredWeaponResult{}, err
	}

	scores, err := ParseAbilityScores(char.AbilityScores)
	if err != nil {
		return SacredWeaponResult{}, fmt.Errorf("parsing ability scores: %w", err)
	}
	chaMod := max(AbilityModifier(scores.Cha), 1)

	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyChannelDivinity, featureUses, usesRemaining)
	if err != nil {
		return SacredWeaponResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return SacredWeaponResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return SacredWeaponResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Apply sacred_weapon condition
	cond := CombatCondition{
		Condition:         "sacred_weapon",
		DurationRounds:    10,
		StartedRound:      cmd.CurrentRound,
		SourceCombatantID: cmd.Paladin.ID.String(),
		ExpiresOn:         "end_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, cmd.Paladin.ID, cond); err != nil {
		return SacredWeaponResult{}, fmt.Errorf("applying sacred_weapon condition: %w", err)
	}

	combatLog := fmt.Sprintf("✝️ %s channels Sacred Weapon — +%d to attack rolls for 1 minute", cmd.Paladin.DisplayName, chaMod)

	return SacredWeaponResult{
		UsesLeft:    newUsesRemaining,
		CombatLog:   combatLog,
		Turn:        updatedTurn,
		CHAModifier: chaMod,
	}, nil
}

// VowOfEnmityCommand holds inputs for Vow of Enmity (Vengeance Paladin).
type VowOfEnmityCommand struct {
	Paladin      refdata.Combatant
	Target       refdata.Combatant
	Turn         refdata.Turn
	CurrentRound int
}

// VowOfEnmityResult holds the result of Vow of Enmity activation.
type VowOfEnmityResult struct {
	UsesLeft  int
	CombatLog string
	Turn      refdata.Turn
}

// VowOfEnmity handles the Vow of Enmity (Vengeance Paladin) Channel Divinity option.
// Grants advantage on attack rolls against one creature within 10ft for 1 minute.
func (s *Service) VowOfEnmity(ctx context.Context, cmd VowOfEnmityCommand) (VowOfEnmityResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return VowOfEnmityResult{}, err
	}
	if !cmd.Paladin.CharacterID.Valid {
		return VowOfEnmityResult{}, fmt.Errorf("Vow of Enmity requires a character (not NPC)")
	}

	// Range check: 10ft
	dist := combatantDistance(cmd.Paladin, cmd.Target)
	if dist > 10 {
		return VowOfEnmityResult{}, fmt.Errorf("target is out of range — %dft away (max 10ft)", dist)
	}

	char, err := s.store.GetCharacter(ctx, cmd.Paladin.CharacterID.UUID)
	if err != nil {
		return VowOfEnmityResult{}, fmt.Errorf("getting character: %w", err)
	}

	paladinLevel := ClassLevelFromJSON(char.Classes, "Paladin")
	featureUses, usesRemaining, err := ParseFeatureUses(char, FeatureKeyChannelDivinity)
	if err != nil {
		return VowOfEnmityResult{}, err
	}
	if err := ValidateChannelDivinity("Paladin", paladinLevel, usesRemaining); err != nil {
		return VowOfEnmityResult{}, err
	}

	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyChannelDivinity, featureUses, usesRemaining)
	if err != nil {
		return VowOfEnmityResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return VowOfEnmityResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return VowOfEnmityResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Apply vow_of_enmity condition to the target
	cond := CombatCondition{
		Condition:         "vow_of_enmity",
		DurationRounds:    10,
		StartedRound:      cmd.CurrentRound,
		SourceCombatantID: cmd.Paladin.ID.String(),
		ExpiresOn:         "end_of_turn",
	}
	if _, _, err := s.ApplyCondition(ctx, cmd.Target.ID, cond); err != nil {
		return VowOfEnmityResult{}, fmt.Errorf("applying vow_of_enmity condition: %w", err)
	}

	combatLog := fmt.Sprintf("✝️ %s channels Vow of Enmity against %s — advantage on attacks for 1 minute", cmd.Paladin.DisplayName, cmd.Target.DisplayName)

	return VowOfEnmityResult{
		UsesLeft:  newUsesRemaining,
		CombatLog: combatLog,
		Turn:      updatedTurn,
	}, nil
}

// ChannelDivinityDMQueueCommand holds inputs for DM-resolved Channel Divinity options.
type ChannelDivinityDMQueueCommand struct {
	Caster     refdata.Combatant
	Turn       refdata.Turn
	OptionName string
	ClassName  string
	GuildID    string
	CampaignID string
}

// ChannelDivinityDMQueue handles DM-resolved Channel Divinity options by routing to #dm-queue.
// Deducts a use and the action, then returns a log message directing the DM to resolve.
func (s *Service) ChannelDivinityDMQueue(ctx context.Context, cmd ChannelDivinityDMQueueCommand) (DMQueueResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return DMQueueResult{}, err
	}
	if !cmd.Caster.CharacterID.Valid {
		return DMQueueResult{}, fmt.Errorf("Channel Divinity requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Caster.CharacterID.UUID)
	if err != nil {
		return DMQueueResult{}, fmt.Errorf("getting character: %w", err)
	}

	classLevel := ClassLevelFromJSON(char.Classes, cmd.ClassName)
	featureUses, usesRemaining, err := ParseFeatureUses(char, FeatureKeyChannelDivinity)
	if err != nil {
		return DMQueueResult{}, err
	}
	if err := ValidateChannelDivinity(cmd.ClassName, classLevel, usesRemaining); err != nil {
		return DMQueueResult{}, err
	}

	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyChannelDivinity, featureUses, usesRemaining)
	if err != nil {
		return DMQueueResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return DMQueueResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return DMQueueResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	combatLog := fmt.Sprintf("✝️ %s channels %s — routed to #dm-queue for DM resolution", cmd.Caster.DisplayName, cmd.OptionName)

	dmItemID, err := s.postChannelDivinityToDMQueue(ctx, cmd)
	if err != nil {
		return DMQueueResult{}, fmt.Errorf("posting to dm queue: %w", err)
	}

	return DMQueueResult{
		UsesLeft:      newUsesRemaining,
		CombatLog:     combatLog,
		Turn:          updatedTurn,
		OptionName:    cmd.OptionName,
		DMQueueItemID: dmItemID,
	}, nil
}

// postChannelDivinityToDMQueue dispatches a Channel Divinity notification
// through the wired Notifier (when present) and returns the resulting item ID.
// When no notifier is wired the call is a silent no-op.
func (s *Service) postChannelDivinityToDMQueue(ctx context.Context, cmd ChannelDivinityDMQueueCommand) (string, error) {
	if s.dmNotifier == nil {
		return "", nil
	}
	return s.dmNotifier.Post(ctx, dmqueue.Event{
		Kind:       dmqueue.KindChannelDivinity,
		PlayerName: cmd.Caster.DisplayName,
		Summary:    cmd.OptionName,
		GuildID:    cmd.GuildID,
		CampaignID: cmd.CampaignID,
	})
}

// PreserveLife handles the Preserve Life (Life Domain) Channel Divinity option.
// Distributes up to 5 * cleric_level HP among creatures within 30ft,
// each restored to at most half their max HP.
func (s *Service) PreserveLife(ctx context.Context, cmd PreserveLifeCommand) (PreserveLifeResult, error) {
	if err := ValidateResource(cmd.Turn, ResourceAction); err != nil {
		return PreserveLifeResult{}, err
	}
	if !cmd.Cleric.CharacterID.Valid {
		return PreserveLifeResult{}, fmt.Errorf("Preserve Life requires a character (not NPC)")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Cleric.CharacterID.UUID)
	if err != nil {
		return PreserveLifeResult{}, fmt.Errorf("getting character: %w", err)
	}

	clericLevel := ClassLevelFromJSON(char.Classes, "Cleric")
	featureUses, usesRemaining, err := ParseFeatureUses(char, FeatureKeyChannelDivinity)
	if err != nil {
		return PreserveLifeResult{}, err
	}
	if err := ValidateChannelDivinity("Cleric", clericLevel, usesRemaining); err != nil {
		return PreserveLifeResult{}, err
	}

	budget := int32(5 * clericLevel)

	// Build a map of combatants for quick lookup
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, cmd.Cleric.EncounterID)
	if err != nil {
		return PreserveLifeResult{}, fmt.Errorf("listing combatants: %w", err)
	}
	combatantMap := make(map[string]refdata.Combatant, len(combatants))
	for _, c := range combatants {
		combatantMap[c.ID.String()] = c
	}

	// Validate targets and total budget
	var totalHealing int32
	for targetID, heal := range cmd.TargetHealing {
		target, ok := combatantMap[targetID]
		if !ok {
			return PreserveLifeResult{}, fmt.Errorf("target %q not found in encounter", targetID)
		}

		dist := combatantDistance(cmd.Cleric, target)
		if dist > 30 {
			return PreserveLifeResult{}, fmt.Errorf("%s is out of range (30ft)", target.DisplayName)
		}

		halfMax := target.HpMax / 2
		if target.HpCurrent >= halfMax {
			return PreserveLifeResult{}, fmt.Errorf("%s is above half max HP (%d/%d)", target.DisplayName, target.HpCurrent, target.HpMax)
		}

		maxHeal := halfMax - target.HpCurrent
		if heal > maxHeal {
			return PreserveLifeResult{}, fmt.Errorf("healing %s by %d would exceed half max HP (can heal at most %d)", target.DisplayName, heal, maxHeal)
		}

		totalHealing += heal
	}

	if totalHealing > budget {
		return PreserveLifeResult{}, fmt.Errorf("total healing %d exceeds budget %d", totalHealing, budget)
	}

	// Deduct use
	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyChannelDivinity, featureUses, usesRemaining)
	if err != nil {
		return PreserveLifeResult{}, err
	}

	// Use action
	updatedTurn, err := UseResource(cmd.Turn, ResourceAction)
	if err != nil {
		return PreserveLifeResult{}, err
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return PreserveLifeResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Apply healing
	var healed []PreserveLifeHeal
	var logParts []string
	for targetID, heal := range cmd.TargetHealing {
		target := combatantMap[targetID]
		newHP := target.HpCurrent + heal

		if _, err := s.store.UpdateCombatantHP(ctx, refdata.UpdateCombatantHPParams{
			ID:        target.ID,
			HpCurrent: newHP,
			TempHp:    target.TempHp,
			IsAlive:   true,
		}); err != nil {
			return PreserveLifeResult{}, fmt.Errorf("updating HP for %s: %w", target.DisplayName, err)
		}
		// C-43 / Phase 43: lifting a dying combatant back above 0 HP must
		// reset death-save tallies and clear the dying-condition bundle.
		if _, err := s.MaybeResetDeathSavesOnHeal(ctx, target, newHP); err != nil {
			return PreserveLifeResult{}, fmt.Errorf("resetting death state for %s: %w", target.DisplayName, err)
		}

		healed = append(healed, PreserveLifeHeal{
			CombatantID: targetID,
			DisplayName: target.DisplayName,
			HPRestored:  heal,
			HPAfter:     newHP,
		})
		logParts = append(logParts, fmt.Sprintf("%s +%dHP (%d/%d)", target.DisplayName, heal, newHP, target.HpMax))
	}

	combatLog := fmt.Sprintf("✝️ %s channels Preserve Life — %s", cmd.Cleric.DisplayName, strings.Join(logParts, " | "))

	return PreserveLifeResult{
		HealedTargets: healed,
		UsesLeft:      newUsesRemaining,
		CombatLog:     combatLog,
		Turn:          updatedTurn,
	}, nil
}
