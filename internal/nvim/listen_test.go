package nvim

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareListen(t *testing.T) {
	dir := t.TempDir()

	// Nothing there: fine.
	if err := prepareListen(filepath.Join(dir, "new.sock")); err != nil {
		t.Errorf("new path: %v", err)
	}

	// A live server: refuse, and point to --server.
	live := filepath.Join(dir, "live.sock")
	l, err := net.Listen("unix", live)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if err := prepareListen(live); err == nil || !strings.Contains(err.Error(), "ng --server "+live) {
		t.Errorf("live server: %v", err)
	}

	// A socket file whose server is gone: removed so Neovim can listen.
	stale := filepath.Join(dir, "stale.sock")
	sl, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	sl.(*net.UnixListener).SetUnlinkOnClose(false)
	sl.Close()
	if err := prepareListen(stale); err != nil {
		t.Errorf("stale socket: %v", err)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Error("stale socket not removed")
	}

	// A regular file is never removed.
	file := filepath.Join(dir, "notes.txt")
	os.WriteFile(file, []byte("keep"), 0o600)
	if err := prepareListen(file); err == nil {
		t.Error("regular file accepted")
	}
	if _, err := os.Stat(file); err != nil {
		t.Error("regular file removed")
	}

	// TCP addresses are left to Neovim.
	if err := prepareListen("127.0.0.1:1"); err != nil {
		t.Errorf("tcp: %v", err)
	}
}
