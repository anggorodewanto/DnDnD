package renderer

import (
	"bytes"
	"image"
	"image/png"
	_ "image/jpeg"

	"github.com/fogleman/gg"
)

// drawBackgroundImage composites the map's background image beneath the terrain
// layer at the configured opacity. No-op when BackgroundImage is nil.
func drawBackgroundImage(dc *gg.Context, md *MapData) {
	if len(md.BackgroundImage) == 0 {
		return
	}

	img, err := png.Decode(bytes.NewReader(md.BackgroundImage))
	if err != nil {
		// Also try generic image decode for JPEG support
		img, _, err = image.Decode(bytes.NewReader(md.BackgroundImage))
		if err != nil {
			return // silently skip undecodable images
		}
	}

	opacity := md.BackgroundOpacity
	if opacity <= 0 {
		return
	}
	if opacity > 1 {
		opacity = 1
	}

	mapW := md.Width * md.TileSize
	mapH := md.Height * md.TileSize

	// Scale the background image to fill the map area
	dc.Push()
	imgBounds := img.Bounds()
	sx := float64(mapW) / float64(imgBounds.Dx())
	sy := float64(mapH) / float64(imgBounds.Dy())
	dc.Scale(sx, sy)
	dc.DrawImage(img, 0, 0)
	dc.Pop()

	// If opacity < 1, overlay a semi-transparent white rectangle to reduce
	// the background's visual weight (simulating reduced opacity).
	if opacity < 1 {
		dc.SetRGBA(1, 1, 1, 1-opacity)
		dc.DrawRectangle(0, 0, float64(mapW), float64(mapH))
		dc.Fill()
	}
}
