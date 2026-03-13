package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// BardicInspirationDie returns the die type for Bardic Inspiration at the given Bard level.
// d6 (1-4), d8 (5-9), d10 (10-14), d12 (15+).
func BardicInspirationDie(bardLevel int) string {
	if bardLevel >= 15 {
		return "d12"
	}
	if bardLevel >= 10 {
		return "d10"
	}
	if bardLevel >= 5 {
		return "d8"
	}
	return "d6"
}

// BardicInspirationMaxUses returns the maximum number of Bardic Inspiration uses
// based on CHA modifier (minimum 1).
func BardicInspirationMaxUses(chaScore int) int {
	mod := AbilityModifier(chaScore)
	if mod < 1 {
		return 1
	}
	return mod
}

// BardicInspirationRechargeType returns "long" or "short" based on Bard level.
// At level 5+ (Font of Inspiration), recharges on short rest.
func BardicInspirationRechargeType(bardLevel int) string {
	if bardLevel >= 5 {
		return "short"
	}
	return "long"
}

// ValidateBardicInspiration checks all preconditions for granting Bardic Inspiration.
func ValidateBardicInspiration(bardLevel int, targetHasInspiration bool, usesRemaining int) error {
	if bardLevel <= 0 {
		return fmt.Errorf("Bardic Inspiration requires Bard class")
	}
	if targetHasInspiration {
		return fmt.Errorf("target already has Bardic Inspiration")
	}
	if usesRemaining <= 0 {
		return fmt.Errorf("no Bardic Inspiration uses remaining")
	}
	return nil
}

// FormatBardicInspirationGrant returns the combat log string for granting Bardic Inspiration.
func FormatBardicInspirationGrant(sourceName, targetName, die string) string {
	return fmt.Sprintf("\U0001f3b5 %s grants Bardic Inspiration (%s) to %s", sourceName, die, targetName)
}

// FormatBardicInspirationNotification returns the notification sent to the target.
func FormatBardicInspirationNotification(die, sourceName string) string {
	return fmt.Sprintf("\U0001f3b5 You received Bardic Inspiration (%s) from %s! You can add it to one attack roll, ability check, or saving throw.", die, sourceName)
}

// FormatBardicInspirationUse returns the combat log string for using Bardic Inspiration.
func FormatBardicInspirationUse(name string, dieResult int, die string, newTotal int) string {
	return fmt.Sprintf("\U0001f3b5 %s uses Bardic Inspiration \u2014 +%d (%s) \u2192 new total: %d", name, dieResult, die, newTotal)
}

// FormatBardicInspirationExpired returns the notification for expired Bardic Inspiration.
func FormatBardicInspirationExpired(sourceName string) string {
	return fmt.Sprintf("\U0001f3b5 Your Bardic Inspiration from %s has expired.", sourceName)
}

// FormatBardicInspirationStatus returns the turn status display for Bardic Inspiration.
func FormatBardicInspirationStatus(die string) string {
	return fmt.Sprintf("\U0001f3b5 Bardic Inspiration (%s)", die)
}

// bardLevelFromJSON returns the bard level from character classes JSON.
func bardLevelFromJSON(classesJSON []byte) int {
	if len(classesJSON) == 0 {
		return 0
	}
	var classes []CharacterClass
	if err := json.Unmarshal(classesJSON, &classes); err != nil {
		return 0
	}
	return classLevel(classes, "Bard")
}

// HasBardClass checks whether a character's classes JSON includes a Bard entry.
func HasBardClass(classesJSON json.RawMessage) bool {
	return bardLevelFromJSON(classesJSON) > 0
}

// BardicInspirationExpirationDuration is the real-time expiration for Bardic Inspiration.
const BardicInspirationExpirationDuration = 10 * time.Minute

// IsBardicInspirationExpired checks if the granted-at time is past the expiration window.
func IsBardicInspirationExpired(grantedAt time.Time, now time.Time) bool {
	return now.Sub(grantedAt) >= BardicInspirationExpirationDuration
}

// ApplyBardicInspirationToCombatant sets bardic inspiration state on a combatant.
func ApplyBardicInspirationToCombatant(c refdata.Combatant, die, source string, grantedAt time.Time) refdata.Combatant {
	c.BardicInspirationDie = sql.NullString{String: die, Valid: true}
	c.BardicInspirationSource = sql.NullString{String: source, Valid: true}
	c.BardicInspirationGrantedAt = sql.NullTime{Time: grantedAt, Valid: true}
	return c
}

// ClearBardicInspirationFromCombatant removes bardic inspiration state from a combatant.
func ClearBardicInspirationFromCombatant(c refdata.Combatant) refdata.Combatant {
	c.BardicInspirationDie = sql.NullString{Valid: false}
	c.BardicInspirationSource = sql.NullString{Valid: false}
	c.BardicInspirationGrantedAt = sql.NullTime{Valid: false}
	return c
}

// CombatantHasBardicInspiration returns true if the combatant has active Bardic Inspiration.
func CombatantHasBardicInspiration(c refdata.Combatant) bool {
	return c.BardicInspirationDie.Valid && c.BardicInspirationDie.String != ""
}

// parseBardicInspirationUses extracts bardic-inspiration uses from character feature_uses JSON.
func parseBardicInspirationUses(char refdata.Character) (map[string]int, int, error) {
	featureUses := make(map[string]int)
	if char.FeatureUses.Valid && len(char.FeatureUses.RawMessage) > 0 {
		if err := json.Unmarshal(char.FeatureUses.RawMessage, &featureUses); err != nil {
			return nil, 0, fmt.Errorf("parsing feature_uses: %w", err)
		}
	}
	remaining, _ := featureUses["bardic-inspiration"]
	return featureUses, remaining, nil
}

// BardicInspirationCommand holds the service-level inputs for granting Bardic Inspiration.
type BardicInspirationCommand struct {
	Bard   refdata.Combatant
	Target refdata.Combatant
	Turn   refdata.Turn
}

// BardicInspirationResult holds the result of granting Bardic Inspiration.
type BardicInspirationResult struct {
	Bard         refdata.Combatant
	Target       refdata.Combatant
	Turn         refdata.Turn
	CombatLog    string
	Notification string
	Remaining    string
	UsesLeft     int
	Die          string
}

// GrantBardicInspiration handles the /bonus bardic-inspiration command.
func (s *Service) GrantBardicInspiration(ctx context.Context, cmd BardicInspirationCommand) (BardicInspirationResult, error) {
	// Validate bonus action
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return BardicInspirationResult{}, err
	}

	// Must be a PC
	if !cmd.Bard.CharacterID.Valid {
		return BardicInspirationResult{}, fmt.Errorf("Bardic Inspiration requires a character (not NPC)")
	}

	// Cannot target self
	if cmd.Bard.ID == cmd.Target.ID {
		return BardicInspirationResult{}, fmt.Errorf("cannot grant Bardic Inspiration to yourself")
	}

	// Get character
	char, err := s.store.GetCharacter(ctx, cmd.Bard.CharacterID.UUID)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("getting character: %w", err)
	}

	// Must be bard
	bl := bardLevelFromJSON(char.Classes)
	if bl <= 0 {
		return BardicInspirationResult{}, fmt.Errorf("Bardic Inspiration requires Bard class")
	}

	// Check target doesn't already have inspiration
	targetHasInspiration := CombatantHasBardicInspiration(cmd.Target)

	// Parse feature_uses
	featureUses, usesRemaining, err := parseBardicInspirationUses(char)
	if err != nil {
		return BardicInspirationResult{}, err
	}

	// Validate
	if err := ValidateBardicInspiration(bl, targetHasInspiration, usesRemaining); err != nil {
		return BardicInspirationResult{}, err
	}

	// Deduct use
	newUsesRemaining := usesRemaining - 1
	featureUses["bardic-inspiration"] = newUsesRemaining
	featureUsesJSON, err := json.Marshal(featureUses)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("marshaling feature_uses: %w", err)
	}
	if _, err := s.store.UpdateCharacterFeatureUses(ctx, refdata.UpdateCharacterFeatureUsesParams{
		ID:          char.ID,
		FeatureUses: pqtype.NullRawMessage{RawMessage: featureUsesJSON, Valid: true},
	}); err != nil {
		return BardicInspirationResult{}, fmt.Errorf("updating feature_uses: %w", err)
	}

	// Use bonus action
	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("using bonus action: %w", err)
	}

	// Persist turn
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return BardicInspirationResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	// Determine die
	die := BardicInspirationDie(bl)

	// Apply inspiration to target
	now := time.Now()
	updatedTarget := ApplyBardicInspirationToCombatant(cmd.Target, die, cmd.Bard.DisplayName, now)
	updatedTarget, err = s.persistBardicInspirationState(ctx, updatedTarget)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("updating combatant bardic inspiration: %w", err)
	}

	combatLog := FormatBardicInspirationGrant(cmd.Bard.DisplayName, cmd.Target.DisplayName, die)
	notification := FormatBardicInspirationNotification(die, cmd.Bard.DisplayName)
	remaining := FormatRemainingResources(updatedTurn)

	return BardicInspirationResult{
		Bard:         cmd.Bard,
		Target:       updatedTarget,
		Turn:         updatedTurn,
		CombatLog:    combatLog,
		Notification: notification,
		Remaining:    remaining,
		UsesLeft:     newUsesRemaining,
		Die:          die,
	}, nil
}

// persistBardicInspirationState saves the combatant's bardic inspiration fields to the database.
func (s *Service) persistBardicInspirationState(ctx context.Context, c refdata.Combatant) (refdata.Combatant, error) {
	return s.store.UpdateCombatantBardicInspiration(ctx, refdata.UpdateCombatantBardicInspirationParams{
		ID:                         c.ID,
		BardicInspirationDie:       c.BardicInspirationDie,
		BardicInspirationSource:    c.BardicInspirationSource,
		BardicInspirationGrantedAt: c.BardicInspirationGrantedAt,
	})
}
