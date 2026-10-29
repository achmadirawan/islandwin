package render

import (
	"fmt"
	"image/color"
)

const (
	// PlaylistH is the island height while the playlist view is open.
	PlaylistH = 260.0
	// PlaylistRows is how many tracks are visible at once.
	PlaylistRows = 7

	playlistRowH = 28.0
)

// PlaylistItem is one visible row.
type PlaylistItem struct {
	Title, Artist string
}

// PlaylistUIRects holds hit-test boxes for the playlist view.
type PlaylistUIRects struct {
	Back, Up, Down Rect
	Rows           [PlaylistRows]Rect
}

// GetPlaylistUIRects calculates playlist view hit boxes.
func GetPlaylistUIRects(canvasW, pillW float64) PlaylistUIRects {
	x := (canvasW - pillW) / 2
	tabX := x + pillW - 66.0 - 16.0
	r := PlaylistUIRects{
		Back: Rect{X: tabX, Y: 14, W: 66, H: 22},
		Down: Rect{X: tabX - 30, Y: 14, W: 24, H: 22},
		Up:   Rect{X: tabX - 58, Y: 14, W: 24, H: 22},
	}
	for i := range r.Rows {
		r.Rows[i] = Rect{X: x + 14, Y: 46 + float64(i)*playlistRowH, W: pillW - 28, H: playlistRowH - 2}
	}
	return r
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// DrawPlaylist renders the track list. items[i] is track number first+i; cur is the playing index (-1 if none).
func (c *Canvas) DrawPlaylist(pillW, pillH float64, items []PlaylistItem, first, total, cur int) {
	alpha := (pillH - 120.0) / 100.0
	if alpha > 1 {
		alpha = 1
	}
	if alpha < 0.05 {
		return
	}
	fade := func(b color.RGBA) color.RGBA { return FadeColor(b, alpha) }

	rects := GetPlaylistUIRects(float64(c.W), pillW)
	x := (float64(c.W) - pillW) / 2

	// Header
	c.SetFontTitle()
	c.dc.SetColor(fade(color.RGBA{R: 250, G: 250, B: 252, A: 255}))
	c.dc.DrawString(fmt.Sprintf("Playlist (%d)", total), x+18, 31)

	btn := func(r Rect) {
		c.dc.SetColor(fade(color.RGBA{R: 36, G: 36, B: 42, A: 255}))
		c.dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, 11)
		c.dc.Fill()
	}
	btn(rects.Back)
	btn(rects.Up)
	btn(rects.Down)

	c.SetFontSmall()
	c.dc.SetColor(fade(color.RGBA{R: 210, G: 210, B: 220, A: 255}))
	c.dc.DrawString("‹ Player", rects.Back.X+11, rects.Back.Y+15)

	// Up / down triangles
	ux, uy := rects.Up.X+rects.Up.W/2, rects.Up.Y+rects.Up.H/2
	c.dc.MoveTo(ux-4, uy+2.5)
	c.dc.LineTo(ux+4, uy+2.5)
	c.dc.LineTo(ux, uy-3)
	c.dc.ClosePath()
	c.dc.Fill()
	dx, dy := rects.Down.X+rects.Down.W/2, rects.Down.Y+rects.Down.H/2
	c.dc.MoveTo(dx-4, dy-2.5)
	c.dc.LineTo(dx+4, dy-2.5)
	c.dc.LineTo(dx, dy+3)
	c.dc.ClosePath()
	c.dc.Fill()

	if total == 0 {
		c.SetFontRegular()
		c.dc.SetColor(fade(color.RGBA{R: 140, G: 140, B: 150, A: 255}))
		c.dc.DrawString("Belum ada lagu. Pilih folder musik dulu.", x+18, 70)
		return
	}

	for i, it := range items {
		row := rects.Rows[i]
		if row.Y+row.H > pillH-10 { // still growing: don't draw outside the notch
			break
		}
		idx := first + i
		isCur := idx == cur

		titleCol := color.RGBA{R: 235, G: 235, B: 242, A: 255}
		if isCur {
			c.dc.SetColor(fade(color.RGBA{R: 30, G: 30, B: 38, A: 255}))
			c.dc.DrawRoundedRectangle(row.X, row.Y, row.W, row.H, 8)
			c.dc.Fill()
			titleCol = color.RGBA{R: 74, G: 222, B: 128, A: 255}

			// Now-playing marker
			c.dc.SetColor(fade(titleCol))
			c.dc.MoveTo(row.X+10, row.Y+8)
			c.dc.LineTo(row.X+10, row.Y+row.H-8)
			c.dc.LineTo(row.X+18, row.Y+row.H/2)
			c.dc.ClosePath()
			c.dc.Fill()
		} else {
			c.SetFontSmall()
			c.dc.SetColor(fade(color.RGBA{R: 110, G: 110, B: 120, A: 255}))
			c.dc.DrawString(fmt.Sprintf("%d", idx+1), row.X+8, row.Y+17)
		}

		c.SetFontRegular()
		c.dc.SetColor(fade(titleCol))
		c.dc.DrawString(truncRunes(it.Title, 26), row.X+28, row.Y+17)

		c.SetFontSmall()
		c.dc.SetColor(fade(color.RGBA{R: 130, G: 130, B: 140, A: 255}))
		art := truncRunes(it.Artist, 14)
		aw, _ := c.dc.MeasureString(art)
		c.dc.DrawString(art, row.X+row.W-aw-10, row.Y+17)
	}
}
