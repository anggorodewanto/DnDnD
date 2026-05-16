package renderer

import (
	"testing"

	"github.com/fogleman/gg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 3: Shadowcasting in open area — all tiles within range visible
func TestShadowcast_OpenArea(t *testing.T) {
	visible := shadowcast(2, 2, 3, nil, 5, 5)

	// Origin tile always visible
	assert.True(t, visible[GridPos{2, 2}])
	// Adjacent tiles visible
	assert.True(t, visible[GridPos{3, 2}])
	assert.True(t, visible[GridPos{2, 3}])
	assert.True(t, visible[GridPos{1, 1}])
	// (0,0) at distance sqrt(8) ~ 2.83 is within range 3
	assert.True(t, visible[GridPos{0, 0}])
}

func TestShadowcast_OpenArea_AllInRange(t *testing.T) {
	visible := shadowcast(2, 2, 10, nil, 5, 5)

	// All tiles should be visible with large range and no walls
	for row := 0; row < 5; row++ {
		for col := 0; col < 5; col++ {
			assert.True(t, visible[GridPos{col, row}], "tile (%d,%d) should be visible", col, row)
		}
	}
}

// TDD Cycle 4: Wall blocks LOS — tiles behind a wall are not visible
func TestShadowcast_WallBlocksLOS(t *testing.T) {
	// 7x7 grid, viewer at (3,3), wall across entire width at y=2
	walls := []WallSegment{
		{X1: 0, Y1: 2, X2: 7, Y2: 2}, // horizontal wall at y=2
	}

	visible := shadowcast(3, 3, 10, walls, 7, 7)

	// Tiles south of wall (and on wall boundary row 2) should be visible
	assert.True(t, visible[GridPos{3, 3}])  // origin
	assert.True(t, visible[GridPos{3, 2}])  // row 2 — south side of wall
	assert.True(t, visible[GridPos{0, 3}])  // same row
	assert.True(t, visible[GridPos{6, 6}])  // far south-east

	// Tiles north of wall should NOT be visible (row 0 and row 1)
	assert.False(t, visible[GridPos{3, 1}], "tile behind wall should not be visible")
	assert.False(t, visible[GridPos{3, 0}], "tile behind wall should not be visible")
	assert.False(t, visible[GridPos{0, 0}], "tile behind wall should not be visible")
}

// TDD Cycle 4b: Symmetry — if A can see B then B can see A
func TestShadowcast_Symmetry(t *testing.T) {
	walls := []WallSegment{
		{X1: 3, Y1: 2, X2: 3, Y2: 4}, // vertical wall from y=2 to y=4 at x=3
	}

	for r := 0; r < 7; r++ {
		for c := 0; c < 7; c++ {
			vis1 := shadowcast(c, r, 10, walls, 7, 7)
			for r2 := 0; r2 < 7; r2++ {
				for c2 := 0; c2 < 7; c2++ {
					if vis1[GridPos{c2, r2}] {
						vis2 := shadowcast(c2, r2, 10, walls, 7, 7)
						assert.True(t, vis2[GridPos{c, r}],
							"symmetry violated: (%d,%d) sees (%d,%d) but not vice versa", c, r, c2, r2)
					}
				}
			}
		}
	}
}

// TDD Cycle 5: ComputeVisibility unions multiple vision sources
func TestComputeVisibility_UnionSources(t *testing.T) {
	// Two sources on opposite sides of a wall, each sees different parts
	walls := []WallSegment{
		{X1: 3, Y1: 0, X2: 3, Y2: 7}, // vertical wall splitting map in half
	}

	sources := []VisionSource{
		{Col: 1, Row: 3, RangeTiles: 10}, // left side
		{Col: 5, Row: 3, RangeTiles: 10}, // right side
	}

	fow := ComputeVisibility(sources, walls, 7, 7)
	require.NotNil(t, fow)

	// Tiles on both sides should be visible (union)
	assert.Equal(t, Visible, fow.StateAt(1, 3))
	assert.Equal(t, Visible, fow.StateAt(5, 3))
	assert.Equal(t, Visible, fow.StateAt(0, 0))
	assert.Equal(t, Visible, fow.StateAt(6, 6))
}

// TDD Cycle 5b: ComputeVisibility with no sources yields all unexplored
func TestComputeVisibility_NoSources(t *testing.T) {
	fow := ComputeVisibility(nil, nil, 5, 5)
	require.NotNil(t, fow)

	for row := 0; row < 5; row++ {
		for col := 0; col < 5; col++ {
			assert.Equal(t, Unexplored, fow.StateAt(col, row))
		}
	}
}

// TDD Cycle 6: Vision range limits visibility
func TestComputeVisibility_RangeLimits(t *testing.T) {
	sources := []VisionSource{
		{Col: 0, Row: 0, RangeTiles: 2},
	}

	fow := ComputeVisibility(sources, nil, 10, 10)
	require.NotNil(t, fow)

	assert.Equal(t, Visible, fow.StateAt(0, 0))
	assert.Equal(t, Visible, fow.StateAt(1, 1))
	assert.Equal(t, Unexplored, fow.StateAt(9, 9))
}

// TDD Cycle 7: VisionSource with darkvision extends range
func TestComputeVisibility_Darkvision(t *testing.T) {
	sources := []VisionSource{
		{Col: 5, Row: 5, RangeTiles: 2, DarkvisionTiles: 6},
	}

	fow := ComputeVisibility(sources, nil, 11, 11)
	require.NotNil(t, fow)

	// Tile at distance 4 — beyond base range but within darkvision
	assert.Equal(t, Visible, fow.StateAt(5, 1))
	// Tile at distance 7+ — beyond darkvision
	assert.Equal(t, Unexplored, fow.StateAt(0, 0)) // dist ~7.07
}

// TDD Cycle 7b: VisionSource with blindsight extends range
func TestComputeVisibility_Blindsight(t *testing.T) {
	sources := []VisionSource{
		{Col: 5, Row: 5, RangeTiles: 2, BlindsightTiles: 5},
	}

	fow := ComputeVisibility(sources, nil, 11, 11)
	require.NotNil(t, fow)

	assert.Equal(t, Visible, fow.StateAt(5, 1)) // dist 4
	assert.Equal(t, Visible, fow.StateAt(5, 0)) // dist 5
}

// TDD Cycle 7c: VisionSource with truesight extends range
func TestComputeVisibility_Truesight(t *testing.T) {
	sources := []VisionSource{
		{Col: 5, Row: 5, RangeTiles: 2, TruesightTiles: 6},
	}

	fow := ComputeVisibility(sources, nil, 11, 11)
	require.NotNil(t, fow)

	assert.Equal(t, Visible, fow.StateAt(5, 0)) // dist 5
}

// TDD Cycle 8: FogOfWar field on MapData — rendering with fog
func TestRenderMap_WithFog(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
		VisionSources: []VisionSource{
			{Col: 2, Row: 2, RangeTiles: 3},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TDD Cycle 9: Enemy tokens hidden in fog
func TestRenderMap_EnemyHiddenInFog(t *testing.T) {
	fow := &FogOfWar{
		Width:  5,
		Height: 5,
		States: make([]VisibilityState, 25),
	}
	// Only tile (2,2) is visible
	fow.States[2*5+2] = Visible

	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			return g
		}(),
		FogOfWar: fow,
		Combatants: []Combatant{
			{ShortID: "P1", Col: 2, Row: 2, HPMax: 10, HPCurrent: 10, IsPlayer: true},
			{ShortID: "E1", Col: 4, Row: 4, HPMax: 10, HPCurrent: 10, IsPlayer: false}, // in fog
		},
	}

	// Should render without error, with enemy hidden
	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TDD Cycle 9b: filterCombatantsForFog unit test
func TestFilterCombatantsForFog(t *testing.T) {
	fow := &FogOfWar{
		Width:  5,
		Height: 5,
		States: make([]VisibilityState, 25),
	}
	fow.States[2*5+2] = Visible  // (2,2) visible
	fow.States[3*5+3] = Explored // (3,3) explored

	combatants := []Combatant{
		{ShortID: "P1", Col: 2, Row: 2, IsPlayer: true},  // player in visible
		{ShortID: "P2", Col: 4, Row: 4, IsPlayer: true},  // player in fog
		{ShortID: "E1", Col: 2, Row: 2, IsPlayer: false, IsVisible: true}, // enemy in visible
		{ShortID: "E2", Col: 4, Row: 4, IsPlayer: false, IsVisible: true}, // enemy in unexplored fog
		{ShortID: "E3", Col: 3, Row: 3, IsPlayer: false, IsVisible: true}, // enemy in explored tile
	}

	filtered := filterCombatantsForFog(combatants, fow)
	assert.Len(t, filtered, 4) // P1, P2, E1, E3 (not E2)

	byID := make(map[string]Combatant)
	for _, c := range filtered {
		byID[c.ShortID] = c
	}
	assert.Contains(t, byID, "P1")  // player always shown
	assert.Contains(t, byID, "P2")  // player always shown even in fog
	assert.Contains(t, byID, "E1")  // enemy in visible tile shown
	assert.NotContains(t, byID, "E2") // enemy in unexplored hidden
	assert.Contains(t, byID, "E3")  // enemy in explored tile shown

	// E3 should be marked as in fog (greyed)
	assert.True(t, byID["E3"].InFog, "enemy on explored tile should have InFog=true")
	// E1 visible enemy should NOT be in fog
	assert.False(t, byID["E1"].InFog, "enemy on visible tile should not have InFog")
	// Players never marked InFog
	assert.False(t, byID["P1"].InFog)
	assert.False(t, byID["P2"].InFog)
}

// TDD Cycle 9c: filterCombatantsForFog with nil fog returns all
func TestFilterCombatantsForFog_NilFog(t *testing.T) {
	combatants := []Combatant{
		{ShortID: "E1", Col: 4, Row: 4, IsPlayer: false},
	}

	filtered := filterCombatantsForFog(combatants, nil)
	assert.Len(t, filtered, 1)
}

// TDD Cycle 10: DM mode (no fog) renders everything
func TestRenderMap_DMMode_NoFog(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			return g
		}(),
		// No FogOfWar and no VisionSources = DM mode
		Combatants: []Combatant{
			{ShortID: "E1", Col: 4, Row: 4, HPMax: 10, HPCurrent: 10, IsPlayer: false},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TDD Cycle 11: FogOfWar.StateAt out of bounds returns Unexplored
func TestFogOfWar_StateAt_OutOfBounds(t *testing.T) {
	fow := &FogOfWar{Width: 3, Height: 3, States: make([]VisibilityState, 9)}
	fow.States[0] = Visible

	assert.Equal(t, Unexplored, fow.StateAt(-1, 0))
	assert.Equal(t, Unexplored, fow.StateAt(0, -1))
	assert.Equal(t, Unexplored, fow.StateAt(3, 0))
	assert.Equal(t, Unexplored, fow.StateAt(0, 3))
	assert.Equal(t, Visible, fow.StateAt(0, 0))
}

// TDD Cycle 12: Shadowcast at map corner
func TestShadowcast_Corner(t *testing.T) {
	visible := shadowcast(0, 0, 5, nil, 5, 5)

	assert.True(t, visible[GridPos{0, 0}])
	assert.True(t, visible[GridPos{3, 3}])  // dist ~4.24, within range
	assert.False(t, visible[GridPos{4, 4}]) // dist ~5.66, beyond 5+0.5
}

// TDD Cycle 13: Partial wall blocks only specific lines of sight
func TestShadowcast_PartialWall(t *testing.T) {
	walls := []WallSegment{
		{X1: 2, Y1: 2, X2: 3, Y2: 2}, // 1-tile wide wall between row 1 and row 2 at col 2
	}

	visible := shadowcast(2, 3, 10, walls, 5, 5)

	assert.False(t, visible[GridPos{2, 1}]) // directly behind wall
	assert.True(t, visible[GridPos{3, 1}])  // not behind wall
	assert.True(t, visible[GridPos{1, 1}])  // not behind wall
}

// TDD Cycle 14: segmentsIntersect edge cases
func TestSegmentsIntersect_Parallel(t *testing.T) {
	assert.False(t, segmentsIntersect(0, 0, 1, 0, 0, 1, 1, 1))
}

func TestSegmentsIntersect_Crossing(t *testing.T) {
	assert.True(t, segmentsIntersect(0, 0, 2, 2, 0, 2, 2, 0))
}

func TestSegmentsIntersect_NonOverlapping(t *testing.T) {
	assert.False(t, segmentsIntersect(0, 0, 1, 0, 2, 0, 3, 0))
}

// TDD Cycle 15: DrawFogOfWar with nil fog is no-op
func TestDrawFogOfWar_NilFog(t *testing.T) {
	md := &MapData{Width: 3, Height: 3, TileSize: 48}
	dc := gg.NewContext(3*48, 3*48)
	DrawFogOfWar(dc, md)
}

// TDD Cycle 16: Rendering with explored state
func TestRenderMap_ExploredState(t *testing.T) {
	fow := &FogOfWar{
		Width:  3,
		Height: 3,
		States: make([]VisibilityState, 9),
	}
	fow.States[0] = Visible
	fow.States[1] = Explored

	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		FogOfWar: fow,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 9)
			return g
		}(),
	}

	renderData, renderErr := RenderMap(md)
	require.NoError(t, renderErr)
	assert.NotEmpty(t, renderData)
}

// TDD Cycle 17: RenderMap auto-computes fog from VisionSources
func TestRenderMap_AutoComputesFog(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			return g
		}(),
		VisionSources: []VisionSource{
			{Col: 2, Row: 2, RangeTiles: 2},
		},
		Combatants: []Combatant{
			{ShortID: "E1", Col: 0, Row: 0, HPMax: 10, HPCurrent: 10, IsPlayer: false},
		},
	}

	assert.Nil(t, md.FogOfWar)

	renderData, renderErr := RenderMap(md)
	require.NoError(t, renderErr)
	assert.NotEmpty(t, renderData)

	require.NotNil(t, md.FogOfWar)
	assert.Equal(t, Visible, md.FogOfWar.StateAt(2, 2))
}

// TDD Cycle 19: Light sources contribute visible tiles
func TestComputeVisibility_LightSources(t *testing.T) {
	// No vision sources, but a light source at (5,5) with range 3
	lights := []LightSource{
		{Col: 5, Row: 5, RangeTiles: 3},
	}

	fow := ComputeVisibilityWithLights(nil, lights, nil, 11, 11)
	require.NotNil(t, fow)

	// Tile at origin of light should be visible
	assert.Equal(t, Visible, fow.StateAt(5, 5))
	// Tile within range should be visible
	assert.Equal(t, Visible, fow.StateAt(5, 3)) // dist 2
	// Tile beyond range should be unexplored
	assert.Equal(t, Unexplored, fow.StateAt(0, 0)) // dist ~7.07
}

// TDD Cycle 20: Light sources union with vision sources
func TestComputeVisibility_LightAndVisionUnion(t *testing.T) {
	sources := []VisionSource{
		{Col: 0, Row: 0, RangeTiles: 2},
	}
	lights := []LightSource{
		{Col: 9, Row: 9, RangeTiles: 2},
	}

	fow := ComputeVisibilityWithLights(sources, lights, nil, 10, 10)
	require.NotNil(t, fow)

	// Vision source area visible
	assert.Equal(t, Visible, fow.StateAt(0, 0))
	assert.Equal(t, Visible, fow.StateAt(1, 1))
	// Light source area visible
	assert.Equal(t, Visible, fow.StateAt(9, 9))
	assert.Equal(t, Visible, fow.StateAt(8, 8))
	// Middle of map unexplored
	assert.Equal(t, Unexplored, fow.StateAt(5, 5))
}

// TDD Cycle 21: Light sources blocked by walls
func TestComputeVisibility_LightBlockedByWall(t *testing.T) {
	walls := []WallSegment{
		{X1: 0, Y1: 3, X2: 7, Y2: 3}, // horizontal wall at y=3
	}
	lights := []LightSource{
		{Col: 3, Row: 4, RangeTiles: 10},
	}

	fow := ComputeVisibilityWithLights(nil, lights, walls, 7, 7)
	require.NotNil(t, fow)

	// Below wall is visible
	assert.Equal(t, Visible, fow.StateAt(3, 4))
	assert.Equal(t, Visible, fow.StateAt(3, 3)) // row 3 south side of wall
	// Above wall is NOT visible
	assert.Equal(t, Unexplored, fow.StateAt(3, 1))
}

// TDD Cycle 22: LightSources in MapData auto-computed
func TestRenderMap_WithLightSources(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			return g
		}(),
		LightSources: []LightSource{
			{Col: 2, Row: 2, RangeTiles: 3},
		},
	}

	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	require.NotNil(t, md.FogOfWar)
	assert.Equal(t, Visible, md.FogOfWar.StateAt(2, 2))
}

// TDD Cycle 23: InFog enemies rendered with greyed appearance
func TestDrawTokens_InFogEnemyGreyed(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		Combatants: []Combatant{
			{ShortID: "E1", Col: 2, Row: 2, HPMax: 10, HPCurrent: 10, IsPlayer: false, InFog: true},
		},
	}

	dc := gg.NewContext(5*48, 5*48)
	// Should render without error — greyed token
	DrawTokens(dc, md)
}

