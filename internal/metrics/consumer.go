package metrics

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"marbl/internal/persistence/db"
)

type Consumer struct {
	TasksAccepted    prometheus.Counter
	TasksDuplicate   prometheus.Counter
	TasksRateLimited prometheus.Counter
	TasksInvalid     prometheus.Counter
	TasksUnknown     prometheus.Counter
	TasksProcessed   *prometheus.CounterVec
	ValueSumDone     *prometheus.GaugeVec
	CountDoneByType  *prometheus.GaugeVec
	reg              *prometheus.Registry
}

func NewConsumer(reg *prometheus.Registry) *Consumer {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	f := promauto.With(reg)
	return &Consumer{
		TasksAccepted: f.NewCounter(prometheus.CounterOpts{
			Name: "consumer_tasks_accepted_total",
			Help: "Tasks that started processing (202).",
		}),
		TasksDuplicate: f.NewCounter(prometheus.CounterOpts{
			Name: "consumer_tasks_duplicate_total",
			Help: "Idempotent replays returning 200.",
		}),
		TasksRateLimited: f.NewCounter(prometheus.CounterOpts{
			Name: "consumer_tasks_rate_limited_total",
			Help: "Requests rejected with 429.",
		}),
		TasksInvalid: f.NewCounter(prometheus.CounterOpts{
			Name: "consumer_tasks_invalid_total",
			Help: "Malformed or out-of-bounds payloads (400).",
		}),
		TasksUnknown: f.NewCounter(prometheus.CounterOpts{
			Name: "consumer_tasks_unknown_total",
			Help: "Unknown task ids (404).",
		}),
		TasksProcessed: f.NewCounterVec(prometheus.CounterOpts{
			Name: "consumer_tasks_processed_total",
			Help: "Tasks completed by type.",
		}, []string{"type"}),
		ValueSumDone: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "consumer_value_sum_done_by_type",
			Help: "Authoritative sum of value for done tasks by type.",
		}, []string{"type"}),
		CountDoneByType: f.NewGaugeVec(prometheus.GaugeOpts{
			Name: "consumer_done_count_by_type",
			Help: "Authoritative done task counts by type.",
		}, []string{"type"}),
		reg: reg,
	}
}

func (c *Consumer) RefreshTypeAggregates(ctx context.Context, q *db.Queries) error {
	for typ := int16(0); typ <= 9; typ++ {
		sum, err := q.SumValueDoneByType(ctx, typ)
		if err != nil {
			return err
		}
		cnt, err := q.CountDoneByType(ctx, typ)
		if err != nil {
			return err
		}
		l := strconv.Itoa(int(typ))
		c.ValueSumDone.WithLabelValues(l).Set(float64(sum))
		c.CountDoneByType.WithLabelValues(l).Set(float64(cnt))
	}
	return nil
}

func (c *Consumer) RunAggregateGaugeLoop(ctx context.Context, log *slog.Logger, q *db.Queries, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := c.RefreshTypeAggregates(ctx, q); err != nil && log != nil {
			log.Warn("refresh aggregate gauges", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
