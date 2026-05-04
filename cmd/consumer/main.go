package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"marbl/internal/assets"
	"marbl/internal/config"
	"marbl/internal/consumer"
	"marbl/internal/logging"
	"marbl/internal/metrics"
	"marbl/internal/openapi"
	"marbl/internal/persistence"
	"marbl/internal/persistence/db"
	"marbl/internal/serverutil"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.LoadConsumer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(2)
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reg := prometheus.NewRegistry()
	cm := metrics.NewConsumer(reg)
	serverutil.StartMetrics(ctx, log, cfg.PrometheusPort, reg)
	serverutil.StartPprof(ctx, log, cfg.PprofPort)

	pool, err := persistence.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := db.New(pool)
	limiter := consumer.NewWindowLimiter(cfg.RateLimitCount, cfg.RateLimitWindow)
	h := &consumer.TaskHandler{
		Log:     log,
		Queries: q,
		Limiter: limiter,
		Metrics: cm,
	}

	mux := http.NewServeMux()
	mux.Handle("/tasks", h)
	mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/swagger" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/swagger/", http.StatusFound)
	})
	mux.HandleFunc("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(assets.SwaggerIndexHTML)
	})
	mux.HandleFunc("/swagger.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openapi.Spec())
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server", "err", err)
		}
	}()

	go consumer.RunReaperLoop(ctx, log, q, cfg.ReaperStaleSec, cfg.ReaperInterval)
	go cm.RunAggregateGaugeLoop(ctx, log, q, 5*time.Second)

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown", "err", err)
	}
}
