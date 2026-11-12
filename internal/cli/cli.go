// Package cli parses ng's command line.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// AutoServer is the Server value of "ng --server" given without an
// address: attach to the Neovim that ng left behind.
const AutoServer = "\x00auto"

// Config is the parsed command line.
type Config struct {
	Nvim       string
	Server     string // "", an address, or AutoServer
	Cols, Rows int
	Version    bool
	Trace      string // file for --trace, or ""
	NvimArgs   []string
}

// ErrHelp is returned for -h/--help after the usage has been printed.
var ErrHelp = flag.ErrHelp

// Parse parses args (without the program name). Usage and errors are
// written to out.
func Parse(args []string, out io.Writer) (*Config, error) {
	c := &Config{}
	fs := flag.NewFlagSet("ng", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&c.Nvim, "nvim", "nvim", "Neovim executable to start")
	fs.StringVar(&c.Server, "server", "",
		"attach to a running Neovim server (socket path or host:port);\n"+
			"without an address, attach to the one a crashed ng left behind")
	geometry := fs.String("geometry", "80x24", "initial grid size as COLSxROWS")
	fs.BoolVar(&c.Version, "version", false, "print the version and exit")
	fs.StringVar(&c.Trace, "trace", "", "write received redraw events and frame timing to `FILE` (for bug reports)")
	fs.Usage = func() {
		fmt.Fprintf(out, "usage: ng [options] [--] [nvim arguments...]\n       ng --server [ADDRESS]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(allowBareServer(args)); err != nil {
		return nil, err
	}
	if _, err := fmt.Sscanf(*geometry, "%dx%d", &c.Cols, &c.Rows); err != nil || c.Cols < 1 || c.Rows < 1 {
		return nil, fmt.Errorf("invalid --geometry %q (want COLSxROWS)", *geometry)
	}
	c.NvimArgs = fs.Args()
	if c.Server != "" && len(c.NvimArgs) > 0 {
		return nil, errors.New("nvim arguments cannot be used with --server")
	}
	return c, nil
}

// allowBareServer turns a "--server" that has no address after it into
// "--server=<AutoServer>", since package flag requires a value.
func allowBareServer(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(out, args[i:]...)
		}
		if a == "--server" || a == "-server" {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				out = append(out, "--server="+AutoServer)
				continue
			}
		}
		out = append(out, a)
	}
	return out
}
