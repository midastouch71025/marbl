package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"marbl/internal/persistence/db"
)

type deliveryPayload struct {
	ID    int64 `json:"id"`
	Type  int16 `json:"type"`
	Value int16 `json:"value"`
}

// PostTask POSTs a single task to the consumer /tasks endpoint.
func PostTask(ctx context.Context, client *http.Client, consumerBase string, task db.Task) (status int, _ error) {
	url := fmt.Sprintf("%s/tasks", consumerBase)
	body, err := json.Marshal(deliveryPayload{
		ID:    task.ID,
		Type:  task.Type,
		Value: task.Value,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func sleepBackoff(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
