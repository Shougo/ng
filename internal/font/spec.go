package font

import (
	"strconv"
	"strings"
)

// DefaultSize is used when 'guifont' gives no :h option.
const DefaultSize = 11.0

// Spec is a parsed 'guifont' or 'guifontwide' value.
type Spec struct {
	Families []string
	Size     float64 // points; 0 if not given
}

// ParseSpec parses the Neovim GUI convention used by most clients:
//
//	Family One,Family Two:h11
//
// Families are separated by commas (a backslash escapes a comma) and
// options follow a colon. Only :h<size> is used; other options such as :b
// are accepted and ignored because Neovim highlights choose bold/italic.
// The size may be given on any entry and applies to all of them.
func ParseSpec(s string) Spec {
	var spec Spec
	for _, item := range splitEscaped(s, ',') {
		parts := splitEscaped(item, ':')
		name := strings.TrimSpace(parts[0])
		for _, opt := range parts[1:] {
			opt = strings.TrimSpace(opt)
			if len(opt) > 1 && opt[0] == 'h' {
				if f, err := strconv.ParseFloat(opt[1:], 64); err == nil && f > 0 {
					spec.Size = f
				}
			}
		}
		if name != "" && name != "*" {
			spec.Families = append(spec.Families, strings.ReplaceAll(name, "_", " "))
		}
	}
	return spec
}

// splitEscaped splits s at sep, treating "\sep" as a literal sep.
func splitEscaped(s string, sep byte) []string {
	var (
		out []string
		cur strings.Builder
	)
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == sep:
			cur.WriteByte(sep)
			i++
		case s[i] == sep:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	return append(out, cur.String())
}
