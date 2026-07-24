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

// recordAttackAction persists an attack to action_log like recordCombatAction,
// but also serializes the structured outcome (hit/crit/damage per swing) into
// the dice_rolls column so EndCombat can aggregate post-combat "fun stats"
// without re-parsing the description prose. Pure observability: the write is
// best-effort and errors are swallowed, exactly as recordCombatAction.
func (s *Service) recordAttackAction(ctx context.Context, turnID, encounterID, actorID uuid.UUID, targetID uuid.NullUUID, r AttackResult) {
	if turnID == uuid.Nil || encounterID == uuid.Nil || actorID == uuid.Nil {
		return
	}
	var tid uuid.UUID
	if targetID.Valid {
		tid = targetID.UUID
	}
	_, _ = s.CreateActionLog(ctx, CreateActionLogInput{
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  actionTypeAttack,
		ActorID:     actorID,
		TargetID:    targetID,
		Description: describeAttack(r),
		DiceRolls:   encodeAttackSwings(attackResultSwings(r, tid)),
	})
}

// recordCastAction persists a resolved spell cast to action_log like
// recordCombatAction, but also serializes any spell-attack swings (Eldritch
// Blast beams / a single-target spell attack, each with its crit + damage) into
// the dice_rolls column so EndCombat's fun stats count spell attacks alongside
// weapon attacks. When swings is empty (save-based or utility casts)
// encodeAttackSwings returns nil → the column stays NULL, exactly as
// recordCombatAction wrote it before. Best-effort observability: errors are
// swallowed and a missing NOT-NULL parent is skipped.
func (s *Service) recordCastAction(ctx context.Context, turnID, encounterID, actorID uuid.UUID, targetID uuid.NullUUID, description string, swings []attackSwing) {
	if turnID == uuid.Nil || encounterID == uuid.Nil || actorID == uuid.Nil {
		return
	}
	_, _ = s.CreateActionLog(ctx, CreateActionLogInput{
		TurnID:      turnID,
		EncounterID: encounterID,
		ActionType:  actionTypeCast,
		ActorID:     actorID,
		TargetID:    targetID,
		Description: description,
		DiceRolls:   encodeAttackSwings(swings),
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
		return fmt.Sprintf("%s attacked %s — %s", r.AttackerName, r.TargetName, outcome) + describeBreakdown(r) + describeCleave(r.CleaveAttack)
	}
	return fmt.Sprintf("%s attacked %s with %s — %s", r.AttackerName, r.TargetName, r.WeaponName, outcome) + describeBreakdown(r) + describeCleave(r.CleaveAttack)
}

// describeBreakdown renders a compact " (incl. +4 necrotic Hex, +3 Great Weapon
// Master)" suffix for the DM action-log line so the timeline records which feat
// riders contributed to a hit. Sneak Attack is skipped (already folded into the
// damage total and not separately tagged in the terse timeline). Returns "" when
// no rider fired.
func describeBreakdown(r AttackResult) string {
	return describeComponents(r.DamageBreakdown)
}

// describeComponents renders the compact " (incl. +4 necrotic Hex, +6 force
// Agonizing Blast)" action-log suffix from a damage breakdown, shared by weapon
// attacks and spell casts. Sneak Attack is skipped (already folded into the hit
// total and not separately tagged in the terse timeline). Returns "" when empty.
func describeComponents(comps []DamageComponent) string {
	var parts []string
	for _, c := range comps {
		if c.SourceName == "Sneak Attack" {
			continue
		}
		if c.DamageType != "" {
			parts = append(parts, fmt.Sprintf("+%d %s %s", c.Amount, c.DamageType, c.SourceName))
		} else {
			parts = append(parts, fmt.Sprintf("+%d %s", c.Amount, c.SourceName))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (incl. " + strings.Join(parts, ", ") + ")"
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
