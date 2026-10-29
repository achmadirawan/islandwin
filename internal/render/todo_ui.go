package render

import (
	"image/color"
	"time"

	"islandwin/internal/todo"
)

// TodoItemRects stores the clickable regions for a single task row.
type TodoItemRects struct {
	ID       string
	Checkbox Rect
	Delete   Rect
}

// TodoUIRects stores interactive regions in the todo panel.
type TodoUIRects struct {
	SwitchTab Rect
	InputBox  Rect
	Items     []TodoItemRects
}

// GetTodoUIRects calculates bounding boxes for todo elements.
func GetTodoUIRects(canvasW, pillW, pillH float64, items []todo.Item) TodoUIRects {
	x := (canvasW - pillW) / 2

	// Switch tab button in top right
	tabW := 66.0
	tabH := 22.0
	tabX := x + pillW - tabW - 16.0
	tabY := 14.0

	// Bottom input box
	inputX := x + 18.0
	inputY := pillH - 38.0
	inputW := pillW - 36.0
	inputH := 26.0

	itemRects := make([]TodoItemRects, 0, len(items))
	startY := 60.0
	rowH := 26.0

	for i, it := range items {
		if i >= 6 {
			break // Show up to 6 items on panel
		}
		y := startY + float64(i)*rowH

		itemRects = append(itemRects, TodoItemRects{
			ID:       it.ID,
			Checkbox: Rect{X: x + 18.0, Y: y - 10.0, W: 18.0, H: 18.0},
			Delete:   Rect{X: x + pillW - 36.0, Y: y - 10.0, W: 18.0, H: 18.0},
		})
	}

	return TodoUIRects{
		SwitchTab: Rect{X: tabX, Y: tabY, W: tabW, H: tabH},
		InputBox:  Rect{X: inputX, Y: inputY, W: inputW, H: inputH},
		Items:     itemRects,
	}
}

// DrawTodoPanel renders complete daily todo list and input field.
func (c *Canvas) DrawTodoPanel(pillW, pillH float64, items []todo.Item, inputText string, inputFocused bool, cursorBlink bool) {
	if pillW < 180 || pillH < 75 {
		return // Still expanding
	}

	// Smooth alpha transition: content dissolves in gracefully as island expands
	alphaProgress := (pillH - 75.0) / (ExpandedTodoH - 120.0)
	if alphaProgress > 1.0 {
		alphaProgress = 1.0
	}
	if alphaProgress < 0.05 {
		return
	}

	fade := func(base color.RGBA) color.RGBA {
		return FadeColor(base, alphaProgress)
	}

	x := (float64(c.W) - pillW) / 2
	rects := GetTodoUIRects(float64(c.W), pillW, pillH, items)

	// 1. Header: Date (Segoe UI)
	dateStr := time.Now().Format("Monday, 02 Jan 2006")
	c.SetFontTitle()
	c.dc.SetColor(fade(color.RGBA{R: 250, G: 250, B: 252, A: 255}))
	c.dc.DrawString("To-Do Hari Ini", x+18, 28)

	c.SetFontSmall()
	c.dc.SetColor(fade(color.RGBA{R: 140, G: 140, B: 150, A: 255}))
	c.dc.DrawString(dateStr, x+18, 44)

	// 2. Switch Tab button ("‹ Music")
	c.dc.SetColor(fade(color.RGBA{R: 36, G: 36, B: 42, A: 255}))
	c.dc.DrawRoundedRectangle(rects.SwitchTab.X, rects.SwitchTab.Y, rects.SwitchTab.W, rects.SwitchTab.H, 11)
	c.dc.Fill()
	c.SetFontSmall()
	c.dc.SetColor(fade(color.RGBA{R: 210, G: 210, B: 220, A: 255}))
	c.dc.DrawString("Sholat ›", rects.SwitchTab.X+11, rects.SwitchTab.Y+15)

	// 3. Render Items
	startY := 64.0

	if len(items) == 0 {
		c.SetFontRegular()
		c.dc.SetColor(fade(color.RGBA{R: 120, G: 120, B: 130, A: 255}))
		c.dc.DrawString("Semua tugas selesai! Santai sejenak.", x+18, startY+14)
	}

	for i, it := range items {
		if i >= 6 {
			break
		}
		cb := rects.Items[i].Checkbox

		if it.Done {
			// Filled green checkbox with vector checkmark
			c.dc.SetColor(fade(color.RGBA{R: 34, G: 197, B: 94, A: 255}))
			c.dc.DrawRoundedRectangle(cb.X, cb.Y, cb.W, cb.H, 4)
			c.dc.Fill()

			c.dc.SetColor(fade(color.RGBA{R: 255, G: 255, B: 255, A: 255}))
			c.dc.SetLineWidth(2.0)
			c.dc.SetLineCapRound()
			c.dc.MoveTo(cb.X+4.0, cb.Y+9.0)
			c.dc.LineTo(cb.X+7.0, cb.Y+12.5)
			c.dc.LineTo(cb.X+13.5, cb.Y+5.0)
			c.dc.Stroke()
		} else {
			// Clean hollow checkbox
			c.dc.SetColor(fade(color.RGBA{30, 30, 36, 255}))
			c.dc.DrawRoundedRectangle(cb.X, cb.Y, cb.W, cb.H, 4)
			c.dc.Fill()

			c.dc.SetColor(fade(color.RGBA{R: 100, G: 100, B: 115, A: 255}))
			c.dc.SetLineWidth(1.2)
			c.dc.DrawRoundedRectangle(cb.X+0.5, cb.Y+0.5, cb.W-1, cb.H-1, 4)
			c.dc.Stroke()
		}

		// Text
		c.SetFontRegular()
		itemText := it.Text
		if len(itemText) > 38 {
			itemText = itemText[:35] + "..."
		}

		textY := cb.Y + 13.0
		if it.Done {
			// Muted text with vector strikethrough line
			c.dc.SetColor(fade(color.RGBA{R: 115, G: 115, B: 125, A: 255}))
			c.dc.DrawString(itemText, cb.X+cb.W+10, textY)

			tw, _ := c.dc.MeasureString(itemText)
			c.dc.SetColor(fade(color.RGBA{R: 115, G: 115, B: 125, A: 200}))
			c.dc.SetLineWidth(1.2)
			c.dc.DrawLine(cb.X+cb.W+10, textY-4.0, cb.X+cb.W+10+tw, textY-4.0)
			c.dc.Stroke()
		} else {
			c.dc.SetColor(fade(color.RGBA{R: 242, G: 242, B: 246, A: 255}))
			c.dc.DrawString(itemText, cb.X+cb.W+10, textY)
		}

		// Delete button (Vector '✕')
		del := rects.Items[i].Delete
		c.dc.SetColor(fade(color.RGBA{R: 130, G: 130, B: 140, A: 200}))
		c.dc.SetLineWidth(1.5)
		c.dc.SetLineCapRound()
		c.dc.DrawLine(del.X+4.5, del.Y+4.5, del.X+13.5, del.Y+13.5)
		c.dc.DrawLine(del.X+13.5, del.Y+4.5, del.X+4.5, del.Y+13.5)
		c.dc.Stroke()
	}

	// 4. Input Box
	ib := rects.InputBox
	if inputFocused {
		c.dc.SetColor(fade(color.RGBA{R: 26, G: 26, B: 32, A: 255}))
		c.dc.DrawRoundedRectangle(ib.X, ib.Y, ib.W, ib.H, 6)
		c.dc.Fill()

		// Accent blue active border
		c.dc.SetColor(fade(color.RGBA{R: 59, G: 130, B: 246, A: 255}))
		c.dc.SetLineWidth(1.5)
		c.dc.DrawRoundedRectangle(ib.X+0.5, ib.Y+0.5, ib.W-1, ib.H-1, 6)
		c.dc.Stroke()
	} else {
		c.dc.SetColor(fade(color.RGBA{R: 20, G: 20, B: 25, A: 255}))
		c.dc.DrawRoundedRectangle(ib.X, ib.Y, ib.W, ib.H, 6)
		c.dc.Fill()

		c.dc.SetColor(fade(color.RGBA{R: 50, G: 50, B: 60, A: 255}))
		c.dc.SetLineWidth(1.0)
		c.dc.DrawRoundedRectangle(ib.X+0.5, ib.Y+0.5, ib.W-1, ib.H-1, 6)
		c.dc.Stroke()
	}

	// Input Text or Placeholder
	textY := ib.Y + 17.0
	if inputText == "" && !inputFocused {
		c.SetFontSmall()
		c.dc.SetColor(fade(color.RGBA{R: 110, G: 110, B: 120, A: 255}))
		c.dc.DrawString("+ Tambah tugas baru... (klik di sini)", ib.X+10, textY)
	} else {
		c.SetFontRegular()
		c.dc.SetColor(fade(color.RGBA{R: 250, G: 250, B: 255, A: 255}))
		c.dc.DrawString(inputText, ib.X+10, textY)

		// Caret cursor when focused
		if inputFocused && cursorBlink {
			textW, _ := c.dc.MeasureString(inputText)
			caretX := ib.X + 10 + textW + 1
			c.dc.SetColor(fade(color.RGBA{R: 255, G: 255, B: 255, A: 255}))
			c.dc.SetLineWidth(1.5)
			c.dc.DrawLine(caretX, ib.Y+5, caretX, ib.Y+ib.H-5)
			c.dc.Stroke()
		}
	}
}
