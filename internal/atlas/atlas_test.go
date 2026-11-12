package atlas

import "testing"

func TestPackerNoOverlap(t *testing.T) {
	p := NewPacker(64, 64)
	type r struct{ x, y, w, h int }
	var got []r
	sizes := [][2]int{{10, 12}, {20, 8}, {30, 14}, {5, 5}, {40, 10}, {7, 20}, {60, 3}}
	for _, s := range sizes {
		x, y, ok := p.Alloc(s[0], s[1])
		if !ok {
			t.Fatalf("alloc %v failed", s)
		}
		if x+s[0] > 64 || y+s[1] > 64 {
			t.Fatalf("alloc %v out of bounds at %d,%d", s, x, y)
		}
		got = append(got, r{x, y, s[0], s[1]})
	}
	for i := range got {
		for j := i + 1; j < len(got); j++ {
			a, b := got[i], got[j]
			if a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h {
				t.Errorf("%v overlaps %v", a, b)
			}
		}
	}
}

func TestPackerFull(t *testing.T) {
	p := NewPacker(16, 16)
	n := 0
	for {
		if _, _, ok := p.Alloc(7, 7); !ok {
			break
		}
		n++
	}
	if n != 4 {
		t.Errorf("fit %d glyphs of 7x7 in 16x16, want 4", n)
	}
	if _, _, ok := p.Alloc(100, 1); ok {
		t.Error("oversized glyph accepted")
	}
	p.Reset()
	if _, _, ok := p.Alloc(7, 7); !ok {
		t.Error("reset did not free space")
	}
}

func TestAlphaToRGBA(t *testing.T) {
	out := AlphaToRGBA([]byte{0, 128, 255, 9, 1, 2}, 2, 2, 3, nil)
	want := []byte{255, 255, 255, 0, 255, 255, 255, 128, 255, 255, 255, 9, 255, 255, 255, 1}
	if string(out) != string(want) {
		t.Errorf("got %v", out)
	}
}
