# Investigation notes

Records of problems that took investigation: what was seen, what was
tried, what turned out to be the cause, and what ng does about it. The
README says what ng does; these notes say why, so that the next person
(or the next session) does not have to find it out again.

Add a note when a problem needed more than reading the code: logs,
experiments, or a wrong first guess. Keep the commands that settled it.

| Note | Topic |
| --- | --- |
| [ime.md](ime.md) | Input methods: Normal mode keys, X11 preedit, GVim and other GUIs |
| [restart.md](restart.md) | `:restart` and the UI events ng must follow |
| [latency.md](latency.md) | Measuring keystroke latency, and a measurement bug |
| [images.md](images.md) | `vim.ui.img`: the kitty protocol subset, and `stdout_tty` at startup |
