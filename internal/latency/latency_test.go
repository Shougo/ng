package latency

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Unix(1000, 0)

func at(ms float64) time.Time { return t0.Add(time.Duration(ms * float64(time.Millisecond))) }

func TestOneKey(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0.2), "k")
	if _, ok := r.Present(at(1)); ok {
		t.Fatal("a present before Neovim's flush must not complete the key")
	}
	r.Flush(at(3.2))
	r.Flush(at(3.5)) // a second flush of the same answer
	s, ok := r.Present(at(7.2))
	if !ok {
		t.Fatal("no sample")
	}
	want := Sample{Queue: 200 * time.Microsecond, Nvim: 3 * time.Millisecond, Frame: 4 * time.Millisecond, Key: "k"}
	if s != want || s.Total() != 7200*time.Microsecond {
		t.Errorf("got %+v, want %+v", s, want)
	}
	if _, ok := r.Present(at(20)); ok {
		t.Error("a later present must not repeat the sample")
	}
}

// A key that changes nothing (the start of a mapping) is answered with
// the next one: measure from the last key, not from the first.
func TestMeasuredFromLastKeyBeforeFlush(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0), "s")
	r.Key(at(90), at(90), "j")
	r.Flush(at(95))
	s, _ := r.Present(at(100))
	if s.Total() != 10*time.Millisecond || s.Key != "j" {
		t.Errorf("got %v for %q, want 10ms for \"j\"", s.Total(), s.Key)
	}
}

// A key typed after the flush but before the present does not steal the
// frame from the key it answers.
func TestKeyAfterFlushKeepsFrame(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0), "a")
	r.Flush(at(2))
	r.Key(at(3), at(3), "b")
	s, _ := r.Present(at(6))
	if s.Total() != 6*time.Millisecond || s.Key != "a" {
		t.Errorf("got %v for %q, want 6ms for \"a\"", s.Total(), s.Key)
	}
}

func TestKeyWithoutRedrawIsForgotten(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0), "k") // changes nothing on screen
	r.Key(at(1000), at(1000), "k")
	r.Flush(at(1004))
	s, _ := r.Present(at(1008))
	if s.Total() != 8*time.Millisecond {
		t.Errorf("total %v, want 8ms", s.Total())
	}
}

func TestFlushBeforeKeyIgnored(t *testing.T) {
	var r Recorder
	r.Flush(at(0))
	r.Key(at(1), at(1), "k")
	if _, ok := r.Present(at(2)); ok {
		t.Error("a flush from before the key must not answer it")
	}
}

func TestSummary(t *testing.T) {
	var samples []Sample
	for i := 1; i <= 10; i++ {
		samples = append(samples, Sample{Nvim: time.Duration(i) * time.Millisecond})
	}
	got := Summary(samples)
	for _, want := range []string{"10 keystrokes", "total             5.00    9.00   10.00", "queue             0.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
	if Summary(nil) != "no keystrokes measured" {
		t.Error("empty summary")
	}
}

func TestLateFlushIsNotTheAnswer(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0), "k") // e.g. a key the input method kept
	r.Flush(at(2000))        // a timer redraw much later
	if _, ok := r.Present(at(2004)); ok {
		t.Error("a flush long after the key must not be counted as its answer")
	}
}
