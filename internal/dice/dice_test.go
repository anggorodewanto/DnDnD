package dice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExpression_Simple1d20(t *testing.T) {
	expr, err := ParseExpression("1d20")
	require.NoError(t, err)
	require.Len(t, expr.Groups, 1)
	assert.Equal(t, 1, expr.Groups[0].Count)
	assert.Equal(t, 20, expr.Groups[0].Sides)
	assert.Equal(t, 0, expr.Modifier)
}

func TestParseExpression_WithPositiveModifier(t *testing.T) {
	expr, err := ParseExpression("2d6+3")
	require.NoError(t, err)
	require.Len(t, expr.Groups, 1)
	assert.Equal(t, 2, expr.Groups[0].Count)
	assert.Equal(t, 6, expr.Groups[0].Sides)
	assert.Equal(t, 3, expr.Modifier)
}

func TestParseExpression_WithNegativeModifier(t *testing.T) {
	expr, err := ParseExpression("1d20-2")
	require.NoError(t, err)
	require.Len(t, expr.Groups, 1)
	assert.Equal(t, 1, expr.Groups[0].Count)
	assert.Equal(t, 20, expr.Groups[0].Sides)
	assert.Equal(t, -2, expr.Modifier)
}

func TestParseExpression_MultipleGroups(t *testing.T) {
	expr, err := ParseExpression("2d6+1d4+3")
	require.NoError(t, err)
	require.Len(t, expr.Groups, 2)
	assert.Equal(t, 2, expr.Groups[0].Count)
	assert.Equal(t, 6, expr.Groups[0].Sides)
	assert.Equal(t, 1, expr.Groups[1].Count)
	assert.Equal(t, 4, expr.Groups[1].Sides)
	assert.Equal(t, 3, expr.Modifier)
}

func TestParseExpression_Invalid(t *testing.T) {
	_, err := ParseExpression("")
	assert.Error(t, err)

	_, err = ParseExpression("abc")
	assert.Error(t, err)
}

func TestRoll_Basic1d20(t *testing.T) {
	roller := NewRoller(nil) // nil means use default crypto/rand
	result, err := roller.Roll("1d20")
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.Total, 1)
	assert.LessOrEqual(t, result.Total, 20)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, 20, result.Groups[0].Die)
	assert.Equal(t, 1, result.Groups[0].Count)
	require.Len(t, result.Groups[0].Results, 1)
	assert.GreaterOrEqual(t, result.Groups[0].Results[0], 1)
	assert.LessOrEqual(t, result.Groups[0].Results[0], 20)
	assert.Equal(t, "1d20", result.Expression)
}

// fixedRand returns a RandSource that cycles through the given values.
func fixedRand(values ...int) RandSource {
	i := 0
	return func(max int) int {
		v := values[i%len(values)]
		i++
		return v
	}
}

func TestRoll_Deterministic2d6Plus3(t *testing.T) {
	roller := NewRoller(fixedRand(3, 5))
	result, err := roller.Roll("2d6+3")
	require.NoError(t, err)

	assert.Equal(t, 11, result.Total) // 3+5+3
	require.Len(t, result.Groups, 1)
	assert.Equal(t, []int{3, 5}, result.Groups[0].Results)
	assert.Equal(t, 3, result.Modifier)
	assert.Equal(t, "[3 5] + 3 = 11", result.Breakdown)
}

func TestRoll_NegativeModifier(t *testing.T) {
	roller := NewRoller(fixedRand(10))
	result, err := roller.Roll("1d20-2")
	require.NoError(t, err)

	assert.Equal(t, 8, result.Total) // 10-2
	assert.Equal(t, "[10] - 2 = 8", result.Breakdown)
}

func TestRoll_MultipleGroups(t *testing.T) {
	roller := NewRoller(fixedRand(4, 3, 2))
	result, err := roller.Roll("2d6+1d4+3")
	require.NoError(t, err)

	assert.Equal(t, 12, result.Total) // 4+3+2+3
	require.Len(t, result.Groups, 2)
	assert.Equal(t, []int{4, 3}, result.Groups[0].Results)
	assert.Equal(t, []int{2}, result.Groups[1].Results)
}

func TestRoll_InvalidExpression(t *testing.T) {
	roller := NewRoller(nil)
	_, err := roller.Roll("not_dice")
	assert.Error(t, err)
}

func TestRollD20_Advantage_TakesHigher(t *testing.T) {
	roller := NewRoller(fixedRand(7, 18))
	result, err := roller.RollD20(5, Advantage)
	require.NoError(t, err)

	assert.Equal(t, 23, result.Total) // 18 + 5
	assert.Equal(t, 7, result.Rolls[0])
	assert.Equal(t, 18, result.Rolls[1])
	assert.Equal(t, 18, result.Chosen)
	assert.Equal(t, Advantage, result.Mode)
	assert.False(t, result.CriticalHit)
	assert.False(t, result.CriticalFail)
}

func TestRollD20_Disadvantage_TakesLower(t *testing.T) {
	roller := NewRoller(fixedRand(15, 4))
	result, err := roller.RollD20(3, Disadvantage)
	require.NoError(t, err)

	assert.Equal(t, 7, result.Total) // 4 + 3
	assert.Equal(t, 4, result.Chosen)
	assert.Equal(t, Disadvantage, result.Mode)
}

func TestRollD20_Normal(t *testing.T) {
	roller := NewRoller(fixedRand(12))
	result, err := roller.RollD20(2, Normal)
	require.NoError(t, err)

	assert.Equal(t, 14, result.Total) // 12 + 2
	assert.Equal(t, 12, result.Chosen)
	require.Len(t, result.Rolls, 1)
}

func TestRollD20_AdvantageAndDisadvantageCancelOut(t *testing.T) {
	roller := NewRoller(fixedRand(12))
	result, err := roller.RollD20(2, AdvantageAndDisadvantage)
	require.NoError(t, err)

	assert.Equal(t, 14, result.Total)
	require.Len(t, result.Rolls, 1)
	assert.Equal(t, Normal, result.Mode) // cancelled out
}

func TestRollD20_CriticalHit(t *testing.T) {
	roller := NewRoller(fixedRand(20))
	result, err := roller.RollD20(5, Normal)
	require.NoError(t, err)

	assert.True(t, result.CriticalHit)
	assert.False(t, result.CriticalFail)
	assert.Equal(t, 25, result.Total)
}

func TestRollD20_CriticalFail(t *testing.T) {
	roller := NewRoller(fixedRand(1))
	result, err := roller.RollD20(5, Normal)
	require.NoError(t, err)

	assert.False(t, result.CriticalHit)
	assert.True(t, result.CriticalFail)
	assert.Equal(t, 6, result.Total)
}

func TestRollD20_CriticalHitWithAdvantage(t *testing.T) {
	roller := NewRoller(fixedRand(5, 20))
	result, err := roller.RollD20(3, Advantage)
	require.NoError(t, err)

	assert.True(t, result.CriticalHit)
	assert.Equal(t, 20, result.Chosen)
}

func TestRollDamage_Normal(t *testing.T) {
	roller := NewRoller(fixedRand(4, 3))
	result, err := roller.RollDamage("2d6+3", false)
	require.NoError(t, err)

	assert.Equal(t, 10, result.Total) // 4+3+3
	require.Len(t, result.Groups, 1)
	assert.Equal(t, 2, result.Groups[0].Count)
	assert.Equal(t, []int{4, 3}, result.Groups[0].Results)
}

func TestRollDamage_CriticalHit_DoublesDice(t *testing.T) {
	// Critical hit: double dice count, modifier applied once
	roller := NewRoller(fixedRand(4, 3, 5, 2))
	result, err := roller.RollDamage("2d6+3", true)
	require.NoError(t, err)

	// 4 dice instead of 2, plus modifier once: 4+3+5+2+3 = 17
	assert.Equal(t, 17, result.Total)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, 4, result.Groups[0].Count)
	assert.Equal(t, []int{4, 3, 5, 2}, result.Groups[0].Results)
	assert.True(t, result.Critical)
}

func TestRollDamage_CriticalHit_MultipleGroups(t *testing.T) {
	roller := NewRoller(fixedRand(4, 3, 2, 1, 3, 2))
	result, err := roller.RollDamage("2d6+1d4+3", true)
	require.NoError(t, err)

	// 4d6: 4+3+2+1 = 10, 2d4: 3+2 = 5, modifier: 3 = 18
	assert.Equal(t, 18, result.Total)
	require.Len(t, result.Groups, 2)
	assert.Equal(t, 4, result.Groups[0].Count)
	assert.Equal(t, 2, result.Groups[1].Count)
}

func TestRollLog_ToJSON(t *testing.T) {
	entry := RollLogEntry{
		DiceRolls: []GroupResult{
			{Die: 20, Count: 1, Results: []int{15}, Total: 15, Purpose: "attack"},
		},
		Total:      18,
		Expression: "1d20+3",
		Roller:     "Aria",
		Purpose:    "Longsword attack",
	}

	data := entry.ToJSONRolls()
	assert.Contains(t, string(data), `"die":20`)
	assert.Contains(t, string(data), `"purpose":"attack"`)
}

func TestRollHistoryLogger_Interface(t *testing.T) {
	var logger RollHistoryLogger
	mock := &mockLogger{}
	logger = mock

	entry := RollLogEntry{
		Expression: "1d20+5",
		Total:      15,
		Roller:     "TestChar",
		Purpose:    "attack",
	}
	err := logger.LogRoll(entry)
	require.NoError(t, err)
	assert.Equal(t, 1, mock.calls)
}

type mockLogger struct {
	calls int
}

func (m *mockLogger) LogRoll(entry RollLogEntry) error {
	m.calls++
	return nil
}

func TestRollD20_Breakdown_NormalWithModifier(t *testing.T) {
	roller := NewRoller(fixedRand(12))
	result, err := roller.RollD20(3, Normal)
	require.NoError(t, err)
	assert.Equal(t, "12 + 3 = 15", result.Breakdown)
}

func TestRollD20_Breakdown_NormalNoModifier(t *testing.T) {
	roller := NewRoller(fixedRand(14))
	result, err := roller.RollD20(0, Normal)
	require.NoError(t, err)
	assert.Equal(t, "14", result.Breakdown)
}

func TestRollD20_Breakdown_Advantage(t *testing.T) {
	roller := NewRoller(fixedRand(7, 18))
	result, err := roller.RollD20(5, Advantage)
	require.NoError(t, err)
	assert.Equal(t, "7 / 18 (higher: 18 + 5 = 23)", result.Breakdown)
}

func TestRollD20_Breakdown_Disadvantage(t *testing.T) {
	roller := NewRoller(fixedRand(15, 4))
	result, err := roller.RollD20(3, Disadvantage)
	require.NoError(t, err)
	assert.Equal(t, "15 / 4 (lower: 4 + 3 = 7)", result.Breakdown)
}

func TestRollD20_Breakdown_AdvantageNoModifier(t *testing.T) {
	roller := NewRoller(fixedRand(8, 16))
	result, err := roller.RollD20(0, Advantage)
	require.NoError(t, err)
	assert.Equal(t, "8 / 16 (higher: 16)", result.Breakdown)
}

func TestParseExpression_RawPreserved(t *testing.T) {
	expr, err := ParseExpression("2d6+1d4+3")
	require.NoError(t, err)
	assert.Equal(t, "2d6+1d4+3", expr.Raw)
}

func TestRollDamage_InvalidExpression(t *testing.T) {
	roller := NewRoller(nil)
	_, err := roller.RollDamage("bad", false)
	assert.Error(t, err)
}

func TestRollD20_CriticalFailWithDisadvantage(t *testing.T) {
	roller := NewRoller(fixedRand(1, 15))
	result, err := roller.RollD20(5, Disadvantage)
	require.NoError(t, err)

	assert.True(t, result.CriticalFail)
	assert.Equal(t, 1, result.Chosen)
	assert.Equal(t, 6, result.Total)
}

func TestRoll_Breakdown_MultipleGroups(t *testing.T) {
	roller := NewRoller(fixedRand(4, 3, 2))
	result, err := roller.Roll("2d6+1d4+3")
	require.NoError(t, err)
	assert.Equal(t, "[4 3] + [2] + 3 = 12", result.Breakdown)
}

func TestRoll_Breakdown_NoModifier(t *testing.T) {
	roller := NewRoller(fixedRand(4, 3))
	result, err := roller.Roll("2d6")
	require.NoError(t, err)
	assert.Equal(t, "[4 3] = 7", result.Breakdown)
}

func TestRollMode_String(t *testing.T) {
	assert.Equal(t, "normal", Normal.String())
	assert.Equal(t, "advantage", Advantage.String())
	assert.Equal(t, "disadvantage", Disadvantage.String())
	assert.Equal(t, "advantage+disadvantage", AdvantageAndDisadvantage.String())
}

func TestRollD20_Advantage_FirstRollHigher(t *testing.T) {
	roller := NewRoller(fixedRand(18, 7))
	result, err := roller.RollD20(0, Advantage)
	require.NoError(t, err)
	assert.Equal(t, 18, result.Chosen) // first is higher
}

func TestRollD20_Disadvantage_FirstRollHigher(t *testing.T) {
	roller := NewRoller(fixedRand(18, 7))
	result, err := roller.RollD20(0, Disadvantage)
	require.NoError(t, err)
	assert.Equal(t, 7, result.Chosen) // takes lower
}

func TestParseExpression_WithSpaces(t *testing.T) {
	expr, err := ParseExpression("2d6 + 3")
	require.NoError(t, err)
	assert.Equal(t, 2, expr.Groups[0].Count)
	assert.Equal(t, 3, expr.Modifier)
}

func TestRollDamage_CritWithNoModifier(t *testing.T) {
	roller := NewRoller(fixedRand(3, 4, 2, 5))
	result, err := roller.RollDamage("2d6", true)
	require.NoError(t, err)
	assert.Equal(t, 14, result.Total) // 3+4+2+5
	assert.Equal(t, 4, result.Groups[0].Count)
}

func TestParseExpression_InvalidModifier(t *testing.T) {
	// This creates a scenario where stripping dice leaves invalid text
	_, err := ParseExpression("1d6xyz")
	assert.Error(t, err)
}
