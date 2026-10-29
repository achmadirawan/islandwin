// Package render provides a software canvas for drawing the IslandWin overlay.
// All drawing happens on image.RGBA buffers; no GPU context required.
package render

import (
	"image"
	"image/color"
	"math"
	"os"
	"sync"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// Dimensions for each UI state (logical pixels, before DPI scaling).
const (
	CanvasW    = 440
	CanvasH    = 280
	CollapsedW = 120
	CollapsedH = 32
	// Collapsed widths when the "Maghrib - 17:49 (00:35)" label is shown (wider while music plays: disc + equalizer).
	CollapsedPrayerW  = 205
	CollapsedPlayingW = 250
	ExpandedMusicW    = 420
	ExpandedMusicH    = 140
	ExpandedTodoW     = 420
	ExpandedTodoH     = 260
	ExpandedPrayerH   = 210
	CornerRadius      = 16.0
)

var (
	fontOnce    sync.Once
	fontRegular font.Face
	fontBold    font.Face
	fontTitle   font.Face
	fontSmall   font.Face
	fontPill    font.Face
)

// SetFontPill switches to the larger small-font used for the collapsed pill label.
func (c *Canvas) SetFontPill() {
	if fontPill != nil {
		c.dc.SetFontFace(fontPill)
	}
}

func initFonts() {
	fontOnce.Do(func() {
		// Segoe UI font files on Windows
		regularData, err := os.ReadFile("C:\\Windows\\Fonts\\segoeui.ttf")
		if err == nil {
			if ft, err := truetype.Parse(regularData); err == nil {
				fontRegular = truetype.NewFace(ft, &truetype.Options{Size: 11, DPI: 72, Hinting: font.HintingFull})
				fontSmall = truetype.NewFace(ft, &truetype.Options{Size: 9.5, DPI: 72, Hinting: font.HintingFull})
				// ponytail: DPI 96 hanya untuk label pill (~33% lebih besar); font lain tetap supaya layout panel tidak bergeser.
				fontPill = truetype.NewFace(ft, &truetype.Options{Size: 9.5, DPI: 96, Hinting: font.HintingFull})
			}
		}

		boldData, err := os.ReadFile("C:\\Windows\\Fonts\\segoeuib.ttf")
		if err == nil {
			if ft, err := truetype.Parse(boldData); err == nil {
				fontBold = truetype.NewFace(ft, &truetype.Options{Size: 11, DPI: 72, Hinting: font.HintingFull})
				fontTitle = truetype.NewFace(ft, &truetype.Options{Size: 13, DPI: 72, Hinting: font.HintingFull})
			}
		}
	})
}

// Canvas wraps a gg.Context and reuses a single image.RGBA buffer.
// Never allocate a new buffer per frame — resize only when dimensions change.
type Canvas struct {
	img *image.RGBA
	dc  *gg.Context
	W   int
	H   int
}

// New creates a Canvas of the given size.
func New(w, h int) *Canvas {
	initFonts()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := &Canvas{img: img, dc: gg.NewContextForRGBA(img), W: w, H: h}
	c.SetFontRegular()
	return c
}

// Resize reallocates only when dimensions actually change.
func (c *Canvas) Resize(w, h int) {
	if c.W == w && c.H == h {
		return
	}
	c.img = image.NewRGBA(image.Rect(0, 0, w, h))
	c.dc = gg.NewContextForRGBA(c.img)
	c.W = w
	c.H = h
	c.SetFontRegular()
}

// SetFontRegular switches the context font to smooth 11pt regular Segoe UI.
func (c *Canvas) SetFontRegular() {
	if fontRegular != nil {
		c.dc.SetFontFace(fontRegular)
	}
}

// SetFontBold switches the context font to smooth 11pt bold Segoe UI.
func (c *Canvas) SetFontBold() {
	if fontBold != nil {
		c.dc.SetFontFace(fontBold)
	}
}

// SetFontTitle switches the context font to smooth 13pt bold Segoe UI.
func (c *Canvas) SetFontTitle() {
	if fontTitle != nil {
		c.dc.SetFontFace(fontTitle)
	}
}

// SetFontSmall switches the context font to smooth 9.5pt regular Segoe UI.
func (c *Canvas) SetFontSmall() {
	if fontSmall != nil {
		c.dc.SetFontFace(fontSmall)
	}
}

// Clear fills the canvas with fully transparent black.
func (c *Canvas) Clear() {
	c.dc.SetColor(color.Transparent)
	c.dc.Clear()
}

// FadeColor scales the alpha of an RGBA color by progress in [0, 1].
func FadeColor(c color.RGBA, progress float64) color.RGBA {
	if progress <= 0 {
		return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0}
	}
	if progress >= 1 {
		return c
	}
	a := float64(c.A) * progress
	if a > 255 {
		a = 255
	}
	return color.RGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: uint8(a),
	}
}

// DrawNotchPath builds a closed path where the top edge is flat at y=0,
// and the bottom corners are rounded with radius r.
func DrawNotchPath(dc *gg.Context, x, y, w, h, r float64) {
	if r > h {
		r = h
	}
	if r > w/2 {
		r = w / 2
	}

	x0, x1, x2, x3 := x, x+r, x+w-r, x+w
	y0, y2, y3 := y, y+h-r, y+h

	dc.NewSubPath()
	dc.MoveTo(x0, y0)
	dc.LineTo(x3, y0)
	dc.LineTo(x3, y2)
	dc.DrawArc(x2, y2, r, gg.Radians(0), gg.Radians(90))
	dc.LineTo(x1, y3)
	dc.DrawArc(x1, y2, r, gg.Radians(90), gg.Radians(180))
	dc.LineTo(x0, y0)
	dc.ClosePath()
}

// StrokeNotchBorder strokes only the left edge, bottom corners, and right edge.
// It leaves the top at y=0 completely open so the notch merges seamlessly into the display bezel.
func StrokeNotchBorder(dc *gg.Context, x, y, w, h, r float64) {
	if r > h {
		r = h
	}
	if r > w/2 {
		r = w / 2
	}

	x0, x1, x2, x3 := x, x+r, x+w-r, x+w
	y0, y2, y3 := y, y+h-r, y+h

	dc.NewSubPath()
	dc.MoveTo(x3, y0)
	dc.LineTo(x3, y2)
	dc.DrawArc(x2, y2, r, gg.Radians(0), gg.Radians(90))
	dc.LineTo(x1, y3)
	dc.DrawArc(x1, y2, r, gg.Radians(90), gg.Radians(180))
	dc.LineTo(x0, y0)
	dc.Stroke()
}

// DrawPill renders a notch attached to the top screen bezel with rounded bottom corners.
// w and h are the notch dimensions (may be smaller than canvas during animation).
func (c *Canvas) DrawPill(w, h float64) {
	// Centre the notch within the canvas horizontally, anchor flush to top y=0.
	x := (float64(c.W) - w) / 2

	radius := CornerRadius
	if h > 40 {
		radius = 22.0
	} else {
		radius = 16.0
	}
	if radius > h {
		radius = h
	}

	// 1. Soft multi-layer ambient drop shadow underneath bottom and sides
	shadows := []struct {
		expand float64
		yOff   float64
		alpha  uint8
	}{
		{expand: 4.0, yOff: 4.0, alpha: 12},
		{expand: 2.5, yOff: 2.5, alpha: 22},
		{expand: 1.0, yOff: 1.2, alpha: 36},
	}

	for _, s := range shadows {
		c.dc.SetColor(color.RGBA{R: 0, G: 0, B: 0, A: s.alpha})
		DrawNotchPath(c.dc, x-s.expand, -3.0, w+s.expand*2, h+s.yOff+3.0, radius+s.expand)
		c.dc.Fill()
	}

	// 2. Main notch fill — dark vertical gradient (top #14141A -> bottom pure black).
	// Fill starts at y=-3.0 to ensure zero subpixel anti-aliasing gap at the monitor top bezel.
	// No light rim stroke: edges stay pure dark so corners never show a white outline.
	grad := gg.NewLinearGradient(0, 0, 0, h)
	grad.AddColorStop(0, color.RGBA{R: 20, G: 20, B: 26, A: 255})
	grad.AddColorStop(1, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	c.dc.SetFillStyle(grad)
	DrawNotchPath(c.dc, x, -3.0, w, h+3.0, radius)
	c.dc.Fill()
}

// DrawCollapsedIndicator renders subtle mini indicators when collapsed (PRD F-ISL-07).
func (c *Canvas) DrawCollapsedIndicator(pillW, pillH float64, isPlaying bool, tick uint64, baseW float64) {
	if pillW > baseW+80 {
		return // Only visible when collapsed or near collapsed
	}

	// Smoothly fade out as island expands
	alpha := 1.0
	if pillW > baseW+15 {
		alpha = (baseW + 80.0 - pillW) / 65.0
	}
	if alpha <= 0.05 {
		return
	}

	x := (float64(c.W) - pillW) / 2
	centerY := pillH / 2

	fade := func(base color.RGBA) color.RGBA {
		return FadeColor(base, alpha)
	}

	if isPlaying {
		// Mini vinyl disc indicator on left
		discX := x + 20.0
		discR := 5.5
		c.dc.SetColor(fade(color.RGBA{R: 35, G: 35, B: 42, A: 255}))
		c.dc.DrawCircle(discX, centerY, discR)
		c.dc.Fill()

		c.dc.SetColor(fade(color.RGBA{R: 59, G: 130, B: 246, A: 255})) // Accent blue hub
		c.dc.DrawCircle(discX, centerY, 2.0)
		c.dc.Fill()

		// 3 mini animated equalizer soundwave bars on right
		eqX := x + pillW - 28.0
		barW := 2.2
		barGap := 2.0
		t := float64(tick) * 0.18

		h1 := 4.5 + 3.0*math.Sin(t)
		h2 := 6.5 + 4.0*math.Cos(t*1.2)
		h3 := 4.0 + 2.5*math.Sin(t*0.8+1.5)

		c.dc.SetColor(fade(color.RGBA{R: 34, G: 197, B: 94, A: 255})) // Apple Music emerald green
		c.dc.DrawRoundedRectangle(eqX, centerY-h1/2, barW, h1, 1.0)
		c.dc.DrawRoundedRectangle(eqX+barW+barGap, centerY-h2/2, barW, h2, 1.0)
		c.dc.DrawRoundedRectangle(eqX+(barW+barGap)*2, centerY-h3/2, barW, h3, 1.0)
		c.dc.Fill()
	}
}

// DrawMusicContent renders basic music panel UI elements when sufficiently expanded.
func (c *Canvas) DrawMusicContent(w, h float64, title, artist string) {
	if w < 250 || h < 80 {
		return // Still animating in
	}
	x := (float64(c.W) - w) / 2

	// Placeholder album art rect (gray rounded rect on left)
	artSize := 64.0
	artX := x + 16.0
	artY := 16.0
	c.dc.SetColor(color.RGBA{R: 40, G: 40, B: 40, A: 255})
	c.dc.DrawRoundedRectangle(artX, artY, artSize, artSize, 8)
	c.dc.Fill()

	// Text: Title
	c.dc.SetColor(color.RGBA{R: 240, G: 240, B: 240, A: 255})
	c.dc.DrawString(title, artX+artSize+14, artY+20)

	// Text: Artist
	c.dc.SetColor(color.RGBA{R: 150, G: 150, B: 150, A: 255})
	c.dc.DrawString(artist, artX+artSize+14, artY+40)

	// Bottom progress bar track
	barX := x + 16.0
	barY := h - 24.0
	barW := w - 32.0
	c.dc.SetColor(color.RGBA{R: 50, G: 50, B: 50, A: 255})
	c.dc.DrawRoundedRectangle(barX, barY, barW, 4, 2)
	c.dc.Fill()
}

// DrawTodoContent renders basic todo panel UI elements when sufficiently expanded.
func (c *Canvas) DrawTodoContent(w, h float64, dateStr string) {
	if w < 250 || h < 100 {
		return // Still animating in
	}
	x := (float64(c.W) - w) / 2

	// Header date
	c.dc.SetColor(color.RGBA{R: 240, G: 240, B: 240, A: 255})
	c.dc.DrawString("To-Do Hari Ini", x+20, 28)

	c.dc.SetColor(color.RGBA{R: 140, G: 140, B: 140, A: 255})
	c.dc.DrawString(dateStr, x+20, 48)

	// Placeholder hint
	c.dc.SetColor(color.RGBA{R: 80, G: 80, B: 80, A: 255})
	c.dc.DrawString("+ Tambah tugas baru...", x+20, 85)
}

// BGRA returns the canvas pixels converted to BGRA premultiplied-alpha format
// required by UpdateLayeredWindow / BLENDFUNCTION.
func (c *Canvas) BGRA() []byte {
	src := c.img.Pix // RGBA layout
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += 4 {
		a := src[i+3]
		// Premultiply and swap R↔B for BGRA.
		dst[i+0] = mulAlpha(src[i+2], a) // B
		dst[i+1] = mulAlpha(src[i+1], a) // G
		dst[i+2] = mulAlpha(src[i+0], a) // R
		dst[i+3] = a
	}
	return dst
}

// Image returns the underlying image (for direct use if needed).
func (c *Canvas) Image() *image.RGBA { return c.img }

func mulAlpha(c, a byte) byte {
	return byte(uint16(c) * uint16(a) / 255)
}
