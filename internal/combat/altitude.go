package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/ab/dndnd/internal/dice"
	"github.com/ab/dndnd/internal/refdata"
)

const (
	FlySpellID                  = "fly"
	PolymorphSpellID            = "polymorph"
	FlySpeedCondition           = "fly_speed"
	PolymorphNonFlyingCondition = "polymorphed_non_flying"
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
	HasFlySpeed         bool
}

// CombatantHasFlySpeed returns true if the combatant has a fly speed
// (either from the fly_speed condition or innate creature fly speed).
func CombatantHasFlySpeed(conditions json.RawMessage) bool {
	return HasCondition(conditions, FlySpeedCondition)
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
	if !req.HasFlySpeed {
		return &FlyResult{Valid: false, Reason: "You don't have a fly speed."}
	}

	if req.TargetAltitude < 0 {
		return &FlyResult{Valid: false, Reason: "Altitude cannot be negative."}
	}

	if req.TargetAltitude == req.CurrentAltitude {
		return &FlyResult{Valid: false, Reason: fmt.Sprintf("Already at %dft altitude.", req.CurrentAltitude)}
	}

	if req.MovementRemainingFt <= 0 {
		return &FlyResult{Valid: false, Reason: "No movement remaining."}
	}

	cost := abs32(req.TargetAltitude - req.CurrentAltitude)
	remaining := req.MovementRemainingFt - cost

	if remaining < 0 {
		return &FlyResult{
			Valid:  false,
			Reason: fmt.Sprintf("Not enough movement: %dft needed, %dft remaining.", cost, req.MovementRemainingFt),
		}
	}

	return &FlyResult{
		Valid:       true,
		CostFt:      int(cost),
		RemainingFt: int(remaining),
		NewAltitude: req.TargetAltitude,
	}
}

// abs32 returns the absolute value of an int32.
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
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

// applyFallDamageOnProne fires the Phase 31 fall-damage hook whenever a
// combatant transitions to prone while airborne (C-31). It rolls
// FallDamage at the current altitude, applies the rolled damage through
// the standard ApplyDamage pipeline (so resistances / vulnerabilities /
// concentration saves all fire), and resets altitude to 0 via
// UpdateCombatantPosition. Returns the combat-log message and the
// final combatant snapshot. A 0ft altitude is treated as a no-op by the
// caller (we never reach here on grounded prone). Errors are returned
// so the surrounding ApplyCondition surfaces them.
func (s *Service) applyFallDamageOnProne(ctx context.Context, combatant refdata.Combatant) (string, refdata.Combatant, error) {
	return s.applyFallDamageFromAltitude(ctx, combatant)
}

func (s *Service) applyFallDamageFromAltitude(ctx context.Context, combatant refdata.Combatant) (string, refdata.Combatant, error) {
	if combatant.AltitudeFt <= 0 {
		return "", combatant, nil
	}
	fall, err := FallDamage(combatant.AltitudeFt, s.roller)
	if err != nil {
		return "", combatant, fmt.Errorf("rolling fall damage: %w", err)
	}
	altitudeBefore := combatant.AltitudeFt
	// Reset altitude regardless of damage (a 5ft fall still grounds the
	// combatant); UpdateCombatantPosition writes through the same column.
	groundedCombatant, perr := s.store.UpdateCombatantPosition(ctx, refdata.UpdateCombatantPositionParams{
		ID:          combatant.ID,
		PositionCol: combatant.PositionCol,
		PositionRow: combatant.PositionRow,
		AltitudeFt:  0,
	})
	if perr != nil {
		return "", combatant, fmt.Errorf("grounding combatant after fall: %w", perr)
	}
	if fall.TotalDamage <= 0 {
		return fmt.Sprintf("\U0001f4a8 %s falls %dft (no damage).", combatant.DisplayName, altitudeBefore), groundedCombatant, nil
	}
	dmgResult, derr := s.ApplyDamage(ctx, ApplyDamageInput{
		EncounterID: combatant.EncounterID,
		Target:      groundedCombatant,
		RawDamage:   fall.TotalDamage,
		DamageType:  "bludgeoning",
		Override:    false,
	})
	if derr != nil {
		return "", groundedCombatant, fmt.Errorf("applying fall damage: %w", derr)
	}
	msg := fmt.Sprintf("\U0001f4a8 %s falls %dft \u2014 %dd6 = %d bludgeoning damage.",
		combatant.DisplayName, altitudeBefore, fall.NumDice, fall.TotalDamage)
	return msg, dmgResult.Updated, nil
}
