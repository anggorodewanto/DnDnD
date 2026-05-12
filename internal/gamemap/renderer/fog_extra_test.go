package renderer

import (
	"testing"

	"github.com/fogleman/gg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E-68 sub-item 1: the FoW algorithm's symmetry property — explicitly
// asserted as a documented invariant of the (renamed-and-documented)
// algorithm. This guards the doc comment on shadowcast: changing the
// algorithm to anything asymmetric must break this test.
func TestShadowcast_SymmetryInvariant_Documented(t *testing.T) {
	// L-shaped walls — non-trivial geometry that nonetheless must be
	// fully symmetric (center-to-center raycasting is symmetric by
	// construction; equivalent to Albert Ford's symmetric shadowcasting
	// for our use).
	walls := []WallSegment{
		{X1: 2, Y1: 1, X2: 2, Y2: 4}, // vertical
		{X1: 2, Y1: 4, X2: 5, Y2: 4}, // horizontal
	}
	const dim = 6
	for r := 0; r < dim; r++ {
		for c := 0; c < dim; c++ {
			vis1 := shadowcast(c, r, 20, walls, dim, dim)
			for r2 := 0; r2 < dim; r2++ {
				for c2 := 0; c2 < dim; c2++ {
					if !vis1[GridPos{c2, r2}] {
						continue
					}
					vis2 := shadowcast(c2, r2, 20, walls, dim, dim)
					require.True(t, vis2[GridPos{c, r}],
						"symmetry broken: (%d,%d) sees (%d,%d) but not vice versa", c, r, c2, r2)
				}
			}
		}
	}
}

// E-68 sub-item 2: VisionSource carries HasDevilsSight and the FoW union
// honors it against magical darkness. A devil's-sight viewer sees through
// a magical-darkness zone covering tiles between viewer and target;
// a vanilla darkvision viewer does not.
func TestComputeVisibility_DevilsSight_SeesThroughMagicalDarkness(t *testing.T) {
	// Viewer at (0,0). Magical darkness blob covering tiles (1,0) and (2,0).
	// Target tile is (3,0). Devil's Sight viewer sees the target as Visible.
	darkness := []GridPos{{Col: 1, Row: 0}, {Col: 2, Row: 0}}
	src := VisionSource{
		Col: 0, Row: 0, RangeTiles: 1, DarkvisionTiles: 6,
		HasDevilsSight: true,
	}
	fow := ComputeVisibilityWithZones([]VisionSource{src}, nil, nil, darkness, 5, 1)
	require.NotNil(t, fow)

	assert.Equal(t, Visible, fow.StateAt(3, 0), "Devil's Sight should pierce magical darkness")
}

func TestComputeVisibility_Darkvision_BlockedByMagicalDarkness(t *testing.T) {
	// Same geometry, but viewer has only darkvision (no Devil's Sight).
	// Tiles inside magical-darkness blob must not be Visible to a
	// darkvision-only viewer.
	darkness := []GridPos{{Col: 1, Row: 0}, {Col: 2, Row: 0}}
	src := VisionSource{
		Col: 0, Row: 0, RangeTiles: 1, DarkvisionTiles: 6,
	}
	fow := ComputeVisibilityWithZones([]VisionSource{src}, nil, nil, darkness, 5, 1)
	require.NotNil(t, fow)

	assert.NotEqual(t, Visible, fow.StateAt(1, 0),
		"darkvision-only viewer must not see Visible through magical darkness")
	assert.NotEqual(t, Visible, fow.StateAt(2, 0),
		"darkvision-only viewer must not see Visible through magical darkness")
}

// E-68 sub-item 3: magical-darkness zones demote darkvision inside
// ComputeVisibilityWithLights/Zones — tiles in a magical-darkness zone
// are NOT returned as Visible for a darkvision-only viewer (zone-level
// pass, not just attack/check time).
func TestComputeVisibilityWithZones_MagicalDarknessDemotesDarkvision(t *testing.T) {
	// Viewer with darkvision-only at (5,5). Magical darkness covers
	// (5,3). Without the zone-level pass the tile would be Visible
	// (within darkvision range); with the pass it must NOT be Visible.
	darkness := []GridPos{{Col: 5, Row: 3}}
	src := VisionSource{Col: 5, Row: 5, RangeTiles: 0, DarkvisionTiles: 6}

	fow := ComputeVisibilityWithZones([]VisionSource{src}, nil, nil, darkness, 11, 11)
	require.NotNil(t, fow)

	assert.NotEqual(t, Visible, fow.StateAt(5, 3),
		"tile inside magical darkness must not be Visible to a darkvision-only viewer")
	// Adjacent tile outside the zone should still be Visible.
	assert.Equal(t, Visible, fow.StateAt(5, 4))
}

// Mirror of EffectiveObscurement semantics: base vision (no darkvision)
// is ALSO blocked by magical darkness. Only blindsight, truesight, and
// Devil's Sight pierce it. Guards against the implementation accidentally
// letting base-range vision leak into a magical-darkness tile.
func TestComputeVisibilityWithZones_BaseVisionAlsoBlockedByMagicalDarkness(t *testing.T) {
	darkness := []GridPos{{Col: 1, Row: 0}}
	src := VisionSource{Col: 0, Row: 0, RangeTiles: 5}

	fow := ComputeVisibilityWithZones([]VisionSource{src}, nil, nil, darkness, 5, 1)
	require.NotNil(t, fow)
	assert.NotEqual(t, Visible, fow.StateAt(1, 0),
		"base vision must not penetrate magical darkness")
}

// Blindsight still penetrates magical darkness within its range.
func TestComputeVisibilityWithZones_BlindsightPenetratesMagicalDarkness(t *testing.T) {
	darkness := []GridPos{{Col: 2, Row: 0}}
	src := VisionSource{Col: 0, Row: 0, BlindsightTiles: 5}

	fow := ComputeVisibilityWithZones([]VisionSource{src}, nil, nil, darkness, 5, 1)
	require.NotNil(t, fow)
	assert.Equal(t, Visible, fow.StateAt(2, 0),
		"blindsight must penetrate magical darkness within range")
}

// Devil's Sight at extreme range (beyond 24 tiles) is again blocked.
func TestComputeVisibilityWithZones_DevilsSightOutOfRange(t *testing.T) {
	// 30-tile-wide strip; darkness tile sits at (27,0); viewer at (0,0)
	// with HasDevilsSight. Chebyshev distance = 27, beyond 24.
	darkness := []GridPos{{Col: 27, Row: 0}}
	src := VisionSource{Col: 0, Row: 0, RangeTiles: 30, HasDevilsSight: true}

	fow := ComputeVisibilityWithZones([]VisionSource{src}, nil, nil, darkness, 30, 1)
	require.NotNil(t, fow)
	assert.NotEqual(t, Visible, fow.StateAt(27, 0),
		"Devil's Sight beyond 24 tiles must not pierce magical darkness")
}

// E-68 sub-item 4: DM-sees-all explicitly bypasses FoW. Currently the
// caller has to omit the FogOfWar entirely. We need an explicit flag
// so a caller can compute fog for player view while still asking the
// renderer to show the DM everything.
func TestDrawFogOfWar_DMSeesAll_DoesNotPaintOverlay(t *testing.T) {
	// Build a fog where everything is Unexplored. DMSeesAll=true must
	// cause DrawFogOfWar to skip rendering — the dc must be untouched.
	// We assert by starting from a white background and checking that
	// a sample pixel inside what would be the fog overlay remains white.
	fow := &FogOfWar{
		Width: 3, Height: 3,
		States:    make([]VisibilityState, 9), // all Unexplored
		DMSeesAll: true,
	}
	md := &MapData{Width: 3, Height: 3, TileSize: 48, FogOfWar: fow}
	dc := gg.NewContext(3*48, 3*48)
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	DrawFogOfWar(dc, md)

	// Sample a pixel that would be inside the unexplored overlay if
	// DMSeesAll did not bypass it. White background must remain.
	r, g, b, _ := dc.Image().At(24, 24).RGBA()
	assert.Equal(t, uint32(0xFFFF), r, "DMSeesAll: pixel must remain white (no overlay painted)")
	assert.Equal(t, uint32(0xFFFF), g)
	assert.Equal(t, uint32(0xFFFF), b)
}

func TestDrawFogOfWar_NotDMSeesAll_DoesPaintOverlay(t *testing.T) {
	// Counter-test: with DMSeesAll=false the overlay IS painted on
	// Unexplored tiles (sanity-check that the test above would fail
	// without the bypass).
	fow := &FogOfWar{
		Width: 3, Height: 3,
		States: make([]VisibilityState, 9), // all Unexplored
	}
	md := &MapData{Width: 3, Height: 3, TileSize: 48, FogOfWar: fow}
	dc := gg.NewContext(3*48, 3*48)
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	DrawFogOfWar(dc, md)

	r, _, _, _ := dc.Image().At(24, 24).RGBA()
	assert.NotEqual(t, uint32(0xFFFF), r,
		"without DMSeesAll the overlay must be painted (pixel no longer white)")
}

func TestFilterCombatantsForFog_DMSeesAll_ReturnsAll(t *testing.T) {
	fow := &FogOfWar{
		Width: 5, Height: 5,
		States:    make([]VisibilityState, 25),
		DMSeesAll: true,
	}
	combatants := []Combatant{
		{ShortID: "E1", Col: 4, Row: 4, IsPlayer: false}, // would be hidden without DMSeesAll
		{ShortID: "E2", Col: 0, Row: 0, IsPlayer: false},
	}
	filtered := filterCombatantsForFog(combatants, fow)
	assert.Len(t, filtered, 2, "DMSeesAll must show all combatants regardless of FoW")
	for _, c := range filtered {
		assert.False(t, c.InFog, "DMSeesAll combatants should not be marked InFog")
	}
}

func TestRenderMap_DMSeesAll_RendersEnemyOnUnexplored(t *testing.T) {
	// Vision sources cover only (0,0). DM-sees-all flag is set on
	// MapData. Enemy at (4,4) (which would otherwise be hidden in fog)
	// must still appear in the rendered combatants list.
	md := &MapData{
		Width: 5, Height: 5, TileSize: 48,
		TerrainGrid: make([]TerrainType, 25),
		VisionSources: []VisionSource{
			{Col: 0, Row: 0, RangeTiles: 1},
		},
		Combatants: []Combatant{
			{ShortID: "E1", Col: 4, Row: 4, HPMax: 10, HPCurrent: 10, IsPlayer: false},
		},
		DMSeesAll: true,
	}
	data, err := RenderMap(md)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	require.NotNil(t, md.FogOfWar)
	assert.True(t, md.FogOfWar.DMSeesAll, "RenderMap must propagate DMSeesAll into FogOfWar")
}
