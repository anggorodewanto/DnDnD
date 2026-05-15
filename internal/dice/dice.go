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
		if count < 1 || sides < 1 {
			return Expression{}, fmt.Errorf("invalid dice expression: count and sides must be >= 1, got %dd%d", count, sides)
		}
		expr.Groups = append(expr.Groups, DiceGroup{Count: count, Sides: sides})
	}

	// Sum signed integer tokens from the residue after removing dice groups.
	residue := diceGroupRe.ReplaceAllString(input, "")
	// Remove leading/trailing operators left by dice group removal (e.g. "++2+3" → "+2+3").
	residue = strings.TrimLeft(residue, "+")
	if residue != "" {
		mod, err := sumSignedTokens(residue)
		if err != nil {
			return Expression{}, fmt.Errorf("invalid modifier in dice expression: %s", residue)
		}
		expr.Modifier = mod
	}

	return expr, nil
}

var signedTokenRe = regexp.MustCompile(`[+-]\d+`)

// sumSignedTokens parses a string like "+5+5" or "-2+3" into the sum of its signed integers.
func sumSignedTokens(s string) (int, error) {
	// Ensure the string starts with a sign for uniform tokenization.
	if s[0] != '+' && s[0] != '-' {
		s = "+" + s
	}
	tokens := signedTokenRe.FindAllString(s, -1)
	if len(tokens) == 0 {
		return 0, fmt.Errorf("no valid tokens")
	}
	// Verify full coverage: joined tokens must equal the input.
	if strings.Join(tokens, "") != s {
		return 0, fmt.Errorf("unexpected characters")
	}
	sum := 0
	for _, tok := range tokens {
		n, err := strconv.Atoi(tok)
		if err != nil {
			return 0, err
		}
		sum += n
	}
	return sum, nil
}
