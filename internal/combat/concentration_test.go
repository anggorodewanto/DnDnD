package combat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

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
		spellName  string
		components []string
		wantErr    bool
		wantMsg    string
	}{
		{"not in silence — OK", false, "Bless", []string{"V", "S", "M"}, false, ""},
		{"in silence with V — blocked", true, "Bless", []string{"V", "M"}, true, "You cannot cast Bless — you are inside a zone of Silence (requires verbal/somatic components)."},
		{"in silence with S — blocked", true, "Hold Person", []string{"S"}, true, "You cannot cast Hold Person — you are inside a zone of Silence (requires verbal/somatic components)."},
		{"in silence with M only — OK", true, "Identify", []string{"M"}, false, ""},
		{"in silence with no components — OK", true, "Test", nil, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spell := refdata.Spell{Name: tc.spellName, Components: tc.components}
			err := ValidateSilenceZone(tc.inSilence, spell)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.wantMsg, err.Error())
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
				Name:       tc.wantSpell,
				Components: tc.components,
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
		Name:       "Hold Person",
		Components: []string{"V", "S", "M"},
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

// TDD Cycle 9: RemoveSpellSourcedConditions strips conditions whose
// (source_combatant_id, source_spell) match across every combatant in an
// encounter and persists the updates. Generalizes the per-combatant
// breakInvisibilityAndPersist helper from Phase 113.
func TestRemoveSpellSourcedConditions(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()
	otherCasterID := uuid.New()
	target1ID := uuid.New()
	target2ID := uuid.New()
	target3ID := uuid.New()

	c1Conds, _ := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: casterID.String(), SourceSpell: "invisibility"},
	})
	// Target 2 has TWO matching conditions; both should be stripped.
	c2Conds, _ := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: casterID.String(), SourceSpell: "invisibility"},
		{Condition: "blessed", SourceCombatantID: casterID.String(), SourceSpell: "invisibility"},
	})
	// Target 3 has unrelated conditions that must be preserved.
	c3Conds, _ := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: otherCasterID.String(), SourceSpell: "invisibility"},
		{Condition: "charmed", SourceCombatantID: casterID.String(), SourceSpell: "charm-person"},
	})

	updates := make(map[uuid.UUID]json.RawMessage)
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			assert.Equal(t, encounterID, encID)
			return []refdata.Combatant{
				{ID: target1ID, EncounterID: encounterID, Conditions: c1Conds},
				{ID: target2ID, EncounterID: encounterID, Conditions: c2Conds},
				{ID: target3ID, EncounterID: encounterID, Conditions: c3Conds},
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			updates[arg.ID] = arg.Conditions
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, Conditions: arg.Conditions}, nil
		},
	}

	svc := NewService(ms)
	n, err := svc.RemoveSpellSourcedConditions(context.Background(), encounterID, casterID, "invisibility")
	require.NoError(t, err)
	// 1 (target1) + 2 (target2) = 3 conditions removed.
	assert.Equal(t, 3, n)

	// target1 had its sole condition stripped → empty array
	require.Contains(t, updates, target1ID)
	var c1Out []CombatCondition
	require.NoError(t, json.Unmarshal(updates[target1ID], &c1Out))
	assert.Empty(t, c1Out)

	// target2 had both conditions stripped → empty array
	require.Contains(t, updates, target2ID)
	var c2Out []CombatCondition
	require.NoError(t, json.Unmarshal(updates[target2ID], &c2Out))
	assert.Empty(t, c2Out)

	// target3 was not updated (no matches)
	assert.NotContains(t, updates, target3ID)
}

func TestRemoveSpellSourcedConditions_NoMatches(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()
	combatantID := uuid.New()

	conds, _ := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: uuid.New().String(), SourceSpell: "invisibility"},
	})

	called := false
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{{ID: combatantID, EncounterID: encounterID, Conditions: conds}}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			called = true
			return refdata.Combatant{ID: arg.ID}, nil
		},
	}
	svc := NewService(ms)
	n, err := svc.RemoveSpellSourcedConditions(context.Background(), encounterID, casterID, "invisibility")
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, called, "no update should be issued when nothing matched")
}

func TestRemoveSpellSourcedConditions_ListError(t *testing.T) {
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, errors.New("boom")
		},
	}
	svc := NewService(ms)
	_, err := svc.RemoveSpellSourcedConditions(context.Background(), uuid.New(), uuid.New(), "x")
	require.Error(t, err)
}

// TDD Cycle 13 (Phase 118): MaybeCreateConcentrationSaveOnDamage creates a
// pending CON save row when a concentrating combatant takes damage.
func TestMaybeCreateConcentrationSaveOnDamage(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var captured refdata.CreatePendingSaveParams
	called := false
	ms := &mockStore{
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			captured = arg
			called = true
			return refdata.PendingSafe{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID, Source: arg.Source}, nil
		},
	}
	svc := NewService(ms)
	ps, err := svc.MaybeCreateConcentrationSaveOnDamage(context.Background(), encounterID, combatantID, 30)
	require.NoError(t, err)
	require.True(t, called)
	require.NotNil(t, ps)
	assert.Equal(t, encounterID, captured.EncounterID)
	assert.Equal(t, combatantID, captured.CombatantID)
	assert.Equal(t, "con", captured.Ability)
	// damage 30 → DC max(10, 15) = 15
	assert.Equal(t, int32(15), captured.Dc)
	assert.Equal(t, "concentration", captured.Source)
}

func TestMaybeCreateConcentrationSaveOnDamage_NotConcentrating(t *testing.T) {
	called := false
	ms := &mockStore{
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			called = true
			return refdata.PendingSafe{}, nil
		},
	}
	svc := NewService(ms)
	ps, err := svc.MaybeCreateConcentrationSaveOnDamage(context.Background(), uuid.New(), uuid.New(), 30)
	require.NoError(t, err)
	assert.Nil(t, ps)
	assert.False(t, called)
}

func TestMaybeCreateConcentrationSaveOnDamage_NoDamage(t *testing.T) {
	called := false
	ms := &mockStore{
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			called = true
			return refdata.PendingSafe{}, nil
		},
	}
	svc := NewService(ms)
	ps, err := svc.MaybeCreateConcentrationSaveOnDamage(context.Background(), uuid.New(), uuid.New(), 0)
	require.NoError(t, err)
	assert.Nil(t, ps)
	assert.False(t, called)
}

// TDD Cycle 14 (Phase 118): ResolveConcentrationSave fires
// BreakConcentrationFully when the resolved save failed.
func TestResolveConcentrationSave_FailureTriggersBreak(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var (
		zoneCleanupCalled bool
		clearCalled       bool
	)
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, EncounterID: encounterID, DisplayName: "Aria"}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zoneCleanupCalled = true
			return 0, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearCalled = true
			return nil
		},
	}
	svc := NewService(ms)
	result, err := svc.ResolveConcentrationSave(context.Background(), refdata.PendingSafe{
		ID:          uuid.New(),
		EncounterID: encounterID,
		CombatantID: combatantID,
		Source:      "concentration",
		Success:     sql.NullBool{Bool: false, Valid: true},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Broken)
	assert.True(t, zoneCleanupCalled)
	assert.True(t, clearCalled)
	assert.Contains(t, result.ConsolidatedMessage, "Bless")
}

func TestResolveConcentrationSave_SuccessIsNoop(t *testing.T) {
	zoneCleanupCalled := false
	ms := &mockStore{
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zoneCleanupCalled = true
			return 0, nil
		},
	}
	svc := NewService(ms)
	result, err := svc.ResolveConcentrationSave(context.Background(), refdata.PendingSafe{
		Source:  "concentration",
		Success: sql.NullBool{Bool: true, Valid: true},
	})
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.False(t, zoneCleanupCalled)
}

func TestResolveConcentrationSave_WrongSourceIsNoop(t *testing.T) {
	zoneCleanupCalled := false
	ms := &mockStore{
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zoneCleanupCalled = true
			return 0, nil
		},
	}
	svc := NewService(ms)
	result, err := svc.ResolveConcentrationSave(context.Background(), refdata.PendingSafe{
		Source:  "spell",
		Success: sql.NullBool{Bool: false, Valid: true},
	})
	require.NoError(t, err)
	assert.Nil(t, result)
	assert.False(t, zoneCleanupCalled)
}

// TDD Cycle 11 (Phase 118): ApplyCondition triggers concentration auto-break
// when an incapacitating condition is applied to a concentrating target.
func TestApplyCondition_AutoBreaksConcentration_OnIncapacitation(t *testing.T) {
	encounterID := uuid.New()
	targetID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[]`),
	}

	var (
		zonesDeletedForCombatant uuid.UUID
		clearedConcCombatant     uuid.UUID
		setConditionsCalled      bool
	)
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return target, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			setConditionsCalled = true
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, Conditions: arg.Conditions, DisplayName: target.DisplayName}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{}, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zonesDeletedForCombatant = id
			return 0, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearedConcCombatant = id
			return nil
		},
		// Simulate the target was concentrating on Bless.
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
	}

	svc := NewService(ms)
	updated, msgs, err := svc.ApplyCondition(context.Background(), targetID, CombatCondition{Condition: "stunned"})
	require.NoError(t, err)
	assert.True(t, setConditionsCalled)
	assert.NotEmpty(t, updated.ID)

	// Concentration cleanup ran for the target.
	assert.Equal(t, targetID, zonesDeletedForCombatant, "auto-break must delete concentration zones for the target")
	assert.Equal(t, targetID, clearedConcCombatant, "auto-break must clear the concentration columns")

	// At least one of the messages mentions the consolidated cleanup.
	var foundCleanup bool
	for _, m := range msgs {
		if strings.Contains(m, "💨") && strings.Contains(m, "Bless") {
			foundCleanup = true
			break
		}
	}
	assert.True(t, foundCleanup, "expected a 💨 cleanup line in messages, got %v", msgs)
}

// TDD Cycle 12 (Phase 118): non-incapacitating condition does NOT trigger break.
func TestApplyCondition_NonIncapacitatingDoesNotBreakConcentration(t *testing.T) {
	encounterID := uuid.New()
	targetID := uuid.New()

	target := refdata.Combatant{
		ID:          targetID,
		EncounterID: encounterID,
		DisplayName: "Aria",
		Conditions:  json.RawMessage(`[]`),
	}

	zoneCleanupCalled := false
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return target, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, Conditions: arg.Conditions, DisplayName: target.DisplayName}, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zoneCleanupCalled = true
			return 0, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
	}

	svc := NewService(ms)
	_, _, err := svc.ApplyCondition(context.Background(), targetID, CombatCondition{Condition: "frightened"})
	require.NoError(t, err)
	assert.False(t, zoneCleanupCalled, "non-incapacitating condition must not trigger concentration cleanup")
}

// TDD Cycle 15 (Phase 118 iter-2): consolidated cleanup log line carries the
// trigger reason in parentheses and is the ONLY concentration log line emitted
// from the cleanup path (no legacy 🔮).
func TestFormatConcentrationCleanupLog_IncludesReason(t *testing.T) {
	tests := []struct {
		name   string
		caster string
		spell  string
		reason string
		n      int
		want   string
	}{
		{
			name:   "failed CON save",
			caster: "Aria",
			spell:  "Bless",
			reason: "failed CON save",
			n:      2,
			want:   "💨 Aria lost concentration on Bless (failed CON save) — effects ended on 2 targets.",
		},
		{
			name:   "incapacitation",
			caster: "Aria",
			spell:  "Web",
			reason: "incapacitated — stunned",
			n:      1,
			want:   "💨 Aria lost concentration on Web (incapacitated — stunned) — effects ended on 1 targets.",
		},
		{
			name:   "voluntary drop",
			caster: "Aria",
			spell:  "Invisibility",
			reason: "voluntary drop",
			n:      1,
			want:   "💨 Aria lost concentration on Invisibility (voluntary drop) — effects ended on 1 targets.",
		},
		{
			name:   "silence",
			caster: "Aria",
			spell:  "Hold Person",
			reason: "silence",
			n:      0,
			want:   "💨 Aria lost concentration on Hold Person (silence) — effects ended on 0 targets.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatConcentrationCleanupLog(tc.caster, tc.spell, tc.reason, tc.n)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TDD Cycle 10: BreakConcentrationFully orchestrates the full concentration
// break pipeline: strip spell-sourced conditions across the encounter,
// delete concentration zones, dismiss summons, clear the caster's
// concentration columns, and emit a consolidated combat log line.
func TestBreakConcentrationFully(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()
	target1ID := uuid.New()
	target2ID := uuid.New()
	wolf1ID := uuid.New()

	c1Conds, _ := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: casterID.String(), SourceSpell: "invisibility"},
	})
	c2Conds, _ := json.Marshal([]CombatCondition{
		{Condition: "invisible", SourceCombatantID: casterID.String(), SourceSpell: "invisibility"},
	})

	var (
		clearedCasterID         uuid.UUID
		zonesDeletedForCombatID uuid.UUID
		deletedCombatantIDs     []uuid.UUID
		conditionUpdates        = make(map[uuid.UUID]json.RawMessage)
	)
	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: target1ID, EncounterID: encounterID, Conditions: c1Conds, DisplayName: "Goblin 1"},
				{ID: target2ID, EncounterID: encounterID, Conditions: c2Conds, DisplayName: "Goblin 2"},
				{ID: wolf1ID, EncounterID: encounterID, Conditions: json.RawMessage(`[]`), SummonerID: uuid.NullUUID{UUID: casterID, Valid: true}, ShortID: "WF1"},
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			conditionUpdates[arg.ID] = arg.Conditions
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, Conditions: arg.Conditions}, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, combID uuid.UUID) (int64, error) {
			zonesDeletedForCombatID = combID
			return 0, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			deletedCombatantIDs = append(deletedCombatantIDs, id)
			return nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearedCasterID = id
			return nil
		},
	}

	svc := NewService(ms)
	result, err := svc.BreakConcentrationFully(context.Background(), BreakConcentrationFullyInput{
		EncounterID: encounterID,
		CasterID:    casterID,
		CasterName:  "Aria",
		SpellID:     "invisibility",
		SpellName:   "Invisibility",
		Reason:      "failed CON save",
	})
	require.NoError(t, err)

	assert.True(t, result.Broken)
	assert.Equal(t, 2, result.ConditionsRemoved, "two combatants had spell-sourced invisibility")
	assert.Equal(t, 1, result.SummonsDismissed)
	assert.Equal(t, 0, result.ZonesRemoved)
	assert.Equal(t, casterID, zonesDeletedForCombatID)
	assert.Equal(t, casterID, clearedCasterID)
	assert.Contains(t, deletedCombatantIDs, wolf1ID)

	// Consolidated 💨 log line is the only line emitted from cleanup,
	// includes the trigger reason in parens.
	assert.Equal(t,
		"💨 Aria lost concentration on Invisibility (failed CON save) — effects ended on 3 targets.",
		result.ConsolidatedMessage,
	)

	// Conditions on both targets were cleared.
	require.Contains(t, conditionUpdates, target1ID)
	require.Contains(t, conditionUpdates, target2ID)
}

// TDD Cycle 16 (Phase 118b): when the zone-cleanup query removes >0 rows,
// the consolidated N counter must include them alongside conditions+summons.
func TestBreakConcentrationFully_IncludesZonesInN(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()
	target1ID := uuid.New()

	c1Conds, _ := json.Marshal([]CombatCondition{
		{Condition: "restrained", SourceCombatantID: casterID.String(), SourceSpell: "web"},
	})

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: target1ID, EncounterID: encounterID, Conditions: c1Conds, DisplayName: "Goblin 1"},
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, Conditions: arg.Conditions}, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, combID uuid.UUID) (int64, error) {
			return 2, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error { return nil },
	}

	svc := NewService(ms)
	result, err := svc.BreakConcentrationFully(context.Background(), BreakConcentrationFullyInput{
		EncounterID: encounterID,
		CasterID:    casterID,
		CasterName:  "Aria",
		SpellID:     "web",
		SpellName:   "Web",
		Reason:      "failed CON save",
	})
	require.NoError(t, err)

	assert.True(t, result.Broken)
	assert.Equal(t, 1, result.ConditionsRemoved)
	assert.Equal(t, 0, result.SummonsDismissed)
	assert.Equal(t, 2, result.ZonesRemoved, "Phase 118b: zone rowcount surfaces in the result")

	// N = 1 condition + 0 summons + 2 zones = 3
	assert.Equal(t,
		"💨 Aria lost concentration on Web (failed CON save) — effects ended on 3 targets.",
		result.ConsolidatedMessage,
	)
}

func TestBreakConcentrationFully_ZeroEffects(t *testing.T) {
	// Caster broke concentration but had no spell-sourced conditions or summons.
	// The consolidated log line should still emit with N=0.
	encounterID := uuid.New()
	casterID := uuid.New()

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, combID uuid.UUID) (int64, error) { return 0, nil },
		clearCombatantConcentrationFn:         func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	svc := NewService(ms)
	result, err := svc.BreakConcentrationFully(context.Background(), BreakConcentrationFullyInput{
		EncounterID: encounterID,
		CasterID:    casterID,
		CasterName:  "Aria",
		SpellID:     "bless",
		SpellName:   "Bless",
		Reason:      "voluntary drop",
	})
	require.NoError(t, err)
	assert.True(t, result.Broken)
	assert.Equal(t, 0, result.ConditionsRemoved)
	assert.Equal(t, 0, result.SummonsDismissed)
	assert.Equal(t, 0, result.ZonesRemoved)
	assert.Equal(t,
		"💨 Aria lost concentration on Bless (voluntary drop) — effects ended on 0 targets.",
		result.ConsolidatedMessage,
	)
}

func TestBreakConcentrationAndDismissSummons(t *testing.T) {
	summonerID := uuid.New()
	encounterID := uuid.New()
	wolf1ID := uuid.New()
	wolf2ID := uuid.New()

	var deletedIDs []uuid.UUID
	var cleanedSpell string

	ms := &mockStore{
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: wolf1ID, SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, ShortID: "WF1"},
				{ID: wolf2ID, SummonerID: uuid.NullUUID{UUID: summonerID, Valid: true}, ShortID: "WF2"},
				{ID: uuid.New(), SummonerID: uuid.NullUUID{}, ShortID: "G1"},
			}, nil
		},
		deleteCombatantFn: func(ctx context.Context, id uuid.UUID) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		},
	}

	svc := NewService(ms)
	cleanup := func(spellName string) {
		cleanedSpell = spellName
	}

	result, dismissed, err := svc.BreakConcentrationAndDismissSummons(
		context.Background(), encounterID, summonerID,
		"Aria", "Conjure Animals", "failed CON save", cleanup,
	)
	require.NoError(t, err)
	assert.True(t, result.Broken)
	assert.Equal(t, "Conjure Animals", result.SpellName)
	assert.Contains(t, result.Message, "Aria")
	assert.Equal(t, "Conjure Animals", cleanedSpell)
	assert.Equal(t, 2, dismissed)
	assert.Contains(t, deletedIDs, wolf1ID)
	assert.Contains(t, deletedIDs, wolf2ID)
}

// TDD Cycle 16 (Phase 118 iter-2): applyDamageHP is the centralized HP-update
// helper for damage paths. It updates HP, then enqueues a concentration save
// when damage > 0 AND target is concentrating, AND applies the unconscious
// condition when HP transitions from >0 to 0 on a non-dead target.
func TestApplyDamageHP_TriggersConcentrationSave(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var capturedSave refdata.CreatePendingSaveParams
	saveCreated := false
	ms := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "bless", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Bless", Valid: true},
			}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			capturedSave = arg
			saveCreated = true
			return refdata.PendingSafe{ID: uuid.New(), EncounterID: arg.EncounterID, CombatantID: arg.CombatantID}, nil
		},
	}
	svc := NewService(ms)
	// prevHP=20, newHP=5 → damage=15 → DC 10
	_, err := svc.applyDamageHP(context.Background(), encounterID, combatantID, 20, 5, 0, true)
	require.NoError(t, err)
	require.True(t, saveCreated)
	assert.Equal(t, "concentration", capturedSave.Source)
	assert.Equal(t, int32(10), capturedSave.Dc)
}

func TestApplyDamageHP_NoSaveWhenHealing(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	saveCreated := false
	ms := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, HpCurrent: arg.HpCurrent, IsAlive: arg.IsAlive}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			saveCreated = true
			return refdata.PendingSafe{}, nil
		},
	}
	svc := NewService(ms)
	// prevHP=10, newHP=20 → no damage
	_, err := svc.applyDamageHP(context.Background(), encounterID, combatantID, 10, 20, 0, true)
	require.NoError(t, err)
	assert.False(t, saveCreated, "healing must not enqueue concentration save")
}

func TestApplyDamageHP_AppliesUnconsciousAtZeroHP(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	var appliedConditions []CombatCondition
	ms := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          arg.ID,
				EncounterID: encounterID,
				HpCurrent:   arg.HpCurrent,
				IsAlive:     arg.IsAlive,
				DisplayName: "Aria",
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          id,
				EncounterID: encounterID,
				HpCurrent:   0,
				IsAlive:     true,
				DisplayName: "Aria",
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			var conds []CombatCondition
			_ = json.Unmarshal(arg.Conditions, &conds)
			appliedConditions = conds
			return refdata.Combatant{ID: arg.ID, EncounterID: encounterID, Conditions: arg.Conditions, DisplayName: "Aria"}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			return refdata.PendingSafe{}, nil
		},
	}
	svc := NewService(ms)
	// prevHP=10, newHP=0, isAlive=true (dying, not dead)
	_, err := svc.applyDamageHP(context.Background(), encounterID, combatantID, 10, 0, 0, true)
	require.NoError(t, err)
	require.Len(t, appliedConditions, 1)
	assert.Equal(t, "unconscious", appliedConditions[0].Condition)
}

func TestApplyDamageHP_DoesNotApplyUnconsciousWhenAlreadyDead(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	conditionsCalled := false
	ms := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          arg.ID,
				EncounterID: encounterID,
				HpCurrent:   arg.HpCurrent,
				IsAlive:     arg.IsAlive,
				DisplayName: "Goblin",
				Conditions:  json.RawMessage(`[]`),
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			conditionsCalled = true
			return refdata.Combatant{ID: arg.ID}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			return refdata.PendingSafe{}, nil
		},
	}
	svc := NewService(ms)
	// isAlive=false → corpse, no unconscious condition.
	_, err := svc.applyDamageHP(context.Background(), encounterID, combatantID, 5, 0, 0, false)
	require.NoError(t, err)
	assert.False(t, conditionsCalled, "dead targets must not be marked unconscious")
}

// TDD Cycle 18 (Phase 118 iter-2): silence-zone entry detection. When a
// concentrating caster (V/S spell) ends up inside a Silence zone — either
// by moving into it or by the zone being placed over them — concentration
// breaks via the cleanup pipeline.

func TestCheckSilenceBreaksConcentration_BreaksWhenInside(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()

	zoneID := uuid.New()
	clearCalled := false
	zoneDeleteCalled := false
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID: id, EncounterID: encounterID, DisplayName: "Aria",
				PositionCol: "B", PositionRow: 2,
			}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "hold-person", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true},
			}, nil
		},
		getSpellFn: func(ctx context.Context, id string) (refdata.Spell, error) {
			assert.Equal(t, "hold-person", id)
			return refdata.Spell{ID: id, Name: "Hold Person", Components: []string{"V", "S"}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{{
				ID: zoneID, EncounterID: encID, ZoneType: "silence",
				OriginCol: "A", OriginRow: 1, Shape: "square",
				Dimensions: json.RawMessage(`{"side_ft":20}`),
			}}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) {
			zoneDeleteCalled = true
			return 0, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearCalled = true
			return nil
		},
	}

	svc := NewService(ms)
	cleanup, err := svc.CheckSilenceBreaksConcentration(context.Background(), casterID)
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.True(t, cleanup.Broken)
	assert.True(t, clearCalled)
	assert.True(t, zoneDeleteCalled)
	assert.Contains(t, cleanup.ConsolidatedMessage, "silence")
}

func TestCheckSilenceBreaksConcentration_NoSilenceZone(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()

	clearCalled := false
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, EncounterID: encounterID, PositionCol: "B", PositionRow: 2}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "hold-person", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true},
			}, nil
		},
		getSpellFn: func(ctx context.Context, id string) (refdata.Spell, error) {
			return refdata.Spell{ID: id, Components: []string{"V", "S"}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.EncounterZone, error) {
			return nil, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearCalled = true
			return nil
		},
	}
	svc := NewService(ms)
	cleanup, err := svc.CheckSilenceBreaksConcentration(context.Background(), casterID)
	require.NoError(t, err)
	assert.Nil(t, cleanup)
	assert.False(t, clearCalled)
}

func TestCheckSilenceBreaksConcentration_NotConcentrating(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()

	listCalled := false
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, EncounterID: encounterID}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.EncounterZone, error) {
			listCalled = true
			return nil, nil
		},
	}
	svc := NewService(ms)
	cleanup, err := svc.CheckSilenceBreaksConcentration(context.Background(), casterID)
	require.NoError(t, err)
	assert.Nil(t, cleanup)
	assert.False(t, listCalled, "non-concentrating combatants should short-circuit before zone lookup")
}

func TestCheckSilenceBreaksConcentration_NonVSSpell(t *testing.T) {
	encounterID := uuid.New()
	casterID := uuid.New()

	clearCalled := false
	ms := &mockStore{
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{ID: id, EncounterID: encounterID, PositionCol: "A", PositionRow: 1}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "wall-of-force", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Wall of Force", Valid: true},
			}, nil
		},
		// Wall of Force has no V/S components — material only.
		getSpellFn: func(ctx context.Context, id string) (refdata.Spell, error) {
			return refdata.Spell{ID: id, Components: []string{"M"}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{{
				ID: uuid.New(), EncounterID: encID, ZoneType: "silence",
				OriginCol: "A", OriginRow: 1, Shape: "square",
				Dimensions: json.RawMessage(`{"side_ft":20}`),
			}}, nil
		},
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearCalled = true
			return nil
		},
	}
	svc := NewService(ms)
	cleanup, err := svc.CheckSilenceBreaksConcentration(context.Background(), casterID)
	require.NoError(t, err)
	assert.Nil(t, cleanup)
	assert.False(t, clearCalled, "non-V/S spell must not break in silence")
}

// TDD Cycle 20 (Phase 118 iter-2): CreateZone runs the silence hook so any
// concentrating caster (V/S) standing inside the new Silence zone's
// footprint breaks at zone creation.
func TestCreateZone_SilenceFiresHookForConcentratorsInside(t *testing.T) {
	encounterID := uuid.New()
	concentratorID := uuid.New()
	otherID := uuid.New()

	clearedIDs := make(map[uuid.UUID]bool)
	ms := &mockStore{
		createEncounterZoneFn: func(ctx context.Context, arg refdata.CreateEncounterZoneParams) (refdata.EncounterZone, error) {
			return refdata.EncounterZone{
				ID: uuid.New(), EncounterID: arg.EncounterID, ZoneType: arg.ZoneType,
				OriginCol: arg.OriginCol, OriginRow: arg.OriginRow,
				Shape: arg.Shape, Dimensions: arg.Dimensions,
				SourceCombatantID: arg.SourceCombatantID, SourceSpell: arg.SourceSpell,
			}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return []refdata.Combatant{
				{ID: concentratorID, EncounterID: encID, DisplayName: "Aria", PositionCol: "A", PositionRow: 1},
				{ID: otherID, EncounterID: encID, DisplayName: "Goblin", PositionCol: "Z", PositionRow: 99},
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			if id == concentratorID {
				return refdata.Combatant{ID: id, EncounterID: encounterID, DisplayName: "Aria", PositionCol: "A", PositionRow: 1}, nil
			}
			return refdata.Combatant{ID: id, EncounterID: encounterID, DisplayName: "Goblin", PositionCol: "Z", PositionRow: 99}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			if id == concentratorID {
				return refdata.GetCombatantConcentrationRow{
					ConcentrationSpellID:   sql.NullString{String: "hold-person", Valid: true},
					ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true},
				}, nil
			}
			return refdata.GetCombatantConcentrationRow{}, nil
		},
		getSpellFn: func(ctx context.Context, id string) (refdata.Spell, error) {
			return refdata.Spell{ID: id, Components: []string{"V", "S"}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{{
				ID: uuid.New(), EncounterID: encID, ZoneType: "silence",
				OriginCol: "A", OriginRow: 1, Shape: "square",
				Dimensions: json.RawMessage(`{"side_ft":20}`),
			}}, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) { return 0, nil },
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearedIDs[id] = true
			return nil
		},
	}
	svc := NewService(ms)
	_, err := svc.CreateZone(context.Background(), CreateZoneInput{
		EncounterID:       encounterID,
		SourceCombatantID: uuid.New(),
		SourceSpell:       "silence",
		Shape:             "square",
		OriginCol:         "A",
		OriginRow:         1,
		Dimensions:        json.RawMessage(`{"side_ft":20}`),
		AnchorMode:        "fixed",
		ZoneType:          "silence",
		OverlayColor:      "#888888",
	})
	require.NoError(t, err)
	assert.True(t, clearedIDs[concentratorID], "concentrator inside the silence zone must be broken")
	assert.False(t, clearedIDs[otherID], "non-concentrator outside footprint must be untouched")
}

// TDD Cycle 19 (Phase 118 iter-2): UpdateCombatantPosition runs the silence
// hook so a concentrating caster who walks into a Silence zone breaks.
func TestUpdateCombatantPosition_FiresSilenceHook(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	clearCalled := false
	ms := &mockStore{
		updateCombatantPositionFn: func(ctx context.Context, arg refdata.UpdateCombatantPositionParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID: arg.ID, EncounterID: encounterID, DisplayName: "Aria",
				PositionCol: arg.PositionCol, PositionRow: arg.PositionRow,
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID: id, EncounterID: encounterID, DisplayName: "Aria",
				PositionCol: "B", PositionRow: 1,
			}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{
				ConcentrationSpellID:   sql.NullString{String: "hold-person", Valid: true},
				ConcentrationSpellName: sql.NullString{String: "Hold Person", Valid: true},
			}, nil
		},
		getSpellFn: func(ctx context.Context, id string) (refdata.Spell, error) {
			return refdata.Spell{ID: id, Components: []string{"V", "S"}}, nil
		},
		listEncounterZonesByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.EncounterZone, error) {
			return []refdata.EncounterZone{{
				ID: uuid.New(), EncounterID: encID, ZoneType: "silence",
				OriginCol: "A", OriginRow: 1, Shape: "square",
				Dimensions: json.RawMessage(`{"side_ft":20}`),
			}}, nil
		},
		listCombatantsByEncounterIDFn: func(ctx context.Context, encID uuid.UUID) ([]refdata.Combatant, error) {
			return nil, nil
		},
		deleteConcentrationZonesByCombatantFn: func(ctx context.Context, id uuid.UUID) (int64, error) { return 0, nil },
		clearCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) error {
			clearCalled = true
			return nil
		},
	}
	svc := NewService(ms)
	_, err := svc.UpdateCombatantPosition(context.Background(), combatantID, "B", 1, 0)
	require.NoError(t, err)
	assert.True(t, clearCalled, "movement into silence zone must trigger concentration cleanup")
}

func TestApplyDamageHP_DoesNotDoubleApplyUnconscious(t *testing.T) {
	encounterID := uuid.New()
	combatantID := uuid.New()

	existingConds, _ := json.Marshal([]CombatCondition{{Condition: "unconscious"}})
	conditionsCalled := false
	ms := &mockStore{
		updateCombatantHPFn: func(ctx context.Context, arg refdata.UpdateCombatantHPParams) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          arg.ID,
				EncounterID: encounterID,
				HpCurrent:   arg.HpCurrent,
				IsAlive:     arg.IsAlive,
				DisplayName: "Aria",
				Conditions:  existingConds,
			}, nil
		},
		getCombatantFn: func(ctx context.Context, id uuid.UUID) (refdata.Combatant, error) {
			return refdata.Combatant{
				ID:          id,
				EncounterID: encounterID,
				HpCurrent:   0,
				IsAlive:     true,
				DisplayName: "Aria",
				Conditions:  existingConds,
			}, nil
		},
		updateCombatantConditionsFn: func(ctx context.Context, arg refdata.UpdateCombatantConditionsParams) (refdata.Combatant, error) {
			conditionsCalled = true
			return refdata.Combatant{ID: arg.ID}, nil
		},
		getCombatantConcentrationFn: func(ctx context.Context, id uuid.UUID) (refdata.GetCombatantConcentrationRow, error) {
			return refdata.GetCombatantConcentrationRow{}, nil
		},
		createPendingSaveFn: func(ctx context.Context, arg refdata.CreatePendingSaveParams) (refdata.PendingSafe, error) {
			return refdata.PendingSafe{}, nil
		},
	}
	svc := NewService(ms)
	_, err := svc.applyDamageHP(context.Background(), encounterID, combatantID, 5, 0, 0, true)
	require.NoError(t, err)
	assert.False(t, conditionsCalled, "must not re-add unconscious if already present")
}
