package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"

	"marbl/internal/config"
	"marbl/internal/logging"
	"marbl/internal/metrics"
	"marbl/internal/openapi"
	"marbl/internal/persistence"
	"marbl/internal/persistence/db"
	"marbl/internal/producer"
	"marbl/internal/serverutil"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	showOpenAPI := flag.Bool("openapi", false, "print embedded OpenAPI spec and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *showOpenAPI {
		_, _ = os.Stdout.Write(openapi.Spec())
		return
	}

	cfg, err := config.LoadProducer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(2)
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reg := prometheus.NewRegistry()
	pm := metrics.NewProducer(reg)
	serverutil.StartMetrics(ctx, log, cfg.PrometheusPort, reg)
	serverutil.StartPprof(ctx, log, cfg.PprofPort)

	pool, err := persistence.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := db.New(pool)
	if err := pm.RefreshTaskStateGauges(ctx, q); err != nil {
		log.Warn("initial gauge refresh", "err", err)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		producer.RunGenerationLoop(ctx, log, pool, cfg, pm)
	}()
	go func() {
		defer wg.Done()
		pm.RunStateGaugeLoop(ctx, log, q, cfg.TasksInStateRefresh)
	}()
	go func() {
		defer wg.Done()
		producer.RunDeliveryLoop(ctx, log, pool, cfg, pm)
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		log.Warn("shutdown timeout waiting for loops")
	}
}
