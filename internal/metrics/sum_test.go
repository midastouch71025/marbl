package metrics

import "testing"

func TestSumInt64(t *testing.T) {
	if got := SumInt64([]int64{1, 2, 3}); got != 6 {
		t.Fatalf("got %d", got)
	}
	if got := SumInt64(nil); got != 0 {
		t.Fatalf("nil: got %d", got)
	}
}
