package refdata

import (
	"bytes"
	"log/slog"
	"testing"
)

func hasWarning(warnings []SpellWarning, spellID, check string) bool {
	for _, w := range warnings {
		if w.SpellID == spellID && w.Check == check {
			return true
		}
	}
	return false
}

func TestValidateSpells_SaveAbilityWithoutSaveEffect(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			SaveAbility:    optStr("dex"),
			ResolutionMode: "auto",
			Damage:         optJSON(map[string]any{"dice": "1d6", "type": "fire"}),
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "save_ability_without_save_effect") {
		t.Fatalf("expected save_ability_without_save_effect warning, got %v", warnings)
	}
}

func TestValidateSpells_SaveEffectWithoutSaveAbility(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			SaveEffect:     optStr("half_damage"),
			ResolutionMode: "auto",
			Damage:         optJSON(map[string]any{"dice": "1d6", "type": "fire"}),
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "save_effect_without_save_ability") {
		t.Fatalf("expected save_effect_without_save_ability warning, got %v", warnings)
	}
}

func TestValidateSpells_MaterialCostWithoutDescription(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			MaterialCostGp: optFloat(100),
			ResolutionMode: "dm_required",
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "material_cost_without_description") {
		t.Fatalf("expected material_cost_without_description warning, got %v", warnings)
	}
}

func TestValidateSpells_MaterialConsumedWithoutCost(t *testing.T) {
	spells := []sp{
		{
			ID:               "test-spell",
			Name:             "Test Spell",
			MaterialConsumed: optBool(true),
			ResolutionMode:   "dm_required",
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "material_consumed_without_cost") {
		t.Fatalf("expected material_consumed_without_cost warning, got %v", warnings)
	}
}

func TestValidateSpells_ConcentrationDurationMismatch(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			Concentration:  optBool(true),
			Duration:       "1 minute",
			ResolutionMode: "dm_required",
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "concentration_duration_mismatch") {
		t.Fatalf("expected concentration_duration_mismatch warning, got %v", warnings)
	}
}

func TestValidateSpells_DamageWithoutResolution(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			Damage:         optJSON(map[string]any{"dice": "1d6", "type": "fire"}),
			ResolutionMode: "auto",
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "damage_without_resolution") {
		t.Fatalf("expected damage_without_resolution warning, got %v", warnings)
	}
}

func TestValidateSpells_AoeWithoutSave(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			AreaOfEffect:   optJSON(map[string]any{"shape": "sphere", "radius_ft": 20}),
			ResolutionMode: "auto",
			Damage:         optJSON(map[string]any{"dice": "1d6", "type": "fire"}),
			AttackType:     optStr("ranged"),
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "aoe_without_save") {
		t.Fatalf("expected aoe_without_save warning, got %v", warnings)
	}
}

func TestValidateSpells_AutoWithoutMechanicalEffect(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-spell",
			Name:           "Test Spell",
			ResolutionMode: "auto",
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "test-spell", "auto_without_mechanical_effect") {
		t.Fatalf("expected auto_without_mechanical_effect warning, got %v", warnings)
	}
}

func TestValidateSpells_NoWarningForValidSpell(t *testing.T) {
	spells := []sp{
		{
			ID:             "fireball",
			Name:           "Fireball",
			Damage:         optJSON(map[string]any{"dice": "8d6", "type": "fire"}),
			SaveAbility:    optStr("dex"),
			SaveEffect:     optStr("half_damage"),
			AreaOfEffect:   optJSON(map[string]any{"shape": "sphere", "radius_ft": 20}),
			ResolutionMode: "auto",
			Duration:       "Instantaneous",
		},
	}

	warnings := ValidateSpells(spells)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for valid spell, got %v", warnings)
	}
}

func TestValidateSpells_NoWarningForDmRequired(t *testing.T) {
	// dm_required spells should not trigger "auto_without_mechanical_effect"
	spells := []sp{
		{
			ID:             "wish",
			Name:           "Wish",
			ResolutionMode: "dm_required",
			Duration:       "Instantaneous",
		},
	}

	warnings := ValidateSpells(spells)
	if hasWarning(warnings, "wish", "auto_without_mechanical_effect") {
		t.Fatal("dm_required spell should not trigger auto_without_mechanical_effect")
	}
}

func TestLogSpellValidationWarnings_LogsOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	warnings := LogSpellValidationWarnings(logger)

	// Verify it processes all SRD spells without panic
	if warnings == nil {
		t.Fatal("expected non-nil warnings slice")
	}

	// If there are warnings, verify they were logged
	if len(warnings) > 0 {
		logOutput := buf.String()
		if logOutput == "" {
			t.Fatal("expected log output for warnings")
		}
		// Check that structured fields are present
		if !bytes.Contains(buf.Bytes(), []byte("spell_id")) {
			t.Fatal("expected 'spell_id' in log output")
		}
	}
}

func TestValidateSpells_SRDDataQuality(t *testing.T) {
	spells := srdSpells()
	warnings := ValidateSpells(spells)

	// Log all warnings for visibility
	for _, w := range warnings {
		t.Logf("WARNING: spell=%s check=%s msg=%s", w.SpellID, w.Check, w.Message)
	}

	// SRD data should have no critical issues (save_ability/save_effect mismatch)
	for _, w := range warnings {
		if w.Check == "save_ability_without_save_effect" || w.Check == "save_effect_without_save_ability" {
			t.Errorf("SRD spell %s has save mismatch: %s", w.SpellID, w.Message)
		}
	}
}

func TestValidateSpells_ConcentrationNoWarning(t *testing.T) {
	spells := []sp{
		{
			ID:             "test-conc",
			Name:           "Test Concentration",
			Concentration:  optBool(true),
			Duration:       "Concentration, up to 1 minute",
			ResolutionMode: "dm_required",
		},
	}

	warnings := ValidateSpells(spells)
	if hasWarning(warnings, "test-conc", "concentration_duration_mismatch") {
		t.Fatal("spell with Concentration in duration should not trigger mismatch")
	}
}

func TestValidateSpells_MaterialConsumedFalseNoCostOK(t *testing.T) {
	// material_consumed=false without cost should NOT warn
	spells := []sp{
		{
			ID:               "test-spell",
			Name:             "Test Spell",
			MaterialConsumed: optBool(false),
			ResolutionMode:   "dm_required",
		},
	}

	warnings := ValidateSpells(spells)
	if hasWarning(warnings, "test-spell", "material_consumed_without_cost") {
		t.Fatal("material_consumed=false should not trigger warning")
	}
}

func TestValidateSpells_MultipleWarningsOnSameSpell(t *testing.T) {
	spells := []sp{
		{
			ID:               "bad-spell",
			Name:             "Bad Spell",
			SaveAbility:      optStr("dex"),
			MaterialConsumed: optBool(true),
			Concentration:    optBool(true),
			Duration:         "1 minute",
			ResolutionMode:   "auto",
		},
	}

	warnings := ValidateSpells(spells)
	if !hasWarning(warnings, "bad-spell", "save_ability_without_save_effect") {
		t.Fatal("expected save_ability_without_save_effect warning")
	}
	if !hasWarning(warnings, "bad-spell", "material_consumed_without_cost") {
		t.Fatal("expected material_consumed_without_cost warning")
	}
	if !hasWarning(warnings, "bad-spell", "concentration_duration_mismatch") {
		t.Fatal("expected concentration_duration_mismatch warning")
	}
	if !hasWarning(warnings, "bad-spell", "auto_without_mechanical_effect") {
		t.Fatal("expected auto_without_mechanical_effect warning")
	}
}
