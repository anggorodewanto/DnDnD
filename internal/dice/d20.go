package dice

import (
	"fmt"
	"time"
)

// RollMode represents advantage, disadvantage, or normal rolling.
type RollMode int

const (
	Normal                  RollMode = iota
	Advantage                        // Roll 2d20, take higher
	Disadvantage                     // Roll 2d20, take lower
	AdvantageAndDisadvantage         // Both apply, cancel out to normal
)

// String returns a human-readable label for the roll mode.
func (m RollMode) String() string {
	switch m {
	case Advantage:
		return "advantage"
	case Disadvantage:
		return "disadvantage"
	case AdvantageAndDisadvantage:
		return "advantage+disadvantage"
	default:
		return "normal"
	}
}

// D20Result holds the result of a d20 roll with full breakdown.
type D20Result struct {
	Rolls        []int    `json:"rolls"`
	Chosen       int      `json:"chosen"`
	Modifier     int      `json:"modifier"`
	Total        int      `json:"total"`
	Mode         RollMode `json:"mode"`
	CriticalHit  bool     `json:"critical_hit"`
	CriticalFail bool     `json:"critical_fail"`
	Breakdown    string   `json:"breakdown"`
	Timestamp    time.Time `json:"timestamp"`
}

// RollD20 rolls a d20 with the given modifier and roll mode.
// It handles advantage, disadvantage, and their cancellation.
func (r *Roller) RollD20(modifier int, mode RollMode) (D20Result, error) {
	// Advantage + Disadvantage cancel out
	if mode == AdvantageAndDisadvantage {
		mode = Normal
	}

	result := D20Result{
		Modifier:  modifier,
		Mode:      mode,
		Timestamp: time.Now(),
	}

	switch mode {
	case Normal:
		roll := r.randFn(20)
		result.Rolls = []int{roll}
		result.Chosen = roll
	case Advantage:
		r1 := r.randFn(20)
		r2 := r.randFn(20)
		result.Rolls = []int{r1, r2}
		result.Chosen = max(r1, r2)
	case Disadvantage:
		r1 := r.randFn(20)
		r2 := r.randFn(20)
		result.Rolls = []int{r1, r2}
		result.Chosen = min(r1, r2)
	}

	result.CriticalHit = result.Chosen == 20
	result.CriticalFail = result.Chosen == 1
	result.Total = result.Chosen + modifier
	result.Breakdown = formatD20Breakdown(result)

	return result, nil
}

// CombineRollModes merges two roll modes. When both are non-normal and
// different (advantage vs disadvantage), they cancel to AdvantageAndDisadvantage.
func CombineRollModes(a, b RollMode) RollMode {
	if a == Normal {
		return b
	}
	if b == Normal {
		return a
	}
	if a == b {
		return a
	}
	return AdvantageAndDisadvantage
}

func formatD20Breakdown(r D20Result) string {
	if len(r.Rolls) == 1 {
		if r.Modifier == 0 {
			return fmt.Sprintf("%d", r.Chosen)
		}
		return fmt.Sprintf("%d + %d = %d", r.Chosen, r.Modifier, r.Total)
	}

	label := "higher"
	if r.Mode == Disadvantage {
		label = "lower"
	}

	if r.Modifier == 0 {
		return fmt.Sprintf("%d / %d (%s: %d)", r.Rolls[0], r.Rolls[1], label, r.Chosen)
	}
	return fmt.Sprintf("%d / %d (%s: %d + %d = %d)", r.Rolls[0], r.Rolls[1], label, r.Chosen, r.Modifier, r.Total)
}
