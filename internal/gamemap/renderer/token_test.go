package renderer

import (
	"image/color"
	"testing"

	"github.com/fogleman/gg"
)

func TestDrawTokens_BasicRendering(t *testing.T) {
	md := &MapData{
		Width:    5,
		Height:   5,
		TileSize: 48,
		Combatants: []Combatant{
			{ShortID: "G1", Col: 1, Row: 1, HPMax: 10, HPCurrent: 10},
			{ShortID: "AR", Col: 3, Row: 3, HPMax: 20, HPCurrent: 20, IsPlayer: true},
		},
	}
	dc := gg.NewContext(5*48, 5*48)
	dc.SetColor(color.White)
	dc.Clear()

	DrawTokens(dc, md)

	img := dc.Image()
	// Token at (1,1) center: x=1*48+24=72, y=1*48+24=72
	// It should not be white (token circle is drawn there)
	r, g, b, _ := img.At(72, 72).RGBA()
	wr, wg, wb, _ := color.White.RGBA()
	if r == wr && g == wg && b == wb {
		t.Error("expected token drawn at (72,72) but found white")
	}
}

func TestDrawTokens_HealthTierVisualDifference(t *testing.T) {
	// Tokens at different health tiers should look different
	md := &MapData{
		Width:    10,
		Height:   1,
		TileSize: 48,
		Combatants: []Combatant{
			{ShortID: "A", Col: 0, Row: 0, HPMax: 100, HPCurrent: 100}, // uninjured
			{ShortID: "B", Col: 2, Row: 0, HPMax: 100, HPCurrent: 50},  // bloodied
			{ShortID: "C", Col: 4, Row: 0, HPMax: 100, HPCurrent: 10},  // critical
			{ShortID: "D", Col: 6, Row: 0, HPMax: 100, HPCurrent: 0},   // dead
		},
	}
	dc := gg.NewContext(10*48, 48)
	dc.SetColor(color.White)
	dc.Clear()

	DrawTokens(dc, md)

	img := dc.Image()
	// Sample center pixel of each token
	centers := []int{24, 2*48 + 24, 4*48 + 24, 6*48 + 24}
	colors := make([]color.RGBA, len(centers))
	for i, cx := range centers {
		r, g, b, a := img.At(cx, 24).RGBA()
		colors[i] = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}

	// At least some of these should differ
	distinct := map[color.RGBA]bool{}
	for _, c := range colors {
		distinct[c] = true
	}
	if len(distinct) < 2 {
		t.Errorf("expected different colors for different health tiers, got %d distinct", len(distinct))
	}
}

func TestDrawTokens_StackedAtSamePosition(t *testing.T) {
	md := &MapData{
		Width:    3,
		Height:   3,
		TileSize: 48,
		Combatants: []Combatant{
			{ShortID: "G1", Col: 1, Row: 1, AltitudeFt: 0, HPMax: 10, HPCurrent: 10},
			{ShortID: "F1", Col: 1, Row: 1, AltitudeFt: 30, HPMax: 10, HPCurrent: 10},
			{ShortID: "F2", Col: 1, Row: 1, AltitudeFt: 60, HPMax: 10, HPCurrent: 10},
		},
	}
	dc := gg.NewContext(3*48, 3*48)
	dc.SetColor(color.White)
	dc.Clear()

	DrawTokens(dc, md)

	// Just verify no panic and something is drawn
	img := dc.Image()
	foundNonWhite := false
	for x := 48; x < 96; x++ {
		for y := 48; y < 96; y++ {
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
		t.Error("stacked tokens should render something in the tile area")
	}
}

func TestDrawTokens_Empty(t *testing.T) {
	md := &MapData{Width: 2, Height: 2, TileSize: 48}
	dc := gg.NewContext(96, 96)
	dc.SetColor(color.White)
	dc.Clear()

	DrawTokens(dc, md)

	// Should not panic, canvas stays white
	img := dc.Image()
	r, g, b, _ := img.At(48, 48).RGBA()
	wr, wg, wb, _ := color.White.RGBA()
	if r != wr || g != wg || b != wb {
		t.Error("no tokens means canvas should stay white")
	}
}

func TestGroupByPosition(t *testing.T) {
	combatants := []Combatant{
		{ShortID: "A", Col: 0, Row: 0},
		{ShortID: "B", Col: 0, Row: 0, AltitudeFt: 30},
		{ShortID: "C", Col: 1, Row: 1},
	}
	groups := groupByPosition(combatants)
	key00 := GridPos{Col: 0, Row: 0}
	key11 := GridPos{Col: 1, Row: 1}

	if len(groups[key00]) != 2 {
		t.Errorf("expected 2 at (0,0), got %d", len(groups[key00]))
	}
	if len(groups[key11]) != 1 {
		t.Errorf("expected 1 at (1,1), got %d", len(groups[key11]))
	}
}
