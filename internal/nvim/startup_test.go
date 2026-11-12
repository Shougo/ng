package nvim

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Shougo/ng/internal/msgpack"
	"github.com/Shougo/ng/internal/rpc"
)

// blockedNvim imitates a Neovim waiting at a prompt: nvim_get_mode is
// answered at once (it is a fast function), every other request only after
// an nvim_input notification arrives.
type blockedNvim struct {
	mu       sync.Mutex
	blocked  bool
	held     []any // request ids waiting for the prompt
	requests []string
}

func (b *blockedNvim) serve(t *testing.T, conn net.Conn) {
	dec, enc := msgpack.NewDecoder(conn), msgpack.NewEncoder(conn)
	var wmu sync.Mutex
	reply := func(id any) {
		wmu.Lock()
		enc.Encode([]any{int64(1), id, nil, nil})
		wmu.Unlock()
	}
	for {
		v, err := dec.Decode()
		if err != nil {
			return
		}
		msg := v.([]any)
		kind, _ := msgpack.ToInt(msg[0])
		b.mu.Lock()
		switch {
		case kind == 2 && msg[1] == "nvim_input":
			b.blocked = false
			held := b.held
			b.held = nil
			b.mu.Unlock()
			for _, id := range held {
				reply(id)
			}
			continue
		case kind == 0 && msg[2] == "nvim_get_chan_info":
			b.requests = append(b.requests, "nvim_get_chan_info")
			wmu.Lock()
			enc.Encode([]any{int64(1), msg[1], nil, map[string]any{"id": int64(3)}})
			wmu.Unlock()
			if args, _ := msg[3].([]any); len(args) != 1 || args[0] != int64(0) {
				t.Errorf("nvim_get_chan_info args %#v, want [0]", msg[3])
			}
		case kind == 0 && msg[2] == "nvim_exec_lua":
			// The bootstrap gets ng's channel id as its argument.
			if args, _ := msg[3].([]any); len(args) != 2 || !reflect.DeepEqual(args[1], []any{int64(3)}) {
				t.Errorf("nvim_exec_lua args %#v, want the channel id 3", msg[3])
			}
			b.requests = append(b.requests, "nvim_exec_lua")
			if b.blocked {
				b.held = append(b.held, msg[1])
			} else {
				reply(msg[1])
			}
		case kind == 0 && msg[2] == "nvim_get_mode":
			wmu.Lock()
			enc.Encode([]any{int64(1), msg[1], nil, map[string]any{"mode": "n", "blocking": b.blocked}})
			wmu.Unlock()
		case kind == 0:
			b.requests = append(b.requests, msg[2].(string))
			if b.blocked {
				b.held = append(b.held, msg[1])
			} else {
				reply(msg[1])
			}
		}
		b.mu.Unlock()
	}
}

func newFake(t *testing.T, b *blockedNvim) *Nvim {
	a, c := net.Pipe()
	go b.serve(t, c)
	n := &Nvim{Client: rpc.NewClient(a, nil, nil)}
	go n.Serve()
	t.Cleanup(func() { n.Close(); c.Close() })
	return n
}

func TestPrepareWhenBlocked(t *testing.T) {
	b := &blockedNvim{blocked: true}
	n := newFake(t, b)
	notified := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- n.Prepare("vim.g.ng = 1", 80, 24, func() { notified <- struct{}{} })
	}()
	select {
	case <-notified:
	case <-time.After(2 * time.Second):
		t.Fatal("onBlocked not called")
	}
	select {
	case err := <-done:
		t.Fatalf("Prepare returned while Neovim was blocked: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	// The user answers the prompt; keys are sent while Prepare waits.
	if err := n.Input("<CR>"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Prepare did not finish after the prompt was answered")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if want := []string{"nvim_get_chan_info", "nvim_exec_lua", "nvim_ui_attach"}; !reflect.DeepEqual(b.requests, want) {
		t.Errorf("requests %v, want %v", b.requests, want)
	}
}

func TestPrepareNotBlocked(t *testing.T) {
	n := newFake(t, &blockedNvim{})
	called := false
	if err := n.Prepare("", 80, 24, func() { called = true }); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("onBlocked called for a Neovim that was not blocked")
	}
}

func TestStartupErrors(t *testing.T) {
	log := filepath.Join(t.TempDir(), "ng.1.x.log")
	os.WriteFile(log, []byte("Error detected while processing init.lua:\nE5113: boom\n"), 0o600)
	n := &Nvim{LogPath: log}
	if got := n.StartupErrors(); got != "Error detected while processing init.lua:\nE5113: boom" {
		t.Errorf("got %q", got)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Error("log not removed")
	}
	if n.StartupErrors() != "" {
		t.Error("errors reported twice")
	}
	if (&Nvim{}).StartupErrors() != "" {
		t.Error("attached server has no startup log")
	}
}

// Some API functions exist only for RPC clients and are nil in Lua
// (vim.api.nvim_get_api_info broke ng's startup once). The Lua ng sends to
// Neovim must not use them.
func TestLuaUsesNoRemoteOnlyFunctions(t *testing.T) {
	remoteOnly := []string{
		"nvim_get_api_info", "nvim_ui_attach", "nvim_ui_detach",
		"nvim_ui_try_resize", "nvim_ui_set_option", "nvim_ui_set_focus",
		"nvim_set_client_info", "nvim_ui_pum_set_height", "nvim_ui_pum_set_bounds",
	}
	for name, code := range map[string]string{
		"Bootstrap": Bootstrap,
		"probeLua":  probeLua,
	} {
		for _, f := range remoteOnly {
			if strings.Contains(code, f) {
				t.Errorf("%s calls the remote-only function %s", name, f)
			}
		}
	}
}
