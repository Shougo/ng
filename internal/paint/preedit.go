package paint

import (
	"unicode/utf8"

	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/screen"
	"github.com/Shougo/ng/internal/width"
)

// Preedit is the text an input method is composing, not yet sent to
// Neovim. Cursor and SelLen count characters (code points), as SDL
// reports them; SelLen > 0 marks the segment being converted.
type Preedit struct {
	Text   string
	Cursor int
	SelLen int
}

// Rect is a rectangle in pixels.
type Rect struct{ X, Y, W, H int }

// DrawPreedit paints the preedit text over the grid at the cursor, with
// ng's own glyphs, underlined, the selected segment reversed. It is
// shifted left if it would run past the right edge. It returns the area
// it covers and the x position of the preedit cursor, for placing the
// input method's candidate window.
func DrawPreedit(p Painter, s *screen.Screen, f *font.Set, pe Preedit) (area Rect, cursorX int) {
	if pe.Text == "" || s.Width == 0 || s.Height == 0 {
		return Rect{}, 0
	}
	cells := width.Layout(pe.Text, s.OptionString("ambiwidth") == "double")
	total := width.Cells(cells)

	row, col := s.CursorRow, s.CursorCol
	if s.CellWidth(row, col) == 0 && col > 0 {
		col--
	}
	col = max(0, min(col, s.Width-total))
	x0, y := col*f.CellW, row*f.CellH
	maxX := s.Width * f.CellW

	fg, bg := s.DefaultFG, s.DefaultBG
	t := max(1, f.CellH/16)
	cursorX = -1
	x, ch := x0, 0 // ch counts characters (code points)
	for _, c := range cells {
		w := c.Width * f.CellW
		if x+w > maxX {
			break // wider than the whole row: clip
		}
		n := utf8.RuneCountInString(c.Text)
		cfg, cbg := fg, bg
		if pe.SelLen > 0 && ch >= pe.Cursor && ch < pe.Cursor+pe.SelLen {
			cfg, cbg = bg, fg
		}
		if ch == pe.Cursor && cursorX < 0 {
			cursorX = x
		}
		p.FillRect(x, y, w, f.CellH, cbg)
		if g := f.Glyph(c.Text, 0, c.Width); !g.Empty() {
			p.DrawGlyph(g, x+g.X, y+g.Y, cfg)
		}
		p.FillRect(x, y+f.CellH-t, w, t, cfg) // underline
		x += w
		ch += n
	}
	if cursorX < 0 {
		cursorX = x // cursor at the end
	}
	if pe.SelLen <= 0 && pe.Cursor >= 0 {
		// A bar cursor inside the preedit text, as terminals draw it.
		p.FillRect(min(cursorX, maxX-t), y, max(1, f.CellW/8), f.CellH, fg)
	}
	return Rect{X: x0, Y: y, W: x - x0, H: f.CellH}, cursorX
}

// CursorRect is where the input method should place its candidate window
// when nothing is being composed: the cursor's cell.
func CursorRect(s *screen.Screen, f *font.Set) Rect {
	row, col := s.CursorRow, s.CursorCol
	if s.CellWidth(row, col) == 0 && col > 0 {
		col--
	}
	return Rect{X: col * f.CellW, Y: row * f.CellH, W: max(1, s.CellWidth(row, col)) * f.CellW, H: f.CellH}
}
