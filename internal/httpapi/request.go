package httpapi

import (
	"encoding/json"
	"errors"
	"io"
)

var ErrTrailingJSON = errors.New("trailing json content")

type TaskRequest struct {
	ID    int64 `json:"id"`
	Type  int   `json:"type"`
	Value int   `json:"value"`
}

func DecodeTaskRequest(r io.Reader) (TaskRequest, error) {
	dec := json.NewDecoder(r)
	var req TaskRequest
	if err := dec.Decode(&req); err != nil {
		return TaskRequest{}, err
	}
	if dec.More() {
		return TaskRequest{}, ErrTrailingJSON
	}
	return req, nil
}
