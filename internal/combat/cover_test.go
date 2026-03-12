package combat

import (
	"testing"

	"github.com/ab/dndnd/internal/gamemap/renderer"
)

func TestCoverLevel_String(t *testing.T) {
	tests := []struct {
		level CoverLevel
		want  string
	}{
		{CoverNone, "None"},
		{CoverHalf, "Half"},
		{CoverThreeQuarters, "Three-Quarters"},
		{CoverFull, "Full"},
		{CoverLevel(99), "None"},
	}
	for _, tc := range tests {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("CoverLevel(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestCoverLevel_ACBonus(t *testing.T) {
	tests := []struct {
		level CoverLevel
		want  int
	}{
		{CoverNone, 0},
		{CoverHalf, 2},
		{CoverThreeQuarters, 5},
		{CoverFull, 0},
	}
	for _, tc := range tests {
		if got := tc.level.ACBonus(); got != tc.want {
			t.Errorf("CoverLevel(%d).ACBonus() = %d, want %d", tc.level, got, tc.want)
		}
	}
}

func TestCoverLevel_DEXSaveBonus(t *testing.T) {
	tests := []struct {
		level CoverLevel
		want  int
	}{
		{CoverNone, 0},
		{CoverHalf, 2},
		{CoverThreeQuarters, 5},
		{CoverFull, 0},
	}
	for _, tc := range tests {
		if got := tc.level.DEXSaveBonus(); got != tc.want {
			t.Errorf("CoverLevel(%d).DEXSaveBonus() = %d, want %d", tc.level, got, tc.want)
		}
	}
}

func TestEffectiveAC(t *testing.T) {
	if got := EffectiveAC(15, CoverNone); got != 15 {
		t.Errorf("EffectiveAC(15, None) = %d, want 15", got)
	}
	if got := EffectiveAC(15, CoverHalf); got != 17 {
		t.Errorf("EffectiveAC(15, Half) = %d, want 17", got)
	}
	if got := EffectiveAC(15, CoverThreeQuarters); got != 20 {
		t.Errorf("EffectiveAC(15, ThreeQuarters) = %d, want 20", got)
	}
	if got := EffectiveAC(15, CoverFull); got != 15 {
		t.Errorf("EffectiveAC(15, Full) = %d, want 15", got)
	}
}

func TestLineIntersectsWall(t *testing.T) {
	// Vertical wall from (2,0) to (2,3) — blocks lines crossing x=2
	wall := renderer.WallSegment{X1: 2, Y1: 0, X2: 2, Y2: 3}

	// Line from (1,1) to (3,1) should cross the wall
	if !lineIntersectsSegment(1, 1, 3, 1, wall) {
		t.Error("expected line (1,1)-(3,1) to intersect vertical wall at x=2")
	}

	// Line from (0,0) to (1,1) should NOT cross the wall
	if lineIntersectsSegment(0, 0, 1, 1, wall) {
		t.Error("expected line (0,0)-(1,1) to NOT intersect vertical wall at x=2")
	}

	// Horizontal wall from (0,2) to (3,2)
	hwall := renderer.WallSegment{X1: 0, Y1: 2, X2: 3, Y2: 2}

	// Line from (1,1) to (1,3) should cross horizontal wall at y=2
	if !lineIntersectsSegment(1, 1, 1, 3, hwall) {
		t.Error("expected line (1,1)-(1,3) to intersect horizontal wall at y=2")
	}

	// Line from (1,0) to (1,1) should NOT cross horizontal wall at y=2
	if lineIntersectsSegment(1, 0, 1, 1, hwall) {
		t.Error("expected line (1,0)-(1,1) to NOT intersect horizontal wall at y=2")
	}
}

func TestLineIntersectsWall_Diagonal(t *testing.T) {
	// Vertical wall at x=2 from y=0 to y=4
	wall := renderer.WallSegment{X1: 2, Y1: 0, X2: 2, Y2: 4}

	// Diagonal line from (0,0) to (4,4) should cross x=2
	if !lineIntersectsSegment(0, 0, 4, 4, wall) {
		t.Error("expected diagonal line (0,0)-(4,4) to intersect vertical wall at x=2")
	}

	// Diagonal line from (0,0) to (1,4) should NOT cross x=2
	if lineIntersectsSegment(0, 0, 1, 4, wall) {
		t.Error("expected diagonal line (0,0)-(1,4) to NOT intersect vertical wall at x=2")
	}
}

func TestCalculateCover_NoCover(t *testing.T) {
	// No walls, no occupants — should be no cover
	cover := CalculateCover(0, 0, 3, 3, nil, nil)
	if cover != CoverNone {
		t.Errorf("expected CoverNone, got %v", cover)
	}
}

func TestCalculateCover_SameSquare(t *testing.T) {
	cover := CalculateCover(2, 2, 2, 2, nil, nil)
	if cover != CoverNone {
		t.Errorf("expected CoverNone for same square, got %v", cover)
	}
}

func TestCalculateCover_HalfCover_Wall(t *testing.T) {
	// Attacker at (0,0), target at (3,0).
	// Wall at x=2 from y=0.5 to y=1.5 — blocks lines heading to the lower-right target corners.
	// Attacker corners: (0,0),(1,0),(0,1),(1,1)
	// Target corners: (3,0),(4,0),(3,1),(4,1)
	// Best corners are (0,0) and (1,0) with 2 and 1 blocked lines respectively → Half cover.
	walls := []renderer.WallSegment{{X1: 2, Y1: 0.5, X2: 2, Y2: 1.5}}
	cover := CalculateCover(0, 0, 3, 0, walls, nil)
	if cover != CoverHalf {
		t.Errorf("expected CoverHalf, got %v", cover)
	}
}

func TestCalculateCover_FullCover_Wall(t *testing.T) {
	// Vertical wall at x=2 from y=0 to y=5 — fully blocks attacker at (0,2) to target at (3,2)
	walls := []renderer.WallSegment{{X1: 2, Y1: 0, X2: 2, Y2: 5}}
	cover := CalculateCover(0, 2, 3, 2, walls, nil)
	// All lines from all attacker corners cross this tall wall
	if cover != CoverFull {
		t.Errorf("expected CoverFull, got %v", cover)
	}
}

func TestCalculateCover_ThreeQuartersCover(t *testing.T) {
	// Attacker at (0,0), target at (3,3).
	// Two walls forming an L along the left and top edges of the target square:
	//   Wall 1: x=3 from y=0 to y=4 (vertical, left side of target)
	//   Wall 2: y=3 from x=0 to x=4 (horizontal, top side of target)
	// From every attacker corner, the line to target corner (3,3) reaches it at t=1
	// (an endpoint), so it is NOT blocked. The other 3 target corners are all blocked.
	// Every attacker corner sees exactly 3 of 4 lines blocked → ThreeQuarters.
	walls := []renderer.WallSegment{
		{X1: 3, Y1: 0, X2: 3, Y2: 4},
		{X1: 0, Y1: 3, X2: 4, Y2: 3},
	}
	cover := CalculateCover(0, 0, 3, 3, walls, nil)
	if cover != CoverThreeQuarters {
		t.Errorf("expected CoverThreeQuarters, got %v", cover)
	}
}

func TestCalculateCover_CreatureCover(t *testing.T) {
	// Attacker at (0,0), target at (4,0), creature in between at (2,0)
	occupants := []CoverOccupant{{Col: 2, Row: 0}}
	cover := CalculateCover(0, 0, 4, 0, nil, occupants)
	if cover != CoverHalf {
		t.Errorf("expected CoverHalf from creature, got %v", cover)
	}
}

func TestCalculateCover_CreatureCover_NotOnLine(t *testing.T) {
	// Attacker at (0,0), target at (4,0), creature at (2,3) — not on line
	occupants := []CoverOccupant{{Col: 2, Row: 3}}
	cover := CalculateCover(0, 0, 4, 0, nil, occupants)
	if cover != CoverNone {
		t.Errorf("expected CoverNone, creature not on line, got %v", cover)
	}
}

func TestCalculateCover_CreatureIsAttackerOrTarget(t *testing.T) {
	// Creature at attacker or target position should be ignored
	occupants := []CoverOccupant{{Col: 0, Row: 0}, {Col: 4, Row: 0}}
	cover := CalculateCover(0, 0, 4, 0, nil, occupants)
	if cover != CoverNone {
		t.Errorf("expected CoverNone (creature at attacker/target pos ignored), got %v", cover)
	}
}

func TestCalculateCover_WallUpgradesOverCreature(t *testing.T) {
	// Wall gives three-quarters, creature also present — wall should dominate
	// Attacker (0,0), target (3,0), tall wall mostly blocking
	walls := []renderer.WallSegment{{X1: 2, Y1: 0, X2: 2, Y2: 5}}
	cover := CalculateCover(0, 0, 3, 0, walls, nil)
	if cover != CoverFull {
		t.Errorf("expected CoverFull from tall wall, got %v", cover)
	}
}

func TestCalculateCover_BestCornerChosen(t *testing.T) {
	// Wall at x=1 from y=0 to y=0.5. Attacker at (0,0), target at (2,0).
	// From top corners (y=0), lines cross the wall.
	// From bottom corners (y=1), lines don't cross the wall.
	// Best corner should give CoverNone.
	walls := []renderer.WallSegment{{X1: 1, Y1: 0, X2: 1, Y2: 0.5}}
	cover := CalculateCover(0, 0, 2, 0, walls, nil)
	if cover != CoverNone {
		t.Errorf("expected CoverNone (best corner avoids short wall), got %v", cover)
	}
}

func TestCalculateCoverFromOrigin(t *testing.T) {
	// AoE from origin (0,0) to target (3,0), full wall blocking
	walls := []renderer.WallSegment{{X1: 2, Y1: 0, X2: 2, Y2: 5}}
	cover := CalculateCoverFromOrigin(0, 0, 3, 0, walls)
	if cover != CoverFull {
		t.Errorf("expected CoverFull from origin, got %v", cover)
	}
}

func TestCalculateCoverFromOrigin_NoCover(t *testing.T) {
	cover := CalculateCoverFromOrigin(0, 0, 3, 0, nil)
	if cover != CoverNone {
		t.Errorf("expected CoverNone, got %v", cover)
	}
}

func TestCalculateCoverFromOrigin_SameSquare(t *testing.T) {
	walls := []renderer.WallSegment{{X1: 2, Y1: 0, X2: 2, Y2: 5}}
	cover := CalculateCoverFromOrigin(2, 2, 2, 2, walls)
	if cover != CoverNone {
		t.Errorf("expected CoverNone for same square, got %v", cover)
	}
}

func TestCalculateCover_ThreeQuartersExact(t *testing.T) {
	// We need a wall configuration where the best attacker corner has exactly 3 of 4 lines blocked.
	// Attacker at (0,0), target at (2,0).
	// Attacker corners: (0,0),(1,0),(0,1),(1,1)
	// Target corners: (2,0),(3,0),(2,1),(3,1)
	// Wall at x=1.5 from y=0 to y=0.8 — blocks lines that pass through low y values.
	// From corner (0,1): line to (2,0) crosses x=1.5 at y=0.5 → blocked. to (3,0) at y=0.667 → blocked.
	//   to (2,1) at y=1 → not blocked (wall ends at 0.8). to (3,1) at y=1 → not blocked.
	//   => 2 blocked → Half
	// From corner (1,1): line to (2,0) crosses x=1.5 at y=0.5 → blocked. to (3,0) at y=0.5 → blocked.
	//   to (2,1) at y=1 → not blocked. to (3,1) at y=1 → not blocked.
	//   => 2 blocked → Half
	// From corner (0,0): to (2,0) at y=0 → blocked. to (3,0) at y=0 → blocked.
	//   to (2,1) at y=0.5 → blocked. to (3,1) at y=0.5 → blocked.
	//   => 4 blocked → Full
	// From corner (1,0): to (2,0) at y=0 → blocked. to (3,0) at y=0 → blocked.
	//   to (2,1) at y=0.5 → blocked. to (3,1) at y=0 → blocked (at wall endpoint y=0).
	//   => 4 blocked → Full
	// Best = Half
	walls := []renderer.WallSegment{{X1: 1.5, Y1: 0, X2: 1.5, Y2: 0.8}}
	cover := CalculateCover(0, 0, 2, 0, walls, nil)
	if cover != CoverHalf {
		t.Errorf("expected CoverHalf, got %v", cover)
	}
}

func TestCalculateCover_Adjacent(t *testing.T) {
	// Adjacent tiles with no wall — no cover
	cover := CalculateCover(0, 0, 1, 0, nil, nil)
	if cover != CoverNone {
		t.Errorf("expected CoverNone for adjacent, got %v", cover)
	}
}

func TestCalculateCover_DiagonalAdjacent(t *testing.T) {
	cover := CalculateCover(0, 0, 1, 1, nil, nil)
	if cover != CoverNone {
		t.Errorf("expected CoverNone for diagonal adjacent, got %v", cover)
	}
}

func TestCalculateCover_MultipleWalls(t *testing.T) {
	// Two walls that together create full cover
	walls := []renderer.WallSegment{
		{X1: 2, Y1: 0, X2: 2, Y2: 2},
		{X1: 2, Y1: 2, X2: 2, Y2: 5},
	}
	cover := CalculateCover(0, 2, 3, 2, walls, nil)
	if cover != CoverFull {
		t.Errorf("expected CoverFull from two walls, got %v", cover)
	}
}

func TestBlockedToCover(t *testing.T) {
	tests := []struct {
		blocked int
		want    CoverLevel
	}{
		{0, CoverNone},
		{1, CoverHalf},
		{2, CoverHalf},
		{3, CoverThreeQuarters},
		{4, CoverFull},
	}
	for _, tc := range tests {
		if got := blockedToCover(tc.blocked); got != tc.want {
			t.Errorf("blockedToCover(%d) = %v, want %v", tc.blocked, got, tc.want)
		}
	}
}

func TestCreatureCover_DiagonalLine(t *testing.T) {
	// Attacker (0,0), target (4,4), creature at (2,2) — should be on the line
	occupants := []CoverOccupant{{Col: 2, Row: 2}}
	cover := CalculateCover(0, 0, 4, 4, nil, occupants)
	if cover != CoverHalf {
		t.Errorf("expected CoverHalf from creature on diagonal, got %v", cover)
	}
}

func TestLinePassesThroughTile(t *testing.T) {
	tests := []struct {
		name        string
		ax, ay      float64
		bx, by      float64
		col, row    int
		want        bool
	}{
		{"through center", 0, 0.5, 4, 0.5, 2, 0, true},
		{"misses tile", 0, 0.5, 4, 0.5, 2, 3, false},
		{"diagonal through", 0.5, 0.5, 4.5, 4.5, 2, 2, true},
		{"barely misses", 0.5, 0.5, 4.5, 0.5, 2, 2, false},
		{"enters from left", 0.5, 1.5, 4.5, 1.5, 2, 1, true},
		{"enters from bottom", 1.5, 4.5, 1.5, 0.5, 1, 2, true},
		{"enters from right", 4.5, 1.5, 0.5, 1.5, 2, 1, true},
		// Line that ends inside tile, entering only through bottom edge
		{"bottom edge only", 2.5, 4, 2.5, 2.5, 2, 2, true},
		// Line that ends inside tile, entering only through right edge
		{"right edge only", 4, 2.5, 2.8, 2.5, 2, 2, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := linePassesThroughTile(tc.ax, tc.ay, tc.bx, tc.by, tc.col, tc.row)
			if got != tc.want {
				t.Errorf("linePassesThroughTile(%v,%v,%v,%v, %d,%d) = %v, want %v",
					tc.ax, tc.ay, tc.bx, tc.by, tc.col, tc.row, got, tc.want)
			}
		})
	}
}

func TestSegmentsIntersect_Parallel(t *testing.T) {
	// Two parallel horizontal segments — should not intersect
	if segmentsIntersect(0, 0, 4, 0, 0, 1, 4, 1) {
		t.Error("parallel segments should not intersect")
	}
}

func TestSegmentsIntersect_Collinear(t *testing.T) {
	// Two collinear overlapping segments
	if segmentsIntersect(0, 0, 4, 0, 2, 0, 6, 0) {
		t.Error("collinear segments should not be reported as intersecting")
	}
}

func TestDistSq(t *testing.T) {
	got := distSq(0, 0, 3, 4)
	if got != 25 {
		t.Errorf("distSq(0,0,3,4) = %v, want 25", got)
	}
}

func TestLineIntersectsWall_EndpointOnWall(t *testing.T) {
	// Wall at x=2 from y=0 to y=2
	wall := renderer.WallSegment{X1: 2, Y1: 0, X2: 2, Y2: 2}

	// Line that starts exactly on the wall endpoint — should NOT count as blocked
	// A corner-to-corner line touching a wall endpoint is not considered blocked
	// (the line shares a point but doesn't cross the wall)
	if lineIntersectsSegment(2, 0, 3, 1, wall) {
		t.Error("line starting at wall endpoint should not count as intersection")
	}
}
