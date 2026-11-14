//go:build integration

package test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/Shougo/ng/internal/kitty"
	"github.com/Shougo/ng/internal/screen"
)

// vim.ui.img (Neovim 0.13) writes kitty graphics sequences to the UIs that
// set stdout_tty. ng must answer its support query, place the image where
// it was asked to, move it with the text when the window scrolls, and
// remove it.
func TestUIImg(t *testing.T) {
	h := start(t, 40, 12, false)
	if ok, _ := h.nv.Call("nvim_exec_lua", "return vim.ui.img ~= nil", []any{}); ok != true {
		t.Skip("this Neovim has no vim.ui.img (0.13 or later)")
	}
	h.waitFor("first screen", func(s *screen.Screen) bool { return row(s, 1) == "~" })

	lines := []any{}
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	if _, err := h.nv.Call("nvim_buf_set_lines", 0, 0, -1, false, lines); err != nil {
		t.Fatal(err)
	}

	supported, err := h.nv.Call("nvim_exec_lua", "return vim.ui.img._supported({ timeout = 3000 })", []any{})
	if err != nil {
		t.Fatal(err)
	}
	if supported != true {
		t.Errorf("vim.ui.img._supported() = %v: ng did not answer the kitty query", supported)
	}

	id, err := h.nv.Call("nvim_exec_lua",
		"return vim.ui.img.set(..., { row = 3, col = 5, width = 6, height = 2, zindex = 10 })",
		[]any{testPNG(t, 12, 8)})
	if err != nil {
		t.Fatal(err)
	}
	placed := func(want kitty.Placement) func(s *screen.Screen) bool {
		return func(s *screen.Screen) bool {
			ps := s.Graphics.Placements()
			return len(ps) == 1 && ps[0].Row == want.Row && ps[0].Col == want.Col &&
				ps[0].Cols == want.Cols && ps[0].Rows == want.Rows && ps[0].Z == want.Z &&
				s.Graphics.Image(ps[0].Image) != nil
		}
	}
	h.waitFor("the image at row 3, column 5", placed(kitty.Placement{Row: 2, Col: 4, Cols: 6, Rows: 2, Z: 10}))
	h.mu.Lock()
	img := h.scr.Graphics.Image(h.scr.Graphics.Placements()[0].Image)
	h.mu.Unlock()
	if b := img.Bounds(); b.Dx() != 12 || b.Dy() != 8 {
		t.Errorf("image is %v, want 12x8", b)
	}

	// Moving it keeps the same placement.
	if _, err := h.nv.Call("nvim_exec_lua", "vim.ui.img.set(..., { row = 6 })", []any{id}); err != nil {
		t.Fatal(err)
	}
	h.waitFor("the image moved to row 6", placed(kitty.Placement{Row: 5, Col: 4, Cols: 6, Rows: 2, Z: 10}))

	// The text scrolls up by two lines; the image goes with it.
	h.input("2<C-e>")
	h.waitFor("the text scrolled", func(s *screen.Screen) bool { return row(s, 0) == "line 3" })
	h.waitFor("the image scrolled with the text", placed(kitty.Placement{Row: 3, Col: 4, Cols: 6, Rows: 2, Z: 10}))

	if _, err := h.nv.Call("nvim_exec_lua", "return vim.ui.img.del(...)", []any{id}); err != nil {
		t.Fatal(err)
	}
	h.waitFor("the image deleted", func(s *screen.Screen) bool { return len(s.Graphics.Placements()) == 0 })
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x * 20), uint8(y * 30), 100, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
