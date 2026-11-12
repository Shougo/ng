//go:build integration

// Integration tests run a real Neovim (nvim on $PATH, or $NG_TEST_NVIM)
// and check what ng's screen model and painter make of its redraw events.
// CI runs them against Neovim nightly every day so that protocol changes
// are noticed the day they land.
//
//	go test -tags integration ./test/
package test

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Shougo/ng/internal/font"
	"github.com/Shougo/ng/internal/nvim"
	"github.com/Shougo/ng/internal/paint"
	"github.com/Shougo/ng/internal/screen"
)

type harness struct {
	t   *testing.T
	nv  *nvim.Nvim
	mu  sync.Mutex
	scr *screen.Screen
	err error

	// Optional incremental painter: every flush repaints the dirty rows,
	// exactly like the GUI does.
	fonts *font.Set
	img   *paint.ImagePainter

	flushed   chan struct{}
	uiEntered chan struct{}
	setSize   chan []any
	vars      chan []any
}

func start(t *testing.T, cols, rows int, withPaint bool, args ...string) *harness {
	t.Helper()
	bin := os.Getenv("NG_TEST_NVIM")
	if bin == "" {
		bin = "nvim"
	}
	h := &harness{t: t, scr: screen.New(cols, rows), flushed: make(chan struct{}, 1),
		uiEntered: make(chan struct{}, 1), setSize: make(chan []any, 16),
		vars: make(chan []any, 16)}
	if withPaint {
		f, err := font.NewSet("Courier 10 Pitch,DejaVu Sans Mono,monospace:h11", "", 96, 0)
		if err != nil {
			t.Skipf("no font: %v", err)
		}
		t.Cleanup(f.Close)
		h.fonts = f
		h.img = paint.NewImagePainter(cols*f.CellW, rows*f.CellH)
	}
	nv, err := nvim.Start(bin, "", append([]string{"--clean", "-n", "-i", "NONE"}, args...), h.onNotify, nil)
	if err != nil {
		t.Skipf("cannot start nvim: %v", err)
	}
	h.nv = nv
	go nv.Serve()
	t.Cleanup(func() {
		nv.StartupErrors() // removes the stderr log
		nv.Notify("nvim_command", "qall!")
		select {
		case <-nv.Done():
		case <-time.After(3 * time.Second):
			nv.Proc.Kill()
		}
	})
	// The same startup path as the GUI: bootstrap Lua, then attach.
	if err := nv.Prepare(nvim.Bootstrap, cols, rows, nil); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *harness) onNotify(method string, params []any) {
	if method == nvim.VarNotification {
		if h.vars != nil {
			h.vars <- params
		}
		return
	}
	if method == nvim.SetSizeNotification {
		if h.setSize != nil {
			h.setSize <- params
		}
		return
	}
	if method == nvim.UIEnterNotification {
		select {
		case h.uiEntered <- struct{}{}:
		default:
		}
		return
	}
	if method != "redraw" {
		return
	}
	h.mu.Lock()
	flush, err := h.scr.HandleRedraw(params)
	if err != nil && h.err == nil {
		h.err = err
	}
	if flush && h.img != nil {
		for _, r := range h.scr.TakeDirty() {
			paint.Row(h.img, h.scr, h.fonts, r)
		}
	}
	h.mu.Unlock()
	if flush {
		select {
		case h.flushed <- struct{}{}:
		default:
		}
	}
}

// waitFor waits until cond holds after a flush.
func (h *harness) waitFor(what string, cond func(s *screen.Screen) bool) {
	h.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		h.mu.Lock()
		ok := cond(h.scr)
		err := h.err
		dump := h.scr.Dump()
		h.mu.Unlock()
		if err != nil {
			h.t.Fatal(err)
		}
		if ok {
			return
		}
		select {
		case <-h.flushed:
		case <-deadline:
			h.t.Fatalf("timed out waiting for %s; screen:\n%s", what, dump)
		}
	}
}

func row(s *screen.Screen, r int) string {
	return strings.Split(s.Dump(), "\n")[r]
}

func (h *harness) input(keys string) {
	h.t.Helper()
	if _, err := h.nv.Call("nvim_input", keys); err != nil {
		h.t.Fatal(err)
	}
}

func TestTyping(t *testing.T) {
	h := start(t, 40, 6, false)
	h.input("ihello<lt>world<Esc>")
	h.waitFor("typed text", func(s *screen.Screen) bool { return row(s, 0) == "hello<world" })
}

func TestDoubleWidth(t *testing.T) {
	h := start(t, 40, 6, false)
	h.input("iあいう漢字x<Esc>")
	h.waitFor("japanese text", func(s *screen.Screen) bool {
		return row(s, 0) == "あいう漢字x" && s.CellWidth(0, 0) == 2 &&
			s.CellWidth(0, 1) == 0 && s.CellWidth(0, 10) == 1
	})
}

// The neovim-qt bug, end to end: text shaped like SKK's "▽かんじ" is
// replaced by "漢字" in insert mode. After Neovim's incremental redraw, the
// incrementally painted image must equal a fresh paint of the final screen.
func TestConversionRepaintHasNoFragments(t *testing.T) {
	h := start(t, 30, 5, true)
	h.input("i▽かんじです")
	h.waitFor("preedit-like text", func(s *screen.Screen) bool { return row(s, 0) == "▽かんじです" })
	// Replace the first four characters the way a plugin would.
	if _, err := h.nv.ExecLua(`vim.api.nvim_buf_set_text(0, 0, 0, 0, #"▽かんじ", {"漢字"})`); err != nil {
		t.Fatal(err)
	}
	h.waitFor("converted text", func(s *screen.Screen) bool { return row(s, 0) == "漢字です" })

	h.mu.Lock()
	defer h.mu.Unlock()
	want := paint.NewImagePainter(h.img.Img.Bounds().Dx(), h.img.Img.Bounds().Dy())
	for r := 0; r < h.scr.Height; r++ {
		paint.Row(want, h.scr, h.fonts, r)
	}
	if !bytes.Equal(h.img.Img.Pix, want.Img.Pix) {
		t.Error("incremental repaint differs from a full repaint")
	}
}

func TestScrollRepaintHasNoFragments(t *testing.T) {
	h := start(t, 30, 6, true)
	h.input(":call setline(1, map(range(1, 50), {_, v -> v .. ' 日本語テキスト ' .. v}))<CR>")
	h.waitFor("lines", func(s *screen.Screen) bool { return strings.HasPrefix(row(s, 0), "1 日本語") })
	h.input("<C-e><C-e><C-d>")
	h.waitFor("scrolled", func(s *screen.Screen) bool { return !strings.HasPrefix(row(s, 0), "1 ") })
	h.mu.Lock()
	defer h.mu.Unlock()
	want := paint.NewImagePainter(h.img.Img.Bounds().Dx(), h.img.Img.Bounds().Dy())
	for r := 0; r < h.scr.Height; r++ {
		paint.Row(want, h.scr, h.fonts, r)
	}
	if !bytes.Equal(h.img.Img.Pix, want.Img.Pix) {
		t.Error("incremental repaint after scrolling differs from a full repaint")
	}
}

func TestModesAndOptions(t *testing.T) {
	h := start(t, 40, 6, false)
	h.input("i")
	h.waitFor("insert mode", func(s *screen.Screen) bool { return s.ModeName == "insert" })
	h.mu.Lock()
	shape := h.scr.CurrentMode().Shape
	h.mu.Unlock()
	if shape != screen.CursorVertical {
		t.Errorf("insert cursor shape %v, want vertical", shape)
	}
	h.input("<Esc>:set guifont=Courier\\ 10\\ Pitch:h12<CR>")
	h.waitFor("guifont option", func(s *screen.Screen) bool {
		return s.OptionString("guifont") == "Courier 10 Pitch:h12"
	})
}

func TestResize(t *testing.T) {
	h := start(t, 40, 6, false)
	h.nv.TryResize(50, 8)
	h.waitFor("resize", func(s *screen.Screen) bool { return s.Width == 50 && s.Height == 8 })
}

// GUI crash safety: the server outlives a dropped connection and a new
// UI can attach and see the same buffer.
func TestReattachAfterDisconnect(t *testing.T) {
	h := start(t, 40, 6, false)
	h.input("ikeep me<Esc>")
	h.waitFor("text", func(s *screen.Screen) bool { return row(s, 0) == "keep me" })
	h.nv.Close()

	h2 := &harness{t: t, scr: screen.New(40, 6), flushed: make(chan struct{}, 1)}
	var nv *nvim.Nvim
	var err error
	for i := 0; i < 50; i++ {
		if nv, err = nvim.Connect(h.nv.Addr, h2.onNotify, nil); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	h2.nv = nv
	go nv.Serve()
	defer nv.Notify("nvim_command", "qall!")
	if err := nv.Attach(40, 6); err != nil {
		t.Fatal(err)
	}
	h2.waitFor("text after reattach", func(s *screen.Screen) bool { return row(s, 0) == "keep me" })
}

// Ctrl+Shift+V pastes the clipboard: the text arrives as is, in insert
// mode and on the command line, without mappings or auto-indent getting in
// the way, and never ahead of keys typed before it.
func TestPaste(t *testing.T) {
	h := start(t, 40, 6, false, "--cmd", "set autoindent", "--cmd", "inoremap x XXX")
	h.input("i  ")
	if err := h.nv.Paste("こんにちは x\r\nworld"); err != nil {
		t.Fatal(err)
	}
	h.input("<Esc>")
	h.waitFor("pasted text", func(s *screen.Screen) bool {
		return row(s, 0) == "  こんにちは x" && row(s, 1) == "world"
	})
	// Keys and paste sent back to back, as when typing fast: the paste
	// must land on the command line opened by the keys before it.
	h.input(":let g:p = '")
	if err := h.nv.Paste("pasted"); err != nil {
		t.Fatal(err)
	}
	h.input("'<CR>")
	for i := 0; i < 100; i++ {
		if v, err := h.nv.Call("nvim_get_var", "p"); err == nil {
			if v != "pasted" {
				t.Errorf("g:p = %#v", v)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("paste on the command line did not arrive")
}
