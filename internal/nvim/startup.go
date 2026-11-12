package nvim

import (
	"os"
	"strings"
)

// Prepare runs the bootstrap Lua and attaches the UI.
//
// It may block for a long time: a Neovim that is waiting for input (an
// input() or "Press ENTER" prompt, or a :confirm dialog left open by a GUI
// that crashed) handles ordinary requests only after the prompt is
// answered. The GUI therefore calls Prepare from a goroutine and keeps
// drawing and sending keys meanwhile. nvim_get_mode is answered even while
// Neovim waits, so onBlocked (if not nil) is called first in that case and
// the GUI can tell the user what is going on.
func (n *Nvim) Prepare(bootstrap string, cols, rows int, onBlocked func()) error {
	if blocked, err := n.Blocking(); err != nil {
		return err
	} else if blocked && onBlocked != nil {
		onBlocked()
	}
	if bootstrap != "" {
		if _, err := n.ExecLua(bootstrap); err != nil {
			return err
		}
	}
	return n.Attach(cols, rows)
}

// Blocking reports whether Neovim is waiting for input. nvim_get_mode is
// a "fast" function that Neovim answers even then.
func (n *Nvim) Blocking() (bool, error) {
	v, err := n.Call("nvim_get_mode")
	if err != nil {
		return false, err
	}
	m, _ := v.(map[string]any)
	b, _ := m["blocking"].(bool)
	return b, nil
}

// StartupErrors returns what Neovim wrote to stderr while starting and
// removes the log. It returns "" when ng did not start this Neovim.
func (n *Nvim) StartupErrors() string {
	if n.LogPath == "" {
		return ""
	}
	out, _ := os.ReadFile(n.LogPath)
	os.Remove(n.LogPath) // Neovim keeps writing to the unlinked file
	n.LogPath = ""
	return strings.TrimRight(string(out), "\n")
}

// ReportStartupErrors shows startup errors in Neovim itself, as an
// ErrorMsg with a "Press ENTER" prompt, as if Neovim had been started in a
// terminal. It must be called after the UI is attached. It reports
// whether there was anything to show.
func (n *Nvim) ReportStartupErrors() bool {
	msg := n.StartupErrors()
	if msg == "" {
		return false
	}
	var chunks []any
	for i, line := range strings.Split(msg, "\n") {
		if i > 0 {
			chunks = append(chunks, []any{"\n"})
		}
		chunks = append(chunks, []any{line, "ErrorMsg"})
	}
	n.Notify("nvim_echo", chunks, true, map[string]any{})
	return true
}
