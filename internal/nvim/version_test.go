package nvim

import "testing"

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b Version
		want bool
	}{
		{Version{0, 11, 5}, Version{0, 12, 0}, true},
		{Version{0, 12, 0}, Version{0, 12, 0}, false},
		{Version{0, 12, 5}, Version{0, 12, 0}, false},
		{Version{0, 13, 0}, Version{0, 12, 9}, false},
		{Version{1, 0, 0}, Version{0, 99, 0}, false},
		{Version{0, 99, 0}, Version{1, 0, 0}, true},
	}
	for _, c := range cases {
		if got := c.a.Less(c.b); got != c.want {
			t.Errorf("%v < %v = %v", c.a, c.b, got)
		}
	}
	if s := (Version{0, 12, 5}).String(); s != "0.12.5" {
		t.Errorf("String() = %q", s)
	}
}
