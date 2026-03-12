package renderer

import (
	"fmt"
	"image/color"
	"sort"

	"github.com/fogleman/gg"
)

type posKey struct {
	Col, Row int
}

// DrawTokens renders all combatant tokens on the map context.
func DrawTokens(dc *gg.Context, md *MapData) {
	if len(md.Combatants) == 0 {
		return
	}

	groups := groupByPosition(md.Combatants)
	ts := float64(md.TileSize)
	radius := ts * 0.38

	for pos, stack := range groups {
		// Sort by altitude so ground-level is first
		sort.Slice(stack, func(i, j int) bool {
			return stack[i].AltitudeFt < stack[j].AltitudeFt
		})

		for i, c := range stack {
			cx := float64(pos.Col)*ts + ts/2
			cy := float64(pos.Row)*ts + ts/2

			// Offset for stacked tokens: shift up-right by altitude order
			if i > 0 {
				offset := float64(i) * ts * 0.2
				cx += offset
				cy -= offset
			}

			tier := c.HealthTier()
			drawTokenCircle(dc, cx, cy, radius, tier)
			drawTokenLabel(dc, cx, cy, c.ShortID, tier, ts)

			// Draw altitude badge for flying tokens
			if c.AltitudeFt > 0 {
				drawAltitudeBadge(dc, cx, cy, radius, c.AltitudeFt, ts)
			}

			// Draw health tier icon overlay
			drawTierIcon(dc, cx, cy, radius, tier)
		}
	}
}

// drawTokenCircle draws the circular token with health-tier-specific styling.
func drawTokenCircle(dc *gg.Context, cx, cy, radius float64, tier HealthTier) {
	dc.SetColor(tier.TierColor())
	dc.DrawCircle(cx, cy, radius)
	dc.Fill()

	dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})

	switch tier {
	case TierUninjured:
		// Full solid border
		dc.SetLineWidth(2)
		dc.DrawCircle(cx, cy, radius)
		dc.Stroke()
	case TierScratched:
		// Mostly solid with a small gap (nick)
		dc.SetLineWidth(2)
		dc.DrawArc(cx, cy, radius, 0, 5.8)
		dc.Stroke()
	case TierBloodied:
		// Dashed border
		dc.SetLineWidth(2)
		dc.SetDash(4, 4)
		dc.DrawCircle(cx, cy, radius)
		dc.Stroke()
		dc.SetDash() // reset
	default:
		// Default solid border for others
		dc.SetLineWidth(1.5)
		dc.DrawCircle(cx, cy, radius)
		dc.Stroke()
	}
}

// drawTokenLabel renders the short ID text in the center of the token.
func drawTokenLabel(dc *gg.Context, cx, cy float64, label string, tier HealthTier, tileSize float64) {
	fontSize := max(8, tileSize*0.22)
	_ = dc.LoadFontFace("", fontSize)

	// Text color: black for yellow/light backgrounds, white for the rest
	if tier == TierScratched {
		dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})
	} else {
		dc.SetColor(color.White)
	}
	dc.DrawStringAnchored(label, cx, cy, 0.5, 0.5)
}

// drawAltitudeBadge draws an altitude indicator badge near the token.
func drawAltitudeBadge(dc *gg.Context, cx, cy, radius float64, altFt int, tileSize float64) {
	badgeText := fmt.Sprintf("\u2191%d", altFt)
	fontSize := max(7, tileSize*0.18)
	_ = dc.LoadFontFace("", fontSize)

	bx := cx + radius*0.7
	by := cy - radius*0.7

	// Badge background
	dc.SetColor(color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xCC})
	w, h := dc.MeasureString(badgeText)
	dc.DrawRoundedRectangle(bx-w/2-2, by-h/2-2, w+4, h+4, 3)
	dc.Fill()

	// Badge text
	dc.SetColor(color.White)
	dc.DrawStringAnchored(badgeText, bx, by, 0.5, 0.5)
}

// drawTierIcon draws the health tier icon overlay on the token.
func drawTierIcon(dc *gg.Context, cx, cy, radius float64, tier HealthTier) {
	iconSize := radius * 0.5
	ix := cx + radius*0.5
	iy := cy + radius*0.5

	switch tier {
	case TierCritical:
		// Warning triangle
		dc.SetColor(color.RGBA{R: 0xFF, G: 0xFF, B: 0x00, A: 0xFF})
		dc.MoveTo(ix, iy-iconSize)
		dc.LineTo(ix+iconSize*0.8, iy+iconSize*0.4)
		dc.LineTo(ix-iconSize*0.8, iy+iconSize*0.4)
		dc.ClosePath()
		dc.Fill()
		// Exclamation mark
		dc.SetColor(color.RGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF})
		dc.DrawCircle(ix, iy+iconSize*0.2, iconSize*0.12)
		dc.Fill()
		dc.DrawLine(ix, iy-iconSize*0.5, ix, iy)
		dc.SetLineWidth(1.5)
		dc.Stroke()

	case TierDying:
		// Heartbeat icon (pulse line)
		dc.SetColor(color.RGBA{R: 0xFF, G: 0x44, B: 0x44, A: 0xFF})
		dc.SetLineWidth(1.5)
		dc.MoveTo(ix-iconSize, iy)
		dc.LineTo(ix-iconSize*0.4, iy)
		dc.LineTo(ix-iconSize*0.2, iy-iconSize*0.6)
		dc.LineTo(ix+iconSize*0.2, iy+iconSize*0.4)
		dc.LineTo(ix+iconSize*0.4, iy)
		dc.LineTo(ix+iconSize, iy)
		dc.Stroke()

	case TierDead:
		// X icon
		dc.SetColor(color.RGBA{R: 0xFF, G: 0x22, B: 0x22, A: 0xFF})
		dc.SetLineWidth(2)
		dc.DrawLine(ix-iconSize*0.5, iy-iconSize*0.5, ix+iconSize*0.5, iy+iconSize*0.5)
		dc.Stroke()
		dc.DrawLine(ix+iconSize*0.5, iy-iconSize*0.5, ix-iconSize*0.5, iy+iconSize*0.5)
		dc.Stroke()

	case TierStable:
		// Cross/bandage icon
		dc.SetColor(color.RGBA{R: 0x44, G: 0xAA, B: 0xFF, A: 0xFF})
		hw := iconSize * 0.2
		dc.DrawRectangle(ix-hw, iy-iconSize*0.5, hw*2, iconSize)
		dc.Fill()
		dc.DrawRectangle(ix-iconSize*0.5, iy-hw, iconSize, hw*2)
		dc.Fill()
	}
}

// groupByPosition groups combatants by their (Col, Row) position.
func groupByPosition(combatants []Combatant) map[posKey][]Combatant {
	groups := map[posKey][]Combatant{}
	for _, c := range combatants {
		key := posKey{Col: c.Col, Row: c.Row}
		groups[key] = append(groups[key], c)
	}
	return groups
}
