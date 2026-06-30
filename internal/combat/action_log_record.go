package combat

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// action_log action_type tags for player-driven combat actions. Kept as
// constants so the DM Console / timeline can group by type and tests assert
// against a single source of truth.
const (
	actionTypeCast           = "cast"
	actionTypeFreeformAction = "freeform_action"
	actionTypeAttack         = "attack"
	// actionTypeDowned marks a combatant's above-0 → 0 HP transition (a PC
	// going unconscious / an NPC being defeated) so the DM Console timeline
	// records the drop alongside the hit that caused it.
	actionTypeDowned = "downed"
	// actionTypeRageExpired marks a Barbarian's end-of-turn rage lapse (raged
	// but neither attacked nor took damage this round, so 5e ends it) so the
	// silent drop is visible on the DM Console timeline, not just in the DB.
	actionTypeRageExpired = "rage_expired"
)

// recordCombatAction best-effort persists a player-driven combat action to
// action_log so it surfaces in the DM Console timeline (GET /api/dm/situation).
//
// ISSUE-014: /cast, /attack and /action (freeform) posted to #combat-log but
// never persisted, leaving the Console's timeline blind to player actions
// (only DM-side / automated flows wrote to action_log). This write is pure
// observability, NOT gameplay state: a failed write — or a missing NOT-NULL
// parent (turn/encounter/actor) — must never abort the cast/attack/freeform
// that has already mutated real state, so the error is intentionally swallowed
// and writes with a missing parent are skipped before the insert is attempted.
func (s *Service) recordCombatAction(ctx context.Context, turnID, encounterID, actorID uuid.UUID, targetID uuid.NullUUID, actionType, description string) {
	if turnID == uuid.Nil || encounterID == uuid.Nil || actorID == uuid.Nil {
		return
	}
	_, _ = s.CreateActionLog(ctx, CreateActionLogInput{
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  actionType,
		ActorID:     actorID,
		TargetID:    targetID,
		Description: description,
	})
}

// nullableCombatantID wraps a combatant UUID into uuid.NullUUID, treating the
// zero UUID (self-targeted or area spells with no single target) as NULL.
func nullableCombatantID(id uuid.UUID) uuid.NullUUID {
	if id == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}

// describeCast renders a one-line action-log summary for a resolved single-
// target (or self) cast.
func describeCast(casterName, spellName, targetName string) string {
	if targetName == "" {
		return fmt.Sprintf("%s cast %s", casterName, spellName)
	}
	return fmt.Sprintf("%s cast %s on %s", casterName, spellName, targetName)
}

// describeAoECast renders a one-line action-log summary for an area cast.
func describeAoECast(casterName, spellName string, affected []string) string {
	if len(affected) == 0 {
		return fmt.Sprintf("%s cast %s", casterName, spellName)
	}
	return fmt.Sprintf("%s cast %s on %s", casterName, spellName, strings.Join(affected, ", "))
}

// describeAttack renders a one-line action-log summary for an attack result.
func describeAttack(r AttackResult) string {
	var outcome string
	switch {
	case r.Hit && (r.CriticalHit || r.AutoCrit):
		outcome = fmt.Sprintf("CRIT for %d", r.DamageTotal)
	case r.Hit:
		outcome = fmt.Sprintf("hit for %d", r.DamageTotal)
	default:
		outcome = "missed"
	}
	if r.WeaponName == "" {
		return fmt.Sprintf("%s attacked %s — %s", r.AttackerName, r.TargetName, outcome) + describeCleave(r.CleaveAttack)
	}
	return fmt.Sprintf("%s attacked %s with %s — %s", r.AttackerName, r.TargetName, r.WeaponName, outcome) + describeCleave(r.CleaveAttack)
}

// describeCleave renders the trailing " — Cleave hits/misses <2nd target>"
// clause for a 2024 Cleave-mastery secondary attack, or "" when none occurred.
// ISSUE-031: mirrors the Discord combat log (FormatAttackLog) so the DM Console
// timeline records the extra attack instead of silently dropping its damage.
func describeCleave(c *AttackResult) string {
	if c == nil {
		return ""
	}
	if c.Hit {
		return fmt.Sprintf(" — Cleave hits %s for %d %s", c.TargetName, c.DamageTotal, c.DamageType)
	}
	return fmt.Sprintf(" — Cleave misses %s", c.TargetName)
}
