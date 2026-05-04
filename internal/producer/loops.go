package producer

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"marbl/internal/config"
	"marbl/internal/metrics"
	"marbl/internal/persistence/db"
)

func RunGenerationLoop(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config.Producer, m *metrics.Producer) {
	q := db.New(pool)
	rate := cfg.RatePerSec
	if rate < 1 {
		rate = 1
	}
	interval := time.Second / time.Duration(rate)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			backlog, err := q.CountBacklog(ctx)
			if err != nil {
				log.Error("count backlog", "err", err)
				continue
			}
			if ShouldPauseGeneration(backlog, cfg.MaxBacklog) {
				m.BacklogPauses.Inc()
				continue
			}
			typ := int16(rand.IntN(10))
			val := int16(rand.IntN(100))
			if _, err := q.CreateTask(ctx, db.CreateTaskParams{Type: typ, Value: val}); err != nil {
				log.Error("create task", "err", err)
				continue
			}
			m.Generated.Inc()
		}
	}
}

func RunDeliveryLoop(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, cfg config.Producer, m *metrics.Producer) {
	q := db.New(pool)
	client := &http.Client{Timeout: cfg.HTTPClientTimeout}
	inflight := &InFlight{}
	poll := time.NewTicker(cfg.DeliveryPollInterval)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			tasks, err := q.ListReceivedTasks(ctx, cfg.DeliveryBatchSize)
			if err != nil {
				log.Error("list received tasks", "err", err)
				continue
			}
			for _, task := range tasks {
				if !inflight.TryAdd(task.ID) {
					continue
				}
				deliverOne(ctx, log, client, cfg, m, task)
				inflight.Remove(task.ID)
			}
		}
	}
}

func deliverOne(ctx context.Context, log *slog.Logger, client *http.Client, cfg config.Producer, m *metrics.Producer, task db.Task) {
	backoff := 100 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return
		}
		code, err := PostTask(ctx, client, cfg.ConsumerURL, task)
		if err != nil {
			m.RecordSend("error")
			log.Warn("delivery error", "id", task.ID, "err", err)
			sleepWithCap(ctx, &backoff, cfg.MaxDeliveryBackoff)
			continue
		}
		switch code {
		case http.StatusOK, http.StatusAccepted:
			m.RecordSend("accepted")
			return
		case http.StatusTooManyRequests:
			m.RecordSend("rate_limited")
			sleepWithCap(ctx, &backoff, cfg.MaxDeliveryBackoff)
		case http.StatusBadRequest:
			m.RecordSend("error")
			log.Error("delivery 400", "id", task.ID)
			return
		default:
			if code == http.StatusNotFound || code >= 500 {
				m.RecordSend("error")
				log.Warn("delivery retryable status", "id", task.ID, "status", code)
				sleepWithCap(ctx, &backoff, cfg.MaxDeliveryBackoff)
				continue
			}
			m.RecordSend("error")
			log.Error("delivery unexpected status", "id", task.ID, "status", code)
			return
		}
	}
}

func sleepWithCap(ctx context.Context, backoff *time.Duration, max time.Duration) {
	sleepBackoff(ctx, *backoff)
	next := *backoff * 2
	if next > max {
		next = max
	}
	*backoff = next
}
