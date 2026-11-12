# ng

**A new minimal Neovim GUI client written in Go.**

ng is not a new editor built on Neovim. It is just a window for Neovim —
what GVim is to Vim.

The name has several meanings: **N**ew **G**UI, **N**eovim **G**UI,
**N**eovim in **G**o, and "NG" (no good): most features are deliberately
not implemented.

## Goals

- **It works.** Correct drawing comes before everything else. Double-width
  characters, ambiguous-width characters and fallback fonts never leave
  fragments on the screen.
- **Fast and quiet.** No animations. ng draws only when Neovim says the
  screen is complete (`flush`) and uses no CPU while idle.
- **Beautiful text.** Fonts are rendered with FreeType using your
  fontconfig settings. Type 1 and bitmap fonts work too (Pango-based GUIs
  can no longer use them).
- **Nothing is lost.** Neovim runs as a separate process. If ng crashes,
  Neovim keeps running and `ng --server` attaches to it again.
- **Maintainable by one person.** Linux only, small, and tested every day
  against Neovim nightly.

## NG features

These are intentionally not implemented. Requests for them will be
declined.

- Animations (cursor, scrolling)
- Externalized UI (`ext_cmdline`, `ext_messages`, `ext_popupmenu`,
  `ext_tabline`, `ext_multigrid`). Neovim draws all of its UI itself, exactly
  as in a terminal. This means:
  - ng and a terminal UI attached to the same Neovim look and behave the
    same; with ext_* features only the subset all UIs support is active.
  - UI improvements in Neovim itself (or in plugins such as ui2 or
    noice.nvim) work unchanged and never conflict with the GUI.
- GUI tab bars, sidebars, file trees, menus and toolbars
- A configuration file. ng is configured with Neovim options, variables
  and command line arguments only.
- Windows and macOS. Other GUIs cover them.
- Cursor blinking

## Planned

- Images through Neovim's official `vim.ui.img` API (not a GUI-specific
  API): plugins that use it will show images in ng and in kitty alike.
- Input method support for those who do not use a Neovim-side input
  method.

## Requirements

- Linux (X11 or Wayland; xrdp sessions work with SDL's software renderer)
- Neovim 0.10 or later
- SDL3, FreeType, fontconfig
- Go 1.24 or later to build

On Arch Linux / Manjaro:

```sh
sudo pacman -S sdl3 freetype2 fontconfig go
go install github.com/Shougo/ng/cmd/ng@latest
```

## Usage

```sh
ng [--nvim PATH] [--geometry COLSxROWS] [--] [nvim arguments...]
ng --server [ADDRESS]
```

| Option | Description |
| --- | --- |
| `--nvim PATH` | Neovim executable to start (default: `nvim`) |
| `--geometry COLSxROWS` | Initial grid size (default: `80x24`) |
| `--server [ADDRESS]` | Attach to a running Neovim server (socket path or `host:port`). Without an address, attach to the Neovim a crashed ng left behind |

Long command lines are best kept in a shell alias.

ng starts `nvim --headless --listen <socket>` in its own session and
attaches to it. Closing the window runs `:confirm qall`, so Neovim asks
about unsaved buffers.

### When ng crashes

Neovim is not affected: it keeps every buffer, unsaved changes included.
Run

```sh
ng --server
```

to attach to it again. ng looks for the Neovim servers it started
(`$XDG_RUNTIME_DIR/ng.*.sock`) that have no window. If there are several,
it lists them with their current file and number of modified buffers.
Inside ng, `:echo v:servername` shows the address.

## Configuration

Everything is configured from Neovim.

```vim
" Fonts: comma separated fallback list, size with :h
set guifont=Courier\ 10\ Pitch,Noto\ Sans\ Mono\ CJK\ JP:h11
" Font for double-width characters (optional)
set guifontwide=Noto\ Sans\ Mono\ CJK\ JP
set linespace=2

" Detect ng
if exists('g:ng')
  " ...
endif
```

Every glyph is fitted into the cells Neovim gives it. If a character is
drawn too small, give it the width you want with `setcellwidths()` or
`'ambiwidth'`; ng follows Neovim's idea of character widths.

Hinting and antialiasing follow your fontconfig settings
(`~/.config/fontconfig/fonts.conf`).

## Development

```sh
go test ./...                              # unit tests
go test -tags integration ./test/          # needs nvim on $PATH
NG_PAINT_DUMP=/tmp go test ./internal/paint/  # write snapshot PNGs
```

The screen model and the painter do not depend on SDL, so drawing is
tested by painting into images: for example, after an SKK-style
conversion the incrementally repainted screen must be pixel-identical to a
full repaint.

| Package | Role |
| --- | --- |
| `internal/msgpack`, `internal/rpc` | msgpack-rpc (standard library only) |
| `internal/nvim` | start/attach Neovim, API calls |
| `internal/screen` | UI protocol state (`ext_linegrid`) |
| `internal/font` | fontconfig + FreeType, glyph fitting |
| `internal/paint` | what to draw; image painter for tests |
| `internal/input` | key and mouse notation |
| `internal/gui` | SDL3 window, renderer and events |

## License

MIT, see [LICENSE](LICENSE).
