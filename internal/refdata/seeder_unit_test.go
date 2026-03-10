package refdata

import (
	"encoding/json"
	"testing"
)

func TestMustJSON(t *testing.T) {
	effects := []MechanicalEffect{
		{EffectType: "cant_see"},
		{EffectType: "auto_fail_ability_check", Condition: "requires_sight"},
	}

	result := mustJSON(effects)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	var parsed []MechanicalEffect
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(parsed))
	}
	if parsed[0].EffectType != "cant_see" {
		t.Fatalf("expected effect_type cant_see, got %q", parsed[0].EffectType)
	}
	if parsed[1].Condition != "requires_sight" {
		t.Fatalf("expected condition requires_sight, got %q", parsed[1].Condition)
	}
}

func TestMechanicalEffectJSON(t *testing.T) {
	effect := MechanicalEffect{
		EffectType:  "grant_advantage",
		Description: "Attacks have advantage",
		Target:      "attack_rolls",
		Condition:   "within_5ft",
		Value:       "2",
	}

	b, err := json.Marshal(effect)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed MechanicalEffect
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.EffectType != "grant_advantage" {
		t.Fatalf("expected effect_type grant_advantage, got %q", parsed.EffectType)
	}
	if parsed.Target != "attack_rolls" {
		t.Fatalf("expected target attack_rolls, got %q", parsed.Target)
	}
}

func TestMechanicalEffectJSON_OmitsEmpty(t *testing.T) {
	effect := MechanicalEffect{
		EffectType: "cant_see",
	}

	b, err := json.Marshal(effect)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := m["description"]; ok {
		t.Fatal("expected description to be omitted when empty")
	}
	if _, ok := m["target"]; ok {
		t.Fatal("expected target to be omitted when empty")
	}
	if _, ok := m["condition"]; ok {
		t.Fatal("expected condition to be omitted when empty")
	}
	if _, ok := m["value"]; ok {
		t.Fatal("expected value to be omitted when empty")
	}
}

func TestOptHelpers(t *testing.T) {
	f := optFloat(3.5)
	if !f.Valid || f.Float64 != 3.5 {
		t.Fatalf("optFloat failed: %v", f)
	}

	i := optInt(10)
	if !i.Valid || i.Int32 != 10 {
		t.Fatalf("optInt failed: %v", i)
	}

	s := optStr("hello")
	if !s.Valid || s.String != "hello" {
		t.Fatalf("optStr failed: %v", s)
	}

	b := optBool(true)
	if !b.Valid || !b.Bool {
		t.Fatalf("optBool(true) failed: %v", b)
	}

	b2 := optBool(false)
	if !b2.Valid || b2.Bool {
		t.Fatalf("optBool(false) failed: %v", b2)
	}
}
