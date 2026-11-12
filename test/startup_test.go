//go:build integration

package test

import (
	"strings"
	"testing"
	"time"

	"github.com/Shougo/ng/internal/nvim"
	"github.com/Shougo/ng/internal/screen"
)

// Errors from startup files must be visible, like Neovim in a terminal
// shows them: either Neovim shows them in the UI itself (it reads the
// config after the UI attached), or ng shows what went to stderr.
func TestStartupErrorIsShown(t *testing.T) {
	h := start(t, 60, 8, false, "--cmd", "call NgNoSuchFunction()")
	h.nv.ReportStartupErrors()
	h.waitFor("E117 on screen", func(s *screen.Screen) bool {
		return strings.Contains(s.Dump(), "E117")
	})
	// Neovim waits for Enter like in a terminal; the UI keeps working.
	h.input("<CR>ok<Esc>")
}

// Attaching to a Neovim that waits at a prompt must not freeze the GUI:
// Prepare reports the blocked state at once and finishes after the prompt
// is answered with keys sent meanwhile.
func TestAttachToBlockedServer(t *testing.T) {
	h := start(t, 60, 8, false)
	h.input(":let g:answer = input('ng-prompt> ')<CR>")
	h.waitFor("prompt", func(s *screen.Screen) bool { return strings.Contains(s.Dump(), "ng-prompt>") })
	addr := h.nv.Addr
	h.nv.Close() // the GUI "crashes" while the prompt is open

	h2 := &harness{t: t, scr: screen.New(60, 8), flushed: make(chan struct{}, 1)}
	nv, err := nvim.Connect(addr, h2.onNotify, nil)
	if err != nil {
		t.Fatal(err)
	}
	h2.nv = nv
	go nv.Serve()
	defer nv.Notify("nvim_command", "qall!")

	blocked := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- nv.Prepare("vim.g.ng = 1", 60, 8, func() { blocked <- struct{}{} })
	}()
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("blocked prompt not reported")
	}
	// Answer the prompt without seeing it, as the waiting message asks.
	if err := nv.Input("yes<CR>"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("attach did not finish after answering the prompt")
	}
	if v, err := nv.Call("nvim_get_var", "answer"); err != nil || v != "yes" {
		t.Errorf("g:answer = %#v, %v", v, err)
	}
	if v, err := nv.Call("nvim_get_var", "ng"); err != nil {
		t.Errorf("bootstrap did not run: %v", err)
	} else if n, ok := v.(int64); !ok || n != 1 {
		t.Errorf("g:ng = %#v", v)
	}
	h2.waitFor("screen after attach", func(s *screen.Screen) bool { return s.Width == 60 })
}

// init.vim must be able to detect ng: g:ng exists before the user's config
// is read, although the UI attaches only afterwards.
func TestGNgVisibleInConfig(t *testing.T) {
	h := start(t, 40, 6, false, "--cmd", "let g:seen_ng = exists('g:ng')")
	v, err := h.nv.Call("nvim_get_var", "seen_ng")
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := v.(int64); !ok || n != 1 {
		t.Errorf("exists('g:ng') during startup = %#v, want 1", v)
	}
}

// Like the TUI, Neovim must read the user's config only after ng's UI has
// attached: plugins such as vim._core.ui2 and anything that looks at
// nvim_list_uis(), &lines or &columns while starting depend on it.
func TestConfigRunsWithUIAttached(t *testing.T) {
	h := start(t, 61, 9, false,
		"-c", "let g:c_uis = len(nvim_list_uis())",
		"-c", "let g:c_size = [&columns, &lines]")
	var uis, size any
	for i := 0; i < 100; i++ {
		var err error
		if uis, err = h.nv.Call("nvim_get_var", "c_uis"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	size, _ = h.nv.Call("nvim_get_var", "c_size")
	if n, ok := uis.(int64); !ok || n != 1 {
		t.Errorf("UIs while reading the config: %#v, want 1", uis)
	}
	if s, ok := size.([]any); !ok || len(s) != 2 || s[0] != int64(61) || s[1] != int64(9) {
		t.Errorf("[&columns, &lines] while reading the config: %#v, want [61 9]", size)
	}
}

// vim._core.ui2 enabled from the config must work in ng as in the TUI:
// the command line it draws is highlighted.
func TestUI2CommandLineIsHighlighted(t *testing.T) {
	h := start(t, 60, 10, false,
		"-c", "lua vim.g.ui2_ok = pcall(function() require('vim._core.ui2').enable({}) end)")
	var ok any
	for i := 0; i < 100; i++ {
		var err error
		if ok, err = h.nv.Call("nvim_get_var", "ui2_ok"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ok != true {
		t.Skip("vim._core.ui2 is not available in this Neovim")
	}
	h.input(":echo 'ng'")
	h.waitFor("highlighted command line", func(s *screen.Screen) bool {
		for r := 0; r < s.Height; r++ {
			if !strings.Contains(row(s, r), "echo 'ng'") {
				continue
			}
			for c := 0; c < s.Width; c++ {
				if s.Cell(r, c).Text == "e" && s.Cell(r, c).HL != 0 {
					return true
				}
			}
		}
		return false
	})
	h.input("<Esc>")
}
