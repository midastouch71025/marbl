package producer

import "testing"

func TestShouldPauseGeneration(t *testing.T) {
	if !ShouldPauseGeneration(10, 10) {
		t.Fatal("should pause at cap")
	}
	if ShouldPauseGeneration(9, 10) {
		t.Fatal("should not pause below cap")
	}
	if ShouldPauseGeneration(10, 0) {
		t.Fatal("max 0 treated as no cap")
	}
}
