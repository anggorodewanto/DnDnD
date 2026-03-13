package combat

import (
	"encoding/json"
	"fmt"

	"github.com/sqlc-dev/pqtype"
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
			DeathSaves: DeathSaves{},
			TokenState: TokenDead,
			HPCurrent:  0,
			IsAlive:    false,
			IsDying:    false,
			IsStable:   false,
			Messages: []string{
				fmt.Sprintf("💀  %s is killed outright! (%d overflow damage ≥ %d max HP — instant death, no death saves)", name, overflowDamage, maxHP),
			},
		}
	}

	return DeathSaveOutcome{
		DeathSaves: DeathSaves{},
		TokenState: TokenDying,
		HPCurrent:  0,
		IsAlive:    true,
		IsDying:    true,
		IsStable:   false,
		Messages: []string{
			fmt.Sprintf("💔  %s drops to 0 HP — unconscious and dying (death saves begin next turn)", name),
		},
	}
}

// RollDeathSave processes a death saving throw with the given d20 roll value.
// Applies nat 20 (regain 1 HP), nat 1 (2 failures), and normal success/failure rules.
func RollDeathSave(name string, ds DeathSaves, roll int) DeathSaveOutcome {
	// Nat 20: regain 1 HP, tallies reset
	if roll == 20 {
		return DeathSaveOutcome{
			DeathSaves: DeathSaves{},
			TokenState: TokenAlive,
			HPCurrent:  1,
			IsAlive:    true,
			IsDying:    false,
			IsStable:   false,
			Messages: []string{
				fmt.Sprintf("🎲  %s rolls death save — 🎯 NAT 20 — %s regains 1 HP and is conscious! (tallies reset)", name, name),
			},
		}
	}

	// Nat 1: 2 failures
	if roll == 1 {
		newDS := DeathSaves{Successes: ds.Successes, Failures: ds.Failures + 2}
		return buildDeathSaveOutcome(name, newDS, roll, true)
	}

	// Normal roll
	if roll >= 10 {
		newDS := DeathSaves{Successes: ds.Successes + 1, Failures: ds.Failures}
		return buildDeathSaveOutcome(name, newDS, roll, false)
	}

	newDS := DeathSaves{Successes: ds.Successes, Failures: ds.Failures + 1}
	return buildDeathSaveOutcome(name, newDS, roll, false)
}

// buildDeathSaveOutcome constructs the outcome after updating death save tallies.
func buildDeathSaveOutcome(name string, ds DeathSaves, roll int, isNat1 bool) DeathSaveOutcome {
	// 3+ failures -> dead
	if ds.Failures >= 3 {
		var msg string
		if isNat1 {
			msg = fmt.Sprintf("🎲  %s rolls death save — 💥 NAT 1 — 2 failures! (%dS / %dF) — dead",
				name, ds.Successes, ds.Failures)
		} else {
			msg = fmt.Sprintf("🎲  %s rolls death save — %d — Failure (%dS / %dF) — dead",
				name, roll, ds.Successes, ds.Failures)
		}
		return DeathSaveOutcome{
			DeathSaves: ds,
			TokenState: TokenDead,
			IsAlive:    false,
			IsDying:    false,
			Messages:   []string{msg},
		}
	}

	// 3+ successes -> stabilized
	if ds.Successes >= 3 {
		msg := fmt.Sprintf("🎲  %s rolls death save — %d — Success (%dS / %dF) — stabilized",
			name, roll, ds.Successes, ds.Failures)
		return DeathSaveOutcome{
			DeathSaves: ds,
			TokenState: TokenStable,
			IsAlive:    true,
			IsDying:    false,
			IsStable:   true,
			Messages:   []string{msg},
		}
	}

	// Still dying
	var msg string
	if isNat1 {
		msg = fmt.Sprintf("🎲  %s rolls death save — 💥 NAT 1 — 2 failures! (%dS / %dF)",
			name, ds.Successes, ds.Failures)
	} else if roll >= 10 {
		msg = fmt.Sprintf("🎲  %s rolls death save — %d — Success (%dS / %dF)",
			name, roll, ds.Successes, ds.Failures)
	} else {
		msg = fmt.Sprintf("🎲  %s rolls death save — %d — Failure (%dS / %dF)",
			name, roll, ds.Successes, ds.Failures)
	}

	return DeathSaveOutcome{
		DeathSaves: ds,
		TokenState: TokenDying,
		IsAlive:    true,
		IsDying:    true,
		Messages:   []string{msg},
	}
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
		DeathSaves: DeathSaves{},
		TokenState: TokenAlive,
		HPCurrent:  healAmount,
		IsAlive:    true,
		IsDying:    false,
		IsStable:   false,
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
		HPCurrent:  0,
		IsAlive:    true,
		IsDying:    false,
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
