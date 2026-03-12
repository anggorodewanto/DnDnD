package pathfinding

import (
	"testing"

	"github.com/ab/dndnd/internal/gamemap/renderer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openGrid(w, h int) *Grid {
	terrain := make([]renderer.TerrainType, w*h)
	return &Grid{Width: w, Height: h, Terrain: terrain}
}

func TestFindPath_StraightLineEast(t *testing.T) {
	g := openGrid(5, 5)
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{3, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 15, res.TotalCostFt) // 3 tiles * 5ft
	assert.Equal(t, Point{0, 0}, res.Path[0])
	assert.Equal(t, Point{3, 0}, res.Path[len(res.Path)-1])
	assert.Len(t, res.Path, 4) // start + 3 steps
}

func TestFindPath_Diagonal(t *testing.T) {
	g := openGrid(5, 5)
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{2, 2},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 10, res.TotalCostFt) // 2 diagonal tiles * 5ft
	assert.Len(t, res.Path, 3)
}

func TestFindPath_DifficultTerrain(t *testing.T) {
	g := openGrid(5, 1)
	// Make tiles 1 and 2 difficult terrain
	g.Terrain[1] = renderer.TerrainDifficultTerrain
	g.Terrain[2] = renderer.TerrainDifficultTerrain
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{3, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	// tile 1: 10ft, tile 2: 10ft, tile 3: 5ft = 25ft
	assert.Equal(t, 25, res.TotalCostFt)
}

func TestFindPath_ProneCrawling(t *testing.T) {
	g := openGrid(3, 1)
	req := PathRequest{
		Start:   Point{0, 0},
		End:     Point{2, 0},
		IsProne: true,
		Grid:    g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	// 2 tiles * 10ft (prone ×2) = 20ft
	assert.Equal(t, 20, res.TotalCostFt)
}

func TestFindPath_ProneOnDifficultTerrain(t *testing.T) {
	g := openGrid(2, 1)
	g.Terrain[1] = renderer.TerrainDifficultTerrain
	req := PathRequest{
		Start:   Point{0, 0},
		End:     Point{1, 0},
		IsProne: true,
		Grid:    g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	// difficult(×2) + prone(×2) = ×3 → 15ft
	assert.Equal(t, 15, res.TotalCostFt)
}

func TestFindPath_WallBlocksCardinal(t *testing.T) {
	g := openGrid(3, 1)
	// Vertical wall between col 0 and col 1: x=1, y from 0 to 1
	g.Walls = []renderer.WallSegment{{X1: 1, Y1: 0, X2: 1, Y2: 1}}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{2, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	// 3×1 grid with wall between col 0 and 1: no path exists (can't go around)
	assert.False(t, res.Found)
}

func TestFindPath_WallBlockedPathAround(t *testing.T) {
	// 3×3 grid with wall between (0,0)→(0,1), path must go around
	g := openGrid(3, 3)
	// Vertical wall at x=1 from y=0 to y=1 blocks col 0→col 1 at row 0
	g.Walls = []renderer.WallSegment{{X1: 1, Y1: 0, X2: 1, Y2: 1}}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{2, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	// Can diagonal corner-cut, so path: (0,0) → diag(1,1) → diag(2,0) = 10ft
	assert.Equal(t, 10, res.TotalCostFt)
}

func TestFindPath_DiagonalCornerCuttingAllowed(t *testing.T) {
	// Walls at a corner should NOT block diagonal movement
	g := openGrid(3, 3)
	// Vertical wall at x=1 from y=0 to y=1 (blocks col 0→1 at row 0)
	// Horizontal wall at y=1 from x=0 to x=1 (blocks row 0→1 at col 0)
	// Both meet at corner (1,1) — diagonal from (0,0) to (1,1) should still be allowed
	g.Walls = []renderer.WallSegment{
		{X1: 1, Y1: 0, X2: 1, Y2: 1},
		{X1: 0, Y1: 1, X2: 1, Y2: 1},
	}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{1, 1},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 5, res.TotalCostFt) // single diagonal step
}

func TestFindPath_EnemyOccupantBlocks(t *testing.T) {
	g := openGrid(3, 1)
	g.Occupants = []Occupant{{Col: 1, Row: 0, IsAlly: false, SizeCategory: SizeMedium}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 0},
		SizeCategory: SizeMedium,
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	// Can't pass through same-size enemy, 3×1 grid, no path
	assert.False(t, res.Found)
}

func TestFindPath_AllyOccupantPassThrough(t *testing.T) {
	g := openGrid(3, 1)
	g.Occupants = []Occupant{{Col: 1, Row: 0, IsAlly: true, SizeCategory: SizeMedium}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 0},
		SizeCategory: SizeMedium,
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 10, res.TotalCostFt)
}

func TestFindPath_EnemySizeDiff2PassThrough(t *testing.T) {
	g := openGrid(3, 1)
	// Tiny enemy, Large mover: diff = 3 >= 2, can pass through
	g.Occupants = []Occupant{{Col: 1, Row: 0, IsAlly: false, SizeCategory: SizeTiny}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 0},
		SizeCategory: SizeLarge,
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 10, res.TotalCostFt)
}

func TestFindPath_EnemySizeDiff1Blocked(t *testing.T) {
	g := openGrid(3, 1)
	// Small enemy, Large mover: diff = 2, can pass through (>= 2)
	// Actually diff = 2 means CAN pass. Let's use diff=1:
	// Medium enemy, Large mover: diff = 1 < 2, blocked
	g.Occupants = []Occupant{{Col: 1, Row: 0, IsAlly: false, SizeCategory: SizeMedium}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 0},
		SizeCategory: SizeLarge,
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.False(t, res.Found)
}

func TestFindPath_SameStartAndEnd(t *testing.T) {
	g := openGrid(3, 3)
	req := PathRequest{
		Start: Point{1, 1},
		End:   Point{1, 1},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 0, res.TotalCostFt)
	assert.Len(t, res.Path, 1)
}

func TestFindPath_OutOfBounds(t *testing.T) {
	g := openGrid(3, 3)
	req := PathRequest{
		Start: Point{-1, 0},
		End:   Point{2, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.False(t, res.Found)
}

func TestFindPath_NilGrid(t *testing.T) {
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{1, 0},
	}
	_, err := FindPath(req)
	assert.Error(t, err)
}

func TestFindPath_TerrainMismatch(t *testing.T) {
	g := &Grid{Width: 3, Height: 3, Terrain: make([]renderer.TerrainType, 5)}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{1, 0},
		Grid:  g,
	}
	_, err := FindPath(req)
	assert.Error(t, err)
}

func TestFindPath_LongWallBlocking(t *testing.T) {
	// 5×5 grid with a long vertical wall from (2,0) to (2,4) blocking col 1→2
	g := openGrid(5, 5)
	g.Walls = []renderer.WallSegment{{X1: 2, Y1: 0, X2: 2, Y2: 5}}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{4, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	// Wall blocks all cardinal crossings from col 1 to col 2
	// But diagonal corner-cutting is allowed, so can cross at corners
	assert.True(t, res.Found)
}

func TestFindPath_SouthwardPath(t *testing.T) {
	g := openGrid(1, 4)
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{0, 3},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 15, res.TotalCostFt) // 3 * 5ft
}

func TestFindPath_HorizontalWallBlocking(t *testing.T) {
	// 1×3 grid, horizontal wall between row 0 and row 1
	g := openGrid(1, 3)
	g.Walls = []renderer.WallSegment{{X1: 0, Y1: 1, X2: 1, Y2: 1}}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{0, 2},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.False(t, res.Found) // 1-wide, can't go around
}

func TestParseSizeCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Tiny", SizeTiny},
		{"Small", SizeSmall},
		{"Medium", SizeMedium},
		{"Large", SizeLarge},
		{"Huge", SizeHuge},
		{"Gargantuan", SizeGargantuan},
		{"Unknown", SizeMedium},
		{"", SizeMedium},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, ParseSizeCategory(tt.input), "for input %q", tt.input)
	}
}

func TestFindPath_EndOutOfBounds(t *testing.T) {
	g := openGrid(3, 3)
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{5, 5},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.False(t, res.Found)
}

func TestFindPath_InvalidGridDimensions(t *testing.T) {
	g := &Grid{Width: 0, Height: 3, Terrain: []renderer.TerrainType{}}
	req := PathRequest{Start: Point{0, 0}, End: Point{0, 1}, Grid: g}
	_, err := FindPath(req)
	assert.Error(t, err)
}

func TestFindPath_PathAroundEnemy(t *testing.T) {
	// 3×3 grid, enemy at (1,1), find path from (0,0) to (2,2)
	g := openGrid(3, 3)
	g.Occupants = []Occupant{{Col: 1, Row: 1, IsAlly: false, SizeCategory: SizeMedium}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 2},
		SizeCategory: SizeMedium,
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	// Must go around the enemy — check it doesn't pass through (1,1)
	for _, p := range res.Path {
		if p.Col == 1 && p.Row == 1 {
			t.Fatal("path should not go through enemy")
		}
	}
}

func TestFindPath_MultipleWallSegments(t *testing.T) {
	// Test multiple wall segments creating a channel
	g := openGrid(5, 3)
	// Wall above row 0→1 from col 1 to col 3
	g.Walls = []renderer.WallSegment{
		{X1: 1, Y1: 1, X2: 4, Y2: 1}, // horizontal wall blocking row 0→1 for cols 1-3
	}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{4, 2},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
}

func TestFindPath_DiagonalMixedTerrain(t *testing.T) {
	// 3×3 grid: destination is difficult terrain, diagonal move
	g := openGrid(3, 3)
	g.Terrain[1*3+1] = renderer.TerrainDifficultTerrain // (1,1) is difficult
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{1, 1},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 10, res.TotalCostFt) // entering difficult terrain: 10ft
}

func TestFindPath_EnemySizeDiffExactly2(t *testing.T) {
	// Size diff of exactly 2 should allow passing through
	g := openGrid(3, 1)
	g.Occupants = []Occupant{{Col: 1, Row: 0, IsAlly: false, SizeCategory: SizeSmall}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 0},
		SizeCategory: SizeLarge, // Large(3) - Small(1) = 2 >= 2
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 10, res.TotalCostFt)
}

func TestFindPath_AllDifficultTerrain(t *testing.T) {
	g := openGrid(3, 1)
	for i := range g.Terrain {
		g.Terrain[i] = renderer.TerrainDifficultTerrain
	}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{2, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 20, res.TotalCostFt) // 2 * 10ft
}

func TestFindPath_ProneFullPath(t *testing.T) {
	g := openGrid(4, 1)
	req := PathRequest{
		Start:   Point{0, 0},
		End:     Point{3, 0},
		IsProne: true,
		Grid:    g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 30, res.TotalCostFt) // 3 * 10ft
	assert.Len(t, res.Path, 4)
}

func TestFindPath_SmallMoverLargeEnemy(t *testing.T) {
	// Tiny mover vs Large enemy: diff = 3 >= 2, can pass through
	g := openGrid(3, 1)
	g.Occupants = []Occupant{{Col: 1, Row: 0, IsAlly: false, SizeCategory: SizeLarge}}
	req := PathRequest{
		Start:        Point{0, 0},
		End:          Point{2, 0},
		SizeCategory: SizeTiny, // Tiny(0) vs Large(3), diff=3 >= 2
		Grid:         g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
}

func TestFindPath_NorthAndWestDirections(t *testing.T) {
	// Start from bottom-right, go to top-left
	g := openGrid(3, 3)
	req := PathRequest{
		Start: Point{2, 2},
		End:   Point{0, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 10, res.TotalCostFt) // 2 diagonal
}

func TestFindPath_WallAtGridEdge(t *testing.T) {
	// Wall at x=0 (left edge of grid) — should not crash
	g := openGrid(3, 3)
	g.Walls = []renderer.WallSegment{{X1: 0, Y1: 0, X2: 0, Y2: 3}}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{2, 0},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found) // wall at left edge doesn't block anything inside
}

func TestFindPath_WallAtBottomEdge(t *testing.T) {
	// Horizontal wall at y=height (bottom edge) — should not crash
	g := openGrid(3, 3)
	g.Walls = []renderer.WallSegment{{X1: 0, Y1: 3, X2: 3, Y2: 3}}
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{2, 2},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
}

func TestFindPath_LargerGrid(t *testing.T) {
	// 10×10 grid, path from corner to corner
	g := openGrid(10, 10)
	req := PathRequest{
		Start: Point{0, 0},
		End:   Point{9, 9},
		Grid:  g,
	}
	res, err := FindPath(req)
	require.NoError(t, err)
	assert.True(t, res.Found)
	assert.Equal(t, 45, res.TotalCostFt) // 9 diagonal * 5ft
}
