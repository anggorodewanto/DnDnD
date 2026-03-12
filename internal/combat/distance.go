package combat

import (
	"fmt"
	"strings"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/ab/dndnd/internal/refdata"
)

// FormatDistance formats a distance message between two named entities.
// When from is "You", uses "You are Xft from <to>." format.
// Otherwise uses "<from> is Xft from <to>." format.
func FormatDistance(distFt int, from, to string) string {
	if from == "You" {
		return fmt.Sprintf("\U0001f4cf You are %dft from %s.", distFt, to)
	}
	return fmt.Sprintf("\U0001f4cf %s is %dft from %s.", from, distFt, to)
}

// FormatRangeRejection formats a range rejection message showing actual distance and max range.
func FormatRangeRejection(distFt, maxRange int, targetName string) string {
	return fmt.Sprintf("\u274c Target is out of range \u2014 %dft away (max %dft).", distFt, maxRange)
}

// ResolveTarget finds a combatant by ShortID (case-insensitive) or by grid coordinate.
// Returns a pointer to the matched combatant or an error if not found.
func ResolveTarget(target string, combatants []refdata.Combatant) (*refdata.Combatant, error) {
	upper := strings.ToUpper(strings.TrimSpace(target))

	// Try case-insensitive ShortID match
	for i := range combatants {
		if strings.ToUpper(combatants[i].ShortID) == upper {
			return &combatants[i], nil
		}
	}

	// Try coordinate match
	col, row, err := renderer.ParseCoordinate(target)
	if err == nil {
		for i := range combatants {
			cCol, cRow, cErr := renderer.ParseCoordinate(
				combatants[i].PositionCol + fmt.Sprintf("%d", combatants[i].PositionRow),
			)
			if cErr != nil {
				continue
			}
			if cCol == col && cRow == row {
				return &combatants[i], nil
			}
		}
	}

	return nil, fmt.Errorf("target %q not found", target)
}
