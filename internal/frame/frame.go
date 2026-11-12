// Package frame defines when ng presents a frame after Neovim flushes.
//
// Neovim often sends several flushes for one change (for example, while
// command line completion is updated, one flush redraws the popup menu
// and blanks the command line row, and the next one redraws the command
// line). A terminal never shows such intermediate screens because it
// repaints at its refresh rate, merging updates that arrive within one
// frame. ng waits until Neovim has been quiet for Quiet before presenting,
// and never longer than MaxDelay (one 60 Hz frame) after the first flush.
package frame

import "time"

const (
	Quiet    = 4 * time.Millisecond
	MaxDelay = 16 * time.Millisecond
)

// Schedule simulates the policy: given the arrival times of flushes
// (sorted), it returns the times frames are presented. Each frame shows
// every flush that arrived before it. It ignores the time spent painting.
func Schedule(arrivals []time.Duration) []time.Duration {
	var out []time.Duration
	for i := 0; i < len(arrivals); {
		first := arrivals[i]
		deadline := first + MaxDelay
		last := first
		i++
		for i < len(arrivals) && arrivals[i] <= last+Quiet && arrivals[i] < deadline {
			last = arrivals[i]
			i++
		}
		out = append(out, min(last+Quiet, deadline))
	}
	return out
}
