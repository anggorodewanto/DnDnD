package renderer

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"github.com/fogleman/gg"
)

const gridLabelMargin = 28

// ParseCoordinate parses a grid coordinate string like "D4" or "AA12" into
// a 0-based column and 0-based row. Column letters are case-insensitive.
// Row numbers are 1-based in the input (D4 = col 3, row 3 in 0-based).
func ParseCoordinate(s string) (col, row int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("empty coordinate")
	}

	// Find boundary between letters and digits
	i := 0
	for i < len(s) && unicode.IsLetter(rune(s[i])) {
		i++
	}
	if i == 0 {
		return 0, 0, fmt.Errorf("coordinate must start with column letters: %q", s)
	}
	if i == len(s) {
		return 0, 0, fmt.Errorf("coordinate must end with row number: %q", s)
	}

	colPart := strings.ToUpper(s[:i])
	rowPart := s[i:]

	// Parse column: A=0, Z=25, AA=26, AB=27, etc.
	col = 0
	for _, ch := range colPart {
		if ch < 'A' || ch > 'Z' {
			return 0, 0, fmt.Errorf("invalid column letter in %q", s)
		}
		col = col*26 + int(ch-'A') + 1
	}
	col-- // convert from 1-based to 0-based

	// Parse row: must be positive integer
	rowNum := 0
	for _, ch := range rowPart {
		if ch < '0' || ch > '9' {
			return 0, 0, fmt.Errorf("invalid row number in %q", s)
		}
		rowNum = rowNum*10 + int(ch-'0')
	}
	if rowNum <= 0 {
		return 0, 0, fmt.Errorf("row must be >= 1, got %d in %q", rowNum, s)
	}

	return col, rowNum - 1, nil
}

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
