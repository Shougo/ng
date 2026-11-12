package width

import (
	"reflect"
	"sort"
	"testing"
)

func TestRune(t *testing.T) {
	cases := []struct {
		r          rune
		single     int
		ambiDouble int
	}{
		{'a', 1, 1},
		{'あ', 2, 2}, {'ア', 2, 2}, {'漢', 2, 2}, {'한', 2, 2}, {'ㄱ', 2, 2},
		{'ｱ', 1, 1}, // halfwidth katakana
		{'Ａ', 2, 2}, {'。', 2, 2}, {'　', 2, 2},
		{'▽', 1, 2}, {'○', 1, 2}, {'※', 1, 2}, {'→', 1, 2}, {'α', 1, 2}, {'─', 1, 2},
		{'é', 1, 2}, // East Asian Ambiguous in Unicode
		{'z', 1, 1},
		{0x0301, 0, 0}, {0x3099, 0, 0}, {0xFE0F, 0, 0}, {0x200D, 0, 0},
		{'𠮷', 2, 2},
		{'😀', 2, 2},
		{'\t', 0, 0},
	}
	for _, c := range cases {
		if got := Rune(c.r, false); got != c.single {
			t.Errorf("Rune(%U, single) = %d, want %d", c.r, got, c.single)
		}
		if got := Rune(c.r, true); got != c.ambiDouble {
			t.Errorf("Rune(%U, double) = %d, want %d", c.r, got, c.ambiDouble)
		}
	}
}

func TestLayout(t *testing.T) {
	got := Layout("かな漢字éがx", false)
	want := []Cell{
		{"か", 2}, {"な", 2}, {"漢", 2}, {"字", 2},
		{"é", 1}, {"が", 2}, {"x", 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	if n := Cells(got); n != 12 {
		t.Errorf("Cells = %d", n)
	}
	// Hangul being composed by an input method: syllables, then a jamo.
	if n := Cells(Layout("한글ㄱ", false)); n != 6 {
		t.Errorf("hangul cells = %d", n)
	}
	// A lone combining mark still takes a cell.
	if got := Layout("́", false); len(got) != 1 || got[0].Width != 1 {
		t.Errorf("lone mark: %q", got)
	}
}

func TestTablesSortedAndDisjoint(t *testing.T) {
	for name, tab := range map[string][]span{"wide": wide, "ambiguous": ambiguous} {
		if !sort.SliceIsSorted(tab, func(i, j int) bool { return tab[i].lo < tab[j].lo }) {
			t.Errorf("%s not sorted", name)
		}
		for i, s := range tab {
			if s.lo > s.hi {
				t.Errorf("%s[%d] empty: %U-%U", name, i, s.lo, s.hi)
			}
			if i > 0 && tab[i-1].hi >= s.lo {
				t.Errorf("%s[%d] overlaps the previous range", name, i)
			}
		}
	}
}
