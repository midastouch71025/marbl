package httpapi

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeTaskRequestValid(t *testing.T) {
	req, err := DecodeTaskRequest(strings.NewReader(`{"id":1,"type":3,"value":42}`))
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != 1 || req.Type != 3 || req.Value != 42 {
		t.Fatalf("got %+v", req)
	}
}

func TestDecodeTaskRequestTrailingRejected(t *testing.T) {
	_, err := DecodeTaskRequest(strings.NewReader(`{"id":1,"type":2,"value":3}{}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTrailingJSON) {
		t.Fatalf("got %v", err)
	}
}
