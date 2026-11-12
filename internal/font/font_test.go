package font

import (
	"reflect"
	"testing"
)

func TestParseSpec(t *testing.T) {
	cases := []struct {
		in   string
		want Spec
	}{
		{"", Spec{}},
		{"Courier 10 Pitch OT:h11", Spec{Families: []string{"Courier 10 Pitch OT"}, Size: 11}},
		{"Courier 10 Pitch,Noto Sans Mono CJK JP:h12.5",
			Spec{Families: []string{"Courier 10 Pitch", "Noto Sans Mono CJK JP"}, Size: 12.5}},
		{"Fira_Code:h10:b", Spec{Families: []string{"Fira Code"}, Size: 10}},
		{`A\,B,C`, Spec{Families: []string{"A,B", "C"}}},
		{"*:h9", Spec{Size: 9}},
	}
	for _, c := range cases {
		if got := ParseSpec(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseSpec(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// newTestSet loads Courier 10 Pitch (a Type 1 font, the reason ng uses
// FreeType directly) if it is installed, else any monospace font.
func newTestSet(t *testing.T) *Set {
	t.Helper()
	s, err := NewSet("Courier 10 Pitch,monospace:h11", "", 96, 0)
	if err != nil {
		t.Skipf("no usable font: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func checkFits(t *testing.T, s *Set, g *Glyph, cells int, what string) {
	t.Helper()
	if g.X < 0 || g.Y < 0 || g.X+g.W > cells*s.CellW || g.Y+g.H > s.CellH {
		t.Errorf("%s: glyph %+v exceeds %d cell(s) of %dx%d",
			what, Glyph{X: g.X, Y: g.Y, W: g.W, H: g.H}, cells, s.CellW, s.CellH)
	}
	if len(g.Pix) != g.W*g.H {
		t.Errorf("%s: pixel buffer size %d, want %d", what, len(g.Pix), g.W*g.H)
	}
}

func TestMetricsAndASCII(t *testing.T) {
	s := newTestSet(t)
	if s.CellW <= 0 || s.CellH <= s.CellW || s.Baseline <= 0 || s.Baseline > s.CellH {
		t.Fatalf("bad metrics: cell %dx%d baseline %d", s.CellW, s.CellH, s.Baseline)
	}
	if g := s.Glyph(" ", 0, 1); !g.Empty() {
		t.Error("space should be empty")
	}
	for _, ch := range []string{"M", "g", "|", "_"} {
		g := s.Glyph(ch, 0, 1)
		if g.Empty() {
			t.Errorf("%q rendered empty", ch)
		}
		checkFits(t, s, g, 1, ch)
	}
	// 'g' has a descender: it must reach below the baseline.
	if g := s.Glyph("g", 0, 1); g.Y+g.H <= s.Baseline {
		t.Errorf("descender of g not below baseline: %+v baseline %d", g.Y+g.H, s.Baseline)
	}
}

func TestStylesAreCachedSeparately(t *testing.T) {
	s := newTestSet(t)
	r := s.Glyph("M", 0, 1)
	b := s.Glyph("M", Bold, 1)
	if r == b {
		t.Error("bold and regular share a cache entry")
	}
	if s.Glyph("M", 0, 1) != r {
		t.Error("glyph not cached")
	}
	checkFits(t, s, b, 1, "bold M")
	checkFits(t, s, s.Glyph("M", Bold|Italic, 1), 1, "bold italic M")
}

// CJK characters come from a fallback font and must fit their two cells;
// ambiguous-width characters such as ▽ must fit the single cell Neovim
// gives them with ambiwidth=single.
func TestFallbackFitsCells(t *testing.T) {
	s := newTestSet(t)
	g := s.Glyph("あ", 0, 2)
	if g.Empty() {
		t.Skip("no font with Japanese glyphs installed")
	}
	checkFits(t, s, g, 2, "あ")
	checkFits(t, s, s.Glyph("漢", 0, 2), 2, "漢")
	for _, ch := range []string{"▽", "▼", "○", "※"} {
		checkFits(t, s, s.Glyph(ch, 0, 1), 1, ch+" in 1 cell")
		checkFits(t, s, s.Glyph(ch, 0, 2), 2, ch+" in 2 cells")
	}
	// A two-cell glyph is roughly centered.
	left, right := g.X, 2*s.CellW-(g.X+g.W)
	if d := left - right; d > 3 || d < -3 {
		t.Errorf("あ not centered: left %d right %d", left, right)
	}
}

func TestGuifontwide(t *testing.T) {
	s, err := NewSet("Courier 10 Pitch,monospace:h11", "IPAGothic,Noto Sans Mono CJK JP", 96, 2)
	if err != nil {
		t.Skip(err)
	}
	defer s.Close()
	g := s.Glyph("漢", 0, 2)
	if g.Empty() {
		t.Skip("no CJK font")
	}
	checkFits(t, s, g, 2, "漢 with guifontwide")
}

func TestCombiningCharacter(t *testing.T) {
	s := newTestSet(t)
	g := s.Glyph("é", 0, 1)
	checkFits(t, s, g, 1, "e + combining acute")
	if g.Empty() {
		t.Error("empty")
	}
}

func TestUnknownFamilyFallsBack(t *testing.T) {
	s, err := NewSet("No Such Font Family 12345,monospace:h10", "", 96, 0)
	if err != nil {
		t.Skip(err)
	}
	defer s.Close()
	if len(s.Warnings) == 0 {
		t.Error("expected a warning for the missing family")
	}
	if s.Glyph("A", 0, 1).Empty() {
		t.Error("monospace fallback did not render")
	}
}

func TestDPIScalesCells(t *testing.T) {
	a := newTestSet(t)
	b, err := NewSet("Courier 10 Pitch,monospace:h11", "", 192, 0)
	if err != nil {
		t.Skip(err)
	}
	defer b.Close()
	if b.CellH < a.CellH*3/2 {
		t.Errorf("192 dpi cell height %d not ~2x of %d", b.CellH, a.CellH)
	}
}
