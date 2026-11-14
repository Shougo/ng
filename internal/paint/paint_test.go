package paint

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/screen"
)

func loadFont(t *testing.T) *font.Set {
	t.Helper()
	f, err := font.NewSet("Courier 10 Pitch,monospace:h11", "", 96, 0)
	if err != nil {
		t.Skipf("no font: %v", err)
	}
	t.Cleanup(f.Close)
	return f
}

func redraw(t *testing.T, s *screen.Screen, events ...[]any) {
	t.Helper()
	params := make([]any, len(events))
	for i, e := range events {
		params[i] = e
	}
	if _, err := s.HandleRedraw(params); err != nil {
		t.Fatal(err)
	}
}

func gridLine(row int64, cells ...string) []any {
	cs := []any{}
	for _, c := range cells {
		cs = append(cs, []any{c})
	}
	return []any{"grid_line", []any{int64(1), row, int64(0), cs, false}}
}

// paintAll renders the whole screen from scratch.
func paintAll(s *screen.Screen, f *font.Set) *ImagePainter {
	p := NewImagePainter(s.Width*f.CellW, s.Height*f.CellH)
	for r := 0; r < s.Height; r++ {
		Row(p, s, f, r)
	}
	return p
}

func dump(t *testing.T, name string, p *ImagePainter) {
	dir := os.Getenv("NG_PAINT_DUMP")
	if dir == "" {
		return
	}
	f, err := os.Create(dir + "/" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, p.Img)
}

// Repainting only the dirty rows after an SKK conversion must give exactly
// the same pixels as painting the final state from scratch. This is the
// bug that makes neovim-qt drop halves of characters.
func TestIncrementalRepaintMatchesFullPaint(t *testing.T) {
	f := loadFont(t)
	s := screen.New(16, 3)
	redraw(t, s,
		[]any{"default_colors_set", []any{int64(0xd0d0d0), int64(0x101010), int64(-1), int64(0), int64(0)}},
		gridLine(0, "▽", "か", "", "ん", "", "じ", "", "で", "", "す"),
		gridLine(1, "a", "b", "c"),
	)
	s.TakeDirty()
	p := paintAll(s, f)
	dump(t, "before", p)

	redraw(t, s, gridLine(0, "漢", "", "字", "", "で", "", "す", " ", " ", " "))
	for _, r := range s.TakeDirty() {
		Row(p, s, f, r)
	}
	want := paintAll(s, f)
	dump(t, "after-incremental", p)
	dump(t, "after-full", want)
	if !bytes.Equal(p.Img.Pix, want.Img.Pix) {
		t.Error("incremental repaint differs from a full repaint (fragments left behind)")
	}
}

func TestScrollRepaintMatchesFullPaint(t *testing.T) {
	f := loadFont(t)
	s := screen.New(8, 4)
	for r := int64(0); r < 4; r++ {
		redraw(t, s, gridLine(r, "日", "", "本", "", string(rune('a'+r))))
	}
	s.TakeDirty()
	p := paintAll(s, f)
	redraw(t, s,
		[]any{"grid_scroll", []any{int64(1), int64(0), int64(4), int64(0), int64(8), int64(1), int64(0)}},
		gridLine(3, "x", "y", "z", " ", " ", " ", " ", " "))
	for _, r := range s.TakeDirty() {
		Row(p, s, f, r)
	}
	if !bytes.Equal(p.Img.Pix, paintAll(s, f).Img.Pix) {
		t.Error("scroll repaint differs from a full repaint")
	}
}

// With attr_id 0 the block cursor swaps the colors of the cell under it,
// so the character stays readable (the GVim Cursor highlight issue).
func TestCursorSwapsColors(t *testing.T) {
	f := loadFont(t)
	s := screen.New(4, 1)
	redraw(t, s,
		[]any{"default_colors_set", []any{int64(0xffffff), int64(0x000000), int64(-1), int64(0), int64(0)}},
		gridLine(0, "M"),
		[]any{"grid_cursor_goto", []any{int64(1), int64(0), int64(0)}},
	)
	p := paintAll(s, f)
	Cursor(p, s, f, true)
	dump(t, "cursor", p)
	// The cell background is now white and the glyph black: the center
	// pixel of the glyph's stem area must not be white everywhere.
	white, dark := 0, 0
	for y := 0; y < f.CellH; y++ {
		for x := 0; x < f.CellW; x++ {
			c := p.Img.RGBAAt(x, y)
			if c.R == 0xff {
				white++
			} else if c.R < 0x80 {
				dark++
			}
		}
	}
	if white == 0 || dark == 0 {
		t.Errorf("cursor cell not inverted: white=%d dark=%d", white, dark)
	}
}

func TestCursorOnRightHalfOfWideChar(t *testing.T) {
	f := loadFont(t)
	s := screen.New(4, 1)
	redraw(t, s, gridLine(0, "漢", ""),
		[]any{"grid_cursor_goto", []any{int64(1), int64(0), int64(1)}})
	p := NewImagePainter(s.Width*f.CellW, f.CellH)
	rec := &recorder{}
	Cursor(rec, s, f, true)
	if len(rec.rects) == 0 || rec.rects[0][0] != 0 || rec.rects[0][2] != 2*f.CellW {
		t.Errorf("cursor rect %v, want x=0 w=%d", rec.rects, 2*f.CellW)
	}
	_ = p
}

func loadFontB() (*font.Set, error) {
	return font.NewSet("Courier 10 Pitch,monospace:h11", "", 96, 0)
}

type recorder struct{ rects [][4]int }

func (r *recorder) FillRect(x, y, w, h int, c screen.Color) {
	r.rects = append(r.rects, [4]int{x, y, w, h})
}
func (r *recorder) DrawGlyph(g *font.Glyph, x, y int, c screen.Color) {}

func TestDecorationsStayInRow(t *testing.T) {
	f := loadFont(t)
	s := screen.New(6, 3)
	redraw(t, s,
		[]any{"hl_attr_define",
			[]any{int64(1), map[string]any{"undercurl": true, "special": int64(0xff0000)}, map[string]any{}, []any{}},
			[]any{int64(2), map[string]any{"underdouble": true}, map[string]any{}, []any{}},
			[]any{int64(3), map[string]any{"strikethrough": true, "underdotted": true}, map[string]any{}, []any{}}},
		[]any{"grid_line",
			[]any{int64(1), int64(1), int64(0), []any{[]any{"a", int64(1), int64(2)}, []any{"b", int64(2), int64(2)}, []any{"c", int64(3), int64(2)}}, false}},
	)
	rec := &recorder{}
	Row(rec, s, f, 1)
	for _, r := range rec.rects {
		if r[1] < f.CellH || r[1]+r[3] > 2*f.CellH {
			t.Errorf("rect %v leaves row 1 (%d..%d)", r, f.CellH, 2*f.CellH)
		}
	}
}

// A Nerd Font icon followed by a blank cell overflows into it, as in a
// terminal; followed by text it is shrunk into its own cell.
func TestWideSymbolOverflow(t *testing.T) {
	f, err := font.NewSet("UDEV Gothic NF:h12", "", 96, 0)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	if !f.Glyph("", 0, 1).Wide {
		t.Skip("font has no wide Nerd Font icons")
	}
	s := screen.New(4, 2)
	redraw(t, s,
		[]any{"default_colors_set", []any{int64(0xffffff), int64(0x000000), int64(0xff0000), int64(0), int64(0)}},
		gridLine(0, "", " ", "x", " "),
		gridLine(1, "", "x", " ", " "))
	p := paintAll(s, f)
	dump(t, "wide_symbol", p)
	ink := func(row, col int) int {
		n := 0
		for y := row * f.CellH; y < (row+1)*f.CellH; y++ {
			for x := col * f.CellW; x < (col+1)*f.CellW; x++ {
				if r, _, _, _ := p.Img.At(x, y).RGBA(); r > 0x8000 {
					n++
				}
			}
		}
		return n
	}
	if ink(0, 1) == 0 {
		t.Error("icon before a blank cell does not overflow into it")
	}
	// Row 1: the icon must stay in its cell; cell 1 holds only the "x".
	s2 := screen.New(4, 1)
	redraw(t, s2,
		[]any{"default_colors_set", []any{int64(0xffffff), int64(0x000000), int64(0xff0000), int64(0), int64(0)}},
		gridLine(0, " ", "x", " ", " "))
	p2 := paintAll(s2, f)
	xOnly := 0
	for y := 0; y < f.CellH; y++ {
		for x := f.CellW; x < 2*f.CellW; x++ {
			if r, _, _, _ := p2.Img.At(x, y).RGBA(); r > 0x8000 {
				xOnly++
			}
		}
	}
	if got := ink(1, 1); got != xOnly {
		t.Errorf("icon before text spills into the text's cell (%d lit pixels, %d from the x alone)", got, xOnly)
	}
}

// An image placed with vim.ui.img's sequences fills its cells over the
// text and is clipped to the grid.
func TestImages(t *testing.T) {
	f := loadFont(t)
	s := screen.New(10, 4)
	redraw(t, s,
		[]any{"default_colors_set", []any{int64(0xffffff), int64(0x000000), int64(0xff0000), int64(0), int64(0)}},
		gridLine(0, "a", "b", "c"))
	// A 2x2 image: red, green / blue, white.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	img.Set(1, 0, color.NRGBA{0, 255, 0, 255})
	img.Set(0, 1, color.NRGBA{0, 0, 255, 255})
	img.Set(1, 1, color.NRGBA{255, 255, 255, 255})
	var b bytes.Buffer
	png.Encode(&b, img)
	enc := base64.StdEncoding.EncodeToString(b.Bytes())
	s.Graphics.Feed([]byte("\x1b_Gf=100,a=t,t=d,i=1,q=2,m=0;" + enc + "\x1b\\"))
	// At row 1, column 2 (1-indexed), 4 columns by 2 rows; a second
	// placement runs past the right edge.
	s.Graphics.Feed([]byte("\x1b7\x1b[1;2H\x1b_Ga=p,i=1,p=2,c=4,r=2,C=1,q=2\x1b\\\x1b8"))
	s.Graphics.Feed([]byte("\x1b7\x1b[3;9H\x1b_Ga=p,i=1,p=3,c=4,r=2,C=1,q=2\x1b\\\x1b8"))

	p := paintAll(s, f)
	Images(p, s, f)
	dump(t, "images", p)

	cw, ch := f.CellW, f.CellH
	at := func(x, y int) color.RGBA { return p.Img.RGBAAt(x, y) }
	// Quadrants of the first placement (x from column 1, y from row 0).
	if c := at(cw+1, 1); c != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("top-left = %v, want red", c)
	}
	if c := at(cw+4*cw-2, 1); c != (color.RGBA{0, 255, 0, 255}) {
		t.Errorf("top-right = %v, want green", c)
	}
	if c := at(cw+1, 2*ch-2); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("bottom-left = %v, want blue", c)
	}
	// Right of the placement: untouched background.
	if c := at(5*cw+1, ch+1); c != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("right of the image = %v, want the background", c)
	}
	// The clipped placement starts at column 8 and shows its left half.
	if c := at(8*cw+1, 2*ch+1); c != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("clipped image = %v, want red", c)
	}
}
