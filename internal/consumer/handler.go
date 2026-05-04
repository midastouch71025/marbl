package consumer

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"marbl/internal/domain"
	"marbl/internal/httpapi"
	"marbl/internal/metrics"
	"marbl/internal/persistence/db"
)

type TaskHandler struct {
	Log     *slog.Logger
	Queries TaskStore
	Limiter *WindowLimiter
	Metrics *metrics.Consumer
}

func (h *TaskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/tasks" {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	req, err := httpapi.DecodeTaskRequest(r.Body)
	if err != nil {
		h.Metrics.TasksInvalid.Inc()
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if err := domain.ValidateTaskFields(req.ID, int16(req.Type), int16(req.Value)); err != nil {
		h.Metrics.TasksInvalid.Inc()
		http.Error(w, `{"error":"invalid task fields"}`, http.StatusBadRequest)
		return
	}
	task, err := h.Queries.GetTask(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.Metrics.TasksUnknown.Inc()
			http.Error(w, `{"error":"unknown task"}`, http.StatusNotFound)
			return
		}
		h.Log.Error("get task", "err", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if task.Type != int16(req.Type) || task.Value != int16(req.Value) {
		h.Metrics.TasksInvalid.Inc()
		http.Error(w, `{"error":"task mismatch"}`, http.StatusBadRequest)
		return
	}
	if task.State == db.TaskStateProcessing || task.State == db.TaskStateDone {
		h.Metrics.TasksDuplicate.Inc()
		w.WriteHeader(http.StatusOK)
		return
	}
	if task.State != db.TaskStateReceived {
		h.Log.Error("unexpected task state", "state", task.State)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if !h.Limiter.Allow(time.Now()) {
		h.Metrics.TasksRateLimited.Inc()
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	n, err := h.Queries.TryMarkProcessing(ctx, req.ID)
	if err != nil {
		h.Log.Error("mark processing", "err", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if n == 0 {
		task2, err := h.Queries.GetTask(ctx, req.ID)
		if err != nil {
			h.Log.Error("get task after lost race", "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		if task2.State == db.TaskStateProcessing || task2.State == db.TaskStateDone {
			h.Metrics.TasksDuplicate.Inc()
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, `{"error":"conflict"}`, http.StatusServiceUnavailable)
		return
	}
	workTask, err := h.Queries.GetTask(ctx, req.ID)
	if err != nil {
		h.Log.Error("reload task", "err", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	time.Sleep(time.Duration(workTask.Value) * time.Millisecond)
	dn, err := h.Queries.MarkDone(ctx, req.ID)
	if err != nil {
		h.Log.Error("mark done", "err", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if dn == 0 {
		h.Log.Warn("mark done affected no rows", "id", req.ID)
	}
	lt := strconv.Itoa(int(workTask.Type))
	sum, err := h.Queries.SumValueDoneByType(ctx, workTask.Type)
	if err != nil {
		h.Log.Error("sum done", "err", err)
	} else {
		h.Log.Info("task done",
			"id", workTask.ID,
			"type", workTask.Type,
			"value", workTask.Value,
			"sum_done_type", sum,
		)
		h.Metrics.ValueSumDone.WithLabelValues(lt).Set(float64(sum))
	}
	if cnt, err := h.Queries.CountDoneByType(ctx, workTask.Type); err != nil {
		h.Log.Error("count done", "err", err)
	} else {
		h.Metrics.CountDoneByType.WithLabelValues(lt).Set(float64(cnt))
	}
	h.Metrics.TasksAccepted.Inc()
	h.Metrics.TasksProcessed.WithLabelValues(strconv.Itoa(int(workTask.Type))).Inc()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
	}{Status: "done"})
}
