package renderer

import (
	"image/color"
	"testing"

	"github.com/fogleman/gg"
)

func TestNeedsLegend_NoTerrainNoEffects(t *testing.T) {
	md := &MapData{
		TerrainGrid: []TerrainType{TerrainOpenGround, TerrainOpenGround},
	}
	if NeedsLegend(md) {
		t.Error("all open ground and no effects should not need legend")
	}
}

func TestNeedsLegend_NonStandardTerrain(t *testing.T) {
	md := &MapData{
		TerrainGrid: []TerrainType{TerrainOpenGround, TerrainWater},
	}
	if !NeedsLegend(md) {
		t.Error("water terrain should trigger legend")
	}
}

func TestNeedsLegend_ActiveEffects(t *testing.T) {
	md := &MapData{
		TerrainGrid:   []TerrainType{TerrainOpenGround},
		ActiveEffects: []ActiveEffect{{Name: "Fog Cloud"}},
	}
	if !NeedsLegend(md) {
		t.Error("active effects should trigger legend")
	}
}

func TestLegendHeight_NoLegend(t *testing.T) {
	md := &MapData{
		TerrainGrid: []TerrainType{TerrainOpenGround},
	}
	h := LegendHeight(md)
	if h != 0 {
		t.Errorf("expected 0 legend height, got %d", h)
	}
}

func TestLegendHeight_WithTerrain(t *testing.T) {
	md := &MapData{
		TerrainGrid: []TerrainType{TerrainWater, TerrainLava},
	}
	h := LegendHeight(md)
	if h <= 0 {
		t.Error("legend should have positive height when terrain is non-standard")
	}
}

func TestLegendHeight_WithEffects(t *testing.T) {
	md := &MapData{
		TerrainGrid:   []TerrainType{TerrainOpenGround},
		ActiveEffects: []ActiveEffect{{Name: "Fog"}, {Name: "Fire"}},
	}
	h := LegendHeight(md)
	if h <= 0 {
		t.Error("legend should have positive height when effects are present")
	}
}

func TestDrawLegend(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		TerrainGrid: []TerrainType{
			TerrainOpenGround, TerrainWater, TerrainLava, TerrainOpenGround, TerrainPit,
		},
		ActiveEffects: []ActiveEffect{
			{Symbol: "\u2601", Name: "Fog Cloud", CasterName: "Kael", Area: "20ft radius", Rounds: 8},
		},
	}
	legendH := LegendHeight(md)
	mapW := md.Width*md.TileSize + gridLabelMargin
	dc := gg.NewContext(mapW, legendH)
	dc.SetColor(color.White)
	dc.Clear()

	DrawLegend(dc, md, 0)

	// Check some pixels are non-white (legend was drawn)
	img := dc.Image()
	foundNonWhite := false
	for y := 0; y < legendH; y++ {
		for x := 0; x < mapW; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			wr, wg, wb, _ := color.White.RGBA()
			if r != wr || g != wg || b != wb {
				foundNonWhite = true
				break
			}
		}
		if foundNonWhite {
			break
		}
	}
	if !foundNonWhite {
		t.Error("legend should render visible content")
	}
}

func TestDrawLegend_NoLegendNeeded(t *testing.T) {
	md := &MapData{
		TerrainGrid: []TerrainType{TerrainOpenGround},
	}
	dc := gg.NewContext(100, 100)
	dc.SetColor(color.White)
	dc.Clear()

	// Should not panic and should not draw anything
	DrawLegend(dc, md, 0)
}
