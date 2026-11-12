package gui

import "testing"

func TestOverridden(t *testing.T) {
	const cw, ch = 10, 20
	cases := []struct {
		name                   string
		reqW, reqH, gotW, gotH int
		want                   bool
	}{
		{"exact", 800, 480, 800, 480, false},
		{"rounding", 800, 480, 801, 479, false},
		{"tiled bigger", 800, 480, 1590, 840, true},
		{"one column narrower", 800, 480, 790, 480, true},
		{"one row shorter", 800, 480, 800, 460, true},
	}
	for _, c := range cases {
		if got := overridden(c.reqW, c.reqH, c.gotW, c.gotH, cw, ch); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
