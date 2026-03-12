package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/dice"
)

func TestDistance3D_SamePosition(t *testing.T) {
	got := Distance3D(0, 0, 0, 0, 0, 0)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestDistance3D_HorizontalOnly(t *testing.T) {
	// 3 tiles east = 15ft horizontal, no altitude difference
	got := Distance3D(0, 0, 0, 3, 0, 0)
	if got != 15 {
		t.Errorf("expected 15, got %d", got)
	}
}

func TestDistance3D_VerticalOnly(t *testing.T) {
	// Same tile, 30ft altitude difference
	got := Distance3D(0, 0, 0, 0, 0, 30)
	if got != 30 {
		t.Errorf("expected 30, got %d", got)
	}
}

func TestDistance3D_3D_Pythagorean(t *testing.T) {
	// 3 tiles east (15ft), 4 tiles south (20ft), 0 altitude => sqrt(15^2+20^2) = 25
	got := Distance3D(0, 0, 0, 3, 4, 0)
	if got != 25 {
		t.Errorf("expected 25, got %d", got)
	}
}

func TestDistance3D_WithAltitude(t *testing.T) {
	// 6 tiles east (30ft), 0 rows, altitude diff 40ft => sqrt(30^2+40^2) = 50
	got := Distance3D(0, 0, 0, 6, 0, 40)
	if got != 50 {
		t.Errorf("expected 50, got %d", got)
	}
}

func TestDistance3D_RoundsToNearest5(t *testing.T) {
	// 2 tiles east (10ft), altitude 10ft => sqrt(100+100)=14.14 => rounds to 15
	got := Distance3D(0, 0, 0, 2, 0, 10)
	if got != 15 {
		t.Errorf("expected 15, got %d", got)
	}
}

func TestDistance3D_RoundsDown(t *testing.T) {
	// 1 tile east (5ft), altitude 5ft => sqrt(25+25) = 7.07 => rounds to 5
	got := Distance3D(0, 0, 0, 1, 0, 5)
	if got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
}

func TestValidateFly_Ascend(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      30,
		CurrentAltitude:     0,
		MovementRemainingFt: 30,
	}
	result := ValidateFly(req)
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Reason)
	}
	if result.CostFt != 30 {
		t.Errorf("expected cost 30, got %d", result.CostFt)
	}
	if result.RemainingFt != 0 {
		t.Errorf("expected 0 remaining, got %d", result.RemainingFt)
	}
}

func TestValidateFly_Descend(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      0,
		CurrentAltitude:     30,
		MovementRemainingFt: 30,
	}
	result := ValidateFly(req)
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Reason)
	}
	if result.CostFt != 30 {
		t.Errorf("expected cost 30, got %d", result.CostFt)
	}
}

func TestValidateFly_PartialAscend(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      20,
		CurrentAltitude:     10,
		MovementRemainingFt: 30,
	}
	result := ValidateFly(req)
	if !result.Valid {
		t.Fatalf("expected valid, got: %s", result.Reason)
	}
	if result.CostFt != 10 {
		t.Errorf("expected cost 10, got %d", result.CostFt)
	}
	if result.RemainingFt != 20 {
		t.Errorf("expected 20 remaining, got %d", result.RemainingFt)
	}
}

func TestValidateFly_NotEnoughMovement(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      50,
		CurrentAltitude:     0,
		MovementRemainingFt: 30,
	}
	result := ValidateFly(req)
	if result.Valid {
		t.Fatal("expected invalid")
	}
	if result.Reason == "" {
		t.Error("expected reason")
	}
}

func TestValidateFly_NegativeAltitude(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      -10,
		CurrentAltitude:     0,
		MovementRemainingFt: 30,
	}
	result := ValidateFly(req)
	if result.Valid {
		t.Fatal("expected invalid for negative altitude")
	}
}

func TestValidateFly_SameAltitude(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      30,
		CurrentAltitude:     30,
		MovementRemainingFt: 30,
	}
	result := ValidateFly(req)
	if result.Valid {
		t.Fatal("expected invalid for same altitude")
	}
}

func TestValidateFly_ZeroMovement(t *testing.T) {
	req := FlyRequest{
		TargetAltitude:      30,
		CurrentAltitude:     0,
		MovementRemainingFt: 0,
	}
	result := ValidateFly(req)
	if result.Valid {
		t.Fatal("expected invalid for zero movement")
	}
}

func TestFallDamage_30ft(t *testing.T) {
	// 30ft fall = 3d6, deterministic roller always returns 3
	roller := dice.NewRoller(func(max int) int { return 3 })
	result, err := FallDamage(30, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3d6, each die rolls 3+1=4 (roller returns 3, die indexing adds 1)
	// Actually dice.Roll returns the roller result + 1 for 1-indexed dice
	if result.NumDice != 3 {
		t.Errorf("expected 3 dice, got %d", result.NumDice)
	}
	if result.AltitudeFt != 30 {
		t.Errorf("expected altitude 30, got %d", result.AltitudeFt)
	}
	if result.TotalDamage <= 0 {
		t.Error("expected positive damage")
	}
}

func TestFallDamage_0ft(t *testing.T) {
	roller := dice.NewRoller(func(max int) int { return 1 })
	result, err := FallDamage(0, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalDamage != 0 {
		t.Errorf("expected 0 damage for 0ft fall, got %d", result.TotalDamage)
	}
	if result.NumDice != 0 {
		t.Errorf("expected 0 dice for 0ft fall, got %d", result.NumDice)
	}
}

func TestFallDamage_5ft(t *testing.T) {
	// 5ft fall = 0d6 (rounds down to 0 ten-foot increments)
	roller := dice.NewRoller(func(max int) int { return 1 })
	result, err := FallDamage(5, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalDamage != 0 {
		t.Errorf("expected 0 damage for 5ft fall, got %d", result.TotalDamage)
	}
}

func TestFallDamage_10ft(t *testing.T) {
	// 10ft fall = 1d6
	roller := dice.NewRoller(func(max int) int { return 5 }) // randFn returns 5 directly
	result, err := FallDamage(10, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumDice != 1 {
		t.Errorf("expected 1 die, got %d", result.NumDice)
	}
	if result.TotalDamage != 5 {
		t.Errorf("expected 5 damage (1d6, roller returns 5), got %d", result.TotalDamage)
	}
}

func TestFallDamage_15ft(t *testing.T) {
	// 15ft fall = 1d6 (15/10 = 1.5, truncated to 1)
	roller := dice.NewRoller(func(max int) int { return 5 })
	result, err := FallDamage(15, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumDice != 1 {
		t.Errorf("expected 1 die, got %d", result.NumDice)
	}
}

func TestFallDamage_100ft(t *testing.T) {
	// 100ft fall = 10d6
	roller := dice.NewRoller(func(max int) int { return 3 })
	result, err := FallDamage(100, roller)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NumDice != 10 {
		t.Errorf("expected 10 dice, got %d", result.NumDice)
	}
	if result.TotalDamage != 30 { // 10 dice * 3 each
		t.Errorf("expected 30 damage, got %d", result.TotalDamage)
	}
	if result.RollResult == nil {
		t.Error("expected roll result")
	}
}

func TestFormatFlyConfirmation_Ascend(t *testing.T) {
	result := &FlyResult{
		Valid:       true,
		CostFt:      30,
		RemainingFt: 0,
		NewAltitude: 30,
	}
	msg := FormatFlyConfirmation(result)
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestFormatFlyConfirmation_Descend(t *testing.T) {
	result := &FlyResult{
		Valid:       true,
		CostFt:      30,
		RemainingFt: 0,
		NewAltitude: 0,
	}
	msg := FormatFlyConfirmation(result)
	if msg == "" {
		t.Error("expected non-empty message")
	}
}
