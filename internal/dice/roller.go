package dice

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// RandSource abstracts random number generation for testability.
type RandSource func(max int) int

// GroupResult holds the results for one dice group in a roll.
type GroupResult struct {
	Die      int   `json:"die"`
	Count    int   `json:"count"`
	Results  []int `json:"results"`
	Modifier int   `json:"modifier"`
	Total    int   `json:"total"`
	Purpose  string `json:"purpose,omitempty"`
}

// RollResult holds the complete result of a dice roll with full breakdown.
type RollResult struct {
	Expression string        `json:"expression"`
	Groups     []GroupResult `json:"dice_rolls"`
	Modifier   int           `json:"modifier"`
	Total      int           `json:"total"`
	Critical   bool          `json:"critical,omitempty"`
	Breakdown  string        `json:"breakdown"`
	Timestamp  time.Time     `json:"timestamp"`
}

// Roller handles dice rolling with a configurable random source.
type Roller struct {
	randFn RandSource
}

// NewRoller creates a new Roller. If randFn is nil, crypto/rand is used.
func NewRoller(randFn RandSource) *Roller {
	if randFn == nil {
		randFn = cryptoRand
	}
	return &Roller{randFn: randFn}
}

func cryptoRand(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return int(n.Int64()) + 1
}

// Roll parses a dice expression and rolls the dice, returning a full breakdown.
func (r *Roller) Roll(expression string) (RollResult, error) {
	expr, err := ParseExpression(expression)
	if err != nil {
		return RollResult{}, err
	}

	result := RollResult{
		Expression: expression,
		Modifier:   expr.Modifier,
		Timestamp:  time.Now(),
	}

	total := 0
	for _, g := range expr.Groups {
		gr := GroupResult{
			Die:   g.Sides,
			Count: g.Count,
		}
		groupTotal := 0
		for i := 0; i < g.Count; i++ {
			roll := r.randFn(g.Sides)
			gr.Results = append(gr.Results, roll)
			groupTotal += roll
		}
		gr.Total = groupTotal
		total += groupTotal
		result.Groups = append(result.Groups, gr)
	}

	total += expr.Modifier
	result.Total = total
	result.Breakdown = formatBreakdown(result)

	return result, nil
}

// RollDamage rolls damage dice. If critical is true, all dice counts are doubled
// but modifiers are applied only once.
func (r *Roller) RollDamage(expression string, critical bool) (RollResult, error) {
	expr, err := ParseExpression(expression)
	if err != nil {
		return RollResult{}, err
	}

	if critical {
		for i := range expr.Groups {
			expr.Groups[i].Count *= 2
		}
	}

	result := RollResult{
		Expression: expression,
		Modifier:   expr.Modifier,
		Critical:   critical,
		Timestamp:  time.Now(),
	}

	total := 0
	for _, g := range expr.Groups {
		gr := GroupResult{
			Die:   g.Sides,
			Count: g.Count,
		}
		groupTotal := 0
		for i := 0; i < g.Count; i++ {
			roll := r.randFn(g.Sides)
			gr.Results = append(gr.Results, roll)
			groupTotal += roll
		}
		gr.Total = groupTotal
		total += groupTotal
		result.Groups = append(result.Groups, gr)
	}

	total += expr.Modifier
	result.Total = total
	result.Breakdown = formatBreakdown(result)

	return result, nil
}

func formatBreakdown(r RollResult) string {
	parts := ""
	for i, g := range r.Groups {
		if i > 0 {
			parts += " + "
		}
		parts += fmt.Sprintf("%v", g.Results)
	}
	if r.Modifier > 0 {
		parts += fmt.Sprintf(" + %d", r.Modifier)
	} else if r.Modifier < 0 {
		parts += fmt.Sprintf(" - %d", -r.Modifier)
	}
	return fmt.Sprintf("%s = %d", parts, r.Total)
}
