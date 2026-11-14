# Images (`vim.ui.img`)

2026-11-14, Neovim 0.13 nightly (v0.13.0-dev-2081).

## How Neovim sends images

`vim.ui.img` is not a UI event. `runtime/lua/vim/ui/img/_kitty.lua`
writes kitty graphics protocol sequences with `nvim_ui_send()`, which
reaches UIs with the `stdout_tty` option as `ui_send` events:

- transmit: `ESC _G f=100,a=t,t=d,i=<id>,q=2,m=1;<base64>` in 4096-byte
  chunks, continuation chunks carrying only `m`;
- place: `ESC 7`, `CSI ?25l`, `CSI <row>;<col> H`,
  `ESC _G a=p,i=,p=,C=1,q=2[,c=,r=,z=]`, `ESC 8`, `CSI ?25h`; an update
  places the same `(i, p)` again;
- delete: `a=d,d=i,i=<id>` or `a=d,d=A`;
- `_supported()`: `a=q,i=<id>,s=1,v=1` without `q`, answered through
  `nvim_ui_term_event("termresponse", ...)` with `ESC _G i=<id>;OK ESC \`.

So ng acts as a terminal that speaks this part of the protocol (package
`kitty`) instead of using an ng-specific API: plugins written for kitty
work unchanged, and changes to the Lua side keep working as long as the
protocol stays the same.

## stdout_tty must be set after startup

Attaching with `stdout_tty` broke `TestGNgVisibleInConfig`. With a TTY
UI present at startup, `vim/_core/defaults.lua` asks the terminal for
its background color (OSC 11, plus a DSR probe) and waits up to 100 ms
with `vim.wait()` before the user's config runs, then reports E1568; it
also probes 'termguicolors'. ng does not answer those, so startup got
slower, and `vim.wait()` let RPC requests run in the middle of startup.
ng now sets the option with `nvim_ui_set_option` when its UIEnter
notification arrives. defaults.lua's UIEnter handler for terminals that
attach later checks the option at that moment, before ng sets it, so no
detection runs either.

With the option set, ng also receives sequences meant for real terminals
(OSC 9;4 progress, DA1 queries from the OSC 52 provider, kitty multiple
cursor shapes): the parser skips every sequence it does not handle.

## Images behave like text

Placements are kept in cells. On `grid_scroll` the placements whose
top-left cell lies in the region move by the same rows, and those that
leave the region are deleted, as the text is (kitty moves images with
scrolled text too). They are drawn over the grid at present time, after
the screen texture and before the cursor. Size: `c` and `r` fill those
cells; one of them keeps the aspect ratio; neither gives the image's own
pixel size.

`grid_clear` does not delete placements: Neovim clears the grid for
`:redraw!` and resizes, and plugins do not place their images again
after those.

## Tests

- `internal/kitty`: the exact byte sequences `_kitty.lua` writes, split
  at every 5 bytes, mixed with foreign sequences; scrolling; raw RGBA.
- `internal/paint` (`TestImages`): pixels of a placed and of a clipped
  image.
- `test/images_test.go` (`TestUIImg`, skipped before 0.13): a real
  Neovim calls `_supported()`, `set()`, moves the image, scrolls the
  window with `2<C-e>` and deletes it.
