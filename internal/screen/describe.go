package screen

import (
	"fmt"
	"strings"
)

// Describe summarizes the params of one "redraw" notification in one line,
// for ng --trace. grid_line shows its row, column and text.
func Describe(params []any) string {
	var parts []string
	for _, p := range params {
		ev, _ := p.([]any)
		if len(ev) == 0 {
			continue
		}
		name, _ := ev[0].(string)
		for _, a := range ev[1:] {
			args, _ := a.([]any)
			parts = append(parts, describeOne(name, args))
		}
		if len(ev) == 1 {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, " | ")
}

func describeOne(name string, args []any) string {
	switch name {
	case "grid_line":
		if len(args) < 4 {
			break
		}
		var text strings.Builder
		cells, _ := args[3].([]any)
		for _, c := range cells {
			cell, _ := c.([]any)
			if len(cell) == 0 {
				continue
			}
			t, _ := cell[0].(string)
			n := 1
			if len(cell) >= 3 {
				n = argInt(cell, 2)
			}
			if n > 3 && t == " " {
				fmt.Fprintf(&text, "␠×%d", n)
				continue
			}
			for i := 0; i < n; i++ {
				text.WriteString(t)
			}
		}
		s := text.String()
		if r := []rune(s); len(r) > 60 {
			s = string(r[:60]) + "…"
		}
		return fmt.Sprintf("line(%d,%d %q)", argInt(args, 1), argInt(args, 2), s)
	case "grid_scroll":
		return fmt.Sprintf("scroll(top=%d bot=%d left=%d right=%d rows=%d)",
			argInt(args, 1), argInt(args, 2), argInt(args, 3), argInt(args, 4), argInt(args, 5))
	case "grid_cursor_goto":
		return fmt.Sprintf("cursor(%d,%d)", argInt(args, 1), argInt(args, 2))
	case "grid_resize":
		return fmt.Sprintf("resize(%dx%d)", argInt(args, 1), argInt(args, 2))
	case "mode_change":
		m, _ := firstString(args)
		return "mode(" + m + ")"
	case "option_set":
		if len(args) >= 2 {
			return fmt.Sprintf("option(%v=%v)", args[0], args[1])
		}
	case "hl_attr_define":
		return fmt.Sprintf("hl(%d)", argInt(args, 0))
	}
	return name
}
