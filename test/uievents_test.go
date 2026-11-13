//go:build integration

package test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Shougo/ng/internal/msgpack"
)

// uiEvents lists every UI event Neovim may send, with what ng does about
// it. TestUIEventsKnown fails when Neovim (nightly, in CI) gains an event
// that is not listed here, so a new part of the protocol, like "restart"
// was, is noticed the day it lands instead of when a user finds it broken.
// Decide what ng should do (see :help api-ui-events), then add it here.
var uiEvents = map[string]string{
	// ui-linegrid: the screen (package screen).
	"grid_resize":        "handled",
	"grid_clear":         "handled",
	"grid_cursor_goto":   "handled",
	"grid_line":          "handled",
	"grid_scroll":        "handled",
	"grid_destroy":       "ignored: only for ext_multigrid",
	"hl_attr_define":     "handled",
	"hl_group_set":       "ignored: ng needs no highlight group names",
	"default_colors_set": "handled",

	// ui-global.
	"mode_info_set": "handled",
	"mode_change":   "handled",
	"option_set":    "handled",
	"set_title":     "handled",
	"set_icon":      "ignored: the window icon is ng's own",
	"busy_start":    "handled",
	"busy_stop":     "handled",
	"mouse_on":      "handled",
	"mouse_off":     "handled",
	"bell":          "handled",
	"visual_bell":   "handled",
	"flush":         "handled",
	"restart":       "handled: attach to the new server (package gui)",
	"connect":       "ignored: :connect is not supported yet",
	"chdir":         "ignored: ng has no file dialogs that would use it",
	"suspend":       "ignored: a GUI has no shell to suspend to",
	"update_menu":   "ignored: ng has no menu bar",
	"error_exit":    "ignored: ng reports the exit from the closed channel",
	"screenshot":    "ignored: nvim__screenshot is for the TUI's own tests",
	"ui_send":       "ignored: raw bytes for a terminal (nvim_ui_send), meaningless to a GUI",

	// ui-grid-old: only sent without ext_linegrid, which ng always sets.
	"resize": "ignored: ui-grid-old", "clear": "ignored: ui-grid-old",
	"eol_clear": "ignored: ui-grid-old", "cursor_goto": "ignored: ui-grid-old",
	"update_fg": "ignored: ui-grid-old", "update_bg": "ignored: ui-grid-old",
	"update_sp": "ignored: ui-grid-old", "highlight_set": "ignored: ui-grid-old",
	"put": "ignored: ui-grid-old", "set_scroll_region": "ignored: ui-grid-old",
	"scroll": "ignored: ui-grid-old",

	// ext_* extensions, which ng never enables: the screen stays the TUI's.
	"win_pos": "ignored: ext_multigrid", "win_float_pos": "ignored: ext_multigrid",
	"win_external_pos": "ignored: ext_multigrid", "win_hide": "ignored: ext_multigrid",
	"win_close": "ignored: ext_multigrid", "msg_set_pos": "ignored: ext_multigrid",
	"win_viewport": "ignored: ext_multigrid", "win_viewport_margins": "ignored: ext_multigrid",
	"win_extmark":    "ignored: ext_multigrid",
	"popupmenu_show": "ignored: ext_popupmenu", "popupmenu_select": "ignored: ext_popupmenu",
	"popupmenu_hide": "ignored: ext_popupmenu",
	"tabline_update": "ignored: ext_tabline",
	"cmdline_show":   "ignored: ext_cmdline", "cmdline_pos": "ignored: ext_cmdline",
	"cmdline_special_char": "ignored: ext_cmdline", "cmdline_hide": "ignored: ext_cmdline",
	"cmdline_block_show": "ignored: ext_cmdline", "cmdline_block_append": "ignored: ext_cmdline",
	"cmdline_block_hide": "ignored: ext_cmdline",
	"wildmenu_show":      "ignored: ext_wildmenu", "wildmenu_select": "ignored: ext_wildmenu",
	"wildmenu_hide": "ignored: ext_wildmenu",
	"msg_show":      "ignored: ext_messages", "msg_clear": "ignored: ext_messages",
	"msg_showmode": "ignored: ext_messages", "msg_showcmd": "ignored: ext_messages",
	"msg_ruler": "ignored: ext_messages", "msg_history_show": "ignored: ext_messages",
	"msg_history_clear": "ignored: ext_messages",
}

func TestUIEventsKnown(t *testing.T) {
	h := start(t, 20, 4, false)
	info, err := h.nv.Call("nvim_get_api_info")
	if err != nil {
		t.Fatal(err)
	}
	pair, _ := info.([]any)
	if len(pair) != 2 {
		t.Fatalf("unexpected nvim_get_api_info result: %#v", info)
	}
	meta := toMap(pair[1])
	events, _ := meta["ui_events"].([]any)
	if len(events) == 0 {
		t.Fatalf("no ui_events in nvim_get_api_info: %#v", meta)
	}
	var unknown []string
	for _, e := range events {
		ev := toMap(e)
		name, _ := ev["name"].(string)
		if _, ok := uiEvents[name]; ok {
			continue
		}
		since, _ := msgpack.ToInt(ev["since"])
		unknown = append(unknown, describeEvent(name, ev["parameters"], since))
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		t.Errorf("Neovim sends UI events ng does not know about; decide what ng should do "+
			"(:help api-ui-events) and add them to uiEvents:\n  %s", strings.Join(unknown, "\n  "))
	}
}

func describeEvent(name string, params any, since int64) string {
	var ps []string
	list, _ := params.([]any)
	for _, p := range list {
		pair, _ := p.([]any)
		if len(pair) == 2 {
			ps = append(ps, toString(pair[1]))
		}
	}
	return name + "(" + strings.Join(ps, ", ") + ") since API level " + strconv.FormatInt(since, 10)
}

func toMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := map[string]any{}
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
}

func toString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	}
	return ""
}
