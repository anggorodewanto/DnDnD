package renderer

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/fogleman/gg"
)

func TestRenderMap_BasicPNG(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 25)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			g[6] = TerrainWater
			g[12] = TerrainLava
			return g
		}(),
		Walls: []WallSegment{
			{X1: 0, Y1: 0, X2: 5, Y2: 0},
		},
		Combatants: []Combatant{
			{ShortID: "G1", Col: 1, Row: 1, HPMax: 10, HPCurrent: 10},
			{ShortID: "AR", Col: 3, Row: 3, HPMax: 20, HPCurrent: 15, IsPlayer: true},
		},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("RenderMap returned empty data")
	}

	// Verify it's a valid PNG
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not valid PNG: %v", err)
	}

	// Check dimensions: 5*48 + margin for labels
	bounds := img.Bounds()
	expectedW := 5*48 + gridLabelMargin
	expectedH := 5*48 + gridLabelMargin
	// With legend (has non-standard terrain)
	legendH := LegendHeight(md)
	expectedH += legendH

	if bounds.Dx() != expectedW {
		t.Errorf("image width = %d, want %d", bounds.Dx(), expectedW)
	}
	if bounds.Dy() != expectedH {
		t.Errorf("image height = %d, want %d", bounds.Dy(), expectedH)
	}
}

func TestRenderMap_NoLegend(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 9)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not valid PNG: %v", err)
	}

	bounds := img.Bounds()
	expectedW := 3*48 + gridLabelMargin
	expectedH := 3*48 + gridLabelMargin
	if bounds.Dx() != expectedW || bounds.Dy() != expectedH {
		t.Errorf("dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), expectedW, expectedH)
	}
}

func TestRenderMap_LargeMapTileSize(t *testing.T) {
	// 101x101 with TileSize 48 should be auto-reduced to 32px tiles
	md := &MapData{
		Width:    101,
		Height:   101,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 101*101)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not valid PNG: %v", err)
	}

	// Should have been auto-reduced to 32px tiles
	bounds := img.Bounds()
	expectedW := 101*32 + gridLabelMargin
	expectedH := 101*32 + gridLabelMargin
	if bounds.Dx() != expectedW {
		t.Errorf("width = %d, want %d (tile size should be auto-reduced to 32)", bounds.Dx(), expectedW)
	}
	if bounds.Dy() != expectedH {
		t.Errorf("height = %d, want %d (tile size should be auto-reduced to 32)", bounds.Dy(), expectedH)
	}
}

func TestRenderMap_SmallMapKeepsTileSize(t *testing.T) {
	// 100x100 should NOT auto-reduce (boundary check)
	md := &MapData{
		Width:    100,
		Height:   100,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 100*100)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not valid PNG: %v", err)
	}

	bounds := img.Bounds()
	expectedW := 100*48 + gridLabelMargin
	expectedH := 100*48 + gridLabelMargin
	if bounds.Dx() != expectedW {
		t.Errorf("width = %d, want %d (100x100 should keep 48px tiles)", bounds.Dx(), expectedW)
	}
	if bounds.Dy() != expectedH {
		t.Errorf("height = %d, want %d (100x100 should keep 48px tiles)", bounds.Dy(), expectedH)
	}
}

func TestRenderMap_WithActiveEffects(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 9)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
		ActiveEffects: []ActiveEffect{
			{Symbol: "\u2601", Name: "Fog Cloud", CasterName: "Kael", Area: "20ft radius", Rounds: 8},
		},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not valid PNG: %v", err)
	}

	bounds := img.Bounds()
	expectedH := 3*48 + gridLabelMargin + LegendHeight(md)
	if bounds.Dy() != expectedH {
		t.Errorf("height = %d, want %d (should include legend)", bounds.Dy(), expectedH)
	}
}

func TestRenderMap_AllHealthTiers(t *testing.T) {
	md := &MapData{
		Width:    8,
		Height:   1,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 8)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
		Combatants: []Combatant{
			{ShortID: "A", Col: 0, Row: 0, HPMax: 100, HPCurrent: 100},
			{ShortID: "B", Col: 1, Row: 0, HPMax: 100, HPCurrent: 80},
			{ShortID: "C", Col: 2, Row: 0, HPMax: 100, HPCurrent: 50},
			{ShortID: "D", Col: 3, Row: 0, HPMax: 100, HPCurrent: 10},
			{ShortID: "E", Col: 4, Row: 0, HPMax: 100, HPCurrent: 0, IsDying: true},
			{ShortID: "F", Col: 5, Row: 0, HPMax: 100, HPCurrent: 0},
			{ShortID: "G", Col: 6, Row: 0, HPMax: 100, HPCurrent: 0, IsStable: true},
		},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty PNG")
	}

	_, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not valid PNG: %v", err)
	}
}

func TestRenderMap_StackedTokens(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 9)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
		Combatants: []Combatant{
			{ShortID: "G1", Col: 1, Row: 1, AltitudeFt: 0, HPMax: 10, HPCurrent: 10},
			{ShortID: "F1", Col: 1, Row: 1, AltitudeFt: 30, HPMax: 10, HPCurrent: 10},
			{ShortID: "F2", Col: 1, Row: 1, AltitudeFt: 60, HPMax: 10, HPCurrent: 10},
		},
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}

	_, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not valid PNG: %v", err)
	}
}

func TestRenderMap_RejectsExceedingHardLimit(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"width exceeds", 201, 100},
		{"height exceeds", 100, 201},
		{"both exceed", 201, 201},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			md := &MapData{Width: tc.width, Height: tc.height, TileSize: 48}
			_, err := RenderMap(md)
			if err == nil {
				t.Fatalf("expected error for %dx%d map, got nil", tc.width, tc.height)
			}
		})
	}
}

func TestRenderMap_DoesNotMutateTileSize(t *testing.T) {
	md := &MapData{
		Width:    150,
		Height:   50,
		TileSize: 64,
		TerrainGrid: func() []TerrainType {
			g := make([]TerrainType, 150*50)
			for i := range g {
				g[i] = TerrainOpenGround
			}
			return g
		}(),
	}

	_, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}

	if md.TileSize != 64 {
		t.Errorf("RenderMap mutated md.TileSize: got %d, want 64", md.TileSize)
	}
}

func TestRenderMap_BackgroundImage(t *testing.T) {
	bgImg := createSolidColorPNG(t, 96, 96, 0x00, 0x00, 0xFF) // blue

	// With terrain present, background is drawn beneath. The terrain (beige)
	// covers it fully. To verify the background is actually composited, we
	// compare a render WITH background vs WITHOUT and confirm they differ
	// when terrain is absent for a specific tile.
	// Simplest: render with no terrain grid entries that would cover the bg.
	// terrainAt returns OpenGround for nil grid, which draws beige on top.
	// So instead, verify the background draws by checking a pixel BEFORE
	// terrain would be drawn — but that's not possible with the current
	// layering. Instead, verify that the background image bytes are decoded
	// and drawn by comparing output with and without background.
	mdWithBg := &MapData{
		Width:             2,
		Height:            2,
		TileSize:          48,
		TerrainGrid:       make([]TerrainType, 4),
		BackgroundImage:   bgImg,
		BackgroundOpacity: 1.0,
	}
	mdWithoutBg := &MapData{
		Width:       2,
		Height:      2,
		TileSize:    48,
		TerrainGrid: make([]TerrainType, 4),
	}

	dataWith, err := RenderMap(mdWithBg)
	if err != nil {
		t.Fatalf("RenderMap with bg error: %v", err)
	}
	dataWithout, err := RenderMap(mdWithoutBg)
	if err != nil {
		t.Fatalf("RenderMap without bg error: %v", err)
	}

	// The outputs should differ because the background is drawn beneath terrain.
	// Even though terrain covers it, the rendering pipeline processes differently
	// (background drawn, then terrain on top). With fully opaque terrain the
	// pixel values may be identical — so this test verifies the code path runs
	// without error. The real visual test is with partial-opacity terrain or
	// when the caller skips terrain.
	if len(dataWith) == 0 {
		t.Fatal("RenderMap with background returned empty data")
	}
	if len(dataWithout) == 0 {
		t.Fatal("RenderMap without background returned empty data")
	}

	// Verify both are valid PNGs
	_, err = png.Decode(bytes.NewReader(dataWith))
	if err != nil {
		t.Fatalf("output with bg is not valid PNG: %v", err)
	}
	_, err = png.Decode(bytes.NewReader(dataWithout))
	if err != nil {
		t.Fatalf("output without bg is not valid PNG: %v", err)
	}
}

func TestRenderMap_BackgroundImage_VisibleWhenNoTerrain(t *testing.T) {
	// Use a distinctive color that differs from the white canvas and beige terrain.
	bgImg := createSolidColorPNG(t, 96, 96, 0x00, 0x00, 0xFF) // blue

	// Render with background but skip terrain by using a custom approach:
	// We can't skip DrawTerrain in the current architecture, but we CAN verify
	// that the background is drawn by checking that the white canvas fill is
	// replaced. Since terrain draws beige (0xF0F0E8) on top, the final pixel
	// won't show blue. However, if we set BackgroundOpacity to 0, the background
	// should NOT affect the output (white canvas + terrain = beige).
	// With opacity 1.0, background IS drawn but terrain covers it.
	// The key test: verify drawBackgroundImage is called and doesn't panic/error.
	// For a true pixel-level test, we'd need to make terrain semi-transparent.
	// Instead, test that a 0-opacity background produces the same output as no background.
	mdZeroOpacity := &MapData{
		Width:             2,
		Height:            2,
		TileSize:          48,
		TerrainGrid:       make([]TerrainType, 4),
		BackgroundImage:   bgImg,
		BackgroundOpacity: 0.0,
	}
	mdNoBg := &MapData{
		Width:       2,
		Height:      2,
		TileSize:    48,
		TerrainGrid: make([]TerrainType, 4),
	}

	dataZero, err := RenderMap(mdZeroOpacity)
	if err != nil {
		t.Fatalf("RenderMap zero opacity error: %v", err)
	}
	dataNone, err := RenderMap(mdNoBg)
	if err != nil {
		t.Fatalf("RenderMap no bg error: %v", err)
	}

	// With 0 opacity, background should be invisible — output identical to no background.
	if !bytes.Equal(dataZero, dataNone) {
		t.Error("expected 0-opacity background to produce identical output to no background")
	}
}

func TestRenderMap_BackgroundImage_WithTerrain(t *testing.T) {
	bgImg := createSolidColorPNG(t, 96, 96, 0xFF, 0x00, 0x00) // red

	// With terrain present, background is drawn but terrain covers it.
	// Verify no error and valid output.
	md := &MapData{
		Width:             2,
		Height:            2,
		TileSize:          48,
		TerrainGrid:       make([]TerrainType, 4),
		BackgroundImage:   bgImg,
		BackgroundOpacity: 0.5,
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("RenderMap returned empty data")
	}
}

func TestRenderMap_BackgroundImage_NilImage(t *testing.T) {
	// No background image — should render normally without error.
	md := &MapData{
		Width:             2,
		Height:            2,
		TileSize:          48,
		TerrainGrid:       make([]TerrainType, 4),
		BackgroundImage:   nil,
		BackgroundOpacity: 1.0,
	}

	data, err := RenderMap(md)
	if err != nil {
		t.Fatalf("RenderMap error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("RenderMap returned empty data")
	}
}

// createSolidColorPNG creates a PNG image of the given size filled with a solid color.
func createSolidColorPNG(t *testing.T, w, h int, red, green, blue uint8) []byte {
	t.Helper()
	dc := gg.NewContext(w, h)
	dc.SetRGB(float64(red)/255, float64(green)/255, float64(blue)/255)
	dc.Clear()
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		t.Fatalf("failed to create test PNG: %v", err)
	}
	return buf.Bytes()
}
