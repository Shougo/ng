package cli

import (
	"io"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		args []string
		want Config
	}{
		{nil, Config{Nvim: "nvim", Cols: 80, Rows: 24, NvimArgs: []string{}}},
		{[]string{"a.txt", "b.txt"}, Config{Nvim: "nvim", Cols: 80, Rows: 24, NvimArgs: []string{"a.txt", "b.txt"}}},
		{[]string{"--nvim", "/opt/nvim", "--geometry", "120x40", "--", "-u", "NONE"},
			Config{Nvim: "/opt/nvim", Cols: 120, Rows: 40, NvimArgs: []string{"-u", "NONE"}}},
		{[]string{"--server", "/run/user/1000/ng.1.a.sock"},
			Config{Nvim: "nvim", Server: "/run/user/1000/ng.1.a.sock", Cols: 80, Rows: 24, NvimArgs: []string{}}},
		{[]string{"--server"}, Config{Nvim: "nvim", Server: AutoServer, Cols: 80, Rows: 24, NvimArgs: []string{}}},
		{[]string{"--server", "--geometry", "100x30"},
			Config{Nvim: "nvim", Server: AutoServer, Cols: 100, Rows: 30, NvimArgs: []string{}}},
		{[]string{"--trace", "/tmp/t.log", "x.txt"},
			Config{Nvim: "nvim", Trace: "/tmp/t.log", Cols: 80, Rows: 24, NvimArgs: []string{"x.txt"}}},
		{[]string{"--listen", "/run/user/1000/thunar.sock", "--", "a.txt"},
			Config{Nvim: "nvim", Listen: "/run/user/1000/thunar.sock", Cols: 80, Rows: 24, NvimArgs: []string{"a.txt"}}},
		{[]string{"-f", "a.txt"},
			Config{Nvim: "nvim", NoFork: true, Cols: 80, Rows: 24, NvimArgs: []string{"a.txt"}}},
		{[]string{"--nofork", "--server"},
			Config{Nvim: "nvim", NoFork: true, Server: AutoServer, Cols: 80, Rows: 24, NvimArgs: []string{}}},
		{[]string{"-server=localhost:6666"},
			Config{Nvim: "nvim", Server: "localhost:6666", Cols: 80, Rows: 24, NvimArgs: []string{}}},
	}
	for _, c := range cases {
		got, err := Parse(c.args, io.Discard)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.args, err)
			continue
		}
		if !reflect.DeepEqual(*got, c.want) {
			t.Errorf("Parse(%q) = %+v, want %+v", c.args, *got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--geometry", "80"},
		{"--geometry", "0x10"},
		{"--server", "/tmp/x.sock", "file.txt"},
		{"--server", "--", "file.txt"},
		{"--nosuchflag"},
		{"--listen", "/tmp/a.sock", "--server", "/tmp/b.sock"},
	} {
		if _, err := Parse(args, io.Discard); err == nil {
			t.Errorf("Parse(%q) succeeded", args)
		}
	}
}
