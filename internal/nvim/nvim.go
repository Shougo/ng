// Package nvim starts or connects to a Neovim server and wraps the API
// calls ng uses.
//
// ng starts "nvim --embed --listen <socket>" as a separate process in its
// own session and attaches to the socket.
//
// --embed makes Neovim wait until a UI has attached before it reads the
// user's config, exactly like the builtin TUI. Things that look at the UI
// while starting (vim._core.ui2, &lines/&columns, nvim_list_uis(), ...)
// therefore behave as in a terminal. ("--headless --listen" would read the
// config with no UI at all.)
//
// An embedded Neovim normally exits when its stdin closes. ng gives it a
// pipe whose write end Neovim itself also holds, so stdin never closes:
// if the GUI crashes Neovim keeps running with every unsaved buffer, and
// "ng --server" attaches to it again.
package nvim

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Shougo/ng/internal/rpc"
)

// Nvim is a connection to a Neovim server.
type Nvim struct {
	*rpc.Client
	Addr string
	Proc *os.Process // nil when attached to an existing server

	// LogPath receives Neovim's stderr when ng started it. Errors Neovim
	// cannot show in a UI (for example before one attached) end up there;
	// ReportStartupErrors shows them once the UI is attached.
	LogPath string
}

// SocketPath returns a fresh socket path in $XDG_RUNTIME_DIR (or the
// temporary directory).
func SocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	var b [4]byte
	rand.Read(b[:])
	return filepath.Join(dir, fmt.Sprintf("ng.%d.%s.sock", os.Getpid(), hex.EncodeToString(b[:])))
}

// Start launches nvimPath with args as an embedded server listening on a
// new socket and connects to it. Neovim reads the user's config only after
// Attach (see the package documentation).
func Start(nvimPath string, args []string, onNotify rpc.NotificationHandler, onRequest rpc.RequestHandler) (*Nvim, error) {
	addr := SocketPath()
	logPath := strings.TrimSuffix(addr, ".sock") + ".log"
	// A file, not a pipe: Neovim must be able to write to its stderr even
	// after ng is gone.
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	defer logFile.Close() // Neovim keeps its own descriptor
	// g:ng is set with --cmd, before the user's config runs, so that
	// "if exists('g:ng')" works in init.vim. (The bootstrap Lua sets it
	// too, for servers ng did not start.)
	cmdArgs := append([]string{"--embed", "--listen", addr, "--cmd", "let g:ng = 1"}, args...)
	cmd := exec.Command(nvimPath, cmdArgs...)
	// stdin is --embed's RPC channel, unused by ng. Neovim inherits the
	// write end too (as fd 3), so the channel never sees EOF and Neovim
	// does not exit when ng goes away.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		os.Remove(logPath)
		return nil, err
	}
	defer stdinR.Close()
	cmd.Stdin = stdinR
	cmd.ExtraFiles = []*os.File{stdinW}
	cmd.Stdout = nil // nothing is ever sent on the stdio channel
	cmd.Stderr = logFile
	// A new session: closing the terminal ng was started from, or ng
	// itself dying, must not take Neovim down.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		stdinW.Close()
		os.Remove(logPath)
		return nil, fmt.Errorf("start %s: %w", nvimPath, err)
	}
	// ng keeps its copy of the write end open for its whole life; Neovim's
	// own copy covers the time after ng is gone.
	keepAlive = append(keepAlive, stdinW)
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, err := dial(addr)
		if err == nil {
			n := &Nvim{Client: rpc.NewClient(conn, onNotify, onRequest),
				Addr: addr, Proc: cmd.Process, LogPath: logPath}
			return n, nil
		}
		select {
		case err := <-exited:
			out, _ := os.ReadFile(logPath)
			os.Remove(logPath)
			if msg := strings.TrimSpace(string(out)); msg != "" {
				return nil, fmt.Errorf("%s exited before listening: %v\n%s", nvimPath, err, msg)
			}
			return nil, fmt.Errorf("%s exited before listening: %v", nvimPath, err)
		case <-time.After(2 * time.Millisecond): // startup time matters
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			return nil, fmt.Errorf("timed out waiting for %s", addr)
		}
	}
}

// keepAlive holds the write ends of embedded Neovims' stdin pipes.
var keepAlive []*os.File

// Connect attaches to an existing server. addr is a Unix socket path or
// host:port.
func Connect(addr string, onNotify rpc.NotificationHandler, onRequest rpc.RequestHandler) (*Nvim, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Nvim{Client: rpc.NewClient(conn, onNotify, onRequest), Addr: addr}, nil
}

func dial(addr string) (net.Conn, error) {
	if strings.Contains(addr, "/") || !strings.Contains(addr, ":") {
		return net.Dial("unix", addr)
	}
	return net.Dial("tcp", addr)
}

// Attach registers ng as a UI. ng only uses ext_linegrid: every other
// part of the UI (cmdline, messages, popup menu, tabline, windows) is drawn
// by Neovim itself, exactly as in the terminal.
func (n *Nvim) Attach(cols, rows int) error {
	_, err := n.Call("nvim_ui_attach", cols, rows, map[string]any{
		"rgb":          true,
		"ext_linegrid": true,
	})
	return err
}

// TryResize asks Neovim to resize the grid. It does not wait.
func (n *Nvim) TryResize(cols, rows int) error {
	return n.Notify("nvim_ui_try_resize", cols, rows)
}

// Input sends keys in Neovim's key notation. It does not wait.
func (n *Nvim) Input(keys string) error {
	if keys == "" {
		return nil
	}
	return n.Notify("nvim_input", keys)
}

// InputMouse sends a mouse event. It does not wait.
func (n *Nvim) InputMouse(button, action, modifier string, row, col int) error {
	return n.Notify("nvim_input_mouse", button, action, modifier, 0, row, col)
}

// Paste inserts text as a terminal's bracketed paste does: as is, in any
// mode, without mappings or auto-indent (it uses vim.paste(), like
// nvim_paste).
//
// It must not overtake keys typed before it, nor fall behind keys typed
// after it. nvim_input only queues keys, while a direct nvim_paste request
// runs as soon as it arrives, so the text is stored in a variable first
// and the paste itself goes through the input queue as a <Cmd> key. On
// the command line vim.paste() would queue the text again (behind later
// keys), so ng inserts the first line at the cursor with setcmdline()
// instead, like vim.paste() pastes only one line there. Paste waits for
// Neovim to store the text; call it from a goroutine.
func (n *Nvim) Paste(text string) error {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	name := fmt.Sprintf("ng_paste_%d", pasteSeq.Add(1))
	if _, err := n.Call("nvim_set_var", name, text); err != nil {
		return err
	}
	return n.Input("<Cmd>lua require'ng_paste'('" + name + "')<CR>")
}

// pasteLua defines the function the <Cmd> key of Paste calls. It is
// installed by the bootstrap Lua as package.loaded.ng_paste.
const pasteLua = `
package.loaded.ng_paste = function(name)
  local text = vim.g[name]
  vim.g[name] = nil
  if not text then return end
  local lines = vim.split(text, '\n', { plain = true })
  if vim.fn.getcmdtype() ~= '' then
    local line, pos = vim.fn.getcmdline(), vim.fn.getcmdpos()
    vim.fn.setcmdline(line:sub(1, pos - 1) .. lines[1] .. line:sub(pos), pos + #lines[1])
    return
  end
  vim.paste(lines, -1)
end`

// UIEnterNotification is sent to ng when its UI has entered, that is after
// Neovim has finished starting (the user's config has run) or, for a
// running server, right after attaching. ng shows its window at the next
// flush, with the fonts and size the config chose.
const UIEnterNotification = "ng_ui_enter"

const uiEnterLua = `
local chan = vim.api.nvim_get_api_info()[1]
vim.api.nvim_create_autocmd('UIEnter', {
  once = true,
  callback = function() vim.rpcnotify(chan, '` + UIEnterNotification + `') end,
})`

// Bootstrap is the Lua ng runs in Neovim before attaching.
const Bootstrap = "vim.g.ng = 1\n" + pasteLua + "\n" + uiEnterLua

var pasteSeq atomic.Int64

// SetFocus tells Neovim whether the window has focus (FocusGained/Lost).
func (n *Nvim) SetFocus(focused bool) error {
	return n.Notify("nvim_ui_set_focus", focused)
}

// ExecLua runs Lua code with arguments and returns its result.
func (n *Nvim) ExecLua(code string, args ...any) (any, error) {
	if args == nil {
		args = []any{}
	}
	return n.Call("nvim_exec_lua", code, args)
}

// Command runs an Ex command without waiting for it, through the input
// queue, so that prompts it opens (e.g. "confirm qall") are shown in the UI
// instead of blocking the RPC channel.
func (n *Nvim) Command(cmd string) error {
	return n.Input("<Cmd>" + strings.ReplaceAll(cmd, "<", "<lt>") + "<CR>")
}
