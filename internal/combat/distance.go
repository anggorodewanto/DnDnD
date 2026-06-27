package combat

import (
	"fmt"
	"strconv"
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
func FormatRangeRejection(distFt, maxRange int) string {
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
				combatants[i].PositionCol + strconv.Itoa(int(combatants[i].PositionRow)),
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

// TargetPos is a resolved /distance endpoint — either a combatant or a bare grid
// tile. Col/Row are 0-based grid coordinates; AltFt is altitude in feet (0 for an
// empty tile). Label is the human-readable name shown in the distance message.
type TargetPos struct {
	Col, Row int
	AltFt    int
	Label    string
}

// CombatantPos converts a combatant into a TargetPos at its current grid square
// and altitude. An unparseable stored position falls back to the origin tile.
func CombatantPos(c refdata.Combatant) TargetPos {
	col, row, err := renderer.ParseCoordinate(c.PositionCol + strconv.Itoa(int(c.PositionRow)))
	if err != nil {
		col, row = 0, 0
	}
	return TargetPos{
		Col:   col,
		Row:   row,
		AltFt: int(c.AltitudeFt),
		Label: fmt.Sprintf("%s (%s)", c.DisplayName, c.ShortID),
	}
}

// ResolveTargetPos resolves a /distance target to a grid position. Unlike
// ResolveTarget, it also accepts a bare grid coordinate for an empty tile
// (e.g. "F6") so distance can be measured to any square, not only one a
// combatant happens to occupy. Resolution order: combatant ShortID, a coordinate
// occupied by a combatant (carrying that combatant's altitude), then a bare
// coordinate at ground level (0ft). Returns an error only when the target is
// neither a known combatant nor a parseable coordinate.
func ResolveTargetPos(target string, combatants []refdata.Combatant) (TargetPos, error) {
	if c, err := ResolveTarget(target, combatants); err == nil {
		return CombatantPos(*c), nil
	}

	col, row, err := renderer.ParseCoordinate(target)
	if err != nil {
		return TargetPos{}, fmt.Errorf("target %q not found", target)
	}
	return TargetPos{
		Col:   col,
		Row:   row,
		AltFt: 0,
		Label: renderer.ColumnLabel(col) + strconv.Itoa(row+1),
	}, nil
}

// DistanceBetween returns the 3D distance in feet between two resolved positions,
// rounded to the nearest 5ft.
func DistanceBetween(a, b TargetPos) int {
	return Distance3D(a.Col, a.Row, a.AltFt, b.Col, b.Row, b.AltFt)
}
