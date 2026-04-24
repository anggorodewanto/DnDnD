package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatAutoSkipMessage_Stunned(t *testing.T) {
	msg := FormatAutoSkipMessage("Aria", "stunned")
	assert.Equal(t, "\u23ed\ufe0f  Aria's turn is auto-skipped (stunned \u2014 can't take actions)", msg)
}

func TestFormatAutoSkipMessage_Paralyzed(t *testing.T) {
	msg := FormatAutoSkipMessage("Goblin #1", "paralyzed")
	assert.Contains(t, msg, "Goblin #1")
	assert.Contains(t, msg, "paralyzed")
	assert.Contains(t, msg, "auto-skipped")
}

func TestGetIncapacitatingConditionName_Stunned(t *testing.T) {
	conds := []CombatCondition{{Condition: "stunned"}}
	name := GetIncapacitatingConditionName(conds)
	assert.Equal(t, "stunned", name)
}

func TestGetIncapacitatingConditionName_Multiple(t *testing.T) {
	conds := []CombatCondition{
		{Condition: "frightened"},
		{Condition: "paralyzed"},
	}
	name := GetIncapacitatingConditionName(conds)
	assert.Equal(t, "paralyzed", name)
}

func TestGetIncapacitatingConditionName_None(t *testing.T) {
	conds := []CombatCondition{{Condition: "frightened"}}
	name := GetIncapacitatingConditionName(conds)
	assert.Equal(t, "", name)
}

// Phase 114: surprised-skip message follows the spec exactly:
// "⏭️ Goblin #2 is surprised — turn skipped" (single space after the emoji,
// long dash between phrase and 'turn skipped'). The incapacitated message keeps
// its current "auto-skipped (condition — can't take actions)" shape, covered by
// TestFormatAutoSkipMessage_Stunned above.
func TestFormatAutoSkipMessage_Surprised(t *testing.T) {
	msg := FormatAutoSkipMessage("Goblin #2", "surprised")
	assert.Equal(t, "⏭️ Goblin #2 is surprised — turn skipped", msg)
}
