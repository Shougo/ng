package paint

import (
	"testing"

	"github.com/Shougo/ng/internal/screen"
)

func preeditScreen(t *testing.T, cols int, cursorCol int64) *screen.Screen {
	s := screen.New(cols, 3)
	redraw(t, s,
		[]any{"default_colors_set", []any{int64(0xd0d0d0), int64(0x101010), int64(-1), int64(0), int64(0)}},
		gridLine(1, "a", "b", "c"),
		[]any{"grid_cursor_goto", []any{int64(1), int64(1), cursorCol}},
	)
	return s
}

func TestPreeditAtCursor(t *testing.T) {
	f := loadFont(t)
	s := preeditScreen(t, 20, 3)
	rec := &recorder{}
	area, cx := DrawPreedit(rec, s, f, Preedit{Text: "にほんご", Cursor: 4, SelLen: 0})
	want := Rect{X: 3 * f.CellW, Y: 1 * f.CellH, W: 8 * f.CellW, H: f.CellH}
	if area != want {
		t.Errorf("area %+v, want %+v", area, want)
	}
	if cx != 11*f.CellW {
		t.Errorf("cursor x %d, want end of text %d", cx, 11*f.CellW)
	}
	for _, r := range rec.rects {
		if r[1] < f.CellH || r[1]+r[3] > 2*f.CellH {
			t.Errorf("rect %v leaves the cursor row", r)
		}
	}
}

func TestPreeditShiftsLeftAtRightEdge(t *testing.T) {
	f := loadFont(t)
	s := preeditScreen(t, 10, 8)
	area, _ := DrawPreedit(&recorder{}, s, f, Preedit{Text: "変換中", Cursor: 0, SelLen: 3})
	if area.X != 4*f.CellW || area.W != 6*f.CellW {
		t.Errorf("area %+v, want x=%d w=%d", area, 4*f.CellW, 6*f.CellW)
	}
}

func TestPreeditClipsWhenWiderThanRow(t *testing.T) {
	f := loadFont(t)
	s := preeditScreen(t, 5, 0)
	area, _ := DrawPreedit(&recorder{}, s, f, Preedit{Text: "あいうえお", Cursor: 5})
	if area.X != 0 || area.W > 5*f.CellW {
		t.Errorf("area %+v exceeds the row", area)
	}
}

// The segment being converted is drawn reversed: its background is the
// default foreground.
func TestPreeditSelectionReversed(t *testing.T) {
	f := loadFont(t)
	s := preeditScreen(t, 20, 0)
	p := NewImagePainter(20*f.CellW, 3*f.CellH)
	for r := 0; r < s.Height; r++ {
		Row(p, s, f, r)
	}
	DrawPreedit(p, s, f, Preedit{Text: "にほんご", Cursor: 2, SelLen: 2})
	dump(t, "preedit", p)
	at := func(cell int) uint8 { return p.Img.RGBAAt(cell*f.CellW+1, f.CellH+1).R }
	if at(1) != 0x10 || at(5) != 0xd0 || at(7) != 0xd0 || at(9) != 0x10 {
		t.Errorf("backgrounds %x %x %x %x, want unselected 10, selected d0", at(1), at(5), at(7), at(9))
	}
}

func TestPreeditCursorInside(t *testing.T) {
	f := loadFont(t)
	s := preeditScreen(t, 20, 2)
	_, cx := DrawPreedit(&recorder{}, s, f, Preedit{Text: "かな", Cursor: 1})
	if cx != (2+2)*f.CellW {
		t.Errorf("cursor x %d, want %d", cx, 4*f.CellW)
	}
}

func TestCursorRectOnWideChar(t *testing.T) {
	f := loadFont(t)
	s := screen.New(10, 2)
	redraw(t, s, gridLine(0, "漢", ""), []any{"grid_cursor_goto", []any{int64(1), int64(0), int64(1)}})
	if r := CursorRect(s, f); r.X != 0 || r.W != 2*f.CellW {
		t.Errorf("rect %+v", r)
	}
}
