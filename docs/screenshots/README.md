# Screenshots

The screenshots in the top-level README. They are still images on
purpose: ng has no animations, so neither does its README.

`demo.lua` is a minimal config that sets up each screen, so that no
personal config shows and the screenshots can be taken again the same
way. Run it from the root of the repository:

```sh
NG_DEMO=edit ng -- --clean -u docs/screenshots/demo.lua
```

`NG_DEMO_FONT` overrides 'guifont' (default `UDEV Gothic NF:h12`; a Nerd
Font is needed for the icons) and `NG_DEMO_SIZE` the size in cells
(`COLSxROWS`).

| File | `NG_DEMO` | What to do |
| --- | --- | --- |
| `edit.png` | `edit` | Nothing: Go code, Japanese text and a floating window with Nerd Font icons |
| `ime.png` | `ime` | Under **Wayland** (under X11 the preedit is in the input method's window), type Japanese with Fcitx 5 and take the screenshot while converting, with the candidate window open |
| `image.png` | `image` | Needs Neovim 0.13. The ng icon is placed with `vim.ui.img` |

Take the window without its decorations, at display scale 1 if possible,
and keep the files small. Under a tiling window manager, make ng float so
that it keeps the size `demo.lua` sets: a full-screen window is shrunk in
the README until its text is hard to read.

```sh
swaymsg 'for_window [app_id="ng"] floating enable'  # sway; undo with swaymsg reload
grim -g "$(slurp)" edit.png                 # Wayland
maim -i "$(xdotool getactivewindow)" edit.png  # X11
oxipng -o max --strip safe *.png
```
