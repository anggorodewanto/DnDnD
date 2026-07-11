package dice

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ErrInvalidBonus is returned by ValidateBonusExpression when a player-supplied
// effect-die expression (e.g. "1d4" Guidance / Bless, "1d8" Bardic Inspiration)
// does not parse or exceeds MaxBonusDice. Callers map it to a player-facing
// message.
var ErrInvalidBonus = errors.New("invalid bonus dice expression")

// MaxBonusDice caps how many effect dice a single roll may add, guarding the
// roller against pathological input like "999d20". Real effect dice are a
// single die, so 20 is generous.
const MaxBonusDice = 20

// ValidateBonusExpression ensures an effect-die expression parses and stays
// within MaxBonusDice. Returns ErrInvalidBonus (wrapped) otherwise. Shared by
// /check, /save, and /attack so every effect-dice rider rejects bad input the
// same way.
func ValidateBonusExpression(expr string) error {
	parsed, err := ParseExpression(expr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBonus, err)
	}
	total := 0
	for _, g := range parsed.Groups {
		total += g.Count
	}
	if total > MaxBonusDice {
		return fmt.Errorf("%w: at most %d dice", ErrInvalidBonus, MaxBonusDice)
	}
	return nil
}

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

// diceGroupRe matches an NdM dice group. The count is optional: a bare "d20"
// (no leading count) is a common shorthand players type — and the /roll help
// text advertises it — so an empty count defaults to 1 in ParseExpression.
var diceGroupRe = regexp.MustCompile(`(\d*)d(\d+)`)

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
		// The count group is `\d*`, so an omitted count (bare "d20") yields an
		// empty string — treat that as the implied count of 1.
		count := 1
		if countStr := input[match[2]:match[3]]; countStr != "" {
			count, _ = strconv.Atoi(countStr)
		}
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
