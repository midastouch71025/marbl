package consumer

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"marbl/internal/metrics"
	"marbl/internal/persistence/db"
)

type mockTaskStore struct {
	getTask            func(ctx context.Context, id int64) (db.Task, error)
	tryMarkProcessing  func(ctx context.Context, id int64) (int64, error)
	markDone           func(ctx context.Context, id int64) (int64, error)
	sumValueDoneByType func(ctx context.Context, typeArg int16) (int64, error)
	countDoneByType    func(ctx context.Context, typeArg int16) (int64, error)
}

func (m *mockTaskStore) GetTask(ctx context.Context, id int64) (db.Task, error) {
	if m.getTask != nil {
		return m.getTask(ctx, id)
	}
	return db.Task{}, pgx.ErrNoRows
}

func (m *mockTaskStore) TryMarkProcessing(ctx context.Context, id int64) (int64, error) {
	if m.tryMarkProcessing != nil {
		return m.tryMarkProcessing(ctx, id)
	}
	return 1, nil
}

func (m *mockTaskStore) MarkDone(ctx context.Context, id int64) (int64, error) {
	if m.markDone != nil {
		return m.markDone(ctx, id)
	}
	return 1, nil
}

func (m *mockTaskStore) SumValueDoneByType(ctx context.Context, typeArg int16) (int64, error) {
	if m.sumValueDoneByType != nil {
		return m.sumValueDoneByType(ctx, typeArg)
	}
	return 0, nil
}

func (m *mockTaskStore) CountDoneByType(ctx context.Context, typeArg int16) (int64, error) {
	if m.countDoneByType != nil {
		return m.countDoneByType(ctx, typeArg)
	}
	return 0, nil
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

func newTestHandler(q TaskStore, lim *WindowLimiter) *TaskHandler {
	reg := prometheus.NewRegistry()
	return &TaskHandler{
		Log:     discardLog(),
		Queries: q,
		Limiter: lim,
		Metrics: metrics.NewConsumer(reg),
	}
}

func TestHandlerWrongMethod(t *testing.T) {
	h := newTestHandler(&mockTaskStore{}, NewWindowLimiter(10, time.Second))
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandlerInvalidJSON(t *testing.T) {
	h := newTestHandler(&mockTaskStore{}, NewWindowLimiter(10, time.Second))
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandlerUnknownTask(t *testing.T) {
	q := &mockTaskStore{
		getTask: func(ctx context.Context, id int64) (db.Task, error) {
			return db.Task{}, pgx.ErrNoRows
		},
	}
	h := newTestHandler(q, NewWindowLimiter(10, time.Second))
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"id":1,"type":2,"value":3}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandlerMismatch(t *testing.T) {
	q := &mockTaskStore{
		getTask: func(ctx context.Context, id int64) (db.Task, error) {
			return db.Task{ID: 1, Type: 9, Value: 1, State: db.TaskStateReceived}, nil
		},
	}
	h := newTestHandler(q, NewWindowLimiter(10, time.Second))
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"id":1,"type":2,"value":3}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandlerDuplicateNoRateLimit(t *testing.T) {
	lim := NewWindowLimiter(0, time.Second) // always deny new work
	q := &mockTaskStore{
		getTask: func(ctx context.Context, id int64) (db.Task, error) {
			return db.Task{ID: 1, Type: 2, Value: 0, State: db.TaskStateDone}, nil
		},
	}
	h := newTestHandler(q, lim)
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"id":1,"type":2,"value":0}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d want 200 duplicate before rate limit", rec.Code)
	}
}

func TestHandlerRateLimited(t *testing.T) {
	lim := NewWindowLimiter(1, time.Minute)
	q := &mockTaskStore{
		getTask: func(ctx context.Context, id int64) (db.Task, error) {
			switch id {
			case 1:
				return db.Task{ID: 1, Type: 0, Value: 0, State: db.TaskStateReceived}, nil
			case 2:
				return db.Task{ID: 2, Type: 0, Value: 0, State: db.TaskStateReceived}, nil
			default:
				return db.Task{}, pgx.ErrNoRows
			}
		},
	}
	h := newTestHandler(q, lim)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"id":1,"type":0,"value":0}`)))
	if rec1.Code != http.StatusAccepted {
		t.Fatalf("first got %d", rec1.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"id":2,"type":0,"value":0}`)))
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second got %d want 429", rec2.Code)
	}
}

func TestHandlerAccepted(t *testing.T) {
	q := &mockTaskStore{
		getTask: func(ctx context.Context, id int64) (db.Task, error) {
			return db.Task{ID: 7, Type: 3, Value: 0, State: db.TaskStateReceived}, nil
		},
		sumValueDoneByType: func(ctx context.Context, typeArg int16) (int64, error) { return 3, nil },
		countDoneByType:    func(ctx context.Context, typeArg int16) (int64, error) { return 1, nil },
	}
	h := newTestHandler(q, NewWindowLimiter(10, time.Second))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"id":7,"type":3,"value":0}`)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("got %d", rec.Code)
	}
}
