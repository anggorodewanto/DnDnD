package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Death saving throws are 2024 d20 Tests, so exhaustion's -2×level penalty
// lowers the total compared against DC 10 — but it must NOT touch the raw-die
// nat-20 (regain 1 HP) / nat-1 (2 failures) detection, which keys off the
// natural die face, not the modified total. These cases pin that split: the
// penalty flips a would-be success to a failure, yet a natural 20/1 is
// unchanged. ExhaustionD20Penalty(1) = -2.
func TestDeathSave_Exhaustion(t *testing.T) {
	t.Run("exhaustion turns a made save into a failure", func(t *testing.T) {
		// Raw 11 alone (≥10) is a success; -2 exhaustion → total 9 → failure.
		outcome := RollDeathSave("Aria", DeathSaves{}, 11, ExhaustionD20Penalty(1))
		assert.Equal(t, 0, outcome.DeathSaves.Successes)
		assert.Equal(t, 1, outcome.DeathSaves.Failures, "11 - 2 exhaustion = 9 → failure")
		assert.True(t, outcome.IsDying)
		assert.Contains(t, outcome.Messages[0], "Failure")
		assert.Contains(t, outcome.Messages[0], "exhaustion", "the log discloses why 11 failed")
	})

	t.Run("a high roll still succeeds despite exhaustion", func(t *testing.T) {
		// Raw 14, -2 exhaustion → total 12 ≥ 10 → success.
		outcome := RollDeathSave("Aria", DeathSaves{}, 14, ExhaustionD20Penalty(1))
		assert.Equal(t, 1, outcome.DeathSaves.Successes, "14 - 2 exhaustion = 12 → success")
		assert.Equal(t, 0, outcome.DeathSaves.Failures)
		assert.Contains(t, outcome.Messages[0], "Success")
	})

	t.Run("exhaustion does not downgrade a natural 20", func(t *testing.T) {
		// Even at exhaustion 3 (-6), a nat 20 still wakes the character.
		outcome := RollDeathSave("Aria", DeathSaves{Failures: 2}, 20, ExhaustionD20Penalty(3))
		assert.Equal(t, 1, outcome.HPCurrent, "nat 20 regains 1 HP regardless of exhaustion")
		assert.Equal(t, TokenAlive, outcome.TokenState)
		assert.True(t, outcome.IsAlive)
		assert.Contains(t, outcome.Messages[0], "NAT 20")
	})

	t.Run("a natural 1 still counts double under exhaustion", func(t *testing.T) {
		// Nat 1 keys off the raw die, not the total — 2 failures either way.
		outcome := RollDeathSave("Aria", DeathSaves{}, 1, ExhaustionD20Penalty(2))
		assert.Equal(t, 2, outcome.DeathSaves.Failures, "nat 1 = 2 failures, exhaustion irrelevant")
		assert.Contains(t, outcome.Messages[0], "NAT 1")
	})

	t.Run("zero exhaustion leaves the base rules untouched", func(t *testing.T) {
		// Penalty 0 must render exactly like the pre-exhaustion log (no note).
		outcome := RollDeathSave("Aria", DeathSaves{}, 11, ExhaustionD20Penalty(0))
		assert.Equal(t, 1, outcome.DeathSaves.Successes, "11 ≥ 10 → success with no penalty")
		assert.NotContains(t, outcome.Messages[0], "exhaustion")
	})
}
