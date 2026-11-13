# `:restart` and new UI events

2026-11-13, ng v0.6.0.

## The bug

`:restart` closed ng instead of restarting Neovim. Neovide handled it;
goneovim and nvim-qt did not.

## The protocol

`:restart` (and `ZR`) makes Neovim start a new server and send the
`restart` UI event, then the old server exits and closes the channel.
The UI is expected to wait for the channel to close and attach to the
new server. From `:help ui-event-restart`:

- Neovim 0.12: `["restart", listen_addr]`.
- Neovim 0.13: `["restart", listen_addr, chan_config]`; `chan_config`
  should be restored with `nvim_chan_set()`.
- Before 0.12, development versions sent `[progpath, argv]` and left
  starting the server to the UI (as in `src/nvim/ui_client.c` of that
  time). ng does not support those versions.

If no UI handles the event, the new server is left running with no
window (`:help :restart`).

## What ng does

`nvim.ParseRestart` finds the event in a redraw batch; the GUI keeps it,
and when the old channel closes it attaches to the new address instead
of exiting (`App.restartSession`), with the same bootstrap and the
current grid size, keeping the window. `TestRestart` (integration) runs
`:restart` on a real Neovim and attaches the way the GUI does.

The new server listens on an address Neovim picks, so `ng --server`
without an address does not find it after a crash.

## Not missing the next one

`restart` was added to the UI protocol and ng broke silently.
`TestUIEventsKnown` compares `nvim_get_api_info().ui_events` with a list
of every event ng handles or ignores on purpose, and fails with the new
names when Neovim gains one; CI runs it against nightly every day. Its
first run found `screenshot` and `ui_send`, both meaningless to a GUI.

`connect` (`:connect`, attach to another server) is listed as not
supported yet; it would reuse the restart path.
