package consumer

import (
	"context"
	"log/slog"
	"time"

	"marbl/internal/persistence/db"
)

func RunReaperLoop(ctx context.Context, log *slog.Logger, q *db.Queries, staleSeconds float64, every time.Duration) {
	reapOnce(ctx, log, q, staleSeconds)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			reapOnce(ctx, log, q, staleSeconds)
		}
	}
}

func reapOnce(ctx context.Context, log *slog.Logger, q *db.Queries, staleSeconds float64) {
	cutoff := StaleCutoff(time.Now(), staleSeconds)
	n, err := q.ReapStaleProcessing(ctx, cutoff)
	if err != nil {
		log.Error("reap stale processing", "err", err)
		return
	}
	if n > 0 {
		log.Info("reaped stale processing rows", "count", n, "cutoff", cutoff)
	}
}
