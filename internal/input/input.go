// Package input translates keyboard and mouse events into Neovim's key
// notation. It knows nothing about SDL; the GUI maps SDL keycodes to Key
// first, which keeps this logic testable.
package input

import "strings"

// Key identifies a non-text key.
type Key int

const (
	KeyNone Key = iota
	KeyEscape
	KeyEnter
	KeyTab
	KeyBackspace
	KeyDelete
	KeyInsert
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeySpace
	KeyF1 // KeyF1 + n-1 is Fn, up to F12
)

// Mod is a set of modifiers.
type Mod uint8

const (
	ModShift Mod = 1 << iota
	ModCtrl
	ModAlt
	ModSuper
)

var keyNames = map[Key]string{
	KeyEscape:    "Esc",
	KeyEnter:     "CR",
	KeyTab:       "Tab",
	KeyBackspace: "BS",
	KeyDelete:    "Del",
	KeyInsert:    "Insert",
	KeyHome:      "Home",
	KeyEnd:       "End",
	KeyPageUp:    "PageUp",
	KeyPageDown:  "PageDown",
	KeyUp:        "Up",
	KeyDown:      "Down",
	KeyLeft:      "Left",
	KeyRight:     "Right",
	KeySpace:     "Space",
}

func keyName(k Key) string {
	if k >= KeyF1 && k < KeyF1+12 {
		return "F" + itoa(int(k-KeyF1)+1)
	}
	return keyNames[k]
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func modPrefix(m Mod, withShift bool) string {
	var b strings.Builder
	if m&ModCtrl != 0 {
		b.WriteString("C-")
	}
	if withShift && m&ModShift != 0 {
		b.WriteString("S-")
	}
	if m&ModAlt != 0 {
		b.WriteString("M-")
	}
	if m&ModSuper != 0 {
		b.WriteString("D-")
	}
	return b.String()
}

// SpecialKey returns the notation for a non-text key, e.g. "<C-Up>". It
// returns "" for KeyNone. A plain space is sent as text instead.
func SpecialKey(k Key, m Mod) string {
	if k == KeyNone {
		return ""
	}
	if k == KeySpace && m&(ModCtrl|ModAlt|ModSuper|ModShift) == 0 {
		return "" // arrives as text input
	}
	name := keyName(k)
	if name == "" {
		return ""
	}
	return "<" + modPrefix(m, true) + name + ">"
}

// ChordKey returns the notation for a printable key pressed together with
// Ctrl, Alt or Super, e.g. "<C-a>" or "<M-x>". ch is the character the key
// produces without modifiers (the SDL keycode for printable keys). It
// returns "" when no such modifier is held, in which case the text input
// event carries the character.
func ChordKey(ch rune, m Mod) string {
	if m&(ModCtrl|ModAlt|ModSuper) == 0 || ch < 0x20 || ch == 0x7f {
		return ""
	}
	// Shift is folded into the character for letters: Ctrl-Shift-a is
	// "<C-S-a>" only when the user really wants the distinction; Vim
	// treats <C-A> and <C-a> the same, so keep the base letter.
	name := string(ch)
	switch ch {
	case '<':
		name = "lt"
	case '\\':
		name = "Bslash"
	case '|':
		name = "Bar"
	case ' ':
		name = "Space"
	}
	return "<" + modPrefix(m, m&ModShift != 0 && !isLetter(ch)) + name + ">"
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

// Text escapes typed text for nvim_input: only "<" is special.
func Text(s string) string {
	return strings.ReplaceAll(s, "<", "<lt>")
}

// MouseModifier returns the modifier string for nvim_input_mouse.
func MouseModifier(m Mod) string {
	var b strings.Builder
	if m&ModCtrl != 0 {
		b.WriteString("C")
	}
	if m&ModShift != 0 {
		b.WriteString("S")
	}
	if m&ModAlt != 0 {
		b.WriteString("A")
	}
	return b.String()
}

// IsPaste reports whether the key is Ctrl+Shift+V, which pastes the
// clipboard as in terminal emulators. ch is the unshifted character.
func IsPaste(ch rune, m Mod) bool {
	return (ch == 'v' || ch == 'V') && m&(ModCtrl|ModShift|ModAlt|ModSuper) == ModCtrl|ModShift
}
