# Input methods

2026-11-13. Fcitx 5 with Mozc on Manjaro, ng v0.5.0 to v0.6.0.

## Input method only in text modes

With the input method active in Normal mode, `dd` or `:w` typed with
Mozc still on went to Mozc. ng now turns SDL text input on only in modes
where text is typed (Insert, Command-line, ...; `input.IMEWanted`,
`g:ng_ime_insert_only`, default on). The input method keeps its own
on/off state, so it is on again when Insert mode is entered.

## ":" typed ";" in Normal mode

With text input off, ng makes the text from key events itself
(`input.DirectText`). The keycode in an SDL key event does not include
Shift: the trace showed `key 0x3b mod=0x1` for `:`, so every shifted
character in Normal mode was sent unshifted. Fixed by asking the keyboard
layout for the character with Shift, CapsLock and AltGr applied
(`SDL_GetKeyFromScancode`, field `typed`). `--trace` logs
`typed=':' text-input=false` for each key, which shows this kind of bug
directly.

## X11: the preedit appears in the input method's window

Under Wayland ng draws the text being composed at the cursor. Under X11
(including XWayland, `SDL_VIDEO_DRIVER=x11`) it appeared in Fcitx's
window and ng received no `TEXT_EDITING` events:

```sh
SDL_VIDEO_DRIVER=x11 ng --trace /tmp/ime-x11.log   # convert something, quit
grep -c preedit /tmp/ime-x11.log                   # 0
```

What was ruled out, in order:

1. **Environment.** `XMODIFIERS=@im=fcitx` and `SDL_IM_MODULE=fcitx` were
   both set already. A commit that set `SDL_IM_MODULE` from
   `GTK_IM_MODULE` etc. was written and reverted: a run with
   `SDL_IM_MODULE=fcitx` had shown 46 preedit events, but that run used
   Wayland (no `SDL_VIDEO_DRIVER`), not X11. Always check which video
   driver a test ran under (`--trace` logs it at startup).
2. **XIM versus D-Bus.** With `XMODIFIERS=` (empty), no Japanese could be
   typed at all: all input went through XIM.
3. **D-Bus.** Fcitx 5 was on the session bus (`busctl --user list | grep
   fcitx`), so a client could reach it. Watching the bus while ng ran
   under X11:

   ```sh
   dbus-monitor --session "interface='org.fcitx.Fcitx.InputMethod1'" \
       "interface='org.fcitx.Fcitx.InputContext1'" > /tmp/im-dbus.log
   grep -o 'member=[A-Za-z]*' /tmp/im-dbus.log | sort | uniq -c
   ```

   ng created no input context. (The one that appeared was GVim's.) A
   first attempt filtered on the name `org.freedesktop.portal.Fcitx` and
   missed traffic: Fcitx 5 owns its names on several connections
   (`:1.509` for `org.fcitx.Fcitx5`, `:1.510` for the portal name), so
   filter on the interface instead.

Conclusion: SDL 3.4 under X11 talks to Fcitx only through XIM, where the
input method draws the preedit itself. ng cannot get it there; the
README documents the limitation. Reporting it to SDL, with the
dbus-monitor result, is the way forward.

## GVim and other GUIs, for comparison

- GVim (GTK) could not type Japanese at all: `fcitx5-gtk` was not
  installed and `GTK_IM_MODULE` was not set. On X11, GTK picks its own
  simple input or XIM unless told otherwise. With both, GVim reached Fcitx
  over D-Bus (`ProcessKeyEventBatch` in dbus-monitor).
- The preedit was still in a popup. Vim's `imstyle=1` (the default) is
  over-the-spot; `imstyle=0` is on-the-spot and must be in the vimrc.
  Fcitx 5 also disables the preedit for GVim by default (toggle with
  Ctrl+Alt+P). With GVim's GTK4 build, even then nothing appeared inline:
  GTK4 input method support in Vim was new and still being fixed.
- goneovim (Qt5) needed `QT_IM_MODULE=fcitx`. nvim-qt (Qt6) worked: Qt
  reaches Fcitx over D-Bus through `fcitx5-qt`, on X11 too.

## Turning the input method off on leaving Insert mode

ng does not switch input methods on or off (that is specific to each
one). The README shows

```vim
autocmd InsertLeave * call jobstart(['fcitx5-remote', '-c'])
```

`system()` made every `<Esc>` wait for the command (tens of
milliseconds). `jobstart()` returns at once; tested with ng, the input
method is still turned off reliably even though ng stops text input
right after the mode change. A list starts no shell.
