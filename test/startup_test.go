//go:build integration

package test

import (
	"strings"
	"testing"
	"time"

	"github.com/Shougo/ng/internal/nvim"
	"github.com/Shougo/ng/internal/screen"
)

// A headless Neovim cannot show errors from its startup files. ng must
// show them after attaching, like Neovim in a terminal would.
func TestStartupErrorIsShown(t *testing.T) {
	h := start(t, 60, 8, false, "--cmd", "call NgNoSuchFunction()")
	if !h.nv.ReportStartupErrors() {
		t.Fatal("no startup errors captured")
	}
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
