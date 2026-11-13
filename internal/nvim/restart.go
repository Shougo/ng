package nvim

// Restart is what the "restart" UI event asks the UI to do after :restart.
// Neovim sends it, then the old server exits and closes the channel; the
// UI attaches to the new server and keeps its window.
//
// Neovim starts the new server itself and sends its address: ["restart",
// listen_addr] in 0.12, ["restart", listen_addr, chan_config] since 0.13.
type Restart struct {
	Addr       string         // the new server
	ChanConfig map[string]any // restore with nvim_chan_set, if not nil
}

// ParseRestart returns the restart request in a redraw notification, or
// nil if there is none.
func ParseRestart(params []any) *Restart {
	var r *Restart
	for _, p := range params {
		ev, ok := p.([]any)
		if !ok || len(ev) < 2 {
			continue
		}
		if name, _ := ev[0].(string); name != "restart" {
			continue
		}
		// Each further element is one call's arguments; the last wins.
		for _, a := range ev[1:] {
			args, _ := a.([]any)
			if x := parseRestartArgs(args); x != nil {
				r = x
			}
		}
	}
	return r
}

func parseRestartArgs(args []any) *Restart {
	if len(args) == 0 {
		return nil
	}
	first, ok := args[0].(string)
	if !ok || first == "" {
		return nil
	}
	r := &Restart{Addr: first}
	if len(args) >= 2 {
		r.ChanConfig = toStringMap(args[1])
	}
	return r
}

func toStringMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		if len(m) == 0 {
			return nil
		}
		return m
	case map[any]any:
		out := map[string]any{}
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// RestoreChannel applies the channel properties the old server had
// (nvim_chan_set on ng's own channel). Neovim versions without
// nvim_chan_set return an error, which callers may ignore.
func (n *Nvim) RestoreChannel(config map[string]any) error {
	if len(config) == 0 {
		return nil
	}
	_, err := n.Call("nvim_chan_set", int64(0), config)
	return err
}
