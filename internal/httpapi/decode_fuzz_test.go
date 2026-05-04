package httpapi

import (
	"bytes"
	"testing"
)

func FuzzDecodeTaskRequest(f *testing.F) {
	f.Add([]byte(`{"id":1,"type":3,"value":42}`))
	f.Add([]byte(`{"id":1,"type":2,"value":3}{"x":1}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeTaskRequest(bytes.NewReader(data))
	})
}
