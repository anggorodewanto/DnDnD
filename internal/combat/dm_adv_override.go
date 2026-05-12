package combat

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/ab/dndnd/internal/refdata"
)

// dmAdvOverride constants are the legal values of
// combatants.next_attack_adv_override. NULL (sql.NullString.Valid=false)
// means no override.
const (
	dmAdvOverrideAdvantage    = "advantage"
	dmAdvOverrideDisadvantage = "disadvantage"
)

// consumeDMAdvOverride looks at the attacker's persisted
// next_attack_adv_override column and, if present, ORs the corresponding
// flag into dmAdv / dmDisadv (so any caller-supplied override still wins
// when set) and clears the row from the DB. The override is single-use
// per Phase 35 — once an attacker rolls an attack, the override is gone.
//
// Returns the updated (advantage, disadvantage) pair. When attacker has
// no override the flags are returned unchanged and no DB write happens.
//
// A non-fatal clear-store error is logged but does not fail the attack;
// the override has already been read so the next attempt would skip the
// flag anyway. A future migration could harden this to "do not roll
// until the clear succeeds" if duplicate consumption proves problematic.
func (s *Service) consumeDMAdvOverride(
	ctx context.Context,
	attacker refdata.Combatant,
	dmAdv, dmDisadv bool,
) (bool, bool) {
	if !attacker.NextAttackAdvOverride.Valid {
		return dmAdv, dmDisadv
	}
	switch attacker.NextAttackAdvOverride.String {
	case dmAdvOverrideAdvantage:
		dmAdv = true
	case dmAdvOverrideDisadvantage:
		dmDisadv = true
	default:
		// Unknown value — leave flags untouched but still clear so a bad
		// row cannot wedge future rolls.
	}
	if err := s.store.ClearCombatantNextAttackAdvOverride(ctx, attacker.ID); err != nil {
		// Best-effort: surface the failure but do not fail the attack.
		log.Printf("failed to clear DM advantage override for %s: %v", attacker.ID, err)
	}
	return dmAdv, dmDisadv
}

// setDMAdvOverride persists a per-attack advantage/disadvantage override
// for combatantID. mode must be "advantage" or "disadvantage" — callers
// should validate before invoking. mode "" or "none" routes through
// ClearCombatantNextAttackAdvOverride.
func (s *Service) setDMAdvOverride(ctx context.Context, combatantID uuid.UUID, mode string) error {
	if mode == "" || mode == "none" {
		return s.store.ClearCombatantNextAttackAdvOverride(ctx, combatantID)
	}
	if mode != dmAdvOverrideAdvantage && mode != dmAdvOverrideDisadvantage {
		return fmt.Errorf("invalid override mode %q", mode)
	}
	return s.store.SetCombatantNextAttackAdvOverride(ctx, refdata.SetCombatantNextAttackAdvOverrideParams{
		ID:                    combatantID,
		NextAttackAdvOverride: sql.NullString{String: mode, Valid: true},
	})
}
