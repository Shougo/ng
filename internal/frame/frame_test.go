package frame

import (
	"reflect"
	"testing"
	"time"
)

func ms(v ...float64) []time.Duration {
	out := make([]time.Duration, len(v))
	for i, x := range v {
		out[i] = time.Duration(x * float64(time.Millisecond))
	}
	return out
}

// Arrival times taken from a real --trace of command line completion
// with cmdheight=0. Each burst must become a single frame, otherwise the
// command line row is shown blank for a frame (flicker).
func TestTraceBurstsAreOneFrame(t *testing.T) {
	cases := []struct {
		name     string
		arrivals []time.Duration
		frames   int
	}{
		// Popup menu update + blanked command line, then the command
		// line redraw 3.0 ms later.
		{"tab completion", ms(8004.863, 8007.896), 1},
		// Key echo, then three flushes from the completion update.
		{"key then completion", ms(8492.970, 8495.598, 8495.937, 8496.107), 1},
		// The usual 0.1-0.5 ms pairs.
		{"pair", ms(2157.884, 2158.312), 1},
		{"triple", ms(3575.285, 3578.365, 3578.595, 3578.767), 1},
	}
	for _, c := range cases {
		if got := Schedule(c.arrivals); len(got) != c.frames {
			t.Errorf("%s: %d frames at %v, want %d", c.name, len(got), got, c.frames)
		}
	}
}

// Separate keystrokes stay separate frames, so typing is not delayed.
func TestKeystrokesAreSeparateFrames(t *testing.T) {
	got := Schedule(ms(1953.005, 2129.167, 2274.041))
	want := ms(1957.005, 2133.167, 2278.041)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A continuous stream of flushes (e.g. a fast-scrolling plugin) is still
// presented at least once per MaxDelay.
func TestStreamIsCapped(t *testing.T) {
	var arrivals []time.Duration
	for i := 0; i < 100; i++ {
		arrivals = append(arrivals, time.Duration(i)*time.Millisecond)
	}
	got := Schedule(arrivals)
	for i := 1; i < len(got); i++ {
		if d := got[i] - got[i-1]; d > MaxDelay+Quiet {
			t.Errorf("gap %v between frames %d and %d", d, i-1, i)
		}
	}
	if len(got) < 100/int(MaxDelay/time.Millisecond) {
		t.Errorf("only %d frames for 100 ms of flushes", len(got))
	}
}
