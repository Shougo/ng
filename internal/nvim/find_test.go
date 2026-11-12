package nvim

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/Shougo/ng/internal/msgpack"
)

// fakeServer answers every request with result, like a Neovim that runs
// the probe Lua.
func fakeServer(t *testing.T, path string, result []any) {
	t.Helper()
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				dec, enc := msgpack.NewDecoder(c), msgpack.NewEncoder(c)
				for {
					v, err := dec.Decode()
					if err != nil {
						return
					}
					msg := v.([]any)
					enc.Encode([]any{int64(1), msg[1], nil, result})
				}
			}(c)
		}
	}()
}

func TestFindServers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("TMPDIR", t.TempDir())

	orphan := filepath.Join(dir, "ng.1.aaaa.sock")
	shown := filepath.Join(dir, "ng.2.bbbb.sock")
	fakeServer(t, orphan, []any{int64(0), "/home/u/notes.md", int64(2)})
	fakeServer(t, shown, []any{int64(1), "", int64(0)})
	// A socket left by a Neovim that is gone: must be skipped.
	stale := filepath.Join(dir, "ng.3.cccc.sock")
	l, _ := net.Listen("unix", stale)
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	if _, err := os.Stat(stale); err != nil {
		t.Fatal("stale socket file missing:", err)
	}
	// Not ours.
	fakeServer(t, filepath.Join(dir, "other.sock"), []any{int64(0), "", int64(0)})

	all := FindServers()
	if len(all) != 2 {
		t.Fatalf("found %+v, want 2 servers", all)
	}
	want := Server{Addr: orphan, UIs: 0, File: "/home/u/notes.md", Modified: 2}
	if all[0] != want {
		t.Errorf("got %+v, want %+v", all[0], want)
	}
	o := Orphans(all)
	if len(o) != 1 || o[0].Addr != orphan {
		t.Errorf("orphans %+v", o)
	}
}
