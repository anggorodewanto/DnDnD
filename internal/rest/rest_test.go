package rest

import (
	"strings"
	"testing"

	"github.com/ab/dndnd/internal/character"
	"github.com/ab/dndnd/internal/dice"
)

// --- TDD Cycle 1: Short rest hit dice spending heals character ---

func TestShortRest_SpendHitDice_SingleClass(t *testing.T) {
	// Fighter level 5 with d10 hit dice, CON 14 (+2), 20/40 HP
	roller := dice.NewRoller(func(max int) int { return 6 }) // always rolls 6
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent: 20,
		HPMax:     40,
		CONModifier: 2,
		HitDiceRemaining: map[string]int{"d10": 5},
		HitDiceSpend:     map[string]int{"d10": 2},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each hit die: 1d10 (rolls 6) + 2 CON = 8 HP each, 2 dice = 16
	if result.HPHealed != 16 {
		t.Errorf("HPHealed = %d, want 16", result.HPHealed)
	}
	if result.HPAfter != 36 {
		t.Errorf("HPAfter = %d, want 36", result.HPAfter)
	}
	if result.HitDiceRemaining["d10"] != 3 {
		t.Errorf("HitDiceRemaining[d10] = %d, want 3", result.HitDiceRemaining["d10"])
	}
}

// --- TDD Cycle 2: Short rest HP capped at max ---

func TestShortRest_HPCappedAtMax(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 10 }) // max roll
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        38,
		HPMax:            40,
		CONModifier:      2,
		HitDiceRemaining: map[string]int{"d10": 5},
		HitDiceSpend:     map[string]int{"d10": 1},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1d10 (10) + 2 CON = 12, but only 2 HP available
	if result.HPAfter != 40 {
		t.Errorf("HPAfter = %d, want 40 (capped at max)", result.HPAfter)
	}
	if result.HPHealed != 2 {
		t.Errorf("HPHealed = %d, want 2", result.HPHealed)
	}
}

// --- TDD Cycle 3: Short rest feature recharge ---

func TestShortRest_FeatureRecharge(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	features := map[string]character.FeatureUse{
		"action-surge":    {Current: 0, Max: 1, Recharge: "short"},
		"second-wind":     {Current: 0, Max: 1, Recharge: "short"},
		"indomitable":     {Current: 0, Max: 1, Recharge: "long"},
		"already-full":    {Current: 2, Max: 2, Recharge: "short"},
	}

	input := ShortRestInput{
		HPCurrent:        40,
		HPMax:            40,
		CONModifier:      2,
		HitDiceRemaining: map[string]int{"d10": 5},
		HitDiceSpend:     map[string]int{},
		FeatureUses:      features,
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recharge action-surge and second-wind (short rest), not indomitable (long) or already-full
	if len(result.FeaturesRecharged) != 2 {
		t.Errorf("FeaturesRecharged count = %d, want 2, got %v", len(result.FeaturesRecharged), result.FeaturesRecharged)
	}
}

// --- TDD Cycle 4: Short rest pact magic slots restore ---

func TestShortRest_PactMagicRestore(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	pact := &character.PactMagicSlots{SlotLevel: 3, Current: 0, Max: 2}

	input := ShortRestInput{
		HPCurrent:        30,
		HPMax:            40,
		CONModifier:      1,
		HitDiceRemaining: map[string]int{"d8": 3},
		HitDiceSpend:     map[string]int{},
		FeatureUses:      map[string]character.FeatureUse{},
		PactMagicSlots:   pact,
		Classes:          []character.ClassEntry{{Class: "warlock", Level: 5}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.PactSlotsRestored {
		t.Error("PactSlotsRestored = false, want true")
	}
	if result.PactSlotsCurrent != 2 {
		t.Errorf("PactSlotsCurrent = %d, want 2", result.PactSlotsCurrent)
	}
}

// --- TDD Cycle 5: Short rest error - spending more dice than available ---

func TestShortRest_Error_SpendMoreThanAvailable(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      2,
		HitDiceRemaining: map[string]int{"d10": 1},
		HitDiceSpend:     map[string]int{"d10": 3},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	_, err := svc.ShortRest(input)
	if err == nil {
		t.Fatal("expected error when spending more hit dice than available")
	}
}

// --- TDD Cycle 6: Multiclass hit dice spending ---

func TestShortRest_Multiclass_HitDice(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 5 }) // always 5
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        20,
		HPMax:            80,
		CONModifier:      1,
		HitDiceRemaining: map[string]int{"d10": 3, "d8": 2},
		HitDiceSpend:     map[string]int{"d10": 1, "d8": 1},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes: []character.ClassEntry{
			{Class: "fighter", Level: 3},
			{Class: "rogue", Level: 2},
		},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// d10: 5 + 1 = 6, d8: 5 + 1 = 6 => 12 total
	if result.HPHealed != 12 {
		t.Errorf("HPHealed = %d, want 12", result.HPHealed)
	}
	if result.HitDiceRemaining["d10"] != 2 {
		t.Errorf("d10 remaining = %d, want 2", result.HitDiceRemaining["d10"])
	}
	if result.HitDiceRemaining["d8"] != 1 {
		t.Errorf("d8 remaining = %d, want 1", result.HitDiceRemaining["d8"])
	}
}

// --- TDD Cycle 7: Long rest - full HP restore ---

func TestLongRest_FullHPRestore(t *testing.T) {
	svc := NewService(nil) // roller not needed for long rest

	input := LongRestInput{
		HPCurrent:        10,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d10": 2},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses: map[string]character.FeatureUse{
			"action-surge": {Current: 0, Max: 1, Recharge: "short"},
			"indomitable":  {Current: 0, Max: 1, Recharge: "long"},
		},
		SpellSlots: map[string]character.SlotInfo{
			"1": {Current: 0, Max: 4},
			"2": {Current: 1, Max: 3},
		},
		PactMagicSlots: &character.PactMagicSlots{SlotLevel: 3, Current: 0, Max: 2},
		DeathSaveSuccesses: 2,
		DeathSaveFailures:  1,
	}

	result := svc.LongRest(input)

	if result.HPAfter != 40 {
		t.Errorf("HPAfter = %d, want 40", result.HPAfter)
	}
	if result.HPHealed != 30 {
		t.Errorf("HPHealed = %d, want 30", result.HPHealed)
	}
}

// --- TDD Cycle 8: Long rest - all spell slots restored ---

func TestLongRest_SpellSlotsRestored(t *testing.T) {
	svc := NewService(nil)

	input := LongRestInput{
		HPCurrent:        40,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d8": 5},
		Classes:          []character.ClassEntry{{Class: "wizard", Level: 5}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots: map[string]character.SlotInfo{
			"1": {Current: 0, Max: 4},
			"2": {Current: 1, Max: 3},
			"3": {Current: 0, Max: 2},
		},
	}

	result := svc.LongRest(input)

	for level, slot := range result.SpellSlots {
		if slot.Current != slot.Max {
			t.Errorf("SpellSlots[%s].Current = %d, want %d", level, slot.Current, slot.Max)
		}
	}
}

// --- TDD Cycle 9: Long rest - features reset (both short and long) ---

func TestLongRest_FeaturesReset(t *testing.T) {
	svc := NewService(nil)

	input := LongRestInput{
		HPCurrent:        40,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d10": 5},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses: map[string]character.FeatureUse{
			"action-surge": {Current: 0, Max: 1, Recharge: "short"},
			"indomitable":  {Current: 0, Max: 1, Recharge: "long"},
			"no-recharge":  {Current: 0, Max: 1, Recharge: ""},
		},
		SpellSlots: map[string]character.SlotInfo{},
	}

	result := svc.LongRest(input)

	if len(result.FeaturesRecharged) != 2 {
		t.Errorf("FeaturesRecharged count = %d, want 2, got %v", len(result.FeaturesRecharged), result.FeaturesRecharged)
	}
}

// --- TDD Cycle 10: Long rest - hit dice restore (half total level, min 1) ---

func TestLongRest_HitDiceRestore(t *testing.T) {
	svc := NewService(nil)

	// Level 5 fighter: total level 5, half = 2 (floor). Had 2 remaining, max is 5.
	// Should restore 2, capped at 3 more to reach 5.
	input := LongRestInput{
		HPCurrent:        40,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d10": 2},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}

	result := svc.LongRest(input)

	if result.HitDiceRemaining["d10"] != 4 {
		t.Errorf("HitDiceRemaining[d10] = %d, want 4 (restored 2)", result.HitDiceRemaining["d10"])
	}
	if result.HitDiceRestored != 2 {
		t.Errorf("HitDiceRestored = %d, want 2", result.HitDiceRestored)
	}
}

// --- TDD Cycle 11: Long rest - death saves reset ---

func TestLongRest_DeathSavesReset(t *testing.T) {
	svc := NewService(nil)

	input := LongRestInput{
		HPCurrent:          40,
		HPMax:              40,
		HitDiceRemaining:   map[string]int{"d10": 5},
		Classes:            []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses:        map[string]character.FeatureUse{},
		SpellSlots:         map[string]character.SlotInfo{},
		DeathSaveSuccesses: 2,
		DeathSaveFailures:  1,
	}

	result := svc.LongRest(input)

	if !result.DeathSavesReset {
		t.Error("DeathSavesReset = false, want true")
	}
}

// --- TDD Cycle 12: Long rest - prepared caster reminder ---

func TestLongRest_PreparedCasterReminder(t *testing.T) {
	svc := NewService(nil)

	tests := []struct {
		name    string
		classes []character.ClassEntry
		want    bool
	}{
		{"cleric", []character.ClassEntry{{Class: "cleric", Level: 5}}, true},
		{"druid", []character.ClassEntry{{Class: "druid", Level: 5}}, true},
		{"paladin", []character.ClassEntry{{Class: "paladin", Level: 5}}, true},
		{"wizard", []character.ClassEntry{{Class: "wizard", Level: 5}}, false},
		{"multiclass cleric/fighter", []character.ClassEntry{
			{Class: "fighter", Level: 3},
			{Class: "cleric", Level: 2},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := LongRestInput{
				HPCurrent:        40,
				HPMax:            40,
				HitDiceRemaining: map[string]int{"d10": 5},
				Classes:          tt.classes,
				FeatureUses:      map[string]character.FeatureUse{},
				SpellSlots:       map[string]character.SlotInfo{},
			}
			result := svc.LongRest(input)
			if result.PreparedCasterReminder != tt.want {
				t.Errorf("PreparedCasterReminder = %v, want %v", result.PreparedCasterReminder, tt.want)
			}
		})
	}
}

// --- TDD Cycle 13: Long rest - pact magic slots restored ---

func TestLongRest_PactMagicRestored(t *testing.T) {
	svc := NewService(nil)

	pact := &character.PactMagicSlots{SlotLevel: 3, Current: 0, Max: 2}
	input := LongRestInput{
		HPCurrent:        40,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d8": 5},
		Classes:          []character.ClassEntry{{Class: "warlock", Level: 5}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
		PactMagicSlots:   pact,
	}

	result := svc.LongRest(input)

	if !result.PactSlotsRestored {
		t.Error("PactSlotsRestored = false, want true")
	}
}

// --- TDD Cycle 14: Long rest hit dice restore minimum 1 ---

func TestLongRest_HitDiceRestore_Min1(t *testing.T) {
	svc := NewService(nil)

	// Level 1 character: half = 0, minimum 1
	input := LongRestInput{
		HPCurrent:        10,
		HPMax:            10,
		HitDiceRemaining: map[string]int{"d10": 0},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 1}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}

	result := svc.LongRest(input)

	if result.HitDiceRemaining["d10"] != 1 {
		t.Errorf("HitDiceRemaining[d10] = %d, want 1 (minimum restore)", result.HitDiceRemaining["d10"])
	}
	if result.HitDiceRestored != 1 {
		t.Errorf("HitDiceRestored = %d, want 1", result.HitDiceRestored)
	}
}

// --- TDD Cycle 15: Short rest spend 0 hit dice (skip) ---

func TestShortRest_SpendZero(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      2,
		HitDiceRemaining: map[string]int{"d10": 5},
		HitDiceSpend:     map[string]int{},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HPHealed != 0 {
		t.Errorf("HPHealed = %d, want 0", result.HPHealed)
	}
	if result.HPAfter != 20 {
		t.Errorf("HPAfter = %d, want 20", result.HPAfter)
	}
}

// --- TDD Cycle 16: Format short rest result ---

func TestFormatShortRestResult(t *testing.T) {
	result := ShortRestResult{
		HPBefore: 20,
		HPAfter:  36,
		HPHealed: 16,
		HitDieRolls: []HitDieRoll{
			{DieType: "d10", Rolled: 6, CONMod: 2, Healed: 8},
			{DieType: "d10", Rolled: 6, CONMod: 2, Healed: 8},
		},
		HitDiceRemaining:  map[string]int{"d10": 3},
		FeaturesRecharged: []string{"action-surge"},
		PactSlotsRestored: false,
	}

	msg := FormatShortRestResult("Thorin", result)
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	if !strings.Contains(msg, "Short Rest") {
		t.Errorf("expected 'Short Rest' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "16 HP") {
		t.Errorf("expected '16 HP' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "action-surge") {
		t.Errorf("expected 'action-surge' in message, got: %s", msg)
	}
}

// --- TDD Cycle 17: Format long rest result ---

func TestFormatLongRestResult(t *testing.T) {
	result := LongRestResult{
		HPBefore:              10,
		HPAfter:               40,
		HPHealed:              30,
		HitDiceRemaining:      map[string]int{"d10": 4},
		HitDiceRestored:       2,
		FeaturesRecharged:     []string{"action-surge", "indomitable"},
		SpellSlots:            map[string]character.SlotInfo{"1": {Current: 4, Max: 4}},
		PactSlotsRestored:     true,
		DeathSavesReset:       true,
		PreparedCasterReminder: false,
	}

	msg := FormatLongRestResult("Thorin", result)
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	if !strings.Contains(msg, "Long Rest") {
		t.Errorf("expected 'Long Rest' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "40/40 HP") {
		t.Errorf("expected '40/40 HP' in message, got: %s", msg)
	}
}

func TestFormatLongRestResult_PreparedCasterReminder(t *testing.T) {
	result := LongRestResult{
		HPBefore:              40,
		HPAfter:               40,
		HitDiceRemaining:      map[string]int{"d8": 5},
		HitDiceRestored:       0,
		FeaturesRecharged:     []string{},
		SpellSlots:            map[string]character.SlotInfo{},
		PreparedCasterReminder: true,
	}

	msg := FormatLongRestResult("Elara", result)
	if !strings.Contains(msg, "/prepare") {
		t.Errorf("expected '/prepare' reminder in message, got: %s", msg)
	}
}


// --- TDD Cycle 18: Short rest negative CON modifier floor at 0 ---

func TestShortRest_NegativeCON_HealingFloor(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 }) // min roll
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      -3, // CON 5 = -3 modifier
		HitDiceRemaining: map[string]int{"d6": 3},
		HitDiceSpend:     map[string]int{"d6": 1},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "wizard", Level: 3}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1d6 (1) + (-3) CON = -2, floored to 0
	if result.HPHealed != 0 {
		t.Errorf("HPHealed = %d, want 0 (negative healing floored)", result.HPHealed)
	}
}

// --- TDD Cycle 19: Long rest multiclass hit dice restore ---

func TestLongRest_Multiclass_HitDiceRestore(t *testing.T) {
	svc := NewService(nil)

	// Fighter 3 (d10) + Rogue 2 (d8) = level 5, restore 2
	// Both empty, should restore across types
	input := LongRestInput{
		HPCurrent:        40,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d10": 0, "d8": 0},
		Classes: []character.ClassEntry{
			{Class: "fighter", Level: 3},
			{Class: "rogue", Level: 2},
		},
		FeatureUses: map[string]character.FeatureUse{},
		SpellSlots:  map[string]character.SlotInfo{},
	}

	result := svc.LongRest(input)

	totalRestored := 0
	for _, v := range result.HitDiceRemaining {
		totalRestored += v
	}
	if totalRestored != 2 {
		t.Errorf("total hit dice restored = %d, want 2", totalRestored)
	}
	if result.HitDiceRestored != 2 {
		t.Errorf("HitDiceRestored = %d, want 2", result.HitDiceRestored)
	}
}

// --- TDD Cycle 20: Short rest with pact already full ---

func TestShortRest_PactMagicAlreadyFull(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	pact := &character.PactMagicSlots{SlotLevel: 3, Current: 2, Max: 2}
	input := ShortRestInput{
		HPCurrent:        30,
		HPMax:            40,
		CONModifier:      1,
		HitDiceRemaining: map[string]int{"d8": 3},
		HitDiceSpend:     map[string]int{},
		FeatureUses:      map[string]character.FeatureUse{},
		PactMagicSlots:   pact,
		Classes:          []character.ClassEntry{{Class: "warlock", Level: 5}},
	}

	result, err := svc.ShortRest(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PactSlotsRestored {
		t.Error("PactSlotsRestored = true, want false (already full)")
	}
}

// --- Short rest with no die type in remaining map ---

func TestShortRest_Error_MissingDieType(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      0,
		HitDiceRemaining: map[string]int{"d10": 3},
		HitDiceSpend:     map[string]int{"d8": 1}, // trying to spend a die type not in remaining
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	_, err := svc.ShortRest(input)
	if err == nil {
		t.Fatal("expected error for missing die type")
	}
}

// --- Format short rest with 0 healed ---

func TestFormatShortRestResult_NoHealing(t *testing.T) {
	result := ShortRestResult{
		HPBefore:         40,
		HPAfter:          40,
		HPHealed:         0,
		HitDieRolls:      nil,
		HitDiceRemaining: map[string]int{"d10": 5},
	}

	msg := FormatShortRestResult("Thorin", result)
	if !strings.Contains(msg, "No hit dice spent") {
		t.Errorf("expected 'No hit dice spent' in message, got: %s", msg)
	}
}

// --- Format short rest with pact slot restore ---

func TestFormatShortRestResult_PactRestore(t *testing.T) {
	result := ShortRestResult{
		HPBefore:          30,
		HPAfter:           30,
		HPHealed:          0,
		HitDiceRemaining:  map[string]int{"d8": 3},
		PactSlotsRestored: true,
		PactSlotsCurrent:  2,
	}

	msg := FormatShortRestResult("Eldarin", result)
	if !strings.Contains(msg, "Pact magic slots restored") {
		t.Errorf("expected pact restore message, got: %s", msg)
	}
}

// --- Long rest with hit dice already full ---

func TestLongRest_HitDiceAlreadyFull(t *testing.T) {
	svc := NewService(nil)

	input := LongRestInput{
		HPCurrent:        40,
		HPMax:            40,
		HitDiceRemaining: map[string]int{"d10": 5},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}

	result := svc.LongRest(input)

	if result.HitDiceRemaining["d10"] != 5 {
		t.Errorf("HitDiceRemaining[d10] = %d, want 5 (already full)", result.HitDiceRemaining["d10"])
	}
	if result.HitDiceRestored != 0 {
		t.Errorf("HitDiceRestored = %d, want 0", result.HitDiceRestored)
	}
}

// --- Test various class hit die mappings ---

func TestLongRest_AllClassHitDice(t *testing.T) {
	svc := NewService(nil)

	classes := []character.ClassEntry{
		{Class: "barbarian", Level: 1},
		{Class: "sorcerer", Level: 1},
		{Class: "bard", Level: 1},
		{Class: "ranger", Level: 1},
	}

	input := LongRestInput{
		HPCurrent:        50,
		HPMax:            50,
		HitDiceRemaining: map[string]int{"d12": 0, "d6": 0, "d8": 0, "d10": 0},
		Classes:          classes,
		FeatureUses:      map[string]character.FeatureUse{},
		SpellSlots:       map[string]character.SlotInfo{},
	}

	result := svc.LongRest(input)

	// Level 4 total, restore 2
	if result.HitDiceRestored != 2 {
		t.Errorf("HitDiceRestored = %d, want 2", result.HitDiceRestored)
	}
}

// --- TDD Cycle 33: FormatShortRestResult shows HPMax not HPAfter ---

func TestFormatShortRestResult_ShowsHPMax(t *testing.T) {
	result := ShortRestResult{
		HPBefore:         20,
		HPAfter:          36,
		HPMax:            44,
		HPHealed:         16,
		HitDieRolls:      []HitDieRoll{{DieType: "d10", Rolled: 6, CONMod: 2, Healed: 8}},
		HitDiceRemaining: map[string]int{"d10": 4},
	}

	msg := FormatShortRestResult("Thorin", result)
	if !strings.Contains(msg, "36/44") {
		t.Errorf("expected '36/44' (HPAfter/HPMax) in message, got: %s", msg)
	}
}

// --- TDD Cycle 34: FormatShortRestResult no healing shows HPMax ---

func TestFormatShortRestResult_NoHealing_ShowsHPMax(t *testing.T) {
	result := ShortRestResult{
		HPBefore:         40,
		HPAfter:          40,
		HPMax:            44,
		HPHealed:         0,
		HitDiceRemaining: map[string]int{"d10": 5},
	}

	msg := FormatShortRestResult("Thorin", result)
	if !strings.Contains(msg, "40/44") {
		t.Errorf("expected '40/44' in message, got: %s", msg)
	}
}

// --- Short rest error invalid die type ---

func TestShortRest_Error_InvalidDieType(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	svc := NewService(roller)

	input := ShortRestInput{
		HPCurrent:        20,
		HPMax:            40,
		CONModifier:      0,
		HitDiceRemaining: map[string]int{"d4": 3},
		HitDiceSpend:     map[string]int{"d4": 1},
		FeatureUses:      map[string]character.FeatureUse{},
		Classes:          []character.ClassEntry{{Class: "fighter", Level: 5}},
	}

	_, err := svc.ShortRest(input)
	if err == nil {
		t.Fatal("expected error for invalid die type d4")
	}
}
