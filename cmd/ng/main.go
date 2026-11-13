// Command ng is a minimal Neovim GUI for Linux.
//
//	ng [-f] [--nvim PATH] [--geometry COLSxROWS] [--] [nvim arguments...]
//	ng [-f] --server [ADDRESS]
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Shougo/ng/internal/cli"
	"github.com/Shougo/ng/internal/detach"
	"github.com/Shougo/ng/internal/gui"
	"github.com/Shougo/ng/internal/nvim"
	"github.com/Shougo/ng/internal/rpc"
)

// version may be set by packagers with -ldflags "-X main.version=...";
// otherwise it comes from Go's build information (see cli.Version).
var version = "dev"

// start is when the process began, for startup timing in --trace.
var start = time.Now()

func main() {
	os.Exit(run())
}

func run() int {
	child := detach.Child()
	cfg, err := cli.Parse(os.Args[1:], os.Stderr)
	if errors.Is(err, cli.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ng:", err)
		return 2
	}
	if cfg.Version {
		fmt.Println(cli.VersionLine(version))
		return 0
	}

	// Like GVim: started from a terminal, give it back once the window is
	// up. Arguments are checked first, so that their errors appear here.
	if !child && !cfg.NoFork && detach.FromTerminal() {
		return detach.Background(os.Stderr)
	}

	if cfg.Server == cli.AutoServer {
		addr, err := pickServer()
		if err != nil {
			fmt.Fprintln(os.Stderr, "ng:", err)
			return 1
		}
		cfg.Server = addr
	}

	opts := gui.Options{Cols: cfg.Cols, Rows: cfg.Rows, Start: start, OnShown: detach.Ready}
	if cfg.Trace != "" {
		f, err := os.Create(cfg.Trace)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ng:", err)
			return 1
		}
		defer f.Close()
		fmt.Fprintf(f, "# %s\n", cli.VersionLine(version))
		opts.Trace = f
	}

	// Neovim is started (or attached to) by the GUI on another goroutine,
	// while the window is being created.
	connect := func(onNotify rpc.NotificationHandler) (*nvim.Nvim, error) {
		if cfg.Server != "" {
			return nvim.Connect(cfg.Server, onNotify, nil)
		}
		return nvim.Start(cfg.Nvim, cfg.Listen, cfg.NvimArgs, onNotify, nil)
	}

	app := gui.New()
	defer func() {
		// Neovim runs in its own process: if the GUI fails, nothing is
		// lost, and the user can attach again.
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "ng: crashed: %v\nNeovim is still running; attach again with:\n  ng --server\n", r)
			panic(r)
		}
	}()
	if err := app.Run(connect, opts); err != nil {
		fmt.Fprintf(os.Stderr, "ng: %v\n", err)
		if addr := app.NvimAddr(); addr != "" {
			select {
			case <-app.NvimDone():
			default:
				fmt.Fprintf(os.Stderr, "Neovim is still running; attach again with:\n  ng --server %s\n", addr)
			}
		}
		return 1
	}
	return 0
}

// pickServer chooses the server for a bare "ng --server": the one no UI
// is attached to. With several candidates it lists them instead of
// guessing.
func pickServer() (string, error) {
	all := nvim.FindServers()
	orphans := nvim.Orphans(all)
	switch {
	case len(orphans) == 1:
		return orphans[0].Addr, nil
	case len(orphans) == 0 && len(all) == 0:
		return "", errors.New("no running Neovim started by ng was found")
	}
	list := orphans
	msg := "several Neovim servers have no window; choose one:"
	if len(orphans) == 0 {
		list = all
		msg = "every Neovim started by ng already has a window; to attach anyway, choose one:"
	}
	fmt.Fprintln(os.Stderr, "ng:", msg)
	for _, s := range list {
		file := s.File
		if file == "" {
			file = "[No Name]"
		}
		fmt.Fprintf(os.Stderr, "  ng --server %s    # %s, %d modified buffer(s)\n", s.Addr, file, s.Modified)
	}
	return "", errors.New("no server selected")
}
