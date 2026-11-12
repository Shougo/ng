// Package nvim starts or connects to a Neovim server and wraps the API
// calls ng uses.
//
// ng never embeds Neovim (--embed). It starts "nvim --headless --listen"
// as a separate process in its own session and attaches to the socket, so
// if the GUI crashes Neovim keeps running with every unsaved buffer, and
// "ng --server <socket>" attaches to it again.
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
	"syscall"
	"time"

	"github.com/Shougo/ng/internal/rpc"
)

// Nvim is a connection to a Neovim server.
type Nvim struct {
	*rpc.Client
	Addr string
	Proc *os.Process // nil when attached to an existing server

	// LogPath receives Neovim's stderr when ng started it. A headless
	// Neovim writes startup errors (e.g. from init.lua) there instead of
	// showing them; ReportStartupErrors shows them once a UI is attached.
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

// Start launches nvimPath with args as a headless server listening on a
// new socket and connects to it.
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
	cmdArgs := append([]string{"--headless", "--listen", addr}, args...)
	cmd := exec.Command(nvimPath, cmdArgs...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = logFile
	// A new session: closing the terminal ng was started from, or ng
	// itself dying, must not take Neovim down.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		os.Remove(logPath)
		return nil, fmt.Errorf("start %s: %w", nvimPath, err)
	}
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
		case <-time.After(20 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			return nil, fmt.Errorf("timed out waiting for %s", addr)
		}
	}
}

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
