package combat

import (
	"github.com/ab/dndnd/internal/gamemap/renderer"
)

// CoverLevel represents the degree of cover between attacker and target.
type CoverLevel int

const (
	CoverNone          CoverLevel = 0
	CoverHalf          CoverLevel = 1
	CoverThreeQuarters CoverLevel = 2
	CoverFull          CoverLevel = 3
)

// String returns the display name of the cover level.
func (c CoverLevel) String() string {
	switch c {
	case CoverHalf:
		return "Half"
	case CoverThreeQuarters:
		return "Three-Quarters"
	case CoverFull:
		return "Full"
	default:
		return "None"
	}
}

// ACBonus returns the AC bonus granted by this cover level.
func (c CoverLevel) ACBonus() int {
	switch c {
	case CoverHalf:
		return 2
	case CoverThreeQuarters:
		return 5
	default:
		return 0
	}
}

// DEXSaveBonus returns the DEX saving throw bonus granted by this cover level.
func (c CoverLevel) DEXSaveBonus() int {
	switch c {
	case CoverHalf:
		return 2
	case CoverThreeQuarters:
		return 5
	default:
		return 0
	}
}

// EffectiveAC returns the target's effective AC including cover bonus.
func EffectiveAC(baseAC int, cover CoverLevel) int {
	return baseAC + cover.ACBonus()
}

// CoverOccupant represents a creature on the grid for cover calculation.
type CoverOccupant struct {
	Col int
	Row int
}

// CalculateCover computes cover between an attacker and target using the DMG grid variant.
// It picks the attacker corner that gives the least cover (best for attacker).
// Creature-granted cover (another creature on the line) counts as half cover.
func CalculateCover(attackerCol, attackerRow, targetCol, targetRow int, walls []renderer.WallSegment, occupants []CoverOccupant) CoverLevel {
	if attackerCol == targetCol && attackerRow == targetRow {
		return CoverNone
	}

	// Attacker corners
	ac := [4][2]float64{
		{float64(attackerCol), float64(attackerRow)},
		{float64(attackerCol + 1), float64(attackerRow)},
		{float64(attackerCol), float64(attackerRow + 1)},
		{float64(attackerCol + 1), float64(attackerRow + 1)},
	}
	// Target corners
	tc := [4][2]float64{
		{float64(targetCol), float64(targetRow)},
		{float64(targetCol + 1), float64(targetRow)},
		{float64(targetCol), float64(targetRow + 1)},
		{float64(targetCol + 1), float64(targetRow + 1)},
	}

	bestCover := CoverFull
	for _, a := range ac {
		blocked := 0
		for _, tgt := range tc {
			if lineBlockedByWalls(a[0], a[1], tgt[0], tgt[1], walls) {
				blocked++
			}
		}
		cover := blockedToCover(blocked)
		if cover < bestCover {
			bestCover = cover
		}
	}

	// Check creature-granted cover
	if bestCover == CoverNone {
		bestCover = creatureCover(attackerCol, attackerRow, targetCol, targetRow, occupants)
	}

	return bestCover
}

// CalculateCoverFromOrigin computes cover from a specific origin point (for AoE spells).
// Unlike CalculateCover, this uses a single origin corner, not best-of-4.
func CalculateCoverFromOrigin(originCol, originRow, targetCol, targetRow int, walls []renderer.WallSegment) CoverLevel {
	if originCol == targetCol && originRow == targetRow {
		return CoverNone
	}

	tcx := float64(targetCol) + 0.5
	tcy := float64(targetRow) + 0.5

	// Pick the origin corner closest to target
	bestCorner := [2]float64{float64(originCol), float64(originRow)}
	bestDist := distSq(float64(originCol), float64(originRow), tcx, tcy)
	for _, c := range [4][2]float64{
		{float64(originCol), float64(originRow)},
		{float64(originCol + 1), float64(originRow)},
		{float64(originCol), float64(originRow + 1)},
		{float64(originCol + 1), float64(originRow + 1)},
	} {
		d := distSq(c[0], c[1], tcx, tcy)
		if d < bestDist {
			bestDist = d
			bestCorner = c
		}
	}

	tc := [4][2]float64{
		{float64(targetCol), float64(targetRow)},
		{float64(targetCol + 1), float64(targetRow)},
		{float64(targetCol), float64(targetRow + 1)},
		{float64(targetCol + 1), float64(targetRow + 1)},
	}

	blocked := 0
	for _, tgt := range tc {
		if lineBlockedByWalls(bestCorner[0], bestCorner[1], tgt[0], tgt[1], walls) {
			blocked++
		}
	}

	return blockedToCover(blocked)
}

func distSq(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return dx*dx + dy*dy
}

// lineBlockedByWalls returns true if the line from (ax,ay) to (bx,by) is blocked by any wall.
func lineBlockedByWalls(ax, ay, bx, by float64, walls []renderer.WallSegment) bool {
	for _, w := range walls {
		if lineIntersectsSegment(ax, ay, bx, by, w) {
			return true
		}
	}
	return false
}

// blockedToCover converts a count of blocked lines (out of 4) to a CoverLevel.
func blockedToCover(blocked int) CoverLevel {
	if blocked == 0 {
		return CoverNone
	}
	if blocked <= 2 {
		return CoverHalf
	}
	if blocked == 3 {
		return CoverThreeQuarters
	}
	return CoverFull
}

// creatureCover checks if any occupant is on the line between attacker center and target center.
// If so, returns CoverHalf; otherwise CoverNone.
func creatureCover(attackerCol, attackerRow, targetCol, targetRow int, occupants []CoverOccupant) CoverLevel {
	ax := float64(attackerCol) + 0.5
	ay := float64(attackerRow) + 0.5
	bx := float64(targetCol) + 0.5
	by := float64(targetRow) + 0.5

	for _, occ := range occupants {
		if (occ.Col == attackerCol && occ.Row == attackerRow) || (occ.Col == targetCol && occ.Row == targetRow) {
			continue
		}
		// Check if center-to-center line passes through the occupant's square
		if linePassesThroughTile(ax, ay, bx, by, occ.Col, occ.Row) {
			return CoverHalf
		}
	}
	return CoverNone
}

// linePassesThroughTile checks if a line from (ax,ay) to (bx,by) passes through
// the tile at (col,row). The tile occupies [col,col+1] x [row,row+1].
func linePassesThroughTile(ax, ay, bx, by float64, col, row int) bool {
	// Check if the line segment intersects any of the 4 edges of the tile
	x0, y0 := float64(col), float64(row)
	x1, y1 := float64(col+1), float64(row+1)

	// Top edge
	if segmentsIntersect(ax, ay, bx, by, x0, y0, x1, y0) {
		return true
	}
	// Bottom edge
	if segmentsIntersect(ax, ay, bx, by, x0, y1, x1, y1) {
		return true
	}
	// Left edge
	if segmentsIntersect(ax, ay, bx, by, x0, y0, x0, y1) {
		return true
	}
	// Right edge
	if segmentsIntersect(ax, ay, bx, by, x1, y0, x1, y1) {
		return true
	}
	return false
}

// lineIntersectsSegment returns true if the line from (ax,ay) to (bx,by) crosses
// the given wall segment. Uses parametric intersection. Touching at endpoints only
// (t=0 or t=1 for the line, or u=0 or u=1 for the wall) does not count.
func lineIntersectsSegment(ax, ay, bx, by float64, w renderer.WallSegment) bool {
	return segmentsIntersect(ax, ay, bx, by, w.X1, w.Y1, w.X2, w.Y2)
}

// segmentsIntersect checks if the line segment (ax,ay)-(bx,by) crosses
// the wall segment (cx,cy)-(dx,dy). The line's own endpoints are excluded
// (a corner touching a wall is not "blocked"), but the wall's extent is inclusive.
func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	rx := bx - ax
	ry := by - ay
	sx := dx - cx
	sy := dy - cy

	denom := rx*sy - ry*sx
	if denom == 0 {
		return false
	}

	diffX := cx - ax
	diffY := cy - ay

	t := (diffX*sy - diffY*sx) / denom
	u := (diffX*ry - diffY*rx) / denom

	const eps = 1e-9
	// t: parameter on the line from a to b — exclude endpoints (corner sharing)
	// u: parameter on the wall — include endpoints (wall edge should block)
	return t > eps && t < 1-eps && u > -eps && u < 1+eps
}
