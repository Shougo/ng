//go:build integration

package test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Shougo/ng/internal/screen"
)

// Neovim's own idea of the screen. ng's screen model must agree with it
// cell by cell: that is what "behaves like the TUI" means.
const screenLua = `
local out = {}
for r = 1, vim.o.lines do
  local t = {}
  for c = 1, vim.o.columns do t[c] = vim.fn.screenstring(r, c) end
  out[r] = t
end
return out`

// matchesNvim waits until ng's screen model equals Neovim's screen.
func (h *harness) matchesNvim(what string) {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var diff string
	for {
		v, err := h.nv.ExecLua(screenLua)
		if err != nil {
			h.t.Fatalf("%s: %v", what, err)
		}
		rows, _ := v.([]any)
		h.mu.Lock()
		diff = h.compare(rows)
		h.mu.Unlock()
		if diff == "" {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("%s: ng's screen differs from Neovim's:\n%s", what, diff)
		}
		select {
		case <-h.flushed:
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// compare returns a description of the differences, or "".
func (h *harness) compare(rows []any) string {
	s := h.scr
	if len(rows) != s.Height {
		return fmt.Sprintf("height: nvim %d, ng %d", len(rows), s.Height)
	}
	var b strings.Builder
	for r, rv := range rows {
		cols, _ := rv.([]any)
		if len(cols) != s.Width {
			return fmt.Sprintf("width: nvim %d, ng %d", len(cols), s.Width)
		}
		var want, got strings.Builder
		bad := false
		for c, cv := range cols {
			nv, _ := cv.(string)
			cell := s.Cell(r, c).Text
			if cell == "" {
				// Right half of a double-width character in ng; Neovim
				// may report it as "" or as the character itself.
				continue
			}
			if nv == "" {
				nv = " "
			}
			want.WriteString(nv)
			got.WriteString(cell)
			if nv != cell {
				bad = true
			}
		}
		if bad {
			fmt.Fprintf(&b, "row %d\n  nvim: %q\n  ng:   %q\n", r, want.String(), got.String())
		}
	}
	return b.String()
}

// cmdheight=0: entering the command line must move the statusline up, as
// in the TUI, and command line completion must look the same.
func TestCmdheightZeroMatchesNeovim(t *testing.T) {
	for _, stl := range []string{
		"STATUS",
		`%{'\ '->repeat(winwidth(0))}`, // a blank statusline
	} {
		t.Run(stl, func(t *testing.T) {
			h := start(t, 40, 10, false,
				"--cmd", "set cmdheight=0 laststatus=2",
				"--cmd", "set statusline="+stl)
			h.matchesNvim("initial screen")
			if stl == "STATUS" {
				h.waitFor("statusline on the last row", func(s *screen.Screen) bool {
					return strings.HasPrefix(row(s, 9), "STATUS")
				})
			}

			h.input(":")
			h.matchesNvim("after entering the command line")
			if stl == "STATUS" {
				h.waitFor("statusline moved up", func(s *screen.Screen) bool {
					return strings.HasPrefix(row(s, 8), "STATUS") && strings.HasPrefix(row(s, 9), ":")
				})
			}

			h.input("set filet")
			h.matchesNvim("while typing")
			h.input("<Tab>")
			h.matchesNvim("completion menu")
			h.input("<Tab><Tab>")
			h.matchesNvim("cycling completions")
			h.input("<Esc>")
			h.matchesNvim("after leaving the command line")
			if stl == "STATUS" {
				h.waitFor("statusline back on the last row", func(s *screen.Screen) bool {
					return strings.HasPrefix(row(s, 9), "STATUS")
				})
			}
		})
	}
}

// The same check for ordinary editing, scrolling and splits.
func TestEditingMatchesNeovim(t *testing.T) {
	h := start(t, 50, 12, false)
	h.input(":call setline(1, map(range(1, 60), {_, v -> v .. ' 日本語 ▽かな ' .. v}))<CR>")
	h.matchesNvim("text")
	h.input("<C-d>")
	h.matchesNvim("scroll down")
	h.input(":vsplit<CR>")
	h.matchesNvim("vertical split")
	h.input("<C-e><C-e>")
	h.matchesNvim("scroll in a split")
	h.input("ggi漢字<Esc>")
	h.matchesNvim("insert in a split")
	h.input(":split<CR>:resize 3<CR>")
	h.matchesNvim("horizontal split")
}
