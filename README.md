# ng

**A new minimal Neovim GUI client written in Go.**

ng is not a new editor built on Neovim. It is just a window for Neovim —
what GVim is to Vim.

The name has several meanings: **N**ew **G**UI, **N**eovim **G**UI,
**N**eovim in **G**o, and "NG" (no good): most features are deliberately
not implemented.

## What ng promises

ng does not compete on features. It competes on **working**.

1. **Text is drawn correctly.** Every glyph stays inside the cells Neovim
   gives it. Double-width, ambiguous-width and fallback glyphs never leave
   fragments, not even in the middle of an input method conversion.
2. **The screen is Neovim's screen.** No GUI-specific widgets and no
   reinterpretation: ng shows Neovim's grid, nothing more and nothing
   less. What works in the terminal works in ng.
3. **Your work survives ng.** Neovim runs in its own process. If ng
   crashes, nothing is lost, and `ng --server` brings it back.
4. **Faster than the terminal.** No escape sequences to encode and parse
   again, drawing only when Neovim's screen is complete, one draw call per
   frame and no animations. Freeing Neovim from the terminal must make it
   faster, never slower.
5. **Tested against Neovim every day.** Stable and nightly, every day.
   A breakage is found the day it lands, and reported upstream when the
   cause is in Neovim.
6. **Small enough to be maintained for years.** The NG features below
   keep it that way.

Every promise is backed by tests or benchmarks in this repository. A
promise that is not tested is a bug.

Other goals:

- **Beautiful text.** Fonts are rendered with FreeType using your
  fontconfig settings. Type 1 and bitmap fonts work too (Pango-based GUIs
  can no longer use them).
- **Linux only.** X11 and Wayland, including xrdp sessions through SDL's
  software renderer. Other platforms are well served by other GUIs.

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
- Clipboard integration beyond what a terminal does: yank and put use
  Neovim's own clipboard provider (`"+y`, `"+p`). Like a terminal, ng
  pastes the clipboard with **Ctrl+Shift+V** (as a bracketed paste, via
  `nvim_paste`).

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
| `--trace FILE` | Log redraws, keys and frames with timestamps (attach it to bug reports) |
| `--version` | Print the version |
| `--server [ADDRESS]` | Attach to a running Neovim server (socket path or `host:port`). Without an address, attach to the Neovim a crashed ng left behind |

Long command lines are best kept in a shell alias.

ng starts `nvim --embed --listen <socket>` in its own session and
attaches to it. As with the builtin TUI, Neovim reads your config only
after ng has attached, so plugins that look at the UI while starting
(for example `vim._core.ui2`) work as in a terminal. Closing the window runs `:confirm qall`, so Neovim asks
about unsaved buffers.

Errors in your startup files are shown in the window with the usual
"Press ENTER" prompt, just as in a terminal. ng never waits for
Neovim on its main thread: if Neovim is blocked at a prompt when ng
attaches, the window says so and the keys you type answer the prompt.

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
" Font for double-width characters (optional, may have its own size)
set guifontwide=Noto\ Sans\ Mono\ CJK\ JP:h10
set linespace=2

" Detect ng (g:ng is set before your config is read)
if exists('g:ng')
  " ...
endif
```

GVim's GTK syntax with the size as the last word also works, so the same
line serves both GVim and ng:

```vim
set guifont=Courier\ 10\ Pitch\ 16
set guifontwide=Noto\ Sans\ Mono\ CJK\ JP\ 14
```

ng reads fonts with FreeType, so the Type 1 "Courier 10 Pitch" works
directly; no conversion to OpenType is needed. The size is converted to
pixels like GTK does (points at 96 dpi times the display scale that SDL
reports), so the same setting should give the same size as in GVim. If it
does not, please report it with your `Xft.dpi` and scaling settings.

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
