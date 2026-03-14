package renderer

import "math"

// wallEdge represents a blocked edge between two adjacent tiles, stored in canonical order.
type wallEdge struct {
	col1, row1, col2, row2 int
}

func makeWallEdge(col1, row1, col2, row2 int) wallEdge {
	if row1 > row2 || (row1 == row2 && col1 > col2) {
		return wallEdge{col2, row2, col1, row1}
	}
	return wallEdge{col1, row1, col2, row2}
}

// buildWallMap pre-processes wall segments into a set of blocked tile edges.
func buildWallMap(walls []WallSegment, width, height int) map[wallEdge]bool {
	m := make(map[wallEdge]bool)
	for _, w := range walls {
		addFOWWallEdges(m, w, width, height)
	}
	return m
}

func addFOWWallEdges(m map[wallEdge]bool, w WallSegment, width, height int) {
	// Horizontal wall segment: y1 == y2, spans x range
	if w.Y1 == w.Y2 {
		y := w.Y1
		row := int(y)
		xMin := math.Min(w.X1, w.X2)
		xMax := math.Max(w.X1, w.X2)
		for col := int(xMin); col < int(xMax); col++ {
			if row > 0 && row <= height && col >= 0 && col < width {
				m[makeWallEdge(col, row-1, col, row)] = true
			}
		}
		return
	}

	// Vertical wall segment: x1 == x2, spans y range
	if w.X1 == w.X2 {
		x := w.X1
		col := int(x)
		yMin := math.Min(w.Y1, w.Y2)
		yMax := math.Max(w.Y1, w.Y2)
		for row := int(yMin); row < int(yMax); row++ {
			if col > 0 && col <= width && row >= 0 && row < height {
				m[makeWallEdge(col-1, row, col, row)] = true
			}
		}
		return
	}
}

// wallsBetween checks if a wall edge exists between two adjacent tiles.
func wallsBetween(wm map[wallEdge]bool, col1, row1, col2, row2 int) bool {
	return wm[makeWallEdge(col1, row1, col2, row2)]
}

// wallLine is a line segment for ray intersection testing.
type wallLine struct {
	x1, y1, x2, y2 float64
}

// shadowcast performs symmetric visibility computation from (originCol, originRow)
// with the given vision range in tiles. Returns a set of visible tile positions.
//
// Uses center-to-center raycasting with wall segment intersection checks.
// A tile is visible if the ray from the origin tile center to the target tile center
// does not cross any wall segment. This is inherently symmetric: if A sees B, B sees A.
func shadowcast(originCol, originRow, visionRange int, walls []WallSegment, width, height int) map[GridPos]bool {
	visible := make(map[GridPos]bool)
	visible[GridPos{originCol, originRow}] = true

	// Convert wall segments to wallLine format for intersection testing
	wLines := make([]wallLine, len(walls))
	for i, w := range walls {
		wLines[i] = wallLine{w.X1, w.Y1, w.X2, w.Y2}
	}

	ox := float64(originCol) + 0.5
	oy := float64(originRow) + 0.5

	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if col == originCol && row == originRow {
				continue
			}

			dx := float64(col - originCol)
			dy := float64(row - originRow)
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > float64(visionRange)+0.5 {
				continue
			}

			tx := float64(col) + 0.5
			ty := float64(row) + 0.5

			if !rayBlockedByWalls(ox, oy, tx, ty, wLines) {
				visible[GridPos{col, row}] = true
			}
		}
	}

	return visible
}

// rayBlockedByWalls checks if a ray from (x1,y1) to (x2,y2) intersects any wall line.
func rayBlockedByWalls(x1, y1, x2, y2 float64, walls []wallLine) bool {
	for _, w := range walls {
		if segmentsIntersect(x1, y1, x2, y2, w.x1, w.y1, w.x2, w.y2) {
			return true
		}
	}
	return false
}

// segmentsIntersect checks if line segment (ax1,ay1)-(ax2,ay2) properly intersects
// segment (bx1,by1)-(bx2,by2). Uses cross products.
// Returns true if the segments cross each other. Uses inclusive bounds on the wall
// segment (u parameter) to prevent rays from slipping through wall endpoints,
// and exclusive bounds on the ray (t parameter) to avoid blocking from origin/target.
func segmentsIntersect(ax1, ay1, ax2, ay2, bx1, by1, bx2, by2 float64) bool {
	dx := ax2 - ax1
	dy := ay2 - ay1
	ex := bx2 - bx1
	ey := by2 - by1

	denom := dx*ey - dy*ex
	if math.Abs(denom) < 1e-12 {
		return false // parallel or collinear
	}

	t := ((bx1-ax1)*ey - (by1-ay1)*ex) / denom
	u := ((bx1-ax1)*dy - (by1-ay1)*dx) / denom

	// t: strict (0,1) — ray must cross interior, not start/end at wall
	// u: inclusive [0,1] — any point along the wall segment counts
	const eps = 1e-9
	return t > eps && t < 1-eps && u >= -eps && u <= 1+eps
}
