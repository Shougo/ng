package nvim

import (
	"fmt"

	"github.com/Shougo/ng/internal/msgpack"
)

// Supported Neovim versions: the latest stable release series and nightly.
// CI tests exactly these two every day. When a new minor version of Neovim
// is released, raise MinVersion to it (and update the README).
var MinVersion = Version{0, 12, 0}

// Version is a Neovim version.
type Version struct{ Major, Minor, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Less reports whether v is older than w.
func (v Version) Less(w Version) bool {
	if v.Major != w.Major {
		return v.Major < w.Major
	}
	if v.Minor != w.Minor {
		return v.Minor < w.Minor
	}
	return v.Patch < w.Patch
}

// Version asks Neovim for its version.
func (n *Nvim) Version() (Version, error) {
	r, err := n.ExecLua("local v = vim.version() return { v.major, v.minor, v.patch }")
	if err != nil {
		return Version{}, err
	}
	a, _ := r.([]any)
	if len(a) != 3 {
		return Version{}, fmt.Errorf("nvim: unexpected version %#v", r)
	}
	var p [3]int
	for i, x := range a {
		n, err := msgpack.ToInt(x)
		if err != nil {
			return Version{}, err
		}
		p[i] = int(n)
	}
	return Version{p[0], p[1], p[2]}, nil
}

// WarnIfUnsupported shows a warning in Neovim when its version is older
// than MinVersion. ng does not refuse to run: an older Neovim may work, it
// is just not tested.
func (n *Nvim) WarnIfUnsupported() {
	v, err := n.Version()
	if err != nil || !v.Less(MinVersion) {
		return
	}
	msg := fmt.Sprintf("ng: Neovim %s is not supported; ng supports Neovim %d.%d or later.",
		v, MinVersion.Major, MinVersion.Minor)
	n.Notify("nvim_echo", []any{[]any{msg, "WarningMsg"}}, true, map[string]any{})
}
