package font

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// TestDumpSample writes a rendered sample line to $NG_FONT_DUMP so the
// rendering can be inspected by eye. Skipped unless the variable is set.
func TestDumpSample(t *testing.T) {
	out := os.Getenv("NG_FONT_DUMP")
	if out == "" {
		t.Skip("set NG_FONT_DUMP=/path/to/out.png")
	}
	s, err := NewSet(os.Getenv("NG_FONT_GUIFONT"), os.Getenv("NG_FONT_WIDE"), 192, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	type c struct {
		text  string
		cells int
		style Style
	}
	line := []c{}
	for _, r := range "Hello, gvim! 0O1lI|" {
		line = append(line, c{string(r), 1, 0})
	}
	for _, r := range "あいう漢字テスト" {
		line = append(line, c{string(r), 2, 0})
	}
	for _, r := range "▽かな▼変換○※" {
		w := 2
		if r == '▽' || r == '▼' || r == '○' || r == '※' {
			w = 1
		}
		line = append(line, c{string(r), w, 0})
	}
	for _, r := range "Bold" {
		line = append(line, c{string(r), 1, Bold})
	}
	for _, r := range "Ital" {
		line = append(line, c{string(r), 1, Italic})
	}
	cols := 0
	for _, x := range line {
		cols += x.cells
	}
	img := image.NewRGBA(image.Rect(0, 0, cols*s.CellW, s.CellH))
	bg := color.RGBA{0x10, 0x10, 0x10, 0xff}
	grid := color.RGBA{0x30, 0x30, 0x50, 0xff}
	col := 0
	for _, x := range line {
		for i := 0; i < x.cells*s.CellW; i++ {
			for y := 0; y < s.CellH; y++ {
				px := bg
				if i == 0 && x.cells > 0 {
					px = grid
				}
				img.Set(col*s.CellW+i, y, px)
			}
		}
		g := s.Glyph(x.text, x.style, x.cells)
		for y := 0; y < g.H; y++ {
			for i := 0; i < g.W; i++ {
				a := uint16(g.Pix[y*g.Stride+i])
				v := uint8((uint16(0xd0)*a + uint16(0x10)*(255-a)) / 255)
				img.Set(col*s.CellW+g.X+i, g.Y+y, color.RGBA{v, v, v, 0xff})
			}
		}
		col += x.cells
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
