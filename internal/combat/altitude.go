package combat

import (
	"fmt"
	"math"

	"github.com/ab/dndnd/internal/dice"
)

// Distance3D computes 3D Euclidean distance between two grid positions with altitude,
// rounded to the nearest 5ft. col/row are 0-based grid coordinates (each tile = 5ft).
// altitudeFt values are in feet.
func Distance3D(col1, row1 int, alt1Ft int, col2, row2 int, alt2Ft int) int {
	dx := float64(col2-col1) * 5.0
	dy := float64(row2-row1) * 5.0
	dz := float64(alt2Ft - alt1Ft)
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	return roundToNearest5(dist)
}

// roundToNearest5 rounds a float to the nearest multiple of 5.
func roundToNearest5(f float64) int {
	return int(math.Round(f/5.0)) * 5
}

// FlyRequest holds everything needed to validate a /fly command.
type FlyRequest struct {
	TargetAltitude      int32
	CurrentAltitude     int32
	MovementRemainingFt int32
}

// FlyResult holds the result of a fly validation.
type FlyResult struct {
	Valid       bool
	Reason      string
	CostFt      int
	RemainingFt int
	NewAltitude int32
}

// ValidateFly checks whether a fly altitude change is valid and returns the cost.
func ValidateFly(req FlyRequest) *FlyResult {
	if req.TargetAltitude < 0 {
		return &FlyResult{Valid: false, Reason: "Altitude cannot be negative."}
	}

	if req.TargetAltitude == req.CurrentAltitude {
		return &FlyResult{Valid: false, Reason: fmt.Sprintf("Already at %dft altitude.", req.CurrentAltitude)}
	}

	diff := req.TargetAltitude - req.CurrentAltitude
	if diff < 0 {
		diff = -diff
	}
	cost := int(diff)

	if req.MovementRemainingFt <= 0 {
		return &FlyResult{Valid: false, Reason: "No movement remaining."}
	}

	if cost > int(req.MovementRemainingFt) {
		return &FlyResult{
			Valid:  false,
			Reason: fmt.Sprintf("Not enough movement: %dft needed, %dft remaining.", cost, req.MovementRemainingFt),
		}
	}

	return &FlyResult{
		Valid:       true,
		CostFt:      cost,
		RemainingFt: int(req.MovementRemainingFt) - cost,
		NewAltitude: req.TargetAltitude,
	}
}

// FallDamageResult holds the outcome of fall damage calculation.
type FallDamageResult struct {
	AltitudeFt  int32
	NumDice     int
	TotalDamage int
	RollResult  *dice.RollResult
}

// FallDamage calculates fall damage: 1d6 per 10ft fallen (truncated).
// Returns zero damage for falls < 10ft.
func FallDamage(altitudeFt int32, roller *dice.Roller) (*FallDamageResult, error) {
	numDice := int(altitudeFt) / 10
	if numDice <= 0 {
		return &FallDamageResult{
			AltitudeFt:  altitudeFt,
			NumDice:     0,
			TotalDamage: 0,
		}, nil
	}

	expr := fmt.Sprintf("%dd6", numDice)
	rollResult, err := roller.Roll(expr)
	if err != nil {
		return nil, fmt.Errorf("rolling fall damage: %w", err)
	}

	return &FallDamageResult{
		AltitudeFt:  altitudeFt,
		NumDice:     numDice,
		TotalDamage: rollResult.Total,
		RollResult:  &rollResult,
	}, nil
}

// FormatFlyConfirmation produces the ephemeral confirmation message for a fly command.
func FormatFlyConfirmation(result *FlyResult) string {
	if result.NewAltitude == 0 {
		return fmt.Sprintf("\U0001f985 Descend to ground level \u2014 %dft, %dft remaining after.", result.CostFt, result.RemainingFt)
	}
	return fmt.Sprintf("\U0001f985 Fly to %dft altitude \u2014 %dft, %dft remaining after.", result.NewAltitude, result.CostFt, result.RemainingFt)
}
