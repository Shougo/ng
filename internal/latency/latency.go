// Package latency measures how long a keystroke takes to reach the
// screen inside ng: from the moment the window system delivered the key
// event to the moment the frame showing Neovim's answer was presented.
//
// It cannot see the compositor or the display, so it gives the part of
// the latency ng (and Neovim) are responsible for. Compare whole-system
// latency against a terminal with an external tool (see the README).
package latency

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// stale is how long a key may wait for a flush before it is forgotten:
// keys that change nothing on screen produce no flush.
const stale = 500 * time.Millisecond

// Sample is one keystroke's latency, split into its parts.
type Sample struct {
	Queue time.Duration // key event waiting in SDL's queue
	Nvim  time.Duration // sent to Neovim until its flush arrived
	Frame time.Duration // flush until present returned
	Key   string        // the key, for traces
}

// Total is the latency from the key event to the present.
func (s Sample) Total() time.Duration { return s.Queue + s.Nvim + s.Frame }

// Recorder pairs keystrokes with the frames that answer them. It is used
// from one goroutine.
type Recorder struct {
	pending bool
	event   time.Time // when the window system delivered the key
	handled time.Time // when ng handled it (sent it to Neovim)
	flush   time.Time // first flush after it, or zero
	label   string
	samples []Sample
}

// Key records a keystroke delivered at event and handled at handled;
// label names it in traces. A frame is measured from the last key before
// Neovim's flush: keys that change nothing on screen (the start of a
// mapping, an operator waiting for its motion) are answered together
// with the key after them, and measuring from them would count the time
// between the two keystrokes.
func (r *Recorder) Key(event, handled time.Time, label string) {
	if r.pending && !r.flush.IsZero() {
		return // its frame is about to be presented
	}
	r.pending, r.event, r.handled, r.flush, r.label = true, event, handled, time.Time{}, label
}

// Flush records a flush from Neovim.
func (r *Recorder) Flush(at time.Time) {
	switch {
	case !r.pending || !r.flush.IsZero() || !at.After(r.handled):
	case at.Sub(r.handled) > stale:
		// A redraw for something else (a timer, a message): the key
		// itself changed nothing on screen.
		r.pending = false
	default:
		r.flush = at
	}
}

// Present records a presented frame and returns the sample it completes.
func (r *Recorder) Present(at time.Time) (Sample, bool) {
	if !r.pending || r.flush.IsZero() {
		return Sample{}, false
	}
	r.pending = false
	s := Sample{
		Queue: max(0, r.handled.Sub(r.event)),
		Nvim:  r.flush.Sub(r.handled),
		Frame: at.Sub(r.flush),
		Key:   r.label,
	}
	r.samples = append(r.samples, s)
	return s, true
}

// Samples returns the samples recorded so far.
func (r *Recorder) Samples() []Sample { return r.samples }

// Summary describes the samples: count, then median, 90th percentile and
// maximum of the total and of each part, in milliseconds.
func Summary(samples []Sample) string {
	if len(samples) == 0 {
		return "no keystrokes measured"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d keystrokes   median    p90     max (ms)", len(samples))
	for _, part := range []struct {
		name string
		get  func(Sample) time.Duration
	}{
		{"total", Sample.Total},
		{"queue", func(s Sample) time.Duration { return s.Queue }},
		{"nvim", func(s Sample) time.Duration { return s.Nvim }},
		{"frame", func(s Sample) time.Duration { return s.Frame }},
	} {
		d := make([]time.Duration, len(samples))
		for i, s := range samples {
			d[i] = part.get(s)
		}
		slices.Sort(d)
		fmt.Fprintf(&b, "\n  %-14s %7.2f %7.2f %7.2f", part.name,
			ms(percentile(d, 50)), ms(percentile(d, 90)), ms(d[len(d)-1]))
	}
	return b.String()
}

// percentile returns the p-th percentile of sorted d (nearest rank).
func percentile(d []time.Duration, p int) time.Duration {
	i := (p*len(d) + 99) / 100
	return d[max(0, i-1)]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
