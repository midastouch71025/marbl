package consumer

import (
	"math"
	"testing"
	"time"
)

func TestStaleCutoff(t *testing.T) {
	now := time.Unix(1000, int64(500*time.Millisecond))
	got := StaleCutoff(now, 5)
	want := float64(now.UnixNano())/1e9 - 5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %v want %v", got, want)
	}
}
