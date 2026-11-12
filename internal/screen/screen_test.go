package screen

import (
	"reflect"
	"testing"
)

// ev builds one redraw event: [name, args1, args2, ...].
func ev(name string, argSets ...[]any) []any {
	e := []any{name}
	for _, a := range argSets {
		e = append(e, a)
	}
	return e
}

func cell(text string, rest ...int64) []any {
	c := []any{text}
	for _, n := range rest {
		c = append(c, n)
	}
	return c
}

func line(row, col int64, cells ...[]any) []any {
	cs := make([]any, len(cells))
	for i, c := range cells {
		cs[i] = c
	}
	return []any{int64(1), row, col, cs, false}
}

func apply(t *testing.T, s *Screen, events ...[]any) bool {
	t.Helper()
	params := make([]any, len(events))
	for i, e := range events {
		params[i] = e
	}
	flush, err := s.HandleRedraw(params)
	if err != nil {
		t.Fatal(err)
	}
	return flush
}

func TestGridLineRepeatAndHL(t *testing.T) {
	s := New(10, 2)
	flush := apply(t, s,
		ev("grid_line", line(0, 0, cell("a", 3), cell("b"), cell("-", 0, 3))),
		ev("flush", []any{}),
	)
	if !flush {
		t.Error("flush not reported")
	}
	if got := s.Dump(); got != "ab---\n\n" {
		t.Errorf("dump %q", got)
	}
	// "b" inherits hl 3 from the previous cell in the same event.
	if s.Cell(0, 1).HL != 3 || s.Cell(0, 2).HL != 0 {
		t.Errorf("hl: %+v %+v", s.Cell(0, 1), s.Cell(0, 2))
	}
}

func TestWideCharacters(t *testing.T) {
	s := New(10, 1)
	apply(t, s, ev("grid_line", line(0, 0,
		cell("あ"), cell(""), cell("x"), cell("漢"), cell(""))))
	if got := s.Dump(); got != "あx漢\n" {
		t.Errorf("dump %q", got)
	}
	want := []int{2, 0, 1, 2, 0, 1}
	for col, w := range want {
		if got := s.CellWidth(0, col); got != w {
			t.Errorf("CellWidth(0,%d)=%d want %d", col, got, w)
		}
	}
}

// The case that breaks neovim-qt: SKK conversion replaces "▽かんじ" with
// "漢字", shifting where every double-width character starts. The whole row
// must be reported dirty so the renderer repaints it in one go.
func TestConversionRedrawsWholeRow(t *testing.T) {
	s := New(12, 2)
	apply(t, s, ev("grid_line", line(0, 0,
		cell("▽"), cell("か"), cell(""), cell("ん"), cell(""), cell("じ"), cell(""))),
		ev("flush", []any{}))
	s.TakeDirty()
	if got := s.Dump(); got != "▽かんじ\n\n" {
		t.Fatalf("before: %q", got)
	}
	apply(t, s, ev("grid_line", line(0, 0,
		cell("漢"), cell(""), cell("字"), cell(""), cell(" ", 0, 3))),
		ev("flush", []any{}))
	if got := s.Dump(); got != "漢字\n\n" {
		t.Errorf("after: %q", got)
	}
	if got := s.TakeDirty(); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("dirty rows %v", got)
	}
	// "字" now starts at column 2, which used to be the right half of "か".
	if s.CellWidth(0, 2) != 2 || s.CellWidth(0, 3) != 0 || s.CellWidth(0, 4) != 1 {
		t.Errorf("widths: %d %d %d", s.CellWidth(0, 2), s.CellWidth(0, 3), s.CellWidth(0, 4))
	}
}

func TestScrollUpAndDown(t *testing.T) {
	s := New(4, 4)
	for r := int64(0); r < 4; r++ {
		apply(t, s, ev("grid_line", line(r, 0, cell(string(rune('a'+r)), 0, 4))))
	}
	s.TakeDirty()
	// Scroll rows 0..3 up by one, then Neovim fills the exposed row.
	apply(t, s, ev("grid_scroll", []any{int64(1), int64(0), int64(4), int64(0), int64(4), int64(1), int64(0)}),
		ev("grid_line", line(3, 0, cell("e", 0, 4))))
	if got := s.Dump(); got != "bbbb\ncccc\ndddd\neeee\n" {
		t.Errorf("up: %q", got)
	}
	if got := s.TakeDirty(); !reflect.DeepEqual(got, []int{0, 1, 2, 3}) {
		t.Errorf("dirty %v", got)
	}
	// Scroll rows 1..3 down by one.
	apply(t, s, ev("grid_scroll", []any{int64(1), int64(1), int64(4), int64(0), int64(4), int64(-1), int64(0)}),
		ev("grid_line", line(1, 0, cell("z", 0, 4))))
	if got := s.Dump(); got != "bbbb\nzzzz\ncccc\ndddd\n" {
		t.Errorf("down: %q", got)
	}
	if got := s.TakeDirty(); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("dirty %v", got)
	}
}

func TestScrollRegionColumns(t *testing.T) {
	s := New(4, 3)
	apply(t, s,
		ev("grid_line", line(0, 0, cell("1", 0, 4))),
		ev("grid_line", line(1, 0, cell("2", 0, 4))),
		ev("grid_line", line(2, 0, cell("3", 0, 4))),
		// Only columns 1..2 scroll (a vertical split).
		ev("grid_scroll", []any{int64(1), int64(0), int64(3), int64(1), int64(3), int64(1), int64(0)}),
	)
	if got := s.Dump(); got != "1221\n2332\n3333\n" {
		t.Errorf("dump %q", got)
	}
}

func TestResizeClearAndCursor(t *testing.T) {
	s := New(4, 2)
	apply(t, s, ev("grid_line", line(0, 0, cell("a"), cell("b"))),
		ev("grid_resize", []any{int64(1), int64(6), int64(3)}),
		ev("grid_cursor_goto", []any{int64(1), int64(2), int64(5)}))
	if s.Width != 6 || s.Height != 3 || s.Dump() != "ab\n\n\n" {
		t.Errorf("after resize %dx%d %q", s.Width, s.Height, s.Dump())
	}
	if s.CursorRow != 2 || s.CursorCol != 5 {
		t.Errorf("cursor %d,%d", s.CursorRow, s.CursorCol)
	}
	apply(t, s, ev("grid_clear", []any{int64(1)}))
	if s.Dump() != "\n\n\n" {
		t.Errorf("after clear %q", s.Dump())
	}
	if len(s.TakeDirty()) != 3 {
		t.Error("clear should dirty all rows")
	}
}

func TestOutOfRangeLineIsIgnored(t *testing.T) {
	s := New(3, 1)
	apply(t, s, ev("grid_line", line(5, 0, cell("x"))),
		ev("grid_line", line(0, 2, cell("y", 0, 10))))
	if s.Dump() != "  y\n" {
		t.Errorf("dump %q", s.Dump())
	}
}

func TestColorsAndCursor(t *testing.T) {
	s := New(4, 1)
	apply(t, s,
		ev("default_colors_set", []any{int64(0xd0d0d0), int64(0x101010), int64(-1), int64(0), int64(0)}),
		ev("hl_attr_define",
			[]any{int64(1), map[string]any{"foreground": int64(0xff0000)}, map[string]any{}, []any{}},
			[]any{int64(2), map[string]any{"background": int64(0xa0a0a0)}, map[string]any{}, []any{}},
			[]any{int64(3), map[string]any{"reverse": true}, map[string]any{}, []any{}}),
		ev("mode_info_set", []any{true, []any{
			map[string]any{"name": "normal", "cursor_shape": "block", "attr_id": int64(0)},
			map[string]any{"name": "insert", "cursor_shape": "vertical", "cell_percentage": int64(25), "attr_id": int64(2)},
		}}),
		ev("mode_change", []any{"normal", int64(0)}),
	)
	if fg, bg, _ := s.Colors(1); fg != 0xff0000 || bg != 0x101010 {
		t.Errorf("hl1 %x %x", fg, bg)
	}
	if fg, bg, _ := s.Colors(3); fg != 0x101010 || bg != 0xd0d0d0 {
		t.Errorf("reverse %x %x", fg, bg)
	}
	// Normal mode: attr_id 0 swaps the cell colors.
	if fg, bg := s.CursorColors(1); fg != 0x101010 || bg != 0xff0000 {
		t.Errorf("normal cursor %x %x", fg, bg)
	}
	// Insert mode: Cursor-like highlight with only a background.
	apply(t, s, ev("mode_change", []any{"insert", int64(1)}))
	m := s.CurrentMode()
	if m.Shape != CursorVertical || m.CellPercentage != 25 {
		t.Errorf("mode %+v", m)
	}
	if fg, bg := s.CursorColors(0); fg != 0x101010 || bg != 0xa0a0a0 {
		t.Errorf("insert cursor %x %x", fg, bg)
	}
	if !s.TakeColorsChanged() || s.TakeColorsChanged() {
		t.Error("colorsChanged flag")
	}
}

func TestOptions(t *testing.T) {
	s := New(1, 1)
	apply(t, s, ev("option_set",
		[]any{"guifont", "Courier 10 Pitch OT:h11"},
		[]any{"linespace", int64(2)}),
		ev("set_title", []any{"ng - main.go"}))
	if s.OptionString("guifont") != "Courier 10 Pitch OT:h11" || s.OptionInt("linespace") != 2 {
		t.Errorf("options %#v", s.Options)
	}
	if got := s.TakeOptionChanges(); len(got) != 2 {
		t.Errorf("changes %v", got)
	}
	if got := s.TakeOptionChanges(); len(got) != 0 {
		t.Errorf("changes not cleared %v", got)
	}
	if s.Title != "ng - main.go" {
		t.Errorf("title %q", s.Title)
	}
}

func TestUnknownEventsAreIgnored(t *testing.T) {
	s := New(1, 1)
	apply(t, s, ev("some_future_event", []any{int64(1)}), ev("hl_group_set", []any{"Normal", int64(0)}))
}
