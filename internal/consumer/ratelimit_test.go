package consumer

import (
	"testing"
	"time"
)

func TestWindowLimiterAllowsWithinWindow(t *testing.T) {
	l := NewWindowLimiter(2, time.Second)
	now := time.Unix(1000, 0)
	if !l.Allow(now) {
		t.Fatal("first")
	}
	if !l.Allow(now.Add(time.Millisecond)) {
		t.Fatal("second")
	}
	if l.Allow(now.Add(2 * time.Millisecond)) {
		t.Fatal("third should block")
	}
}

func TestWindowLimiterPrunesOld(t *testing.T) {
	l := NewWindowLimiter(1, time.Second)
	t0 := time.Unix(2000, 0)
	if !l.Allow(t0) {
		t.Fatal("first")
	}
	if l.Allow(t0.Add(500 * time.Millisecond)) {
		t.Fatal("second within window should block")
	}
	if !l.Allow(t0.Add(1500 * time.Millisecond)) {
		t.Fatal("after window should allow")
	}
}
