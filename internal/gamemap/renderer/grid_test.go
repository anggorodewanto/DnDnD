package renderer

import (
	"image/color"
	"testing"

	"github.com/fogleman/gg"
)

func TestColumnLabel(t *testing.T) {
	tests := []struct {
		col  int
		want string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{77, "BZ"},
		{701, "ZZ"},
	}
	for _, tc := range tests {
		got := ColumnLabel(tc.col)
		if got != tc.want {
			t.Errorf("ColumnLabel(%d) = %q, want %q", tc.col, got, tc.want)
		}
	}
}

func TestDrawGrid_LinesPresent(t *testing.T) {
	md := &MapData{Width: 3, Height: 3, TileSize: 48}
	dc := gg.NewContext(3*48, 3*48)
	// Fill white background
	dc.SetColor(color.White)
	dc.Clear()

	DrawGrid(dc, md)

	img := dc.Image()
	// Grid lines should be drawn at tile boundaries.
	// Check that the pixel at a grid line (x=48, y=24) is not white.
	r, g, b, _ := img.At(48, 24).RGBA()
	white := color.White
	wr, wg, wb, _ := white.RGBA()
	if r == wr && g == wg && b == wb {
		t.Error("expected grid line at x=48 but found white")
	}
}

func TestDrawCoordinateLabels(t *testing.T) {
	md := &MapData{Width: 3, Height: 3, TileSize: 48}
	totalW := 3*48 + gridLabelMargin
	totalH := 3*48 + gridLabelMargin
	dc := gg.NewContext(totalW, totalH)
	dc.SetColor(color.White)
	dc.Clear()

	DrawCoordinateLabels(dc, md)

	// Hard to verify text rendering precisely, but we verify no panic
	// and that some pixels in the label margin area are non-white.
	img := dc.Image()
	foundNonWhite := false
	for x := 0; x < gridLabelMargin; x++ {
		for y := 0; y < totalH; y++ {
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
		t.Error("expected coordinate labels to render some pixels in the margin area")
	}
}

func TestGridLabelMarginConstant(t *testing.T) {
	if gridLabelMargin < 20 {
		t.Errorf("gridLabelMargin should be >= 20, got %d", gridLabelMargin)
	}
}
