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
	r.Key(at(0), at(0.2))
	if _, ok := r.Present(at(1)); ok {
		t.Fatal("a present before Neovim's flush must not complete the key")
	}
	r.Flush(at(3.2))
	r.Flush(at(3.5)) // a second flush of the same answer
	s, ok := r.Present(at(7.2))
	if !ok {
		t.Fatal("no sample")
	}
	want := Sample{Queue: 200 * time.Microsecond, Nvim: 3 * time.Millisecond, Frame: 4 * time.Millisecond}
	if s != want || s.Total() != 7200*time.Microsecond {
		t.Errorf("got %+v, want %+v", s, want)
	}
	if _, ok := r.Present(at(20)); ok {
		t.Error("a later present must not repeat the sample")
	}
}

func TestBurstIsMeasuredFromFirstKey(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0))
	r.Key(at(2), at(2))
	r.Flush(at(5))
	s, _ := r.Present(at(10))
	if s.Total() != 10*time.Millisecond {
		t.Errorf("total %v, want 10ms", s.Total())
	}
}

func TestKeyWithoutRedrawIsForgotten(t *testing.T) {
	var r Recorder
	r.Key(at(0), at(0)) // changes nothing on screen
	r.Key(at(1000), at(1000))
	r.Flush(at(1004))
	s, _ := r.Present(at(1008))
	if s.Total() != 8*time.Millisecond {
		t.Errorf("total %v, want 8ms", s.Total())
	}
}

func TestFlushBeforeKeyIgnored(t *testing.T) {
	var r Recorder
	r.Flush(at(0))
	r.Key(at(1), at(1))
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
	r.Key(at(0), at(0)) // e.g. a key the input method kept
	r.Flush(at(2000))   // a timer redraw much later
	if _, ok := r.Present(at(2004)); ok {
		t.Error("a flush long after the key must not be counted as its answer")
	}
}
