package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"marbl/internal/persistence/db"
)

type Producer struct {
	Generated       prometheus.Counter
	SendAttempts    *prometheus.CounterVec
	BacklogPauses   prometheus.Counter
	TasksInState    *prometheus.GaugeVec
	reg             *prometheus.Registry
	tasksInStateTTL time.Duration
}

func NewProducer(reg *prometheus.Registry) *Producer {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	factory := promauto.With(reg)
	return &Producer{
		Generated: factory.NewCounter(prometheus.CounterOpts{
			Name: "producer_generated_total",
			Help: "Tasks inserted into the database in received state.",
		}),
		SendAttempts: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "producer_send_attempts_total",
			Help: "Delivery attempts to the consumer.",
		}, []string{"outcome"}),
		BacklogPauses: factory.NewCounter(prometheus.CounterOpts{
			Name: "producer_backlog_pause_total",
			Help: "Times generation paused due to backlog cap.",
		}),
		TasksInState: factory.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tasks_in_state",
			Help: "Current task counts by state from the database.",
		}, []string{"state"}),
		reg: reg,
	}
}

func (p *Producer) RecordSend(outcome string) {
	p.SendAttempts.WithLabelValues(outcome).Inc()
}

// RefreshTaskStateGauges queries the database and updates tasks_in_state gauges.
func (p *Producer) RefreshTaskStateGauges(ctx context.Context, q *db.Queries) error {
	rows, err := q.TaskCountsByState(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, r := range rows {
		l := string(r.State)
		p.TasksInState.WithLabelValues(l).Set(float64(r.Cnt))
		seen[l] = true
	}
	for _, s := range []string{"received", "processing", "done"} {
		if !seen[s] {
			p.TasksInState.WithLabelValues(s).Set(0)
		}
	}
	return nil
}

func (p *Producer) RunStateGaugeLoop(ctx context.Context, log *slog.Logger, q *db.Queries, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := p.RefreshTaskStateGauges(ctx, q); err != nil && log != nil {
			log.Warn("refresh task state gauges", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
