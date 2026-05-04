package config

import (
	"testing"
	"time"
)

func TestLoadProducerDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("CONSUMER_URL", "http://localhost:8080")

	cfg, err := LoadProducer()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxBacklog != 1000 {
		t.Fatalf("MaxBacklog: got %d want 1000", cfg.MaxBacklog)
	}
	if cfg.RatePerSec != 10 {
		t.Fatalf("RatePerSec: got %d want 10", cfg.RatePerSec)
	}
	if cfg.PrometheusPort != "9091" {
		t.Fatalf("PrometheusPort: got %q", cfg.PrometheusPort)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("ShutdownTimeout: got %v", cfg.ShutdownTimeout)
	}
}

func TestLoadProducerRateValidation(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("CONSUMER_URL", "http://localhost:8080")
	t.Setenv("PRODUCER_RATE_PER_SEC", "0")
	if _, err := LoadProducer(); err == nil {
		t.Fatal("expected error for PRODUCER_RATE_PER_SEC < 1")
	}
}

func TestLoadConsumerDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	cfg, err := LoadConsumer()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr: got %q", cfg.ListenAddr)
	}
	if cfg.RateLimitCount != 100 {
		t.Fatalf("RateLimitCount: got %d", cfg.RateLimitCount)
	}
	if cfg.RateLimitWindow != time.Second {
		t.Fatalf("RateLimitWindow: got %v", cfg.RateLimitWindow)
	}
	if cfg.ReaperStaleSec != 5 {
		t.Fatalf("ReaperStaleSec: got %v", cfg.ReaperStaleSec)
	}
}
