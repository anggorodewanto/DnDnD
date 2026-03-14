package combat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ab/dndnd/internal/refdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 1: ConcentrationCheckDC computes max(10, floor(damage/2))
func TestConcentrationCheckDC(t *testing.T) {
	tests := []struct {
		name   string
		damage int
		want   int
	}{
		{"low damage returns 10", 5, 10},
		{"damage 10 returns 10", 10, 10},
		{"damage 19 returns 10 (floor(19/2)=9 < 10)", 19, 10},
		{"damage 20 returns 10 (floor(20/2)=10)", 20, 10},
		{"damage 21 returns 10 (floor(21/2)=10)", 21, 10},
		{"damage 22 returns 11", 22, 11},
		{"damage 30 returns 15", 30, 15},
		{"damage 50 returns 25", 50, 25},
		{"damage 0 returns 10", 0, 10},
		{"damage 1 returns 10", 1, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ConcentrationCheckDC(tc.damage))
		})
	}
}

// TDD Cycle 2: CheckConcentrationOnDamage returns whether a save is needed
func TestCheckConcentrationOnDamage(t *testing.T) {
	tests := []struct {
		name                 string
		currentConcentration string
		damage               int
		wantNeedsSave        bool
		wantDC               int
		wantSpell            string
	}{
		{
			name:                 "not concentrating — no save needed",
			currentConcentration: "",
			damage:               15,
			wantNeedsSave:        false,
			wantDC:               0,
			wantSpell:            "",
		},
		{
			name:                 "concentrating and takes damage — save needed",
			currentConcentration: "Bless",
			damage:               15,
			wantNeedsSave:        true,
			wantDC:               10,
			wantSpell:            "Bless",
		},
		{
			name:                 "high damage — DC is half damage",
			currentConcentration: "Hold Person",
			damage:               30,
			wantNeedsSave:        true,
			wantDC:               15,
			wantSpell:            "Hold Person",
		},
		{
			name:                 "zero damage — no save needed",
			currentConcentration: "Bless",
			damage:               0,
			wantNeedsSave:        false,
			wantDC:               0,
			wantSpell:            "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CheckConcentrationOnDamage(tc.currentConcentration, tc.damage)
			assert.Equal(t, tc.wantNeedsSave, result.NeedsSave)
			assert.Equal(t, tc.wantDC, result.DC)
			assert.Equal(t, tc.wantSpell, result.SpellName)
		})
	}
}

// TDD Cycle 3: CheckConcentrationOnIncapacitation auto-breaks concentration
func TestCheckConcentrationOnIncapacitation(t *testing.T) {
	tests := []struct {
		name                 string
		currentConcentration string
		conditions           []CombatCondition
		wantBroken           bool
		wantSpell            string
		wantReason           string
	}{
		{
			name:                 "not concentrating — no break",
			currentConcentration: "",
			conditions:           []CombatCondition{{Condition: "stunned"}},
			wantBroken:           false,
		},
		{
			name:                 "stunned breaks concentration",
			currentConcentration: "Bless",
			conditions:           []CombatCondition{{Condition: "stunned"}},
			wantBroken:           true,
			wantSpell:            "Bless",
			wantReason:           "stunned",
		},
		{
			name:                 "paralyzed breaks concentration",
			currentConcentration: "Hold Person",
			conditions:           []CombatCondition{{Condition: "paralyzed"}},
			wantBroken:           true,
			wantSpell:            "Hold Person",
			wantReason:           "paralyzed",
		},
		{
			name:                 "unconscious breaks concentration",
			currentConcentration: "Bless",
			conditions:           []CombatCondition{{Condition: "unconscious"}},
			wantBroken:           true,
			wantSpell:            "Bless",
			wantReason:           "unconscious",
		},
		{
			name:                 "petrified breaks concentration",
			currentConcentration: "Fog Cloud",
			conditions:           []CombatCondition{{Condition: "petrified"}},
			wantBroken:           true,
			wantSpell:            "Fog Cloud",
			wantReason:           "petrified",
		},
		{
			name:                 "incapacitated breaks concentration",
			currentConcentration: "Bless",
			conditions:           []CombatCondition{{Condition: "incapacitated"}},
			wantBroken:           true,
			wantSpell:            "Bless",
			wantReason:           "incapacitated",
		},
		{
			name:                 "non-incapacitating condition does not break",
			currentConcentration: "Bless",
			conditions:           []CombatCondition{{Condition: "frightened"}},
			wantBroken:           false,
		},
		{
			name:                 "no conditions — no break",
			currentConcentration: "Bless",
			conditions:           nil,
			wantBroken:           false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CheckConcentrationOnIncapacitation(tc.currentConcentration, tc.conditions)
			assert.Equal(t, tc.wantBroken, result.Broken)
			assert.Equal(t, tc.wantSpell, result.SpellName)
			assert.Equal(t, tc.wantReason, result.Reason)
		})
	}
}

// TDD Cycle 3b: CheckConcentrationOnIncapacitationRaw works with JSON conditions
func TestCheckConcentrationOnIncapacitationRaw(t *testing.T) {
	conds, _ := json.Marshal([]CombatCondition{{Condition: "stunned"}})
	result := CheckConcentrationOnIncapacitationRaw("Bless", conds)
	assert.True(t, result.Broken)
	assert.Equal(t, "Bless", result.SpellName)
	assert.Equal(t, "stunned", result.Reason)

	// empty conditions
	result2 := CheckConcentrationOnIncapacitationRaw("Bless", json.RawMessage(`[]`))
	assert.False(t, result2.Broken)

	// invalid JSON returns no break
	result3 := CheckConcentrationOnIncapacitationRaw("Bless", json.RawMessage(`invalid`))
	assert.False(t, result3.Broken)
}

// TDD Cycle 4: HasVerbalOrSomaticComponent checks spell components
func TestHasVerbalOrSomaticComponent(t *testing.T) {
	tests := []struct {
		name       string
		components []string
		want       bool
	}{
		{"V only", []string{"V"}, true},
		{"S only", []string{"S"}, true},
		{"V and S", []string{"V", "S"}, true},
		{"V, S, M", []string{"V", "S", "M"}, true},
		{"M only", []string{"M"}, false},
		{"empty", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spell := refdata.Spell{Components: tc.components}
			assert.Equal(t, tc.want, HasVerbalOrSomaticComponent(spell))
		})
	}
}

// TDD Cycle 4b: ValidateSilenceZone blocks V/S spells in silence
func TestValidateSilenceZone(t *testing.T) {
	tests := []struct {
		name       string
		inSilence  bool
		components []string
		wantErr    bool
	}{
		{"not in silence — OK", false, []string{"V", "S", "M"}, false},
		{"in silence with V — blocked", true, []string{"V", "M"}, true},
		{"in silence with S — blocked", true, []string{"S"}, true},
		{"in silence with M only — OK", true, []string{"M"}, false},
		{"in silence with no components — OK", true, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spell := refdata.Spell{Components: tc.components}
			err := ValidateSilenceZone(tc.inSilence, spell)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "silence")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TDD Cycle 5: CheckConcentrationInSilence breaks concentration on V/S spells
func TestCheckConcentrationInSilence(t *testing.T) {
	tests := []struct {
		name                 string
		currentConcentration string
		inSilence            bool
		components           []string
		wantBroken           bool
		wantSpell            string
	}{
		{
			name:                 "not concentrating — no break",
			currentConcentration: "",
			inSilence:            true,
			components:           []string{"V", "S"},
			wantBroken:           false,
		},
		{
			name:                 "not in silence — no break",
			currentConcentration: "Bless",
			inSilence:            false,
			components:           []string{"V", "S"},
			wantBroken:           false,
		},
		{
			name:                 "in silence with V component — break",
			currentConcentration: "Bless",
			inSilence:            true,
			components:           []string{"V", "S", "M"},
			wantBroken:           true,
			wantSpell:            "Bless",
		},
		{
			name:                 "in silence with S only — break",
			currentConcentration: "Hold Person",
			inSilence:            true,
			components:           []string{"S"},
			wantBroken:           true,
			wantSpell:            "Hold Person",
		},
		{
			name:                 "in silence with M only — no break",
			currentConcentration: "Bless",
			inSilence:            true,
			components:           []string{"M"},
			wantBroken:           false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spell := refdata.Spell{
				Name:          tc.wantSpell,
				Components:    tc.components,
				Concentration: sql.NullBool{Bool: true, Valid: true},
			}
			result := CheckConcentrationInSilence(tc.currentConcentration, tc.inSilence, spell)
			assert.Equal(t, tc.wantBroken, result.Broken)
			assert.Equal(t, tc.wantSpell, result.SpellName)
		})
	}
}

// TDD Cycle 6: BreakConcentration invokes cleanup callback and returns log message
func TestBreakConcentration(t *testing.T) {
	t.Run("calls cleanup callback", func(t *testing.T) {
		var cleanedSpell string
		cleanup := func(spellName string) {
			cleanedSpell = spellName
		}
		result := BreakConcentration("Aria", "Bless", "stunned", cleanup)
		assert.Equal(t, "Bless", cleanedSpell)
		assert.Contains(t, result.Message, "Aria")
		assert.Contains(t, result.Message, "Bless")
		assert.Contains(t, result.Message, "stunned")
		assert.Equal(t, "Bless", result.SpellName)
		assert.True(t, result.Broken)
	})

	t.Run("nil callback is safe", func(t *testing.T) {
		result := BreakConcentration("Aria", "Bless", "stunned", nil)
		assert.True(t, result.Broken)
	})

	t.Run("silence reason", func(t *testing.T) {
		result := BreakConcentration("Aria", "Fog Cloud", "silence", nil)
		assert.Contains(t, result.Message, "silence")
	})

	t.Run("failed CON save reason", func(t *testing.T) {
		result := BreakConcentration("Aria", "Bless", "failed CON save", nil)
		assert.Contains(t, result.Message, "failed CON save")
	})
}

// TDD Cycle 7: FormatConcentrationBreakLog formats log messages per spec
func TestFormatConcentrationBreakLog(t *testing.T) {
	tests := []struct {
		name   string
		caster string
		spell  string
		reason string
		want   string
	}{
		{
			name:   "incapacitation — stunned",
			caster: "Aria",
			spell:  "Bless",
			reason: "incapacitated — stunned",
			want:   "🔮 Aria loses concentration on Bless (incapacitated — stunned)",
		},
		{
			name:   "failed CON save",
			caster: "Aria",
			spell:  "Fog Cloud",
			reason: "failed CON save",
			want:   "🔮 Aria loses concentration on Fog Cloud (failed CON save)",
		},
		{
			name:   "dropped for new spell",
			caster: "Aria",
			spell:  "Hold Person",
			reason: "cast new concentration spell: Bless",
			want:   "🔮 Aria drops concentration on Hold Person (cast new concentration spell: Bless)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, FormatConcentrationBreakLog(tc.caster, tc.spell, tc.reason))
		})
	}
}

// TDD Cycle 8: Integration test — damage triggers concentration check
func TestIntegration_DamageTriggersConcentrationCheck(t *testing.T) {
	// Scenario: Aria is concentrating on Bless, takes 15 damage
	// Expected: ConcentrationCheckResult with NeedsSave=true, DC=10
	currentConcentration := "Bless"
	damage := 15

	checkResult := CheckConcentrationOnDamage(currentConcentration, damage)
	require.True(t, checkResult.NeedsSave)
	assert.Equal(t, 10, checkResult.DC)
	assert.Equal(t, "Bless", checkResult.SpellName)

	// Simulate failed save: concentration breaks, cleanup invoked
	var cleanedUp bool
	breakResult := BreakConcentration("Aria", checkResult.SpellName, "failed CON save", func(s string) {
		cleanedUp = true
	})
	assert.True(t, breakResult.Broken)
	assert.True(t, cleanedUp)
	assert.Equal(t, "🔮 Aria loses concentration on Bless (failed CON save)", breakResult.Message)
}

// TDD Cycle 8b: Integration test — incapacitation auto-breaks concentration
func TestIntegration_IncapacitationAutoBreaks(t *testing.T) {
	conditions := []CombatCondition{{Condition: "stunned"}}
	currentConcentration := "Bless"

	incapResult := CheckConcentrationOnIncapacitation(currentConcentration, conditions)
	require.True(t, incapResult.Broken)

	breakResult := BreakConcentration("Aria", incapResult.SpellName, fmt.Sprintf("incapacitated — %s", incapResult.Reason), nil)
	assert.Equal(t, "🔮 Aria loses concentration on Bless (incapacitated — stunned)", breakResult.Message)
}

// TDD Cycle 8c: Integration test — silence zone blocks V/S cast and breaks concentration
func TestIntegration_SilenceZoneInteraction(t *testing.T) {
	spell := refdata.Spell{
		Name:          "Hold Person",
		Components:    []string{"V", "S", "M"},
		Concentration: sql.NullBool{Bool: true, Valid: true},
	}

	// Cast blocked in silence
	err := ValidateSilenceZone(true, spell)
	require.Error(t, err)

	// Concentration on V/S spell breaks in silence
	silenceResult := CheckConcentrationInSilence("Hold Person", true, spell)
	require.True(t, silenceResult.Broken)

	breakResult := BreakConcentration("Aria", silenceResult.SpellName, "silence", nil)
	assert.Contains(t, breakResult.Message, "silence")
}

// TDD Cycle 8d: Integration test — spell effect cleanup on concentration break
func TestIntegration_SpellEffectCleanupOnBreak(t *testing.T) {
	var cleanedSpells []string
	cleanup := func(spellName string) {
		cleanedSpells = append(cleanedSpells, spellName)
	}

	// Break via incapacitation
	BreakConcentration("Aria", "Bless", "incapacitated — stunned", cleanup)
	assert.Equal(t, []string{"Bless"}, cleanedSpells)

	// Break via silence
	BreakConcentration("Aria", "Fog Cloud", "silence", cleanup)
	assert.Equal(t, []string{"Bless", "Fog Cloud"}, cleanedSpells)
}
