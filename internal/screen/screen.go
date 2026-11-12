// Package screen keeps the state that Neovim's UI protocol (ext_linegrid,
// without ext_multigrid) describes: one grid of cells, highlight
// attributes, cursor, modes and UI options.
//
// It has no dependency on any graphics library, so everything here can be
// tested by feeding it redraw events and comparing text snapshots.
package screen

import (
	"fmt"
	"strings"

	"github.com/Shougo/ng/internal/msgpack"
)

// Cell is one grid cell. The right half of a double-width character is a
// cell with empty Text.
type Cell struct {
	Text string
	HL   int
}

// Color is a 24-bit RGB value, or Default.
type Color int32

// Default means "use the default color".
const Default Color = -1

// RGB returns the components of c. c must not be Default.
func (c Color) RGB() (r, g, b uint8) {
	return uint8(c >> 16), uint8(c >> 8), uint8(c)
}

// Attr is a highlight definition from hl_attr_define.
type Attr struct {
	FG, BG, SP    Color
	Reverse       bool
	Bold, Italic  bool
	Strikethrough bool
	Underline     bool
	Undercurl     bool
	Underdouble   bool
	Underdotted   bool
	Underdashed   bool
	Blend         int
}

var defaultAttr = Attr{FG: Default, BG: Default, SP: Default}

// CursorShape is the cursor shape of a mode.
type CursorShape int

const (
	CursorBlock CursorShape = iota
	CursorHorizontal
	CursorVertical
)

// Mode is one entry of mode_info_set.
type Mode struct {
	Name           string
	ShortName      string
	Shape          CursorShape
	CellPercentage int
	AttrID         int
	BlinkWait      int
	BlinkOn        int
	BlinkOff       int
}

// Screen is the UI state. It is not safe for concurrent use; feed it from
// one goroutine (ng uses the main/render thread).
type Screen struct {
	Width, Height int
	cells         [][]Cell
	dirty         []bool

	CursorRow, CursorCol int

	attrs                map[int]Attr
	DefaultFG, DefaultBG Color
	DefaultSP            Color
	CursorStyleEnabled   bool
	Modes                []Mode
	ModeIndex            int
	ModeName             string
	Options              map[string]any
	Title                string
	Busy                 bool
	MouseEnabled         bool
	Bell                 int // incremented on bell and visual_bell
	optionChanged        map[string]bool
	allDirty             bool
	colorsChanged        bool
}

// New returns a screen of the given size.
func New(width, height int) *Screen {
	s := &Screen{
		attrs:         map[int]Attr{},
		DefaultFG:     0xffffff,
		DefaultBG:     0x000000,
		DefaultSP:     0xff0000,
		Options:       map[string]any{},
		optionChanged: map[string]bool{},
		MouseEnabled:  true,
	}
	s.resize(width, height)
	return s
}

func blankRow(w int) []Cell {
	r := make([]Cell, w)
	for i := range r {
		r[i] = Cell{Text: " "}
	}
	return r
}

func (s *Screen) resize(w, h int) {
	w, h = max(w, 1), max(h, 1)
	cells := make([][]Cell, h)
	for r := range cells {
		cells[r] = blankRow(w)
		if r < len(s.cells) {
			copy(cells[r], s.cells[r])
		}
	}
	s.Width, s.Height = w, h
	s.cells = cells
	s.dirty = make([]bool, h)
	s.allDirty = true
	s.CursorRow = min(s.CursorRow, h-1)
	s.CursorCol = min(s.CursorCol, w-1)
}

// Cell returns the cell at (row, col).
func (s *Screen) Cell(row, col int) Cell { return s.cells[row][col] }

// Row returns the cells of row. The slice must not be modified.
func (s *Screen) Row(row int) []Cell { return s.cells[row] }

// CellWidth returns 2 if the cell at (row, col) starts a double-width
// character, 0 if it is the right half of one and 1 otherwise.
func (s *Screen) CellWidth(row, col int) int {
	r := s.cells[row]
	if r[col].Text == "" {
		return 0
	}
	if col+1 < len(r) && r[col+1].Text == "" {
		return 2
	}
	return 1
}

// Attr returns the highlight for id. Unknown ids use the defaults.
func (s *Screen) Attr(id int) Attr {
	if a, ok := s.attrs[id]; ok {
		return a
	}
	return defaultAttr
}

// Colors resolves the foreground, background and special colors of a
// highlight, applying defaults and reverse.
func (s *Screen) Colors(id int) (fg, bg, sp Color) {
	a := s.Attr(id)
	fg, bg, sp = a.FG, a.BG, a.SP
	if fg == Default {
		fg = s.DefaultFG
	}
	if bg == Default {
		bg = s.DefaultBG
	}
	if sp == Default {
		sp = s.DefaultSP
	}
	if a.Reverse {
		fg, bg = bg, fg
	}
	return fg, bg, sp
}

// CursorColors returns the colors for drawing the cursor over a cell with
// highlight cellHL. When the mode has no cursor highlight (attr_id 0) the
// cell's colors are swapped, as the UI protocol specifies; a cursor
// highlight that leaves a color unset falls back to the swapped cell color.
func (s *Screen) CursorColors(cellHL int) (fg, bg Color) {
	cfg, cbg, _ := s.Colors(cellHL)
	m := s.CurrentMode()
	if m.AttrID == 0 {
		return cbg, cfg
	}
	a := s.Attr(m.AttrID)
	fg, bg = a.FG, a.BG
	if a.Reverse {
		fg, bg = bg, fg
	}
	if fg == Default {
		fg = cbg
	}
	if bg == Default {
		bg = cfg
	}
	return fg, bg
}

// CurrentMode returns the active mode, or a block cursor if Neovim has not
// sent mode information yet.
func (s *Screen) CurrentMode() Mode {
	if s.ModeIndex >= 0 && s.ModeIndex < len(s.Modes) {
		return s.Modes[s.ModeIndex]
	}
	return Mode{Shape: CursorBlock, CellPercentage: 100}
}

// TakeDirty returns the rows that changed since the last call and clears
// the marks. A row is reported whole even if one cell changed: redrawing
// whole rows is what keeps double-width glyphs from leaving fragments.
func (s *Screen) TakeDirty() []int {
	var rows []int
	for r := range s.dirty {
		if s.allDirty || s.dirty[r] {
			rows = append(rows, r)
		}
		s.dirty[r] = false
	}
	s.allDirty = false
	return rows
}

// MarkAllDirty forces a full redraw.
func (s *Screen) MarkAllDirty() { s.allDirty = true }

// TakeOptionChanges returns the UI options set since the last call.
func (s *Screen) TakeOptionChanges() []string {
	var names []string
	for n := range s.optionChanged {
		names = append(names, n)
	}
	clear(s.optionChanged)
	return names
}

// TakeColorsChanged reports whether the default colors changed since the
// last call.
func (s *Screen) TakeColorsChanged() bool {
	c := s.colorsChanged
	s.colorsChanged = false
	return c
}

// OptionString returns a string UI option.
func (s *Screen) OptionString(name string) string {
	v, _ := s.Options[name].(string)
	return v
}

// OptionInt returns an integer UI option.
func (s *Screen) OptionInt(name string) int {
	n, _ := msgpack.ToInt(s.Options[name])
	return int(n)
}

// HandleRedraw applies the params of a "redraw" notification. It reports
// whether the batch contained a flush, which is the only point where the
// screen is consistent and may be drawn.
func (s *Screen) HandleRedraw(params []any) (flush bool, err error) {
	for _, p := range params {
		ev, ok := p.([]any)
		if !ok || len(ev) == 0 {
			return flush, fmt.Errorf("screen: malformed redraw event %#v", p)
		}
		name, _ := ev[0].(string)
		for _, a := range ev[1:] {
			args, _ := a.([]any)
			if err := s.apply(name, args); err != nil {
				return flush, fmt.Errorf("screen: %s: %w", name, err)
			}
		}
		if name == "flush" {
			flush = true
		}
	}
	return flush, nil
}

func argInt(args []any, i int) int {
	if i >= len(args) {
		return 0
	}
	n, _ := msgpack.ToInt(args[i])
	return int(n)
}

func (s *Screen) apply(name string, args []any) error {
	switch name {
	case "grid_resize":
		s.resize(argInt(args, 1), argInt(args, 2))
	case "grid_clear":
		for r := range s.cells {
			s.cells[r] = blankRow(s.Width)
		}
		s.allDirty = true
	case "grid_cursor_goto":
		s.CursorRow = clamp(argInt(args, 1), 0, s.Height-1)
		s.CursorCol = clamp(argInt(args, 2), 0, s.Width-1)
	case "grid_line":
		return s.gridLine(args)
	case "grid_scroll":
		s.gridScroll(argInt(args, 1), argInt(args, 2), argInt(args, 3),
			argInt(args, 4), argInt(args, 5))
	case "hl_attr_define":
		if len(args) < 2 {
			return fmt.Errorf("short args")
		}
		m, _ := args[1].(map[string]any)
		s.attrs[argInt(args, 0)] = parseAttr(m)
	case "default_colors_set":
		s.DefaultFG = toColor(args, 0, 0xffffff)
		s.DefaultBG = toColor(args, 1, 0x000000)
		s.DefaultSP = toColor(args, 2, 0xff0000)
		s.colorsChanged = true
		s.allDirty = true
	case "mode_info_set":
		if len(args) >= 2 {
			s.CursorStyleEnabled, _ = args[0].(bool)
			list, _ := args[1].([]any)
			s.Modes = s.Modes[:0]
			for _, e := range list {
				m, _ := e.(map[string]any)
				s.Modes = append(s.Modes, parseMode(m))
			}
		}
	case "mode_change":
		s.ModeName, _ = firstString(args)
		s.ModeIndex = argInt(args, 1)
	case "option_set":
		if len(args) >= 2 {
			n, _ := args[0].(string)
			s.Options[n] = args[1]
			s.optionChanged[n] = true
		}
	case "set_title":
		s.Title, _ = firstString(args)
	case "busy_start":
		s.Busy = true
	case "busy_stop":
		s.Busy = false
	case "mouse_on":
		s.MouseEnabled = true
	case "mouse_off":
		s.MouseEnabled = false
	case "bell", "visual_bell":
		s.Bell++
	}
	// Everything else (flush, hl_group_set, set_icon, chdir, ...) needs no
	// state here.
	return nil
}

func (s *Screen) gridLine(args []any) error {
	if len(args) < 4 {
		return fmt.Errorf("short args")
	}
	row, col := argInt(args, 1), argInt(args, 2)
	cells, _ := args[3].([]any)
	if row < 0 || row >= s.Height {
		return nil
	}
	r := s.cells[row]
	hl := 0
	for _, c := range cells {
		cell, _ := c.([]any)
		if len(cell) == 0 {
			continue
		}
		text, _ := cell[0].(string)
		if len(cell) >= 2 {
			hl = argInt(cell, 1)
		}
		repeat := 1
		if len(cell) >= 3 {
			repeat = argInt(cell, 2)
		}
		for i := 0; i < repeat && col < s.Width; i++ {
			r[col] = Cell{Text: text, HL: hl}
			col++
		}
	}
	s.dirty[row] = true
	return nil
}

func (s *Screen) gridScroll(top, bot, left, right, rows int) {
	top, bot = clamp(top, 0, s.Height), clamp(bot, 0, s.Height)
	left, right = clamp(left, 0, s.Width), clamp(right, 0, s.Width)
	if rows > 0 {
		for r := top; r < bot-rows; r++ {
			copy(s.cells[r][left:right], s.cells[r+rows][left:right])
		}
	} else if rows < 0 {
		for r := bot - 1; r >= top-rows; r-- {
			copy(s.cells[r][left:right], s.cells[r+rows][left:right])
		}
	}
	for r := top; r < bot; r++ {
		s.dirty[r] = true
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func firstString(args []any) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	v, ok := args[0].(string)
	return v, ok
}

func toColor(args []any, i int, fallback Color) Color {
	if i >= len(args) {
		return fallback
	}
	n, err := msgpack.ToInt(args[i])
	if err != nil || n < 0 {
		return fallback
	}
	return Color(n)
}

func parseAttr(m map[string]any) Attr {
	a := defaultAttr
	color := func(k string) Color {
		if v, ok := m[k]; ok {
			if n, err := msgpack.ToInt(v); err == nil && n >= 0 {
				return Color(n)
			}
		}
		return Default
	}
	flag := func(k string) bool { b, _ := m[k].(bool); return b }
	a.FG, a.BG, a.SP = color("foreground"), color("background"), color("special")
	a.Reverse = flag("reverse")
	a.Bold, a.Italic = flag("bold"), flag("italic")
	a.Strikethrough = flag("strikethrough")
	a.Underline, a.Undercurl = flag("underline"), flag("undercurl")
	a.Underdouble, a.Underdotted = flag("underdouble"), flag("underdotted")
	a.Underdashed = flag("underdashed")
	if v, ok := m["blend"]; ok {
		n, _ := msgpack.ToInt(v)
		a.Blend = int(n)
	}
	return a
}

func parseMode(m map[string]any) Mode {
	str := func(k string) string { v, _ := m[k].(string); return v }
	num := func(k string) int { n, _ := msgpack.ToInt(m[k]); return int(n) }
	md := Mode{
		Name:           str("name"),
		ShortName:      str("short_name"),
		CellPercentage: num("cell_percentage"),
		AttrID:         num("attr_id"),
		BlinkWait:      num("blinkwait"),
		BlinkOn:        num("blinkon"),
		BlinkOff:       num("blinkoff"),
	}
	switch str("cursor_shape") {
	case "horizontal":
		md.Shape = CursorHorizontal
	case "vertical":
		md.Shape = CursorVertical
	default:
		md.Shape = CursorBlock
	}
	if md.CellPercentage <= 0 {
		md.CellPercentage = 100
	}
	return md
}

// Dump renders the grid as text: one line per row, double-width
// characters once, trailing blanks trimmed. Used by snapshot tests.
func (s *Screen) Dump() string {
	var b strings.Builder
	for r := range s.cells {
		var line strings.Builder
		for _, c := range s.cells[r] {
			line.WriteString(c.Text)
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteByte('\n')
	}
	return b.String()
}
