package render

import (
	"image/color"
	"math"
)

const (
	// AlertW/AlertH is the island size while a prayer alert is shown.
	AlertW = 300.0
	AlertH = 72.0
)

// DrawPrayerAlert renders a notification banner (title + subtitle); fades with pill size.
func (c *Canvas) DrawPrayerAlert(pillW, pillH float64, title, sub string) {
	alpha := math.Min((pillH-40)/30, (pillW-200)/80)
	if alpha > 1 {
		alpha = 1
	}
	if alpha < 0.05 {
		return
	}
	cx := float64(c.W) / 2

	c.SetFontTitle()
	c.dc.SetColor(FadeColor(color.RGBA{R: 74, G: 222, B: 128, A: 255}, alpha))
	tw, _ := c.dc.MeasureString(title)
	c.dc.DrawString(title, cx-tw/2, 32)

	c.SetFontSmall()
	c.dc.SetColor(FadeColor(color.RGBA{R: 170, G: 170, B: 180, A: 255}, alpha))
	sw, _ := c.dc.MeasureString(sub)
	c.dc.DrawString(sub, cx-sw/2, 52)
}

// PrayerRow is one line of the prayer list.
type PrayerRow struct {
	Name, Time string
	Next, Past bool
}

// PrayerUIRects stores interactive regions in the prayer panel.
type PrayerUIRects struct{ SwitchTab Rect }

// GetPrayerUIRects mirrors the tab button position used by the other panels.
func GetPrayerUIRects(canvasW, pillW float64) PrayerUIRects {
	x := (canvasW - pillW) / 2
	return PrayerUIRects{SwitchTab: Rect{X: x + pillW - 82, Y: 14, W: 66, H: 22}}
}

// DrawPrayerPanel renders city, countdown to next prayer, and the 5-prayer list.
func (c *Canvas) DrawPrayerPanel(pillW, pillH float64, city, nextName, countdown string, rows []PrayerRow) {
	if pillW < 180 || pillH < 75 {
		return
	}
	alpha := (pillH - 75.0) / (ExpandedPrayerH - 120.0)
	if alpha > 1 {
		alpha = 1
	}
	if alpha < 0.05 {
		return
	}
	fade := func(base color.RGBA) color.RGBA { return FadeColor(base, alpha) }

	x := (float64(c.W) - pillW) / 2
	tab := GetPrayerUIRects(float64(c.W), pillW).SwitchTab

	c.SetFontTitle()
	c.dc.SetColor(fade(color.RGBA{R: 250, G: 250, B: 252, A: 255}))
	c.dc.DrawString("Waktu Sholat", x+18, 28)
	c.SetFontSmall()
	c.dc.SetColor(fade(color.RGBA{R: 140, G: 140, B: 150, A: 255}))
	c.dc.DrawString(city, x+18, 44)

	c.dc.SetColor(fade(color.RGBA{R: 36, G: 36, B: 42, A: 255}))
	c.dc.DrawRoundedRectangle(tab.X, tab.Y, tab.W, tab.H, 11)
	c.dc.Fill()
	c.dc.SetColor(fade(color.RGBA{R: 210, G: 210, B: 220, A: 255}))
	c.dc.DrawString("Music ›", tab.X+11, tab.Y+15)

	// Countdown
	c.SetFontTitle()
	c.dc.SetColor(fade(color.RGBA{R: 74, G: 222, B: 128, A: 255}))
	c.dc.DrawString(nextName, x+18, 72)
	cw, _ := c.dc.MeasureString(countdown)
	c.dc.DrawString(countdown, x+pillW-18-cw, 72)

	// List
	const rowH = 21.0
	for i, r := range rows {
		y := 84 + float64(i)*rowH
		txt := color.RGBA{R: 225, G: 225, B: 232, A: 255}
		if r.Next {
			c.dc.SetColor(fade(color.RGBA{R: 22, G: 60, B: 38, A: 255}))
			c.dc.DrawRoundedRectangle(x+10, y, pillW-20, rowH-2, 6)
			c.dc.Fill()
			txt = color.RGBA{R: 74, G: 222, B: 128, A: 255}
		} else if r.Past {
			txt = color.RGBA{R: 110, G: 110, B: 120, A: 255}
		}
		c.SetFontRegular()
		c.dc.SetColor(fade(txt))
		c.dc.DrawString(r.Name, x+18, y+14)
		tw, _ := c.dc.MeasureString(r.Time)
		c.dc.DrawString(r.Time, x+pillW-18-tw, y+14)
	}
}

// DrawNextPrayer draws "Maghrib - 17:49 (00:35)" inside the collapsed pill; prefix = "Maghrib - 17:49".
// Minutes are amber; when urgent they are red and blink (dim on blinkOff).
// baseW is the current collapsed width; label fades out as the pill grows past it.
func (c *Canvas) DrawNextPrayer(pillW, pillH float64, prefix, hh, mm string, baseW float64, urgent, blinkOn bool) {
	if prefix == "" || pillW > baseW+15 {
		return
	}
	alpha := (baseW + 15 - pillW) / 15
	if alpha > 1 {
		alpha = 1
	}
	mmCol := color.RGBA{R: 251, G: 191, B: 36, A: 255}
	if urgent {
		mmCol = color.RGBA{R: 239, G: 68, B: 68, A: 255}
		if !blinkOn {
			mmCol.A = 70
		}
	}
	gray := color.RGBA{R: 150, G: 150, B: 160, A: 255}
	segs := []struct {
		s   string
		col color.RGBA
	}{{prefix + " (" + hh + ":", gray}, {mm, mmCol}, {")", gray}}

	c.SetFontPill()
	total := 0.0
	for _, sg := range segs {
		w, _ := c.dc.MeasureString(sg.s)
		total += w
	}
	x := float64(c.W)/2 - total/2
	for _, sg := range segs {
		c.dc.SetColor(FadeColor(sg.col, alpha))
		c.dc.DrawString(sg.s, x, pillH/2+3.5)
		w, _ := c.dc.MeasureString(sg.s)
		x += w
	}
}
