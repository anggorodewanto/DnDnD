package dice

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DiceGroup represents a single NdM component of a dice expression.
type DiceGroup struct {
	Count int
	Sides int
}

// Expression represents a parsed dice expression like "2d6+1d4+3".
type Expression struct {
	Groups   []DiceGroup
	Modifier int
	Raw      string
}

var diceGroupRe = regexp.MustCompile(`(\d+)d(\d+)`)

// ParseExpression parses a dice expression string like "2d6+1d4+3" into an Expression.
func ParseExpression(input string) (Expression, error) {
	input = strings.ReplaceAll(input, " ", "")
	if input == "" {
		return Expression{}, fmt.Errorf("empty dice expression")
	}

	expr := Expression{Raw: input}

	// Find all dice groups
	matches := diceGroupRe.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return Expression{}, fmt.Errorf("invalid dice expression: %s", input)
	}

	for _, match := range matches {
		// Regex guarantees these are digit-only strings, so Atoi cannot fail.
		count, _ := strconv.Atoi(input[match[2]:match[3]])
		sides, _ := strconv.Atoi(input[match[4]:match[5]])
		expr.Groups = append(expr.Groups, DiceGroup{Count: count, Sides: sides})
	}

	// Find trailing modifier (strip out all dice groups, what's left should be a modifier)
	stripped := diceGroupRe.ReplaceAllString(input, "")
	stripped = strings.ReplaceAll(stripped, "+", "")
	if stripped != "" {
		mod, err := strconv.Atoi(stripped)
		if err != nil {
			return Expression{}, fmt.Errorf("invalid modifier in dice expression: %s", stripped)
		}
		expr.Modifier = mod
	}

	return expr, nil
}
