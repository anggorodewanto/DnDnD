package renderer

import (
	"fmt"
	"image/color"

	"github.com/fogleman/gg"
)

const gridLabelMargin = 28

// ColumnLabel converts a zero-based column index to a spreadsheet-style label.
// 0 -> "A", 25 -> "Z", 26 -> "AA", 27 -> "AB", etc.
func ColumnLabel(col int) string {
	label := ""
	for {
		label = string(rune('A'+col%26)) + label
		col = col/26 - 1
		if col < 0 {
			break
		}
	}
	return label
}

// DrawGrid renders grid lines on the map.
func DrawGrid(dc *gg.Context, md *MapData) {
	ts := float64(md.TileSize)
	w := float64(md.Width) * ts
	h := float64(md.Height) * ts

	dc.SetColor(color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xAA})
	dc.SetLineWidth(0.5)

	// Vertical lines
	for col := 0; col <= md.Width; col++ {
		x := float64(col) * ts
		dc.DrawLine(x, 0, x, h)
		dc.Stroke()
	}
	// Horizontal lines
	for row := 0; row <= md.Height; row++ {
		y := float64(row) * ts
		dc.DrawLine(0, y, w, y)
		dc.Stroke()
	}
}

// DrawCoordinateLabels renders column (A, B, ...) and row (1, 2, ...) labels
// in the margin area. The drawing context should be sized to include the margin.
func DrawCoordinateLabels(dc *gg.Context, md *MapData) {
	ts := float64(md.TileSize)
	fontSize := max(8, min(14, float64(md.TileSize)*0.25))
	dc.SetColor(color.RGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF})

	_ = dc.LoadFontFace("", fontSize)

	margin := float64(gridLabelMargin)

	// Column labels along the top
	for col := 0; col < md.Width; col++ {
		label := ColumnLabel(col)
		x := margin + float64(col)*ts + ts/2
		y := margin * 0.6
		dc.DrawStringAnchored(label, x, y, 0.5, 0.5)
	}

	// Row labels along the left
	for row := 0; row < md.Height; row++ {
		label := fmt.Sprintf("%d", row+1)
		x := margin * 0.5
		y := margin + float64(row)*ts + ts/2
		dc.DrawStringAnchored(label, x, y, 0.5, 0.5)
	}
}
