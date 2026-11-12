package input

import "testing"

func TestSpecialKey(t *testing.T) {
	cases := []struct {
		k    Key
		m    Mod
		want string
	}{
		{KeyEscape, 0, "<Esc>"},
		{KeyEnter, 0, "<CR>"},
		{KeyTab, ModShift, "<S-Tab>"},
		{KeyUp, ModCtrl, "<C-Up>"},
		{KeyF1, 0, "<F1>"},
		{KeyF1 + 11, ModAlt, "<M-F12>"},
		{KeyBackspace, ModCtrl | ModShift, "<C-S-BS>"},
		{KeySpace, 0, ""},
		{KeySpace, ModCtrl, "<C-Space>"},
		{KeyNone, ModCtrl, ""},
	}
	for _, c := range cases {
		if got := SpecialKey(c.k, c.m); got != c.want {
			t.Errorf("SpecialKey(%d,%d)=%q want %q", c.k, c.m, got, c.want)
		}
	}
}

func TestChordKey(t *testing.T) {
	cases := []struct {
		ch   rune
		m    Mod
		want string
	}{
		{'a', 0, ""},
		{'a', ModShift, ""},
		{'a', ModCtrl, "<C-a>"},
		{'a', ModCtrl | ModShift, "<C-a>"},
		{'x', ModAlt, "<M-x>"},
		{'<', ModCtrl, "<C-lt>"},
		{'\\', ModCtrl, "<C-Bslash>"},
		{'|', ModAlt, "<M-Bar>"},
		{'1', ModCtrl | ModShift, "<C-S-1>"},
		{'^', ModCtrl, "<C-^>"},
		{']', ModCtrl, "<C-]>"},
	}
	for _, c := range cases {
		if got := ChordKey(c.ch, c.m); got != c.want {
			t.Errorf("ChordKey(%q,%d)=%q want %q", c.ch, c.m, got, c.want)
		}
	}
}

func TestText(t *testing.T) {
	if got := Text("a<b>漢"); got != "a<lt>b>漢" {
		t.Errorf("got %q", got)
	}
}

func TestMouseModifier(t *testing.T) {
	if got := MouseModifier(ModCtrl | ModShift | ModAlt); got != "CSA" {
		t.Errorf("got %q", got)
	}
}

func TestIsPaste(t *testing.T) {
	cases := []struct {
		ch   rune
		m    Mod
		want bool
	}{
		{'v', ModCtrl | ModShift, true},
		{'V', ModCtrl | ModShift, true},
		{'v', ModCtrl, false}, // <C-v> stays Neovim's
		{'v', ModCtrl | ModShift | ModAlt, false},
		{'c', ModCtrl | ModShift, false},
	}
	for _, c := range cases {
		if got := IsPaste(c.ch, c.m); got != c.want {
			t.Errorf("IsPaste(%q,%d)=%v", c.ch, c.m, got)
		}
	}
}
