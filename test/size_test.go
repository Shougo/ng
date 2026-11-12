//go:build integration

package test

import (
	"testing"
	"time"

	"github.com/Shougo/ng/internal/screen"
)

// ":set columns=… lines=…" tells ng the size the user wants, so that the
// window can follow it like GVim's.
func TestSetSizeNotifies(t *testing.T) {
	h := start(t, 40, 10, false)
	h.input(":set columns=50 lines=12<CR>")
	deadline := time.After(5 * time.Second)
	for {
		select {
		case p := <-h.setSize:
			if len(p) == 2 && p[0] == int64(50) && p[1] == int64(12) {
				h.waitFor("grid resized by Neovim", func(s *screen.Screen) bool {
					return s.Width == 50 && s.Height == 12
				})
				return
			}
		case <-deadline:
			t.Fatal("no ng_set_size notification with 50x12")
		}
	}
}

// Resizes that come from ng itself must not be reported back, or ng and
// Neovim would keep resizing each other.
func TestTryResizeDoesNotNotify(t *testing.T) {
	h := start(t, 40, 10, false)
	h.nv.TryResize(45, 11)
	h.waitFor("resize", func(s *screen.Screen) bool { return s.Width == 45 && s.Height == 11 })
	select {
	case p := <-h.setSize:
		t.Errorf("nvim_ui_try_resize produced ng_set_size %v", p)
	case <-time.After(300 * time.Millisecond):
	}
}

// A size set in the config is in the grid by the time the UI has entered,
// which is when ng sizes and shows its window.
func TestConfigSizeIsInGridAtUIEnter(t *testing.T) {
	h := start(t, 40, 10, false, "-c", "set columns=55 lines=13")
	select {
	case <-h.uiEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("no UIEnter")
	}
	h.waitFor("grid sized by the config", func(s *screen.Screen) bool {
		return s.Width == 55 && s.Height == 13
	})
}
