package consumer

import (
	"sync"
	"time"
)

// WindowLimiter enforces at most maxEvents within each sliding window.
type WindowLimiter struct {
	mu         sync.Mutex
	max        int
	window     time.Duration
	timestamps []time.Time
}

func NewWindowLimiter(max int, window time.Duration) *WindowLimiter {
	if max < 1 {
		max = 1
	}
	if window < 1*time.Millisecond {
		window = time.Millisecond
	}
	return &WindowLimiter{max: max, window: window}
}

func (l *WindowLimiter) Allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	pruned := l.timestamps[:0]
	for _, t := range l.timestamps {
		if t.After(cutoff) {
			pruned = append(pruned, t)
		}
	}
	l.timestamps = pruned
	if len(l.timestamps) >= l.max {
		return false
	}
	l.timestamps = append(l.timestamps, now)
	return true
}
