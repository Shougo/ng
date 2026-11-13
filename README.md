# ng

<img src="assets/icons/hicolor/scalable/apps/ng.svg" alt="" width="96" align="right">

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
   less. What works in the terminal works in ng (see "The same screen as
   the terminal").
3. **Your work survives ng.** Neovim runs in its own process. If ng
   crashes, nothing is lost, and `ng --server` brings it back (see
   "Reconnecting").
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
  `ext_tabline`, `ext_multigrid`): see "The same screen as the terminal".
- GUI tab bars, sidebars, file trees, menus and toolbars
- A configuration file. ng is configured with Neovim options, variables
  and command line arguments only.
- Windows and macOS. Other GUIs cover them.
- Cursor blinking
- Clipboard integration beyond what a terminal does: yank and put use
  Neovim's own clipboard provider (`"+y`, `"+p`). Like a terminal, ng
  pastes the clipboard with **Ctrl+Shift+V** (as a bracketed paste, via
  `nvim_paste`).

## The same screen as the terminal

ng is a window for Neovim, so it shows exactly what Neovim in a terminal
(the TUI) shows. This is a rule, not a goal:

- **Neovim draws everything.** ng uses only `ext_linegrid`. The command
  line, messages, popup menu, tab line, floating windows and borders are
  drawn by Neovim into the grid, exactly as for a terminal. ng never
  replaces them with widgets of its own (no `ext_cmdline`,
  `ext_messages`, `ext_popupmenu`, `ext_tabline`, `ext_multigrid`).
- **Neovim starts the same way.** Neovim reads your config after ng's UI
  has attached, as with the TUI, so plugins that look at the UI while
  starting (`vim._core.ui2`, anything using `nvim_list_uis()`, `&lines`
  or `&columns`) behave the same.
- **It is tested.** Integration tests compare ng's screen with Neovim's
  own idea of the screen (`screenstring()`) cell by cell, after editing,
  scrolling, splitting and command line completion, against Neovim stable
  and nightly every day.

What follows from it:

- UI improvements in Neovim itself or in plugins (ui2, noice.nvim, ...)
  work in ng unchanged, and never conflict with the GUI.
- A terminal UI and ng attached to the same Neovim show the same screen.
  (With `ext_*` features, Neovim enables only what every attached UI
  supports, so GUIs that rely on them change their look depending on what
  else is attached.)
- When ng and the terminal differ, it is a bug in ng. Please report it.

ng differs from a terminal only where a window can do better without
changing what Neovim shows: how glyphs are rendered (fonts, fallback,
fitting into cells), input methods, Ctrl+Shift+V pasting, and, later,
images through `vim.ui.img`.

## Planned

- Images through Neovim's official `vim.ui.img` API (not a GUI-specific
  API): plugins that use it will show images in ng and in kitty alike.

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
# Debian 13 (trixie) or later, Ubuntu 25.04 or later
sudo apt install libsdl3-dev libfreetype-dev libfontconfig-dev golang-go gcc pkg-config

go install github.com/Shougo/ng/cmd/ng@latest
```

Arch, Fedora and Debian are built in CI every day. Ubuntu 24.04 LTS and Debian 12 do
not package SDL3; there you would have to build SDL3 yourself. If the
distribution's Go is older than the one ng needs, `go install` downloads
a newer Go toolchain by itself. Other distributions work the same way if
they package SDL3 (with `pkg-config` support); ng does not provide
packages itself, but packagers are welcome (set the version with
`-ldflags "-X main.version=..."`).

### Desktop entry and icon

`go install` installs only the binary. To start ng from an application
launcher and get its icon (Wayland compositors take the icon from the
desktop entry; the app ID is `ng`), install them from a clone of the
repository:

```sh
mkdir -p ~/.local/share/applications
cp assets/ng.desktop ~/.local/share/applications/
cp -r assets/icons ~/.local/share/
```

Packagers: install `assets/ng.desktop` to `/usr/share/applications` and
`assets/icons/hicolor` to `/usr/share/icons/hicolor`.

## Usage

```sh
ng [-f] [--nvim PATH] [--geometry COLSxROWS] [--listen ADDRESS] [--] [nvim arguments...]
ng [-f] --server [ADDRESS]
```

| Option | Description |
| --- | --- |
| `-f`, `--nofork` | Stay in the foreground instead of giving the terminal back (like `gvim -f`) |
| `--nvim PATH` | Neovim executable to start (default: `nvim`) |
| `--geometry COLSxROWS` | Grid size until the config sets 'lines'/'columns' (default: `80x24`) |
| `--trace FILE` | Log redraws, keys and frames with timestamps (attach it to bug reports) |
| `--version` | Print the version |
| `--listen ADDRESS` | Start Neovim listening on this socket path or `host:port` instead of a new socket in `$XDG_RUNTIME_DIR` |
| `--server [ADDRESS]` | Attach to a running Neovim server (socket path or `host:port`). Without an address, attach to the Neovim a crashed ng left behind |

Long command lines are best kept in a shell alias.

Like GVim, ng started from a terminal gives it back: the shell prompt
returns once the window is up, and closing the terminal does not close
the window. Errors that happen before the window appears (an unknown
option, a missing `nvim`) are still printed in the terminal. A program
that waits for its editor needs `-f`, as with GVim:

```sh
git config --global core.editor 'ng -f'
```

From an application launcher or a script (no terminal), ng stays in the
foreground anyway.

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

### Reconnecting

ng and Neovim are separate processes, connected through a socket. The
window can go away and come back while Neovim, with every buffer and
unsaved change, keeps running.

**After a crash.** If ng crashes or is killed, Neovim is not affected.
Run

```sh
ng --server
```

to attach to it again. Without an address, ng looks for the Neovim
servers it started (`$XDG_RUNTIME_DIR/ng.*.sock`) that have no window
attached. If there is exactly one, ng attaches to it; if there are
several, ng lists them with their current file and number of modified
buffers and attaches to none. Inside ng, `:echo v:servername` shows the
address.

**`:restart`.** Neovim's `:restart` (and `ZR`) starts a new server and
exits the old one; ng attaches to the new server and keeps its window.
The new server listens on an address Neovim chooses, so `ng --server`
without an address does not find it after a crash: use the address from
`:echo v:servername`.

**To any Neovim server.** ng attaches to any Neovim that listens on a
socket, not only to the ones it started:

```sh
nvim --listen /tmp/work.sock    # in a terminal, for example
ng --server /tmp/work.sock      # the same session in a window
ng --server localhost:6666      # TCP works too
```

**Known addresses for scripts.** `--server` attaches to a Neovim that is
running; `--listen` starts a new one on an address you choose, so that
scripts can reach it. For example, to open files from a file manager in
one ng window:

```sh
#!/bin/sh
SOCK="$XDG_RUNTIME_DIR/nvim-files.sock"
if nvim --server "$SOCK" --remote-expr 1 >/dev/null 2>&1; then
    nvim --server "$SOCK" --remote "$@"   # the window already exists
else
    exec ng --listen "$SOCK" -- "$@"
fi
```

ng refuses to start a second Neovim on an address where one is already
listening (it tells you to use `ng --server`), and removes a socket file
left by a Neovim that is gone. `ng --server` without an address only
looks for ng's own sockets, not for addresses given to `--listen`.

**Together with a terminal.** A terminal UI and ng can be attached to
the same Neovim at once. Both show the same screen (see "The same screen
as the terminal" below); Neovim uses the smaller of the two sizes.

Closing ng's window runs `:confirm qall` in Neovim, as closing a terminal
running Neovim would; Neovim asks about unsaved buffers.

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

" Keep the input method active in every mode (default: only where text
" is typed; see "Input methods")
let g:ng_ime_insert_only = 0

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
later, and the window keeps them when 'guifont' changes. `--geometry`
only sets the size used before your config runs.

ng reads fonts with FreeType, so the Type 1 "Courier 10 Pitch" works
directly; no conversion to OpenType is needed. The size is converted to
pixels like GTK does (points at 96 dpi times the display scale that SDL
reports), so the same setting should give the same size as in GVim. If it
does not, please report it with your `Xft.dpi` and scaling settings.

### Input methods

ng works with system input methods (Fcitx 5, IBus, and the Wayland
text-input protocol) through SDL:

- The text being composed (preedit) is drawn by ng at the cursor, with
  the same fonts and CJK cell fitting as the rest of the screen,
  underlined, with the segment being converted reversed. If it would run
  past the right edge it is shifted left.
- The candidate window is placed at the cursor (or at the text being
  composed).
- While text is being composed, keys such as Enter, Escape and the arrows
  go to the input method, not to Neovim. Only the committed text reaches
  Neovim, as if typed.
- The input method is active only in modes where text is typed: Insert
  and Replace mode, the command line and Terminal mode. In Normal, Visual
  and Operator-pending mode keys reach Neovim directly, so `dd` or `:w`
  work right after `<Esc>` even if the input method is still switched on.
  (A terminal cannot do this: it does not know Neovim's mode.) Typing a
  non-ASCII character after `f` or `r` therefore needs the input method
  switched off; to keep it active in every mode, set
  `let g:ng_ime_insert_only = 0`.

ng does not switch the input method itself on or off: that is specific
to each input method. To turn Fcitx 5 off when leaving Insert mode, as
GVim does, add to your config (it works in a terminal too):

```vim
autocmd InsertLeave * call jobstart(['fcitx5-remote', '-c'])
```

`jobstart()` returns at once; `system()` would make every `<Esc>` wait
for the command (tens of milliseconds). Pass a list so that no shell is
started.

Under X11 (including XWayland), input goes through XIM: typing and
converting work, but the text being composed is shown in the input
method's own window rather than at the cursor, because SDL does not
pass it to ng there. Under Wayland it is drawn at the cursor as
described above. Input methods that run inside Neovim (such as
skkeleton) need nothing from ng.

### Tiling window managers

A tiling window manager (sway, i3, ...) decides the size of tiled windows
itself. When ng starts, it checks whether its window got the size it asked
for. If not, ng leaves the window size to the window manager from then on:
'lines', 'columns' and 'guifont' changes fit the grid to the window
instead of resizing it, as GVim ends up doing. (On sway, resizing a tiled
window would only shrink the drawing surface inside the tile, shown as a
black area.)

This check is made once, when the window first appears, and lasts until
ng restarts. A window that is made floating later still keeps its size
when 'lines' or 'columns' change; resize it with the mouse instead. To
have `:set lines=… columns=…` resize the window, start ng floating:

```
# sway (~/.config/sway/config)
for_window [app_id="ng"] floating enable

# i3
for_window [class="ng"] floating enable
```

The check cannot follow later changes because SDL, which ng uses for its
window, keeps whether a window is tiled to itself: there is no public API
to ask for it.

Every glyph is fitted into the cells Neovim gives it. If a character is
drawn too small, give it the width you want with `setcellwidths()` or
`'ambiwidth'`; ng follows Neovim's idea of character widths.

Hinting and antialiasing follow your fontconfig settings
(`~/.config/fontconfig/fonts.conf`).

## Measuring input latency

`--trace FILE` measures every keystroke from the moment the window
system delivered the key event to the moment the frame showing Neovim's
answer was presented, and ends the file with a summary. A session of
ordinary editing on the author's machine (Manjaro, Neovim 0.13 nightly with the
author's own config and plugins):

```
# latency (key event to present, inside ng)
# 176 keystrokes   median    p90     max (ms)
#   total             7.95    8.46  121.90
#   queue             0.14    0.20    0.82
#   nvim              0.72    1.00  106.18
#   frame             6.99    7.40   15.47
```

- `queue`: the key event waiting in SDL's queue before ng handled it.
- `nvim`: from sending the key to Neovim until its answer (flush)
  arrived, including the socket.
- `frame`: from the flush to the present. Most of it is the 4 ms ng
  waits for Neovim to finish a burst of flushes (see package `frame`:
  presenting earlier shows intermediate screens, such as a blank
  command line); the rest is painting and presenting.

Nine keystrokes in ten reach the screen within 8.5 ms, half of one
60 Hz frame. The three samples over 20 ms were all Neovim's own work:
`:` (106 ms in Neovim, entering the command line), `<CR>` running a
command, and a mapping.

Each frame is measured from the last key before Neovim answered: a key
that changes nothing on screen (the start of a mapping, an operator
waiting for its motion) is counted with the key after it. A slow `nvim`
part is Neovim itself (autocommands, plugins) and is just as slow in a
terminal; the trace names the key of each sample.

This is the part ng and Neovim are responsible for; it does not include
the compositor or the display. To compare the whole path with a
terminal, use an external tool that types and watches the screen, such
as [Typometer](https://github.com/pavelfatin/typometer) (X11 only):
run it against ng and against Neovim in your terminal, with the same
config and file.

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

## Reporting bugs

Please use the issue form. GUI bugs depend on the machine, so the form
asks for the versions, the display server and window manager, whether the
bug also happens with `ng -- --clean` and in a terminal, and if possible a
screenshot and a trace:

```sh
ng --trace /tmp/ng-trace.log   # reproduce the bug once, quit, attach the file
```

The trace records what Neovim sent, the keys you pressed and when each
frame was drawn, which is usually enough to find the cause. It includes
the text that was on the screen.

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

### The icon

The source is `assets/icons/hicolor/scalable/apps/ng.svg`: a grid of
cells with `ng`, the `g` drawn as the cursor cell (colors swapped). The
glyphs are DejaVu Sans Mono Bold converted to outlines. After changing
it, regenerate the PNG sizes and the window icon embedded in ng:

```sh
for s in 16 24 32 48 64 128 256 512; do
  rsvg-convert -w $s -h $s assets/icons/hicolor/scalable/apps/ng.svg \
    -o assets/icons/hicolor/${s}x${s}/apps/ng.png
done
cp assets/icons/hicolor/128x128/apps/ng.png internal/gui/icon.png
```

## License

MIT, see [LICENSE](LICENSE).
