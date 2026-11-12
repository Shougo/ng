// Command ng is a minimal Neovim GUI for Linux.
//
//	ng [--nvim PATH] [--geometry COLSxROWS] [--] [nvim arguments...]
//	ng --server ADDRESS
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Shougo/ng/internal/gui"
	"github.com/Shougo/ng/internal/nvim"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("ng", flag.ContinueOnError)
	nvimPath := fs.String("nvim", "nvim", "Neovim executable to start")
	server := fs.String("server", "", "attach to a running Neovim server (socket path or host:port) instead of starting one")
	geometry := fs.String("geometry", "80x24", "initial grid size as COLSxROWS")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: ng [options] [--] [nvim arguments...]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println("ng", version)
		return 0
	}
	var cols, rows int
	if _, err := fmt.Sscanf(*geometry, "%dx%d", &cols, &rows); err != nil || cols < 1 || rows < 1 {
		fmt.Fprintf(os.Stderr, "ng: invalid --geometry %q (want COLSxROWS)\n", *geometry)
		return 2
	}

	app := gui.New()
	var (
		nv  *nvim.Nvim
		err error
	)
	if *server != "" {
		if fs.NArg() > 0 {
			fmt.Fprintln(os.Stderr, "ng: nvim arguments cannot be used with --server")
			return 2
		}
		nv, err = nvim.Connect(*server, app.Enqueue, nil)
	} else {
		nv, err = nvim.Start(*nvimPath, fs.Args(), app.Enqueue, nil)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ng:", err)
		return 1
	}

	defer func() {
		// Neovim runs in its own process: if the GUI fails, nothing is
		// lost, and the user can attach again.
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "ng: crashed: %v\nNeovim is still running; attach again with:\n  ng --server %s\n", r, nv.Addr)
			panic(r)
		}
	}()
	if err := app.Run(nv, gui.Options{Cols: cols, Rows: rows}); err != nil {
		fmt.Fprintf(os.Stderr, "ng: %v\n", err)
		select {
		case <-nv.Done():
		default:
			fmt.Fprintf(os.Stderr, "Neovim is still running; attach again with:\n  ng --server %s\n", nv.Addr)
		}
		return 1
	}
	return 0
}
