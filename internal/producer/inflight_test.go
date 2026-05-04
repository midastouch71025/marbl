package producer

import "testing"

func TestInFlightSuppressesDuplicate(t *testing.T) {
	var f InFlight
	if !f.TryAdd(1) {
		t.Fatal("first add")
	}
	if f.TryAdd(1) {
		t.Fatal("duplicate add should fail")
	}
	f.Remove(1)
	if !f.TryAdd(1) {
		t.Fatal("after remove")
	}
	if f.Len() != 1 {
		t.Fatalf("len: %d", f.Len())
	}
}
