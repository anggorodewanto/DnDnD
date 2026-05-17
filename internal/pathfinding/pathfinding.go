package pathfinding

import (
	"container/heap"
	"errors"
	"math"

	"github.com/ab/dndnd/internal/gamemap/renderer"
)

// Size category constants.
const (
	SizeTiny       = 0
	SizeSmall      = 1
	SizeMedium     = 2
	SizeLarge      = 3
	SizeHuge       = 4
	SizeGargantuan = 5
)

// ParseSizeCategory converts a size string to an int category.
func ParseSizeCategory(s string) int {
	switch s {
	case "Tiny":
		return SizeTiny
	case "Small":
		return SizeSmall
	case "Medium":
		return SizeMedium
	case "Large":
		return SizeLarge
	case "Huge":
		return SizeHuge
	case "Gargantuan":
		return SizeGargantuan
	default:
		return SizeMedium
	}
}

// Point represents a 0-based grid coordinate.
type Point struct {
	Col int
	Row int
}

// Occupant represents a creature occupying a tile.
type Occupant struct {
	Col          int
	Row          int
	IsAlly       bool
	SizeCategory int
	AltitudeFt   int // altitude in feet; 0 = ground level
}

// Grid holds the pathfinding map data.
type Grid struct {
	Width     int
	Height    int
	Terrain   []renderer.TerrainType // row-major, len = Width*Height
	Walls     []renderer.WallSegment
	Occupants []Occupant
}

// PathRequest holds everything needed to compute a path.
type PathRequest struct {
	Start          Point
	End            Point
	IsProne        bool
	SizeCategory   int
	Grid           *Grid
	MoverAltitudeFt int // mover's altitude; occupants at different altitudes don't block
}

// PathResult is the output of FindPath.
type PathResult struct {
	Path        []Point
	TotalCostFt int
	Found       bool
}

// edge represents a blocked edge between two tiles.
// Stored as two points in canonical order (lower first).
type edge struct {
	r1, c1, r2, c2 int
}

func makeEdge(r1, c1, r2, c2 int) edge {
	if r1 > r2 || (r1 == r2 && c1 > c2) {
		return edge{r2, c2, r1, c1}
	}
	return edge{r1, c1, r2, c2}
}

// buildBlockedEdges pre-processes wall segments into a set of blocked tile edges.
func buildBlockedEdges(walls []renderer.WallSegment, width, height int) map[edge]bool {
	blocked := make(map[edge]bool)
	for _, w := range walls {
		addWallEdges(blocked, w, width, height)
	}
	return blocked
}

func addWallEdges(blocked map[edge]bool, w renderer.WallSegment, width, height int) {
	// Vertical wall segment: x1 == x2, spans y range
	if w.X1 == w.X2 {
		x := w.X1
		col := int(x) // wall is on the left edge of this column
		yMin := math.Min(w.Y1, w.Y2)
		yMax := math.Max(w.Y1, w.Y2)
		for row := int(yMin); row < int(yMax); row++ {
			// This wall blocks movement between (row, col-1) and (row, col)
			if col > 0 && col <= width && row >= 0 && row < height {
				blocked[makeEdge(row, col-1, row, col)] = true
			}
		}
		return
	}

	// Horizontal wall segment: y1 == y2, spans x range
	if w.Y1 == w.Y2 {
		y := w.Y1
		row := int(y) // wall is on the top edge of this row
		xMin := math.Min(w.X1, w.X2)
		xMax := math.Max(w.X1, w.X2)
		for col := int(xMin); col < int(xMax); col++ {
			// This wall blocks movement between (row-1, col) and (row, col)
			if row > 0 && row <= height && col >= 0 && col < width {
				blocked[makeEdge(row-1, col, row, col)] = true
			}
		}
		return
	}

	// Non-axis-aligned walls: not handled (spec only describes axis-aligned walls)
}

// buildOccupantMap creates a lookup from (row, col) -> occupant for O(1) checks.
// Only occupants at the same altitude as the mover are considered blocking.
func buildOccupantMap(occupants []Occupant, moverAltitudeFt int) map[Point]Occupant {
	m := make(map[Point]Occupant, len(occupants))
	for _, o := range occupants {
		if o.AltitudeFt != moverAltitudeFt {
			continue
		}
		m[Point{o.Col, o.Row}] = o
	}
	return m
}

// 8 directions: cardinal + diagonal
var directions = [8][2]int{
	{0, 1}, {0, -1}, {1, 0}, {-1, 0}, // E, W, S, N
	{-1, -1}, {-1, 1}, {1, -1}, {1, 1}, // NW, NE, SW, SE
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// heuristic: Chebyshev distance * 5 (since diagonal = 5ft too)
func heuristic(a, b Point) int {
	dr := abs(a.Row - b.Row)
	dc := abs(a.Col - b.Col)
	if dr > dc {
		return dr * 5
	}
	return dc * 5
}

// isDiagonal returns true if the move is diagonal.
func isDiagonal(dr, dc int) bool {
	return dr != 0 && dc != 0
}

// FindPath computes the shortest path using A*.
func FindPath(req PathRequest) (*PathResult, error) {
	g := req.Grid
	if g == nil {
		return nil, errors.New("grid is nil")
	}
	if g.Width <= 0 || g.Height <= 0 {
		return nil, errors.New("grid dimensions must be positive")
	}
	if len(g.Terrain) != g.Width*g.Height {
		return nil, errors.New("terrain grid size mismatch")
	}

	// Validate start/end in bounds
	if !inBounds(req.Start, g) || !inBounds(req.End, g) {
		return &PathResult{Found: false}, nil
	}

	// Same start and end
	if req.Start == req.End {
		return &PathResult{
			Path:        []Point{req.Start},
			TotalCostFt: 0,
			Found:       true,
		}, nil
	}

	blockedEdges := buildBlockedEdges(g.Walls, g.Width, g.Height)
	occupantMap := buildOccupantMap(g.Occupants, req.MoverAltitudeFt)

	// A* with priority queue
	gCosts := make(map[Point]int)
	cameFrom := make(map[Point]Point)
	gCosts[req.Start] = 0

	pq := &priorityQueue{}
	heap.Init(pq)
	heap.Push(pq, &pqItem{
		point: req.Start,
		fCost: heuristic(req.Start, req.End),
	})

	for pq.Len() > 0 {
		current := heap.Pop(pq).(*pqItem)
		cur := current.point

		if cur == req.End {
			return buildResult(cameFrom, req.Start, req.End, gCosts[req.End]), nil
		}

		curG := gCosts[cur]
		if curG > current.gCost {
			continue
		}

		for _, dir := range directions {
			dr, dc := dir[0], dir[1]
			next := Point{cur.Col + dc, cur.Row + dr}

			if !inBounds(next, g) {
				continue
			}

			// Walls block cardinal moves
			if !isDiagonal(dr, dc) && blockedEdges[makeEdge(cur.Row, cur.Col, next.Row, next.Col)] {
				continue
			}

			// Diagonal: blocked if BOTH perpendicular edges are walled (L-corner)
			if isDiagonal(dr, dc) {
				e1 := blockedEdges[makeEdge(cur.Row, cur.Col, cur.Row+dr, cur.Col)]
				e2 := blockedEdges[makeEdge(cur.Row, cur.Col, cur.Row, cur.Col+dc)]
				if e1 && e2 {
					continue
				}
			}

			// Enemies block unless size difference >= 2
			if occ, ok := occupantMap[next]; ok && !canPassThrough(occ, req.SizeCategory) {
				continue
			}

			// Calculate move cost
			moveCost := tileCost(next, g, req.IsProne)

			tentativeG := curG + moveCost
			if prevG, ok := gCosts[next]; ok && tentativeG >= prevG {
				continue
			}

			gCosts[next] = tentativeG
			cameFrom[next] = cur
			heap.Push(pq, &pqItem{
				point: next,
				fCost: tentativeG + heuristic(next, req.End),
				gCost: tentativeG,
			})
		}
	}

	return &PathResult{Found: false}, nil
}

func inBounds(p Point, g *Grid) bool {
	return p.Col >= 0 && p.Col < g.Width && p.Row >= 0 && p.Row < g.Height
}

func canPassThrough(occ Occupant, moverSize int) bool {
	if occ.IsAlly {
		return true
	}
	return abs(moverSize-occ.SizeCategory) >= 2
}

func tileCost(p Point, g *Grid, isProne bool) int {
	terrain := g.Terrain[p.Row*g.Width+p.Col]
	cost := 5
	if terrain == renderer.TerrainDifficultTerrain {
		cost = 10
	}
	if isProne {
		cost += 5
	}
	return cost
}

func buildResult(cameFrom map[Point]Point, start, end Point, totalCost int) *PathResult {
	path := []Point{end}
	cur := end
	for cur != start {
		cur = cameFrom[cur]
		path = append(path, cur)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return &PathResult{
		Path:        path,
		TotalCostFt: totalCost,
		Found:       true,
	}
}

// Priority queue implementation for A*
type pqItem struct {
	point Point
	fCost int
	gCost int
	index int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int            { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool  { return pq[i].fCost < pq[j].fCost }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	item := x.(*pqItem)
	item.index = len(*pq)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}
