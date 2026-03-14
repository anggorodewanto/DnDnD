package combat

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/refdata"
)

func TestFormatNudgeMessage(t *testing.T) {
	msg := FormatNudgeMessage("Aria", 12*time.Hour)
	assert.Contains(t, msg, "@Aria")
	assert.Contains(t, msg, "12h remaining")
	assert.Contains(t, msg, "/recap")
}

func TestFormatNudgeMessage_WithMinutes(t *testing.T) {
	msg := FormatNudgeMessage("Bob", 6*time.Hour+30*time.Minute)
	assert.Contains(t, msg, "@Bob")
	assert.Contains(t, msg, "6h 30m remaining")
}

func TestFormatNudgeMessage_MinutesOnly(t *testing.T) {
	msg := FormatNudgeMessage("Cara", 45*time.Minute)
	assert.Contains(t, msg, "45m remaining")
}

func TestFormatNudgeMessage_LessThanMinute(t *testing.T) {
	msg := FormatNudgeMessage("Dave", 30*time.Second)
	assert.Contains(t, msg, "< 1m remaining")
}

func TestFormatTacticalSummary_Basic(t *testing.T) {
	combatant := refdata.Combatant{
		DisplayName: "Aria",
		HpCurrent:   28,
		HpMax:       45,
		Ac:          16,
		Conditions:  json.RawMessage(`[{"condition":"poisoned","duration_rounds":3,"started_round":1}]`),
	}
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    2,
	}
	enemies := []refdata.Combatant{
		{DisplayName: "Goblin #1", ShortID: "G1"},
		{DisplayName: "Orc Shaman", ShortID: "OS"},
	}

	msg := FormatTacticalSummary(combatant, turn, enemies, 6*time.Hour)

	assert.Contains(t, msg, "@Aria")
	assert.Contains(t, msg, "skipped in 6h")
	assert.Contains(t, msg, "HP: 28/45")
	assert.Contains(t, msg, "AC: 16")
	assert.Contains(t, msg, "Poisoned")
	assert.Contains(t, msg, "30ft move")
	assert.Contains(t, msg, "2 attacks")
	assert.Contains(t, msg, "Goblin #1 (G1)")
	assert.Contains(t, msg, "Orc Shaman (OS)")
	assert.Contains(t, msg, "#combat-map")
}

func TestFormatTacticalSummary_NoConditions(t *testing.T) {
	combatant := refdata.Combatant{
		DisplayName: "Bob",
		HpCurrent:   50,
		HpMax:       50,
		Ac:          18,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}

	msg := FormatTacticalSummary(combatant, turn, nil, 3*time.Hour)

	assert.Contains(t, msg, "HP: 50/50")
	assert.NotContains(t, msg, "Conditions:")
	assert.NotContains(t, msg, "Adjacent enemies:")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{24 * time.Hour, "24h"},
		{12 * time.Hour, "12h"},
		{6*time.Hour + 30*time.Minute, "6h 30m"},
		{45 * time.Minute, "45m"},
		{30 * time.Second, "< 1m"},
		{0, "< 1m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.d)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCapitalize(t *testing.T) {
	assert.Equal(t, "Poisoned", capitalize("poisoned"))
	assert.Equal(t, "A", capitalize("a"))
	assert.Equal(t, "", capitalize(""))
}

func TestFormatConditionNames(t *testing.T) {
	t.Run("multiple conditions", func(t *testing.T) {
		raw := json.RawMessage(`[{"condition":"poisoned"},{"condition":"blinded"}]`)
		names := formatConditionNames(raw)
		assert.Equal(t, []string{"Poisoned", "Blinded"}, names)
	})

	t.Run("empty conditions", func(t *testing.T) {
		raw := json.RawMessage(`[]`)
		names := formatConditionNames(raw)
		assert.Empty(t, names)
	})

	t.Run("nil conditions", func(t *testing.T) {
		names := formatConditionNames(nil)
		assert.Empty(t, names)
	})

	t.Run("invalid json", func(t *testing.T) {
		raw := json.RawMessage(`not json`)
		names := formatConditionNames(raw)
		assert.Nil(t, names)
	})
}

func TestFormatTacticalSummary_WithBardicInspiration(t *testing.T) {
	combatant := refdata.Combatant{
		DisplayName:              "Bard",
		HpCurrent:               40,
		HpMax:                   40,
		Ac:                      14,
		Conditions:              json.RawMessage(`[]`),
		BardicInspirationDie:    sql.NullString{String: "d8", Valid: true},
		BardicInspirationSource: sql.NullString{String: "Melody", Valid: true},
	}
	turn := refdata.Turn{
		MovementRemainingFt: 30,
		AttacksRemaining:    1,
	}

	msg := FormatTacticalSummary(combatant, turn, nil, 6*time.Hour)
	assert.Contains(t, msg, "d8")
}

func TestFormatTacticalSummary_AllResourcesSpent(t *testing.T) {
	combatant := refdata.Combatant{
		DisplayName: "Fighter",
		HpCurrent:   10,
		HpMax:       50,
		Ac:          20,
		Conditions:  json.RawMessage(`[]`),
	}
	turn := refdata.Turn{
		MovementRemainingFt: 0,
		AttacksRemaining:    0,
		ActionUsed:          true,
		BonusActionUsed:     true,
		ReactionUsed:        true,
		FreeInteractUsed:    true,
	}

	msg := FormatTacticalSummary(combatant, turn, nil, 1*time.Hour)
	assert.Contains(t, msg, "HP: 10/50")
	assert.NotContains(t, msg, "Available:")
}

func TestFormatNudgeMessage_ExactFormat(t *testing.T) {
	msg := FormatNudgeMessage("Aria", 12*time.Hour)
	expected := "\u23f0 @Aria \u2014 it's still your turn! 12h remaining. Use /recap to catch up."
	assert.Equal(t, expected, msg)
}
