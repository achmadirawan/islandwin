package render

import (
	"fmt"
	"image/color"
	"math"
)

// MusicUIRects holds bounding boxes for clickable elements in the music panel.
type MusicUIRects struct {
	PlayPause    Rect
	Next         Rect
	Prev         Rect
	ProgressBar  Rect
	VolumeBar    Rect
	SwitchTab    Rect
	SelectFolder Rect
	AlbumArt     Rect
	Playlist     Rect
}

// Rect defines a simple 2D bounding box.
type Rect struct {
	X, Y, W, H float64
}

// Contains returns true if (px, py) falls within the rectangle.
func (r Rect) Contains(px, py float64) bool {
	return px >= r.X && px <= r.X+r.W && py >= r.Y && py <= r.Y+r.H
}

// FormatTime formats seconds into "m:ss" format.
func FormatTime(sec float64) string {
	if sec < 0 || math.IsNaN(sec) {
		sec = 0
	}
	s := int(sec)
	m := s / 60
	s = s % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// GetMusicUIRects calculates the interactive element bounds for current canvas dimensions.
func GetMusicUIRects(canvasW, pillW, pillH float64) MusicUIRects {
	x := (canvasW - pillW) / 2

	// Album Art
	artSize := 56.0
	artX := x + 18.0
	artY := 16.0

	// Progress bar
	barX := x + 18.0
	barY := pillH - 46.0
	barW := pillW - 36.0
	barH := 8.0

	// Controls centered horizontally below progress bar
	ctrlCenterY := pillH - 22.0

	playW := 32.0
	playH := 32.0
	playX := x + (pillW / 2) - (playW / 2)
	playY := ctrlCenterY - (playH / 2)

	prevW := 28.0
	prevH := 28.0
	prevX := playX - 44.0
	prevY := ctrlCenterY - (prevH / 2)

	nextW := 28.0
	nextH := 28.0
	nextX := playX + playW + 16.0
	nextY := ctrlCenterY - (nextH / 2)

	// Volume bar on bottom right
	volW := 65.0
	volH := 8.0
	volX := x + pillW - volW - 18.0
	volY := ctrlCenterY - (volH / 2)

	// Switch tab button in top right
	tabW := 66.0
	tabH := 22.0
	tabX := x + pillW - tabW - 16.0
	tabY := 14.0

	// Select folder button (shown when empty or clickable on art)
	folderBtn := Rect{X: artX + artSize + 14.0, Y: artY + 28.0, W: 138.0, H: 22.0}

	return MusicUIRects{
		AlbumArt:     Rect{X: artX, Y: artY, W: artSize, H: artSize},
		ProgressBar:  Rect{X: barX, Y: barY, W: barW, H: barH},
		Prev:         Rect{X: prevX, Y: prevY, W: prevW, H: prevH},
		PlayPause:    Rect{X: playX, Y: playY, W: playW, H: playH},
		Next:         Rect{X: nextX, Y: nextY, W: nextW, H: nextH},
		VolumeBar:    Rect{X: volX, Y: volY, W: volW, H: volH},
		SwitchTab:    Rect{X: tabX, Y: tabY, W: tabW, H: tabH},
		Playlist:     Rect{X: tabX - 32, Y: tabY, W: 26, H: tabH},
		SelectFolder: folderBtn,
	}
}

// DrawMusicPlayer renders detailed music UI elements onto the canvas.
func (c *Canvas) DrawMusicPlayer(pillW, pillH float64, title, artist string, isPlaying bool, posSec, durSec, vol float64, hasTracks bool) {
	if pillW < 180 || pillH < 65 {
		return // Still expanding
	}

	// Smooth alpha transition: content dissolves in gracefully as island expands
	alphaProgress := (pillW - 180.0) / (ExpandedMusicW - 220.0)
	if alphaProgress > 1.0 {
		alphaProgress = 1.0
	}
	if alphaProgress < 0.05 {
		return
	}

	fade := func(base color.RGBA) color.RGBA {
		return FadeColor(base, alphaProgress)
	}

	rects := GetMusicUIRects(float64(c.W), pillW, pillH)

	// 1. Album art thumbnail with vector vinyl record
	art := rects.AlbumArt
	c.dc.SetColor(fade(color.RGBA{R: 22, G: 22, B: 26, A: 255}))
	c.dc.DrawRoundedRectangle(art.X, art.Y, art.W, art.H, 8)
	c.dc.Fill()

	// Vector vinyl record inside art
	cx := art.X + art.W/2
	cy := art.Y + art.H/2
	c.dc.SetColor(fade(color.RGBA{R: 44, G: 44, B: 52, A: 255}))
	c.dc.DrawCircle(cx, cy, 18)
	c.dc.Fill()

	c.dc.SetColor(fade(color.RGBA{R: 28, G: 28, B: 34, A: 255}))
	c.dc.DrawCircle(cx, cy, 12)
	c.dc.Fill()

	c.dc.SetColor(fade(color.RGBA{R: 225, G: 70, B: 70, A: 255})) // Red record label
	c.dc.DrawCircle(cx, cy, 6)
	c.dc.Fill()

	c.dc.SetColor(fade(color.RGBA{R: 12, G: 12, B: 15, A: 255})) // Spindle hole
	c.dc.DrawCircle(cx, cy, 2)
	c.dc.Fill()

	// 2. Track Title & Artist (Smooth Segoe UI)
	textX := art.X + art.W + 14.0
	maxTextLen := 26
	if len(title) > maxTextLen {
		title = title[:maxTextLen-3] + "..."
	}
	if len(artist) > maxTextLen {
		artist = artist[:maxTextLen-3] + "..."
	}

	c.SetFontTitle()
	c.dc.SetColor(fade(color.RGBA{R: 250, G: 250, B: 252, A: 255}))
	c.dc.DrawString(title, textX, art.Y+20)

	if hasTracks {
		c.SetFontRegular()
		c.dc.SetColor(fade(color.RGBA{R: 150, G: 150, B: 160, A: 255}))
		c.dc.DrawString(artist, textX, art.Y+38)
	} else {
		// Clickable "+ Pilih Folder Musik" button
		fBtn := rects.SelectFolder
		c.dc.SetColor(fade(color.RGBA{R: 35, G: 70, B: 130, A: 255}))
		c.dc.DrawRoundedRectangle(fBtn.X, fBtn.Y, fBtn.W, fBtn.H, 5)
		c.dc.Fill()

		c.SetFontSmall()
		c.dc.SetColor(fade(color.RGBA{R: 220, G: 235, B: 255, A: 255}))
		c.dc.DrawString("+ Pilih Folder Musik", fBtn.X+12, fBtn.Y+15)
	}

	// 3. Switch Tab button (top right)
	c.dc.SetColor(fade(color.RGBA{R: 36, G: 36, B: 42, A: 255}))
	c.dc.DrawRoundedRectangle(rects.SwitchTab.X, rects.SwitchTab.Y, rects.SwitchTab.W, rects.SwitchTab.H, 11)
	c.dc.Fill()
	c.SetFontSmall()
	c.dc.SetColor(fade(color.RGBA{R: 210, G: 210, B: 220, A: 255}))
	c.dc.DrawString("To-Do ›", rects.SwitchTab.X+12, rects.SwitchTab.Y+15)

	// Playlist button (three lines icon)
	pl := rects.Playlist
	c.dc.SetColor(fade(color.RGBA{R: 36, G: 36, B: 42, A: 255}))
	c.dc.DrawRoundedRectangle(pl.X, pl.Y, pl.W, pl.H, 11)
	c.dc.Fill()
	c.dc.SetColor(fade(color.RGBA{R: 210, G: 210, B: 220, A: 255}))
	for i := 0; i < 3; i++ {
		c.dc.DrawRoundedRectangle(pl.X+7, pl.Y+6.5+float64(i)*4, 12, 1.6, 0.8)
	}
	c.dc.Fill()

	// 4. Progress bar track
	pb := rects.ProgressBar
	c.dc.SetColor(fade(color.RGBA{R: 44, G: 44, B: 50, A: 255}))
	c.dc.DrawRoundedRectangle(pb.X, pb.Y+2, pb.W, 4, 2)
	c.dc.Fill()

	// Progress fill
	progressRatio := 0.0
	if durSec > 0 {
		progressRatio = posSec / durSec
		if progressRatio > 1 {
			progressRatio = 1
		}
	}
	fillW := pb.W * progressRatio
	if fillW > 0 {
		c.dc.SetColor(fade(color.RGBA{R: 245, G: 245, B: 250, A: 255}))
		c.dc.DrawRoundedRectangle(pb.X, pb.Y+2, fillW, 4, 2)
		c.dc.Fill()

		// Playhead circular handle
		c.dc.SetColor(fade(color.RGBA{R: 255, G: 255, B: 255, A: 255}))
		c.dc.DrawCircle(pb.X+fillW, pb.Y+4, 4)
		c.dc.Fill()
	}

	// Time strings (smooth Segoe UI)
	c.SetFontSmall()
	c.dc.SetColor(fade(color.RGBA{R: 120, G: 120, B: 130, A: 255}))
	c.dc.DrawString(FormatTime(posSec), pb.X, pb.Y-3)

	durStr := FormatTime(durSec)
	durW, _ := c.dc.MeasureString(durStr)
	c.dc.DrawString(durStr, pb.X+pb.W-durW, pb.Y-3)

	// 5. Controls: Prev, Play/Pause, Next (Vector Geometries)
	prevCenter := rects.Prev.X + rects.Prev.W/2
	ctrlY := rects.Prev.Y + rects.Prev.H/2

	c.dc.SetColor(fade(color.RGBA{R: 200, G: 200, B: 210, A: 255}))
	// Prev vertical bar
	c.dc.DrawRoundedRectangle(prevCenter-7.0, ctrlY-5.0, 2.0, 10.0, 0.8)
	c.dc.Fill()
	// Prev left triangle
	c.dc.MoveTo(prevCenter-5.0, ctrlY)
	c.dc.LineTo(prevCenter+4.5, ctrlY-5.5)
	c.dc.LineTo(prevCenter+4.5, ctrlY+5.5)
	c.dc.ClosePath()
	c.dc.Fill()

	// --- Play / Pause Button (White Circle Button) ---
	ppCenter := rects.PlayPause.X + rects.PlayPause.W/2
	ppY := rects.PlayPause.Y + rects.PlayPause.H/2

	c.dc.SetColor(fade(color.RGBA{R: 245, G: 245, B: 250, A: 255}))
	c.dc.DrawCircle(ppCenter, ppY, 15)
	c.dc.Fill()

	if isPlaying {
		// Pause icon: two dark vertical bars
		c.dc.SetColor(fade(color.RGBA{R: 15, G: 15, B: 18, A: 255}))
		c.dc.DrawRoundedRectangle(ppCenter-4.5, ppY-5.5, 3.2, 11.0, 1.0)
		c.dc.DrawRoundedRectangle(ppCenter+1.3, ppY-5.5, 3.2, 11.0, 1.0)
		c.dc.Fill()
	} else {
		// Play icon: crisp dark triangle pointing right
		c.dc.SetColor(fade(color.RGBA{R: 15, G: 15, B: 18, A: 255}))
		c.dc.MoveTo(ppCenter-3.5, ppY-6.0)
		c.dc.LineTo(ppCenter+6.0, ppY)
		c.dc.LineTo(ppCenter-3.5, ppY+6.0)
		c.dc.ClosePath()
		c.dc.Fill()
	}

	// --- Next Button ---
	nextCenter := rects.Next.X + rects.Next.W/2
	c.dc.SetColor(fade(color.RGBA{R: 200, G: 200, B: 210, A: 255}))
	// Next right triangle
	c.dc.MoveTo(nextCenter+5.0, ctrlY)
	c.dc.LineTo(nextCenter-4.5, ctrlY-5.5)
	c.dc.LineTo(nextCenter-4.5, ctrlY+5.5)
	c.dc.ClosePath()
	c.dc.Fill()
	// Next vertical bar
	c.dc.DrawRoundedRectangle(nextCenter+5.0, ctrlY-5.0, 2.0, 10.0, 0.8)
	c.dc.Fill()

	// 6. Volume bar & Vector Speaker Icon
	vb := rects.VolumeBar
	spkX := vb.X - 16.0
	spkY := vb.Y + vb.H/2

	c.dc.SetColor(fade(color.RGBA{R: 140, G: 140, B: 150, A: 255}))
	// Speaker back box
	c.dc.DrawRectangle(spkX, spkY-2.5, 2.5, 5.0)
	c.dc.Fill()
	// Speaker cone
	c.dc.MoveTo(spkX+2.5, spkY-2.5)
	c.dc.LineTo(spkX+6.0, spkY-5.0)
	c.dc.LineTo(spkX+6.0, spkY+5.0)
	c.dc.LineTo(spkX+2.5, spkY+2.5)
	c.dc.ClosePath()
	c.dc.Fill()
	// Speaker sound arc
	c.dc.SetLineWidth(1.2)
	c.dc.DrawArc(spkX+6.0, spkY, 3.5, -math.Pi/3, math.Pi/3)
	c.dc.Stroke()

	// Volume slider track
	c.dc.SetColor(fade(color.RGBA{R: 44, G: 44, B: 50, A: 255}))
	c.dc.DrawRoundedRectangle(vb.X, vb.Y+2, vb.W, 4, 2)
	c.dc.Fill()

	volFillW := vb.W * vol
	if volFillW > 0 {
		c.dc.SetColor(fade(color.RGBA{R: 190, G: 190, B: 205, A: 255}))
		c.dc.DrawRoundedRectangle(vb.X, vb.Y+2, volFillW, 4, 2)
		c.dc.Fill()
	}
}
