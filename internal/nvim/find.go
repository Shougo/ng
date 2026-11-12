package nvim

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Shougo/ng/internal/msgpack"
	"github.com/Shougo/ng/internal/rpc"
)

// Server describes a Neovim server that ng started.
type Server struct {
	Addr     string
	UIs      int    // attached UIs; 0 means nothing shows it (e.g. ng crashed)
	File     string // name of the current buffer
	Modified int    // number of modified buffers
}

// socketDirs are the places SocketPath may have used.
func socketDirs() []string {
	dirs := []string{}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	return append(dirs, os.TempDir())
}

// FindServers lists the Neovim servers started by ng that still answer.
// Sockets whose server is gone are skipped (not deleted).
func FindServers() []Server {
	seen := map[string]bool{}
	var out []Server
	for _, dir := range socketDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "ng.*.sock"))
		for _, addr := range matches {
			if seen[addr] {
				continue
			}
			seen[addr] = true
			if s, ok := probe(addr); ok {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Addr < out[j].Addr })
	return out
}

const probeLua = `
local modified = 0
for _, b in ipairs(vim.api.nvim_list_bufs()) do
  if vim.bo[b].modified then modified = modified + 1 end
end
return { #vim.api.nvim_list_uis(), vim.api.nvim_buf_get_name(0), modified }`

func probe(addr string) (Server, bool) {
	conn, err := dial(addr)
	if err != nil {
		return Server{}, false
	}
	c := rpc.NewClient(conn, nil, nil)
	defer c.Close()
	go c.Serve()

	type result struct {
		v   any
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := c.Call("nvim_exec_lua", probeLua, []any{})
		ch <- result{v, err}
	}()
	var r result
	select {
	case r = <-ch:
	case <-time.After(2 * time.Second):
		// Busy or blocked (e.g. waiting at a prompt with no UI); still
		// worth reporting.
		return Server{Addr: addr, UIs: -1}, true
	}
	if r.err != nil {
		return Server{Addr: addr, UIs: -1}, true
	}
	a, _ := r.v.([]any)
	s := Server{Addr: addr}
	if len(a) == 3 {
		uis, _ := msgpack.ToInt(a[0])
		mod, _ := msgpack.ToInt(a[2])
		s.UIs, s.Modified = int(uis), int(mod)
		s.File, _ = a[1].(string)
	}
	return s, true
}

// Orphans returns the servers no UI is attached to.
func Orphans(servers []Server) []Server {
	var out []Server
	for _, s := range servers {
		if s.UIs == 0 {
			out = append(out, s)
		}
	}
	return out
}
