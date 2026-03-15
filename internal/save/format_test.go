package save

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/dice"
)

func TestFormatSaveResult_Basic(t *testing.T) {
	result := SaveResult{
		Ability:  "dex",
		Modifier: 5,
		Total:    15,
		D20Result: dice.D20Result{
			Rolls:     []int{10},
			Chosen:    10,
			Total:     15,
			Mode:      dice.Normal,
			Breakdown: "10 + 5 = 15",
		},
	}
	msg := FormatSaveResult("Aria", result)
	if !strings.Contains(msg, "DEX Save") {
		t.Errorf("expected DEX Save in message, got: %s", msg)
	}
	if !strings.Contains(msg, "15") {
		t.Errorf("expected total 15 in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Aria") {
		t.Errorf("expected Aria in message, got: %s", msg)
	}
}

func TestFormatSaveResult_AutoFail(t *testing.T) {
	result := SaveResult{
		Ability:          "dex",
		AutoFail:         true,
		ConditionReasons: []string{"paralyzed: auto-fail DEX save"},
	}
	msg := FormatSaveResult("Aria", result)
	if !strings.Contains(msg, "Auto-fail") {
		t.Errorf("expected Auto-fail in message, got: %s", msg)
	}
	if !strings.Contains(msg, "paralyzed") {
		t.Errorf("expected paralyzed reason in message, got: %s", msg)
	}
}

func TestFormatSaveResult_WithConditionReasons(t *testing.T) {
	result := SaveResult{
		Ability:  "dex",
		Modifier: 2,
		Total:    12,
		D20Result: dice.D20Result{
			Rolls:     []int{10, 8},
			Chosen:    8,
			Total:     12,
			Mode:      dice.Disadvantage,
			Breakdown: "10 / 8 (lower: 8 + 2 = 10)",
		},
		ConditionReasons: []string{"restrained: disadvantage on DEX save"},
	}
	msg := FormatSaveResult("Aria", result)
	if !strings.Contains(msg, "restrained") {
		t.Errorf("expected restrained reason in message, got: %s", msg)
	}
	if !strings.Contains(msg, "disadvantage") {
		t.Errorf("expected disadvantage in message, got: %s", msg)
	}
}

func TestFormatSaveResult_WithFeatureReasons(t *testing.T) {
	result := SaveResult{
		Ability:      "dex",
		Modifier:     2,
		FeatureBonus: 3,
		Total:        15,
		D20Result: dice.D20Result{
			Rolls:     []int{10},
			Chosen:    10,
			Total:     15,
			Mode:      dice.Normal,
			Breakdown: "10 + 5 = 15",
		},
		FeatureReasons: []string{"Aura of Protection: +3"},
	}
	msg := FormatSaveResult("Aria", result)
	if !strings.Contains(msg, "Aura of Protection") {
		t.Errorf("expected Aura of Protection in message, got: %s", msg)
	}
}

func TestFormatSaveResult_UpperCaseAbility(t *testing.T) {
	result := SaveResult{
		Ability:  "str",
		Modifier: 4,
		Total:    14,
		D20Result: dice.D20Result{
			Rolls:     []int{10},
			Chosen:    10,
			Total:     14,
			Mode:      dice.Normal,
			Breakdown: "10 + 4 = 14",
		},
	}
	msg := FormatSaveResult("Bob", result)
	if !strings.Contains(msg, "STR Save") {
		t.Errorf("expected STR Save, got: %s", msg)
	}
}
