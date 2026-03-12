package renderer

import (
	"bytes"
	"image/png"
	"testing"
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
