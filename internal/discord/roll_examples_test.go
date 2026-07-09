package discord

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/dice"
)

// diceExprRe matches a dice-expression token (e.g. 1d20+4, 2d6, d20, 4d6+2)
// embedded anywhere in a user-facing string.
var diceExprRe = regexp.MustCompile(`(?:\d+)?d\d+(?:[+-]\d+)*`)

// advertisedRollSurfaces returns every user-facing string that shows a /roll
// dice example: the error-reply examples, the no-argument usage help, and the
// slash-command + option descriptions. A new example added to any of these is
// automatically picked up and asserted below.
func advertisedRollSurfaces(t *testing.T) []string {
	t.Helper()
	surfaces := []string{rollExamples, rollUsageHelp}
	for _, cmd := range CommandDefinitions() {
		if cmd.Name != "roll" {
			continue
		}
		surfaces = append(surfaces, cmd.Description)
		for _, opt := range cmd.Options {
			surfaces = append(surfaces, opt.Description)
		}
	}
	require.GreaterOrEqual(t, len(surfaces), 3, "roll command + option descriptions must be scanned")
	return surfaces
}

// TestAdvertisedRollExamples_AllParse guards the whole class of "help
// advertises a dice form the parser rejects" bugs (the /roll d20 regression,
// e967364): every dice-expression token shown to a player must parse.
func TestAdvertisedRollExamples_AllParse(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range advertisedRollSurfaces(t) {
		for _, expr := range diceExprRe.FindAllString(s, -1) {
			seen[expr] = true
			_, err := dice.ParseExpression(expr)
			assert.NoErrorf(t, err, "advertised example %q must parse", expr)
		}
	}
	// Guard against a broken extractor silently passing: these forms are known
	// to be advertised, so they must have been found.
	for _, want := range []string{"1d20+4", "2d6", "d20", "4d6+2"} {
		assert.Truef(t, seen[want], "expected advertised example %q to be found and parsed", want)
	}
}

// TestRollExampleExprs_AllParse asserts the SSOT list itself parses.
func TestRollExampleExprs_AllParse(t *testing.T) {
	require.NotEmpty(t, rollExampleExprs)
	for _, expr := range rollExampleExprs {
		_, err := dice.ParseExpression(expr)
		assert.NoErrorf(t, err, "rollExampleExprs entry %q must parse", expr)
	}
}
