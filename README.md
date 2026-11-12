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
- Neovim: the **latest stable release series** (currently 0.12) and
  **nightly**. These two are tested every day in CI; older versions are
  not supported (ng shows a warning when started with one). When Neovim
  releases a new minor version, ng follows it.
- SDL3, FreeType, fontconfig
- Go 1.24 or later to build

Install the libraries, then build with `go install`:

```sh
# Arch Linux / Manjaro
sudo pacman -S sdl3 freetype2 fontconfig go
# Fedora 42 or later
sudo dnf install SDL3-devel freetype-devel fontconfig-devel golang gcc

go install github.com/Shougo/ng/cmd/ng@latest
```

Both are built in CI every day. Other distributions work the same way if
they package SDL3 (with `pkg-config` support); ng does not provide
packages itself, but packagers are welcome (set the version with
`-ldflags "-X main.version=..."`).

## Usage

```sh
ng [--nvim PATH] [--geometry COLSxROWS] [--] [nvim arguments...]
ng --server [ADDRESS]
```

| Option | Description |
| --- | --- |
| `--nvim PATH` | Neovim executable to start (default: `nvim`) |
| `--geometry COLSxROWS` | Grid size until the config sets 'lines'/'columns' (default: `80x24`) |
| `--trace FILE` | Log redraws, keys and frames with timestamps (attach it to bug reports) |
| `--version` | Print the version |
| `--server [ADDRESS]` | Attach to a running Neovim server (socket path or `host:port`). Without an address, attach to the Neovim a crashed ng left behind |

Long command lines are best kept in a shell alias.

ng uses Wayland when it is available and X11 otherwise (including xrdp
sessions). SDL's own environment variables still work, for example
`SDL_VIDEO_DRIVER=x11 ng` to run under XWayland, or
`SDL_RENDER_DRIVER=software ng` to avoid the GPU.

ng starts `nvim --embed --listen <socket>` in its own session and
attaches to it. Starting Neovim, loading fontconfig and creating the
window run in parallel; the window appears once, after your config has
run, already with your fonts and size. As with the builtin TUI, Neovim reads your config only
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

" Window size in cells, as in GVim (also at any time with :set)
set lines=40 columns=120

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

'lines' and 'columns' size the window, in your config and with `:set`
later, and the window keeps them when 'guifont' changes. A tiling window
manager (sway, i3, ...) decides window sizes itself: when ng sees that
the window did not get the size it asked for, it stops resizing the
window and fits 'lines' and 'columns' to the window instead, as GVim
ends up doing. (On sway, resizing a tiled window would only shrink the
drawing surface inside the tile.) This lasts until ng restarts, also if
the window is made floating later. `--geometry` only sets the size used
before your config runs.

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

## Why SDL3 (and not GTK, Qt or a Go toolkit)

A Neovim GUI does not need widgets: it draws one grid of text. What
matters is how text is drawn and how input reaches Neovim, so ng keeps
those in its own hands and uses SDL3 only for the window, input and
putting pixels on the screen.

- **Text is ng's own.** ng renders glyphs with FreeType, chooses fonts
  with fontconfig and fits every glyph into its cells. GTK and Qt render
  text through Pango or their own layers, which decide for you. Pango,
  for example, dropped Type 1 and bitmap fonts, which is why such fonts
  stopped working in GVim. Go toolkits such as Gio rasterize text
  themselves, without hinting and without Type 1 support.
- **X11 and Wayland with one code base**, including xrdp sessions.
  Writing both backends directly would double the code to maintain.
- **Works without a GPU.** SDL_Renderer uses OpenGL or Vulkan when they
  are available and falls back to software rendering (for example under
  xrdp) with the same drawing code. Toolkits built on OpenGL alone, such
  as GLFW, cannot.
- **Input methods later.** SDL3 text input supports IBus, Fcitx 5 and the
  Wayland text-input protocol, should ng need them.
- **No extra dependencies.** SDL3 is called directly through cgo, without
  third-party Go bindings, and is packaged by the major distributions.

The price on GNOME Wayland is window decorations drawn by libdecor.

### Why cgo

ng uses cgo on purpose, and not only for SDL3:

- **Text rendering needs C libraries.** FreeType and fontconfig are what
  let ng draw Type 1 and bitmap fonts, honour your fontconfig hinting and
  antialiasing settings, and find fallback fonts the way the rest of the
  desktop does. No pure Go library does all of that, and correct text is
  ng's first promise.
- **Avoiding cgo for SDL3 alone would gain little.** Loading SDL3 at run
  time without cgo (for example with purego) removes the C compiler from
  the build, but SDL3 is still needed at run time, and SDL's C unions and
  callbacks become harder to use correctly.
- **The usual costs of cgo do not apply to a Linux-only program.** Cross
  compiling, static single-file binaries and C toolchains on Windows are
  problems for multi-platform distribution. On Linux, a C compiler and the
  development packages are one package-manager command away (see
  [Requirements](#requirements)), and binaries link against the libraries
  the distribution already ships.

If ng ever had to support Windows or macOS, this decision would be
revisited; it does not, so cgo stays.

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
