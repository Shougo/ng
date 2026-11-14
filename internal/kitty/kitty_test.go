package kitty

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// The sequences below are what runtime/lua/vim/ui/img/_kitty.lua writes.

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x * 10), uint8(y * 10), 200, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func seq(control, payload string) string {
	s := "\x1b_G" + control
	if payload != "" {
		s += ";" + payload
	}
	return s + "\x1b\\"
}

// transmit splits like vim.ui.img: 4096 base64 bytes per chunk.
func transmit(id int, data []byte, chunk int) []string {
	enc := base64.StdEncoding.EncodeToString(data)
	var out []string
	for pos := 0; pos < len(enc); pos += chunk {
		end := min(pos+chunk, len(enc))
		m := "1"
		if end >= len(enc) {
			m = "0"
		}
		c := "m=" + m
		if pos == 0 {
			c = fmt.Sprintf("f=100,a=t,t=d,i=%d,q=2,m=%s", id, m)
		}
		out = append(out, seq(c, enc[pos:end]))
	}
	return out
}

func place(img, placement, row, col int, extra string) string {
	c := fmt.Sprintf("a=p,i=%d,p=%d,C=1,q=2", img, placement)
	if extra != "" {
		c += "," + extra
	}
	return "\x1b7\x1b[?25l" + fmt.Sprintf("\x1b[%d;%dH", row, col) + seq(c, "") + "\x1b8\x1b[?25h"
}

func feedAll(term *Terminal, parts ...string) {
	for _, p := range parts {
		term.Feed([]byte(p))
	}
}

func TestSetUpdateDelete(t *testing.T) {
	term := New()
	data := pngBytes(t, 40, 30)
	feedAll(term, transmit(31, data, 64)...) // many chunks
	if term.Image(31) == nil {
		t.Fatal("image 31 not loaded")
	}
	if b := term.Image(31).Bounds(); b.Dx() != 40 || b.Dy() != 30 {
		t.Fatalf("image size %v", b)
	}
	if len(term.Placements()) != 0 {
		t.Fatal("a=t must not display")
	}

	term.Feed([]byte(place(31, 32, 5, 10, "c=8,r=4,z=50")))
	ps := term.Placements()
	if len(ps) != 1 {
		t.Fatalf("placements %+v", ps)
	}
	want := Placement{Image: 31, ID: 32, Row: 4, Col: 9, Cols: 8, Rows: 4, Z: 50}
	if got := ps[0]; got.Image != want.Image || got.ID != want.ID || got.Row != want.Row ||
		got.Col != want.Col || got.Cols != want.Cols || got.Rows != want.Rows || got.Z != want.Z {
		t.Errorf("placement %+v, want %+v", got, want)
	}
	if !term.TakeChanged() || term.TakeChanged() {
		t.Error("TakeChanged")
	}

	// vim.ui.img.set(id, opts): the same ids, a new position.
	term.Feed([]byte(place(31, 32, 8, 12, "c=8,r=4,z=50")))
	ps = term.Placements()
	if len(ps) != 1 || ps[0].Row != 7 || ps[0].Col != 11 {
		t.Errorf("updated placement %+v", ps)
	}

	term.Feed([]byte(seq("a=d,d=i,i=31,q=2", "")))
	if len(term.Placements()) != 0 {
		t.Error("d=i left placements")
	}
	if len(term.TakeResponses()) != 0 {
		t.Error("q=2 must suppress responses")
	}
}

func TestDeleteAll(t *testing.T) {
	term := New()
	data := pngBytes(t, 4, 4)
	feedAll(term, transmit(1, data, 4096)...)
	feedAll(term, transmit(2, data, 4096)...)
	feedAll(term, place(1, 10, 1, 1, ""), place(2, 20, 3, 3, ""))
	term.Feed([]byte(seq("a=d,d=A,q=2", "")))
	if len(term.Placements()) != 0 || term.Image(1) != nil || term.Image(2) != nil {
		t.Error("d=A must delete placements and free images")
	}
}

// vim.ui.img._supported sends a=q without q and waits for i=<id>;OK.
func TestQuery(t *testing.T) {
	term := New()
	term.Feed([]byte(seq("a=q,i=4242,s=1,v=1", "")))
	r := term.TakeResponses()
	if len(r) != 1 || r[0] != "\x1b_Gi=4242;OK\x1b\\" {
		t.Errorf("responses %q", r)
	}
}

// Sequences may be split anywhere between ui_send events, and everything
// that is not a graphics command must be skipped: DA1 queries, OSC 9;4
// progress, kitty's multiple cursor shapes.
func TestSplitAndForeignSequences(t *testing.T) {
	term := New()
	data := pngBytes(t, 8, 8)
	stream := "\x1b[c" + "\x1b]9;4;1;50\x1b\\" + "\x1b]0;title\x07" + "\x1b[>0;4 q" + "plain text" +
		strings.Join(transmit(7, data, 4096), "") + place(7, 8, 2, 3, "")
	for i := 0; i < len(stream); i += 5 {
		term.Feed([]byte(stream[i:min(i+5, len(stream))]))
	}
	ps := term.Placements()
	if len(ps) != 1 || ps[0].Row != 1 || ps[0].Col != 2 {
		t.Errorf("placements %+v", ps)
	}
}

// Placements move with the text, and leave with it.
func TestScroll(t *testing.T) {
	term := New()
	feedAll(term, transmit(1, pngBytes(t, 4, 4), 4096)...)
	feedAll(term, place(1, 10, 5, 1, ""), place(1, 11, 2, 1, ""), place(1, 12, 5, 30, ""))
	term.Scroll(1, 10, 0, 20, 2) // rows 1..9, columns 0..19, up by 2
	got := map[uint32][2]int{}
	for _, p := range term.Placements() {
		got[p.ID] = [2]int{p.Row, p.Col}
	}
	if got[10] != [2]int{2, 0} {
		t.Errorf("placement in the region: %v, want row 2", got[10])
	}
	if _, ok := got[11]; ok {
		t.Errorf("placement scrolled out of the region still exists: %v", got[11])
	}
	if got[12] != [2]int{4, 29} {
		t.Errorf("placement outside the columns moved: %v", got[12])
	}
	term.Scroll(0, 10, 0, 40, -3) // down by 3
	for _, p := range term.Placements() {
		if p.ID == 10 && p.Row != 5 {
			t.Errorf("scrolled down: row %d, want 5", p.Row)
		}
	}
}

func TestRawRGBA(t *testing.T) {
	term := New()
	raw := base64.StdEncoding.EncodeToString([]byte{255, 0, 0, 255, 0, 255, 0, 128})
	term.Feed([]byte(seq("a=T,f=32,s=2,v=1,i=5,q=2", raw)))
	img := term.Image(5)
	if img == nil || img.Bounds().Dx() != 2 {
		t.Fatalf("raw image %v", img)
	}
	if r, g, _, _ := img.At(0, 0).RGBA(); r != 0xffff || g != 0 {
		t.Errorf("pixel 0 = %v", img.At(0, 0))
	}
	if len(term.Placements()) != 1 {
		t.Error("a=T must display")
	}
}

func TestBadDataReported(t *testing.T) {
	term := New()
	term.Feed([]byte(seq("a=t,f=100,i=9", base64.StdEncoding.EncodeToString([]byte("not a png")))))
	if term.Image(9) != nil {
		t.Error("garbage loaded")
	}
	if r := term.TakeResponses(); len(r) != 1 || !strings.Contains(r[0], "EINVAL") {
		t.Errorf("responses %q", r)
	}
}
