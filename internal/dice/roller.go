package dice

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
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

// rollGroups rolls all dice groups and returns the group results and their combined total.
func (r *Roller) rollGroups(groups []DiceGroup) ([]GroupResult, int) {
	var results []GroupResult
	total := 0
	for _, g := range groups {
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
		results = append(results, gr)
	}
	return results, total
}

// Roll parses a dice expression and rolls the dice, returning a full breakdown.
func (r *Roller) Roll(expression string) (RollResult, error) {
	expr, err := ParseExpression(expression)
	if err != nil {
		return RollResult{}, err
	}

	groups, total := r.rollGroups(expr.Groups)
	result := RollResult{
		Expression: expression,
		Groups:     groups,
		Modifier:   expr.Modifier,
		Total:      total + expr.Modifier,
		Timestamp:  time.Now(),
	}
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

	groups, total := r.rollGroups(expr.Groups)
	result := RollResult{
		Expression: expression,
		Groups:     groups,
		Modifier:   expr.Modifier,
		Critical:   critical,
		Total:      total + expr.Modifier,
		Timestamp:  time.Now(),
	}
	result.Breakdown = formatBreakdown(result)

	return result, nil
}

func formatBreakdown(r RollResult) string {
	var b strings.Builder
	for i, g := range r.Groups {
		if i > 0 {
			b.WriteString(" + ")
		}
		fmt.Fprintf(&b, "%v", g.Results)
	}
	if r.Modifier > 0 {
		fmt.Fprintf(&b, " + %d", r.Modifier)
	} else if r.Modifier < 0 {
		fmt.Fprintf(&b, " - %d", -r.Modifier)
	}
	fmt.Fprintf(&b, " = %d", r.Total)
	return b.String()
}
