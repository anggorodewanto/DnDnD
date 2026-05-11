package combat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/ab/dndnd/internal/refdata"
)

// TokenState represents a combatant's visual state on the map.
type TokenState string

const (
	TokenAlive  TokenState = "alive"
	TokenDying  TokenState = "dying"
	TokenDead   TokenState = "dead"
	TokenStable TokenState = "stable"
)

// DeathSaveOutcome represents the result of processing a death save event.
type DeathSaveOutcome struct {
	DeathSaves DeathSaves
	TokenState TokenState
	HPCurrent  int
	IsAlive    bool
	IsDying    bool
	IsStable   bool
	Messages   []string
}

// ParseDeathSaves parses death saves from JSON (either raw or NullRawMessage).
func ParseDeathSaves(raw json.RawMessage) (DeathSaves, error) {
	if len(raw) == 0 {
		return DeathSaves{}, nil
	}
	var ds DeathSaves
	if err := json.Unmarshal(raw, &ds); err != nil {
		return DeathSaves{}, fmt.Errorf("parsing death saves: %w", err)
	}
	return ds, nil
}

// MarshalDeathSaves marshals death saves to a NullRawMessage.
func MarshalDeathSaves(ds DeathSaves) pqtype.NullRawMessage {
	b, _ := json.Marshal(ds)
	return pqtype.NullRawMessage{RawMessage: b, Valid: true}
}

// CheckInstantDeath returns true if overflow damage >= max HP (instant death).
func CheckInstantDeath(overflowDamage, maxHP int) bool {
	return overflowDamage >= maxHP
}

// ProcessDropToZeroHP determines what happens when a character drops to 0 HP.
// overflowDamage is the remaining damage after HP reaches 0.
// Returns the outcome including token state, messages, and whether it's instant death.
func ProcessDropToZeroHP(name string, overflowDamage, maxHP int) DeathSaveOutcome {
	if CheckInstantDeath(overflowDamage, maxHP) {
		return DeathSaveOutcome{
			TokenState: TokenDead,
			Messages: []string{
				fmt.Sprintf("💀  %s is killed outright! (%d overflow damage ≥ %d max HP — instant death, no death saves)", name, overflowDamage, maxHP),
			},
		}
	}

	return DeathSaveOutcome{
		TokenState: TokenDying,
		IsAlive:    true,
		IsDying:    true,
		Messages: []string{
			fmt.Sprintf("💔  %s drops to 0 HP — unconscious and dying (death saves begin next turn)", name),
		},
	}
}

// RollDeathSave processes a death saving throw with the given d20 roll value.
// Applies nat 20 (regain 1 HP), nat 1 (2 failures), and normal success/failure rules.
func RollDeathSave(name string, ds DeathSaves, roll int) DeathSaveOutcome {
	if roll == 20 {
		return DeathSaveOutcome{
			TokenState: TokenAlive,
			HPCurrent:  1,
			IsAlive:    true,
			Messages: []string{
				fmt.Sprintf("🎲  %s rolls death save — 🎯 NAT 20 — %s regains 1 HP and is conscious! (tallies reset)", name, name),
			},
		}
	}

	newDS := ds
	switch {
	case roll == 1:
		newDS.Failures += 2
	case roll >= 10:
		newDS.Successes++
	default:
		newDS.Failures++
	}
	return buildDeathSaveOutcome(name, newDS, roll)
}

// buildDeathSaveOutcome constructs the outcome after updating death save tallies.
func buildDeathSaveOutcome(name string, ds DeathSaves, roll int) DeathSaveOutcome {
	tally := fmt.Sprintf("(%dS / %dF)", ds.Successes, ds.Failures)
	rollDesc := deathSaveRollDesc(roll)

	msg := fmt.Sprintf("🎲  %s rolls death save — %s %s", name, rollDesc, tally)

	if ds.Failures >= 3 {
		return DeathSaveOutcome{
			DeathSaves: ds,
			TokenState: TokenDead,
			Messages:   []string{msg + " — dead"},
		}
	}

	if ds.Successes >= 3 {
		return DeathSaveOutcome{
			DeathSaves: ds,
			TokenState: TokenStable,
			IsAlive:    true,
			IsStable:   true,
			Messages:   []string{msg + " — stabilized"},
		}
	}

	return DeathSaveOutcome{
		DeathSaves: ds,
		TokenState: TokenDying,
		IsAlive:    true,
		IsDying:    true,
		Messages:   []string{msg},
	}
}

// deathSaveRollDesc returns the display text for a death save roll value.
func deathSaveRollDesc(roll int) string {
	if roll == 1 {
		return "💥 NAT 1 — 2 failures!"
	}
	if roll >= 10 {
		return fmt.Sprintf("%d — Success", roll)
	}
	return fmt.Sprintf("%d — Failure", roll)
}

// ApplyDamageAtZeroHP processes damage taken while at 0 HP.
// Each hit = 1 failure, critical hit = 2 failures.
func ApplyDamageAtZeroHP(name string, ds DeathSaves, isCrit bool) DeathSaveOutcome {
	failures := 1
	if isCrit {
		failures = 2
	}

	newDS := DeathSaves{Successes: ds.Successes, Failures: ds.Failures + failures}

	isDead := newDS.Failures >= 3
	tokenState := TokenDying
	if isDead {
		tokenState = TokenDead
	}

	var msg string
	if isCrit {
		msg = fmt.Sprintf("⚠️  %s takes a critical hit at 0 HP — 2 death save failures (%dS / %dF)",
			name, newDS.Successes, newDS.Failures)
	} else {
		msg = fmt.Sprintf("⚠️  %s takes damage at 0 HP — 1 death save failure (%dS / %dF)",
			name, newDS.Successes, newDS.Failures)
	}
	if isDead {
		msg += " — dead"
	}

	return DeathSaveOutcome{
		DeathSaves: newDS,
		TokenState: tokenState,
		IsAlive:    !isDead,
		IsDying:    !isDead,
		Messages:   []string{msg},
	}
}

// HealFromZeroHP processes healing received while at 0 HP.
// HP = 0 + healAmount, tallies reset, character is conscious (still prone).
func HealFromZeroHP(name string, ds DeathSaves, healAmount int) DeathSaveOutcome {
	return DeathSaveOutcome{
		TokenState: TokenAlive,
		HPCurrent:  healAmount,
		IsAlive:    true,
		Messages: []string{
			fmt.Sprintf("💚  %s receives %d HP of healing — conscious at %d HP (death save tallies reset)", name, healAmount, healAmount),
		},
	}
}

// StabilizeTarget stabilizes a dying character (e.g., Medicine check DC 10 or Spare the Dying).
// Sets 3 successes, character remains unconscious at 0 HP.
func StabilizeTarget(name string, ds DeathSaves, method string) DeathSaveOutcome {
	return DeathSaveOutcome{
		DeathSaves: DeathSaves{Successes: 3, Failures: ds.Failures},
		TokenState: TokenStable,
		IsAlive:    true,
		IsStable:   true,
		Messages: []string{
			fmt.Sprintf("🩹  %s is stabilized via %s (unconscious at 0 HP, no further death saves)", name, method),
		},
	}
}

// GetTokenState determines the visual token state based on combatant status.
func GetTokenState(isAlive bool, hpCurrent int, ds DeathSaves) TokenState {
	if !isAlive {
		return TokenDead
	}
	if hpCurrent > 0 {
		return TokenAlive
	}
	// At 0 HP and alive
	if ds.Successes >= 3 {
		return TokenStable
	}
	return TokenDying
}

// IsDying returns true if the combatant is alive at 0 HP and not stabilized.
func IsDying(isAlive bool, hpCurrent int, ds DeathSaves) bool {
	if !isAlive || hpCurrent > 0 {
		return false
	}
	return ds.Successes < 3
}

// ConditionsForDying returns the conditions applied when a character drops to 0 HP.
func ConditionsForDying() []CombatCondition {
	return []CombatCondition{
		{Condition: "unconscious", DurationRounds: 0},
		{Condition: "prone", DurationRounds: 0},
	}
}

// MaybeResetDeathSavesOnHeal is the Phase 43 hook every PC heal call site
// funnels through after writing new HP. When the post-heal HP is >0 AND the
// pre-heal HP was <=0 AND the target is a PC, it:
//
//  1. resets death save tallies (successes + failures) via
//     UpdateCombatantDeathSaves,
//  2. removes the dying-condition bundle (unconscious + prone — see
//     ConditionsForDying) so the combatant becomes fully conscious.
//
// Healing a combatant that is not at 0 HP, or healing an NPC, is a silent
// no-op. The caller passes the combatant snapshot taken BEFORE the HP
// update so the routing sees the pre-heal HP. Returns the (possibly
// further-updated) combatant.
func (s *Service) MaybeResetDeathSavesOnHeal(ctx context.Context, preHeal refdata.Combatant, postHealHP int32) (refdata.Combatant, error) {
	if preHeal.IsNpc || preHeal.HpCurrent > 0 || postHealHP <= 0 {
		return preHeal, nil
	}
	return s.resetDyingState(ctx, preHeal.ID)
}

// resetDyingState clears death save tallies and dying conditions for a
// combatant. Used by the heal-from-zero hook and any future stabilize-then-
// wake-up path. The cleared death saves are persisted as a zero-valued
// DeathSaves JSON object so subsequent reads see the reset.
func (s *Service) resetDyingState(ctx context.Context, combatantID uuid.UUID) (refdata.Combatant, error) {
	updated, err := s.store.UpdateCombatantDeathSaves(ctx, refdata.UpdateCombatantDeathSavesParams{
		ID:         combatantID,
		DeathSaves: MarshalDeathSaves(DeathSaves{}),
	})
	if err != nil {
		return refdata.Combatant{}, fmt.Errorf("resetting death saves: %w", err)
	}
	for _, cond := range ConditionsForDying() {
		if !HasCondition(updated.Conditions, cond.Condition) {
			continue
		}
		next, _, rerr := s.RemoveConditionFromCombatant(ctx, combatantID, cond.Condition)
		if rerr != nil {
			return updated, fmt.Errorf("removing %s on heal-from-0: %w", cond.Condition, rerr)
		}
		updated = next
	}
	return updated, nil
}
