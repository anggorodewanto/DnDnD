package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/dice"
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

// HasBardClass checks whether a character's classes JSON includes a Bard entry.
func HasBardClass(classesJSON json.RawMessage) bool {
	return ClassLevelFromJSON(classesJSON, "Bard") > 0
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

// BardicInspirationCommand holds the service-level inputs for granting Bardic Inspiration.
type BardicInspirationCommand struct {
	Bard   refdata.Combatant
	Target refdata.Combatant
	Turn   refdata.Turn
	Now    time.Time // optional; defaults to time.Now() if zero
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
	if err := ValidateResource(cmd.Turn, ResourceBonusAction); err != nil {
		return BardicInspirationResult{}, err
	}
	if !cmd.Bard.CharacterID.Valid {
		return BardicInspirationResult{}, fmt.Errorf("Bardic Inspiration requires a character (not NPC)")
	}
	if cmd.Bard.ID == cmd.Target.ID {
		return BardicInspirationResult{}, fmt.Errorf("cannot grant Bardic Inspiration to yourself")
	}

	char, err := s.store.GetCharacter(ctx, cmd.Bard.CharacterID.UUID)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("getting character: %w", err)
	}

	bl := ClassLevelFromJSON(char.Classes, "Bard")
	featureUses, usesRemaining, err := ParseFeatureUses(char, FeatureKeyBardicInspiration)
	if err != nil {
		return BardicInspirationResult{}, err
	}
	if err := ValidateBardicInspiration(bl, CombatantHasBardicInspiration(cmd.Target), usesRemaining); err != nil {
		return BardicInspirationResult{}, err
	}

	newUsesRemaining, err := s.DeductFeatureUse(ctx, char, FeatureKeyBardicInspiration, featureUses, usesRemaining)
	if err != nil {
		return BardicInspirationResult{}, err
	}

	updatedTurn, err := UseResource(cmd.Turn, ResourceBonusAction)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("using bonus action: %w", err)
	}
	if _, err := s.store.UpdateTurnActions(ctx, TurnToUpdateParams(updatedTurn)); err != nil {
		return BardicInspirationResult{}, fmt.Errorf("updating turn actions: %w", err)
	}

	die := BardicInspirationDie(bl)
	now := cmd.Now
	if now.IsZero() {
		now = time.Now()
	}
	updatedTarget := ApplyBardicInspirationToCombatant(cmd.Target, die, cmd.Bard.DisplayName, now)
	updatedTarget, err = s.persistBardicInspirationState(ctx, updatedTarget)
	if err != nil {
		return BardicInspirationResult{}, fmt.Errorf("updating combatant bardic inspiration: %w", err)
	}

	return BardicInspirationResult{
		Bard:         cmd.Bard,
		Target:       updatedTarget,
		Turn:         updatedTurn,
		CombatLog:    FormatBardicInspirationGrant(cmd.Bard.DisplayName, cmd.Target.DisplayName, die),
		Notification: FormatBardicInspirationNotification(die, cmd.Bard.DisplayName),
		Remaining:    FormatRemainingResources(updatedTurn, nil),
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

// UseBardicInspirationCommand holds the inputs for using Bardic Inspiration.
type UseBardicInspirationCommand struct {
	Combatant     refdata.Combatant
	OriginalTotal int
}

// UseBardicInspirationResult holds the result of using Bardic Inspiration.
type UseBardicInspirationResult struct {
	Combatant refdata.Combatant
	DieResult int
	NewTotal  int
	Die       string
	CombatLog string
}

// sweepExpiredBardicInspirations walks the encounter's combatants and clears
// any whose Bardic Inspiration grant is older than the 10-minute window. The
// sweep is best-effort: store errors are logged-and-skipped so turn flow is
// never blocked. (med-43 / Phase 49)
func (s *Service) sweepExpiredBardicInspirations(ctx context.Context, encounterID uuid.UUID) {
	combatants, err := s.store.ListCombatantsByEncounterID(ctx, encounterID)
	if err != nil {
		return
	}
	now := time.Now()
	for _, c := range combatants {
		if !CombatantHasBardicInspiration(c) {
			continue
		}
		if !c.BardicInspirationGrantedAt.Valid {
			continue
		}
		if !IsBardicInspirationExpired(c.BardicInspirationGrantedAt.Time, now) {
			continue
		}
		cleared := ClearBardicInspirationFromCombatant(c)
		_, _ = s.persistBardicInspirationState(ctx, cleared)
	}
}

// UseBardicInspiration rolls the inspiration die, adds it to the original total,
// clears the combatant's inspiration state, and returns the formatted result.
func (s *Service) UseBardicInspiration(ctx context.Context, cmd UseBardicInspirationCommand, roller *dice.Roller) (UseBardicInspirationResult, error) {
	if !CombatantHasBardicInspiration(cmd.Combatant) {
		return UseBardicInspirationResult{}, fmt.Errorf("%s does not have Bardic Inspiration", cmd.Combatant.DisplayName)
	}

	die := cmd.Combatant.BardicInspirationDie.String
	rollResult, err := roller.Roll("1" + die)
	if err != nil {
		return UseBardicInspirationResult{}, fmt.Errorf("rolling bardic inspiration die: %w", err)
	}
	dieResult := rollResult.Total
	newTotal := cmd.OriginalTotal + dieResult

	cleared := ClearBardicInspirationFromCombatant(cmd.Combatant)
	cleared, err = s.persistBardicInspirationState(ctx, cleared)
	if err != nil {
		return UseBardicInspirationResult{}, fmt.Errorf("clearing bardic inspiration: %w", err)
	}

	return UseBardicInspirationResult{
		Combatant: cleared,
		DieResult: dieResult,
		NewTotal:  newTotal,
		Die:       die,
		CombatLog: FormatBardicInspirationUse(cmd.Combatant.DisplayName, dieResult, die, newTotal),
	}, nil
}
