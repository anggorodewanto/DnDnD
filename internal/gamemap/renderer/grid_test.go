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

func TestParseCoordinate(t *testing.T) {
	tests := []struct {
		input   string
		wantCol int
		wantRow int
		wantErr bool
	}{
		{"A1", 0, 0, false},
		{"D4", 3, 3, false},
		{"Z1", 25, 0, false},
		{"AA1", 26, 0, false},
		{"AB12", 27, 11, false},
		{"AZ1", 51, 0, false},
		{"BA1", 52, 0, false},
		{"a1", 0, 0, false},    // case insensitive
		{"d4", 3, 3, false},    // case insensitive
		{"aa12", 26, 11, false}, // case insensitive
		{"", 0, 0, true},       // empty
		{"1", 0, 0, true},      // no column
		{"A", 0, 0, true},      // no row
		{"A0", 0, 0, true},     // row 0 invalid (1-based)
		{"1A", 0, 0, true},     // wrong order
		{"A-1", 0, 0, true},    // negative row
		{"A1B", 0, 0, true},    // trailing letters
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			col, row, err := ParseCoordinate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCoordinate(%q) expected error, got col=%d, row=%d", tt.input, col, row)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseCoordinate(%q) unexpected error: %v", tt.input, err)
				return
			}
			if col != tt.wantCol || row != tt.wantRow {
				t.Errorf("ParseCoordinate(%q) = (%d, %d), want (%d, %d)", tt.input, col, row, tt.wantCol, tt.wantRow)
			}
		})
	}
}

func TestParseCoordinate_RoundTrip(t *testing.T) {
	// Verify ParseCoordinate is the inverse of ColumnLabel
	for col := 0; col <= 77; col++ {
		label := ColumnLabel(col)
		coord := label + "1"
		gotCol, gotRow, err := ParseCoordinate(coord)
		if err != nil {
			t.Errorf("round-trip failed for col %d (label %q): %v", col, label, err)
			continue
		}
		if gotCol != col || gotRow != 0 {
			t.Errorf("round-trip col %d: ParseCoordinate(%q) = (%d, %d), want (%d, 0)", col, coord, gotCol, gotRow, col)
		}
	}
}
