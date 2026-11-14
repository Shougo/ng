// Package kitty understands the part of the kitty graphics protocol that
// Neovim writes to a terminal, so that ng can show images the way a
// terminal does.
//
// Neovim's vim.ui.img (0.13) does not send images as UI events: it writes
// kitty graphics escape sequences with nvim_ui_send(), which reaches UIs
// that set the "stdout_tty" option as "ui_send" events. ng sets it and
// feeds those bytes to a Terminal. Besides the graphics commands
// (APC "G"), the Terminal follows the cursor movements Neovim wraps them in
// (DECSC, DECRC, CUP), and skips every other sequence: with stdout_tty, ng
// also receives queries and notifications meant for real terminals.
//
// Supported: transmission of PNG (f=100) and raw RGB/RGBA (f=24, f=32)
// data, directly (t=d) and in chunks (m=1); a=t, a=T, a=p, a=d (d=a/A,
// i/I), a=q. Placements are kept in cell coordinates and move with the
// text when the grid scrolls, as in kitty.
package kitty

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"sort"
	"strconv"
	"strings"
)

// Placement is an image placed on the grid.
type Placement struct {
	Image      uint32 // image id
	ID         uint32 // placement id (0 if the client gave none)
	Row, Col   int    // top-left cell, 0-based
	Cols, Rows int    // size in cells, 0 = from the image size
	Z          int    // stacking order; negative is meant to be below text
	order      uint64 // creation order, to break ties in Z
}

type key struct{ image, placement uint32 }

// Terminal holds the images and placements created by the bytes fed to it.
// It is not safe for concurrent use.
type Terminal struct {
	images     map[uint32]image.Image
	placements map[key]*Placement
	order      uint64

	row, col           int // virtual cursor, 0-based
	savedRow, savedCol int

	pending []byte // an incomplete escape sequence from the last Feed

	// the image being transmitted in chunks
	loading     bool
	loadControl map[string]string
	loadData    []byte
	loadQuiet   int

	responses []string
	changed   bool
}

// New returns an empty Terminal.
func New() *Terminal {
	return &Terminal{images: map[uint32]image.Image{}, placements: map[key]*Placement{}}
}

// Feed processes bytes written to the terminal. Escape sequences may be
// split across calls.
func (t *Terminal) Feed(data []byte) {
	buf := data
	if len(t.pending) > 0 {
		buf = append(t.pending, data...)
		t.pending = nil
	}
	for i := 0; i < len(buf); {
		if buf[i] != 0x1b {
			i++ // text written to the terminal: not ng's business
			continue
		}
		n, complete := t.escape(buf[i:])
		if !complete {
			t.pending = append([]byte(nil), buf[i:]...)
			return
		}
		i += n
	}
}

// escape handles the sequence at the start of b (b[0] == ESC) and returns
// its length, or complete == false if b ends before the sequence does.
func (t *Terminal) escape(b []byte) (n int, complete bool) {
	if len(b) < 2 {
		return 0, false
	}
	switch b[1] {
	case '7': // DECSC
		t.savedRow, t.savedCol = t.row, t.col
		return 2, true
	case '8': // DECRC
		t.row, t.col = t.savedRow, t.savedCol
		return 2, true
	case '[': // CSI: parameters, intermediates, a final byte 0x40-0x7e
		for i := 2; i < len(b); i++ {
			if b[i] >= 0x40 && b[i] <= 0x7e {
				t.csi(b[2:i], b[i])
				return i + 1, true
			}
		}
		return 0, false
	case '_', ']', 'P', '^', 'X': // APC, OSC, DCS, PM, SOS: up to ST (or BEL for OSC)
		for i := 2; i < len(b); i++ {
			if b[i] == 0x07 && b[1] == ']' {
				return i + 1, true
			}
			if b[i] == 0x1b {
				if i+1 >= len(b) {
					return 0, false
				}
				if b[i+1] == '\\' {
					if b[1] == '_' && i > 2 && b[2] == 'G' {
						t.graphics(b[3:i])
					}
					return i + 2, true
				}
			}
		}
		return 0, false
	default: // two-byte sequences and anything unknown
		return 2, true
	}
}

func (t *Terminal) csi(params []byte, final byte) {
	if final != 'H' && final != 'f' {
		return // ?25l / ?25h and every other control: nothing to do
	}
	row, col := 1, 1
	parts := strings.Split(string(params), ";")
	if v, err := strconv.Atoi(parts[0]); err == nil && v > 0 {
		row = v
	}
	if len(parts) > 1 {
		if v, err := strconv.Atoi(parts[1]); err == nil && v > 0 {
			col = v
		}
	}
	t.row, t.col = row-1, col-1
}

// graphics handles the body of an APC G command: "k=v,k=v;payload".
func (t *Terminal) graphics(body []byte) {
	ctrl, payload, _ := bytes.Cut(body, []byte{';'})
	c := map[string]string{}
	for _, kv := range strings.Split(string(ctrl), ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			c[k] = v
		}
	}

	// A continuation chunk carries only m (and perhaps q): it belongs to
	// the transmission in progress.
	if t.loading {
		t.loadData = append(t.loadData, payload...)
		if c["m"] != "1" {
			t.loading = false
			t.finishLoad(t.loadControl, t.loadData, t.loadQuiet)
			t.loadControl, t.loadData = nil, nil
		}
		return
	}

	action := c["a"]
	if action == "" {
		action = "t"
	}
	quiet := num(c, "q")
	switch action {
	case "t", "T":
		if c["m"] == "1" {
			t.loading, t.loadControl, t.loadQuiet = true, c, quiet
			t.loadData = append([]byte(nil), payload...)
			return
		}
		t.finishLoad(c, payload, quiet)
	case "p":
		t.place(c)
	case "d":
		t.delete(c)
	case "q":
		// Neovim asks whether images are supported (vim.ui.img._supported).
		// The data, if any, is not loaded: answering OK is enough.
		if quiet < 1 {
			t.respond(c, "OK")
		}
	}
}

func (t *Terminal) finishLoad(c map[string]string, payload []byte, quiet int) {
	img, err := decode(c, payload)
	id := uint32(num(c, "i"))
	if err != nil {
		if quiet < 2 {
			t.respond(c, "EINVAL:"+err.Error())
		}
		return
	}
	if id == 0 {
		return // kitty assigns numbers (I=); Neovim always gives an id
	}
	t.images[id] = img
	t.changed = true
	if quiet < 1 {
		t.respond(c, "OK")
	}
	if c["a"] == "T" {
		t.place(c)
	}
}

func decode(c map[string]string, payload []byte) (image.Image, error) {
	if m := c["t"]; m != "" && m != "d" {
		return nil, fmt.Errorf("transmission medium %q not supported", m)
	}
	if o := c["o"]; o != "" {
		return nil, fmt.Errorf("compression %q not supported", o)
	}
	raw := make([]byte, base64.StdEncoding.DecodedLen(len(payload)))
	n, err := base64.StdEncoding.Decode(raw, bytes.TrimRight(payload, "\r\n"))
	if err != nil {
		// Some writers drop the padding.
		n, err = base64.RawStdEncoding.Decode(raw, bytes.TrimRight(payload, "="))
		if err != nil {
			return nil, err
		}
	}
	raw = raw[:n]
	switch f := num(c, "f"); f {
	case 0, 32, 24:
		if f == 0 {
			f = 32
		}
		w, h := num(c, "s"), num(c, "v")
		bpp := f / 8
		if w <= 0 || h <= 0 || len(raw) < w*h*bpp {
			return nil, fmt.Errorf("raw image %dx%d needs %d bytes, got %d", w, h, w*h*bpp, len(raw))
		}
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for i := 0; i < w*h; i++ {
			copy(img.Pix[i*4:i*4+3], raw[i*bpp:i*bpp+3])
			if bpp == 4 {
				img.Pix[i*4+3] = raw[i*4+3]
			} else {
				img.Pix[i*4+3] = 0xff
			}
		}
		return img, nil
	case 100:
		return png.Decode(bytes.NewReader(raw))
	default:
		return nil, fmt.Errorf("format %d not supported", f)
	}
}

// place puts image i at the cursor, replacing the placement with the same
// (i, p): an update moves it without flicker.
func (t *Terminal) place(c map[string]string) {
	id := uint32(num(c, "i"))
	if _, ok := t.images[id]; !ok {
		if num(c, "q") < 2 {
			t.respond(c, "ENOENT:image not found")
		}
		return
	}
	k := key{id, uint32(num(c, "p"))}
	p := t.placements[k]
	if p == nil {
		t.order++
		p = &Placement{Image: id, ID: k.placement, order: t.order}
		t.placements[k] = p
	}
	p.Row, p.Col = t.row, t.col
	p.Cols, p.Rows, p.Z = num(c, "c"), num(c, "r"), num(c, "z")
	t.changed = true
}

func (t *Terminal) delete(c map[string]string) {
	d := c["d"]
	if d == "" {
		d = "a"
	}
	free := d == strings.ToUpper(d)
	switch strings.ToLower(d) {
	case "a": // all placements on screen
		for k := range t.placements {
			delete(t.placements, k)
		}
	case "i": // placements of image i (only placement p, if given)
		id, pid := uint32(num(c, "i")), uint32(num(c, "p"))
		for k := range t.placements {
			if k.image == id && (pid == 0 || k.placement == pid) {
				delete(t.placements, k)
			}
		}
	default:
		return // other selectors (by cell, z, ...) are not used by Neovim
	}
	if free {
		t.freeUnused()
	}
	t.changed = true
}

func (t *Terminal) freeUnused() {
	used := map[uint32]bool{}
	for k := range t.placements {
		used[k.image] = true
	}
	for id := range t.images {
		if !used[id] {
			delete(t.images, id)
		}
	}
}

func (t *Terminal) respond(c map[string]string, status string) {
	var b strings.Builder
	b.WriteString("\x1b_G")
	if i := c["i"]; i != "" {
		b.WriteString("i=" + i)
		if p := c["p"]; p != "" {
			b.WriteString(",p=" + p)
		}
	}
	b.WriteString(";" + status + "\x1b\\")
	t.responses = append(t.responses, b.String())
}

// TakeResponses returns the replies a terminal would write back (for
// nvim_ui_term_event "termresponse") and forgets them.
func (t *Terminal) TakeResponses() []string {
	r := t.responses
	t.responses = nil
	return r
}

// TakeChanged reports whether images or placements changed since the last
// call.
func (t *Terminal) TakeChanged() bool {
	c := t.changed
	t.changed = false
	return c
}

// Image returns the image with the given id.
func (t *Terminal) Image(id uint32) image.Image { return t.images[id] }

// Placements returns the placements in drawing order: by Z, then by age.
func (t *Terminal) Placements() []Placement {
	ps := make([]Placement, 0, len(t.placements))
	for _, p := range t.placements {
		ps = append(ps, *p)
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Z != ps[j].Z {
			return ps[i].Z < ps[j].Z
		}
		return ps[i].order < ps[j].order
	})
	return ps
}

// Scroll moves the placements whose top-left cell lies in the region
// [top, bot) x [left, right) up by rows (down if negative), as the text in
// it moves. Placements that leave the region are deleted, like the text.
func (t *Terminal) Scroll(top, bot, left, right, rows int) {
	if rows == 0 {
		return
	}
	for k, p := range t.placements {
		if p.Row < top || p.Row >= bot || p.Col < left || p.Col >= right {
			continue
		}
		p.Row -= rows
		if p.Row < top || p.Row >= bot {
			delete(t.placements, k)
		}
		t.changed = true
	}
}

// Clear deletes every placement (the images stay loaded), as when the
// whole screen is cleared.
func (t *Terminal) Clear() {
	if len(t.placements) > 0 {
		t.placements = map[key]*Placement{}
		t.changed = true
	}
}

func num(c map[string]string, k string) int {
	v, _ := strconv.Atoi(c[k])
	return v
}
