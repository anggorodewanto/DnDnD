package combat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TDD Cycle 1: ParseDeathSaves from valid JSON ---

func TestParseDeathSaves_Valid(t *testing.T) {
	raw := json.RawMessage(`{"successes":1,"failures":2}`)
	ds, err := ParseDeathSaves(raw)
	require.NoError(t, err)
	assert.Equal(t, 1, ds.Successes)
	assert.Equal(t, 2, ds.Failures)
}

// --- TDD Cycle 2: ParseDeathSaves from empty JSON ---

func TestParseDeathSaves_Empty(t *testing.T) {
	ds, err := ParseDeathSaves(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, ds.Successes)
	assert.Equal(t, 0, ds.Failures)
}

// --- TDD Cycle 2b: ParseDeathSaves from invalid JSON ---

func TestParseDeathSaves_Invalid(t *testing.T) {
	raw := json.RawMessage(`{invalid`)
	_, err := ParseDeathSaves(raw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing death saves")
}

// --- TDD Cycle 3: MarshalDeathSaves round-trips ---

func TestMarshalDeathSaves_RoundTrip(t *testing.T) {
	ds := DeathSaves{Successes: 2, Failures: 1}
	nrm := MarshalDeathSaves(ds)
	assert.True(t, nrm.Valid)

	parsed, err := ParseDeathSaves(nrm.RawMessage)
	require.NoError(t, err)
	assert.Equal(t, ds, parsed)
}

// --- TDD Cycle 4: CheckInstantDeath - overflow >= maxHP ---

func TestCheckInstantDeath_OverflowExceedsMax(t *testing.T) {
	result := CheckInstantDeath(26, 24)
	assert.True(t, result)
}

func TestCheckInstantDeath_OverflowEqualsMax(t *testing.T) {
	result := CheckInstantDeath(24, 24)
	assert.True(t, result)
}

func TestCheckInstantDeath_OverflowLessThanMax(t *testing.T) {
	result := CheckInstantDeath(10, 24)
	assert.False(t, result)
}

func TestCheckInstantDeath_ZeroOverflow(t *testing.T) {
	result := CheckInstantDeath(0, 24)
	assert.False(t, result)
}

// --- TDD Cycle 5: ProcessDropToZeroHP - normal drop (not instant death) ---

func TestProcessDropToZeroHP_NormalDrop(t *testing.T) {
	outcome := ProcessDropToZeroHP("Aria", 10, 24)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.True(t, outcome.IsDying)
	assert.True(t, outcome.IsAlive)
	assert.False(t, outcome.IsStable)
	assert.Equal(t, 0, outcome.HPCurrent)
	assert.Equal(t, 0, outcome.DeathSaves.Successes)
	assert.Equal(t, 0, outcome.DeathSaves.Failures)
	assert.Len(t, outcome.Messages, 1)
	assert.Contains(t, outcome.Messages[0], "Aria drops to 0 HP")
	assert.Contains(t, outcome.Messages[0], "unconscious and dying")
}

// --- TDD Cycle 6: ProcessDropToZeroHP - instant death ---

func TestProcessDropToZeroHP_InstantDeath(t *testing.T) {
	outcome := ProcessDropToZeroHP("Aria", 26, 24)
	assert.Equal(t, TokenDead, outcome.TokenState)
	assert.False(t, outcome.IsAlive)
	assert.False(t, outcome.IsDying)
	assert.Equal(t, 0, outcome.HPCurrent)
	assert.Len(t, outcome.Messages, 1)
	assert.Contains(t, outcome.Messages[0], "killed outright")
	assert.Contains(t, outcome.Messages[0], "26 overflow")
	assert.Contains(t, outcome.Messages[0], "24 max HP")
}

// --- TDD Cycle 7: RollDeathSave - normal success (roll >= 10) ---

func TestRollDeathSave_NormalSuccess(t *testing.T) {
	ds := DeathSaves{Successes: 0, Failures: 1}
	outcome := RollDeathSave("Aria", ds, 14)
	assert.Equal(t, 1, outcome.DeathSaves.Successes)
	assert.Equal(t, 1, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.True(t, outcome.IsDying)
	assert.True(t, outcome.IsAlive)
	assert.Contains(t, outcome.Messages[0], "14")
	assert.Contains(t, outcome.Messages[0], "Success")
	assert.Contains(t, outcome.Messages[0], "1S / 1F")
}

// --- TDD Cycle 8: RollDeathSave - normal failure (roll < 10) ---

func TestRollDeathSave_NormalFailure(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 0}
	outcome := RollDeathSave("Aria", ds, 5)
	assert.Equal(t, 1, outcome.DeathSaves.Successes)
	assert.Equal(t, 1, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.True(t, outcome.IsDying)
	assert.Contains(t, outcome.Messages[0], "5")
	assert.Contains(t, outcome.Messages[0], "Failure")
}

// --- TDD Cycle 9: RollDeathSave - 3 successes stabilize ---

func TestRollDeathSave_ThreeSuccessesStabilize(t *testing.T) {
	ds := DeathSaves{Successes: 2, Failures: 1}
	outcome := RollDeathSave("Aria", ds, 15)
	assert.Equal(t, 3, outcome.DeathSaves.Successes)
	assert.Equal(t, TokenStable, outcome.TokenState)
	assert.True(t, outcome.IsStable)
	assert.False(t, outcome.IsDying)
	assert.True(t, outcome.IsAlive)
}

// --- TDD Cycle 10: RollDeathSave - 3 failures die ---

func TestRollDeathSave_ThreeFailuresDead(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 2}
	outcome := RollDeathSave("Aria", ds, 5)
	assert.Equal(t, 3, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDead, outcome.TokenState)
	assert.False(t, outcome.IsAlive)
	assert.False(t, outcome.IsDying)
	assert.Contains(t, outcome.Messages[0], "dead")
}

// --- TDD Cycle 11: RollDeathSave - Nat 20 regain 1 HP ---

func TestRollDeathSave_Nat20(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 2}
	outcome := RollDeathSave("Aria", ds, 20)
	assert.Equal(t, 0, outcome.DeathSaves.Successes) // tallies reset
	assert.Equal(t, 0, outcome.DeathSaves.Failures)  // tallies reset
	assert.Equal(t, 1, outcome.HPCurrent)
	assert.Equal(t, TokenAlive, outcome.TokenState)
	assert.True(t, outcome.IsAlive)
	assert.False(t, outcome.IsDying)
	assert.False(t, outcome.IsStable)
	assert.Contains(t, outcome.Messages[0], "NAT 20")
	assert.Contains(t, outcome.Messages[0], "regains 1 HP")
	assert.Contains(t, outcome.Messages[0], "tallies reset")
}

// --- TDD Cycle 12: RollDeathSave - Nat 1 counts as 2 failures ---

func TestRollDeathSave_Nat1_TwoFailures(t *testing.T) {
	ds := DeathSaves{Successes: 0, Failures: 0}
	outcome := RollDeathSave("Aria", ds, 1)
	assert.Equal(t, 0, outcome.DeathSaves.Successes)
	assert.Equal(t, 2, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.True(t, outcome.IsDying)
	assert.Contains(t, outcome.Messages[0], "NAT 1")
	assert.Contains(t, outcome.Messages[0], "2 failures")
}

// --- TDD Cycle 13: Nat 1 with 2 existing failures -> dead ---

func TestRollDeathSave_Nat1_CausesDeath(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 2}
	outcome := RollDeathSave("Aria", ds, 1)
	assert.Equal(t, TokenDead, outcome.TokenState)
	assert.False(t, outcome.IsAlive)
	// Failures should be capped at 3 or show actual count
	assert.True(t, outcome.DeathSaves.Failures >= 3)
}

// --- TDD Cycle 14: Nat 1 with 1 existing failure -> 3 failures dead ---

func TestRollDeathSave_Nat1_OneExistingFailure(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 1}
	outcome := RollDeathSave("Aria", ds, 1)
	// 1 + 2 = 3 -> dead
	assert.Equal(t, TokenDead, outcome.TokenState)
	assert.False(t, outcome.IsAlive)
	assert.Contains(t, outcome.Messages[0], "dead")
}

// --- TDD Cycle 15: Roll exactly 10 is a success ---

func TestRollDeathSave_RollTen_IsSuccess(t *testing.T) {
	ds := DeathSaves{}
	outcome := RollDeathSave("Aria", ds, 10)
	assert.Equal(t, 1, outcome.DeathSaves.Successes)
	assert.Equal(t, 0, outcome.DeathSaves.Failures)
}

// --- TDD Cycle 16: Roll 9 is a failure ---

func TestRollDeathSave_RollNine_IsFailure(t *testing.T) {
	ds := DeathSaves{}
	outcome := RollDeathSave("Aria", ds, 9)
	assert.Equal(t, 0, outcome.DeathSaves.Successes)
	assert.Equal(t, 1, outcome.DeathSaves.Failures)
}

// --- TDD Cycle 17: ApplyDamageAtZeroHP - normal hit ---

func TestApplyDamageAtZeroHP_NormalHit(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 0}
	outcome := ApplyDamageAtZeroHP("Aria", ds, false)
	assert.Equal(t, 1, outcome.DeathSaves.Successes)
	assert.Equal(t, 1, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.True(t, outcome.IsDying)
	assert.Contains(t, outcome.Messages[0], "takes damage at 0 HP")
	assert.Contains(t, outcome.Messages[0], "1 death save failure")
}

// --- TDD Cycle 18: ApplyDamageAtZeroHP - critical hit (2 failures) ---

func TestApplyDamageAtZeroHP_CriticalHit(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 0}
	outcome := ApplyDamageAtZeroHP("Aria", ds, true)
	assert.Equal(t, 1, outcome.DeathSaves.Successes)
	assert.Equal(t, 2, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.Contains(t, outcome.Messages[0], "critical hit")
	assert.Contains(t, outcome.Messages[0], "2 death save failures")
}

// --- TDD Cycle 19: ApplyDamageAtZeroHP - causes death ---

func TestApplyDamageAtZeroHP_CausesDeath(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 2}
	outcome := ApplyDamageAtZeroHP("Aria", ds, false)
	assert.Equal(t, 3, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenDead, outcome.TokenState)
	assert.False(t, outcome.IsAlive)
	assert.Contains(t, outcome.Messages[0], "dead")
}

// --- TDD Cycle 20: ApplyDamageAtZeroHP - critical causes death from 1 failure ---

func TestApplyDamageAtZeroHP_CritCausesDeath(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 1}
	outcome := ApplyDamageAtZeroHP("Aria", ds, true)
	assert.True(t, outcome.DeathSaves.Failures >= 3)
	assert.Equal(t, TokenDead, outcome.TokenState)
	assert.False(t, outcome.IsAlive)
}

// --- TDD Cycle 21: HealFromZeroHP - resets tallies and sets HP ---

func TestHealFromZeroHP(t *testing.T) {
	ds := DeathSaves{Successes: 2, Failures: 1}
	outcome := HealFromZeroHP("Aria", ds, 7)
	assert.Equal(t, 0, outcome.DeathSaves.Successes) // reset
	assert.Equal(t, 0, outcome.DeathSaves.Failures)  // reset
	assert.Equal(t, 7, outcome.HPCurrent)
	assert.Equal(t, TokenAlive, outcome.TokenState)
	assert.True(t, outcome.IsAlive)
	assert.False(t, outcome.IsDying)
	assert.False(t, outcome.IsStable)
	assert.Contains(t, outcome.Messages[0], "receives 7 HP")
	assert.Contains(t, outcome.Messages[0], "conscious at 7 HP")
	assert.Contains(t, outcome.Messages[0], "tallies reset")
}

// --- TDD Cycle 22: StabilizeTarget ---

func TestStabilizeTarget(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 2}
	outcome := StabilizeTarget("Aria", ds, "Medicine check")
	assert.Equal(t, TokenStable, outcome.TokenState)
	assert.True(t, outcome.IsStable)
	assert.True(t, outcome.IsAlive)
	assert.False(t, outcome.IsDying)
	assert.Equal(t, 0, outcome.HPCurrent) // remains at 0
	assert.Equal(t, 3, outcome.DeathSaves.Successes)
	assert.Contains(t, outcome.Messages[0], "stabilized")
}

// --- TDD Cycle 23: GetTokenState helper ---

func TestGetTokenState_Alive(t *testing.T) {
	assert.Equal(t, TokenAlive, GetTokenState(true, 10, DeathSaves{}))
}

func TestGetTokenState_Dying(t *testing.T) {
	assert.Equal(t, TokenDying, GetTokenState(true, 0, DeathSaves{Successes: 1, Failures: 1}))
}

func TestGetTokenState_Stable(t *testing.T) {
	assert.Equal(t, TokenStable, GetTokenState(true, 0, DeathSaves{Successes: 3, Failures: 1}))
}

func TestGetTokenState_Dead(t *testing.T) {
	assert.Equal(t, TokenDead, GetTokenState(false, 0, DeathSaves{}))
}

// --- TDD Cycle 24: IsDying helper ---

func TestIsDying_AtZeroHPAlive(t *testing.T) {
	assert.True(t, IsDying(true, 0, DeathSaves{Successes: 0, Failures: 1}))
}

func TestIsDying_NotAtZeroHP(t *testing.T) {
	assert.False(t, IsDying(true, 5, DeathSaves{}))
}

func TestIsDying_Dead(t *testing.T) {
	assert.False(t, IsDying(false, 0, DeathSaves{Failures: 3}))
}

func TestIsDying_Stabilized(t *testing.T) {
	assert.False(t, IsDying(true, 0, DeathSaves{Successes: 3}))
}

// --- TDD Cycle 25: Edge case - heal with 0 amount ---

func TestHealFromZeroHP_ZeroHealing(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 1}
	outcome := HealFromZeroHP("Aria", ds, 0)
	// Even 0 healing should reset tallies and set conscious
	assert.Equal(t, 0, outcome.HPCurrent)
	assert.Equal(t, 0, outcome.DeathSaves.Successes)
	assert.Equal(t, 0, outcome.DeathSaves.Failures)
	assert.Equal(t, TokenAlive, outcome.TokenState)
}

// --- TDD Cycle 26: ProcessDropToZeroHP with 0 overflow ---

func TestProcessDropToZeroHP_ZeroOverflow(t *testing.T) {
	outcome := ProcessDropToZeroHP("Aria", 0, 24)
	assert.Equal(t, TokenDying, outcome.TokenState)
	assert.True(t, outcome.IsDying)
}

// --- TDD Cycle 27: ConditionsForDying returns unconscious + prone ---

func TestConditionsForDying(t *testing.T) {
	conds := ConditionsForDying()
	assert.Len(t, conds, 2)
	names := []string{conds[0].Condition, conds[1].Condition}
	assert.Contains(t, names, "unconscious")
	assert.Contains(t, names, "prone")
}

// --- TDD Cycle 28: DeathSave message format - success with tally ---

func TestRollDeathSave_MessageFormat_SuccessWithTally(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 1}
	outcome := RollDeathSave("Aria", ds, 14)
	assert.Contains(t, outcome.Messages[0], "🎲")
	assert.Contains(t, outcome.Messages[0], "Aria rolls death save")
	assert.Contains(t, outcome.Messages[0], "2S / 1F")
}

// --- TDD Cycle 29: DeathSave message format - failure leading to death ---

func TestRollDeathSave_MessageFormat_FailureToDeath(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 2}
	outcome := RollDeathSave("Aria", ds, 5)
	assert.Contains(t, outcome.Messages[0], "1S / 3F")
	assert.Contains(t, outcome.Messages[0], "dead")
}

// --- TDD Cycle 30: ApplyDamageAtZeroHP message format ---

func TestApplyDamageAtZeroHP_MessageFormat(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 1}
	outcome := ApplyDamageAtZeroHP("Aria", ds, false)
	assert.Contains(t, outcome.Messages[0], "⚠️")
	assert.Contains(t, outcome.Messages[0], "1S / 2F")
}

// --- TDD Cycle 31: ApplyDamageAtZeroHP crit message format ---

func TestApplyDamageAtZeroHP_CritMessageFormat(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 0}
	outcome := ApplyDamageAtZeroHP("Aria", ds, true)
	assert.Contains(t, outcome.Messages[0], "⚠️")
	assert.Contains(t, outcome.Messages[0], "critical hit")
	assert.Contains(t, outcome.Messages[0], "1S / 2F")
}

// --- TDD Cycle 32: Nat 20 message format ---

func TestRollDeathSave_Nat20_MessageFormat(t *testing.T) {
	ds := DeathSaves{Successes: 0, Failures: 2}
	outcome := RollDeathSave("Aria", ds, 20)
	assert.Contains(t, outcome.Messages[0], "🎯 NAT 20")
	assert.Contains(t, outcome.Messages[0], "Aria regains 1 HP")
	assert.Contains(t, outcome.Messages[0], "conscious")
}

// --- TDD Cycle 33: Nat 1 message format ---

func TestRollDeathSave_Nat1_MessageFormat(t *testing.T) {
	ds := DeathSaves{Successes: 1, Failures: 0}
	outcome := RollDeathSave("Aria", ds, 1)
	assert.Contains(t, outcome.Messages[0], "💥 NAT 1")
	assert.Contains(t, outcome.Messages[0], "2 failures")
	assert.Contains(t, outcome.Messages[0], "1S / 2F")
}

// --- TDD Cycle 34: Instant death message format ---

func TestProcessDropToZeroHP_InstantDeathMessageFormat(t *testing.T) {
	outcome := ProcessDropToZeroHP("Aria", 26, 24)
	assert.Contains(t, outcome.Messages[0], "💀")
	assert.Contains(t, outcome.Messages[0], "killed outright")
	assert.Contains(t, outcome.Messages[0], "instant death, no death saves")
}

// --- TDD Cycle 35: Normal drop message format ---

func TestProcessDropToZeroHP_NormalDropMessageFormat(t *testing.T) {
	outcome := ProcessDropToZeroHP("Aria", 5, 24)
	assert.Contains(t, outcome.Messages[0], "💔")
	assert.Contains(t, outcome.Messages[0], "death saves begin next turn")
}

// --- TDD Cycle 36: HealFromZeroHP message format ---

func TestHealFromZeroHP_MessageFormat(t *testing.T) {
	ds := DeathSaves{Successes: 2, Failures: 1}
	outcome := HealFromZeroHP("Aria", ds, 7)
	assert.Contains(t, outcome.Messages[0], "💚")
	assert.Contains(t, outcome.Messages[0], "Aria receives 7 HP of healing")
	assert.Contains(t, outcome.Messages[0], "death save tallies reset")
}

// --- TDD Cycle 37: Stabilize after exactly 3 successes via death save ---

func TestRollDeathSave_ExactlyThreeSuccesses(t *testing.T) {
	ds := DeathSaves{Successes: 2, Failures: 2}
	outcome := RollDeathSave("Aria", ds, 10)
	assert.Equal(t, 3, outcome.DeathSaves.Successes)
	assert.Equal(t, TokenStable, outcome.TokenState)
	assert.True(t, outcome.IsStable)
	assert.False(t, outcome.IsDying)
}
