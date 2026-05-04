package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) (int, error) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func getenvInt64(key string, def int64) (int64, error) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func getenvFloat(key string, def float64) (float64, error) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func getenvDuration(key string, def time.Duration) (time.Duration, error) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

type Producer struct {
	DatabaseURL          string
	ConsumerURL          string
	PrometheusPort       string
	PprofPort            string
	LogLevel             string
	LogFormat            string
	MaxBacklog           int64
	RatePerSec           int
	ShutdownTimeout      time.Duration
	DeliveryBatchSize    int32
	DeliveryPollInterval time.Duration
	MaxDeliveryBackoff   time.Duration
	HTTPClientTimeout    time.Duration
	TasksInStateRefresh  time.Duration
}

func LoadProducer() (Producer, error) {
	maxBacklog, err := getenvInt64("PRODUCER_MAX_BACKLOG", 1000)
	if err != nil {
		return Producer{}, err
	}
	rate, err := getenvInt("PRODUCER_RATE_PER_SEC", 10)
	if err != nil {
		return Producer{}, err
	}
	if rate < 1 {
		return Producer{}, fmt.Errorf("PRODUCER_RATE_PER_SEC must be >= 1")
	}
	shutdownSec, err := getenvInt("SHUTDOWN_TIMEOUT_SECONDS", 30)
	if err != nil {
		return Producer{}, err
	}
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return Producer{}, fmt.Errorf("DATABASE_URL is required")
	}
	consumerURL := strings.TrimSpace(os.Getenv("CONSUMER_URL"))
	if consumerURL == "" {
		return Producer{}, fmt.Errorf("CONSUMER_URL is required")
	}
	return Producer{
		DatabaseURL:          dbURL,
		ConsumerURL:          strings.TrimRight(consumerURL, "/"),
		PrometheusPort:       getenv("PROMETHEUS_PORT", "9091"),
		PprofPort:            getenv("PPROF_PORT", "6060"),
		LogLevel:             getenv("LOG_LEVEL", "info"),
		LogFormat:            getenv("LOG_FORMAT", "text"),
		MaxBacklog:           maxBacklog,
		RatePerSec:           rate,
		ShutdownTimeout:      time.Duration(shutdownSec) * time.Second,
		DeliveryBatchSize:    32,
		DeliveryPollInterval: 200 * time.Millisecond,
		MaxDeliveryBackoff:   30 * time.Second,
		HTTPClientTimeout:    10 * time.Second,
		TasksInStateRefresh:  5 * time.Second,
	}, nil
}

type Consumer struct {
	DatabaseURL     string
	ListenAddr      string
	PrometheusPort  string
	PprofPort       string
	LogLevel        string
	LogFormat       string
	RateLimitCount  int
	RateLimitWindow time.Duration
	ReaperStaleSec  float64
	ShutdownTimeout time.Duration
	ReaperInterval  time.Duration
}

func LoadConsumer() (Consumer, error) {
	shutdownSec, err := getenvInt("SHUTDOWN_TIMEOUT_SECONDS", 30)
	if err != nil {
		return Consumer{}, err
	}
	rlCount, err := getenvInt("CONSUMER_RATE_LIMIT_COUNT", 100)
	if err != nil {
		return Consumer{}, err
	}
	rlWindow, err := getenvDuration("CONSUMER_RATE_LIMIT_WINDOW", time.Second)
	if err != nil {
		return Consumer{}, err
	}
	stale, err := getenvFloat("CONSUMER_REAPER_STALE_SECONDS", 5)
	if err != nil {
		return Consumer{}, err
	}
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return Consumer{}, fmt.Errorf("DATABASE_URL is required")
	}
	return Consumer{
		DatabaseURL:     dbURL,
		ListenAddr:      getenv("LISTEN_ADDR", ":8080"),
		PrometheusPort:  getenv("PROMETHEUS_PORT", "9092"),
		PprofPort:       getenv("PPROF_PORT", "6061"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		LogFormat:       getenv("LOG_FORMAT", "text"),
		RateLimitCount:  rlCount,
		RateLimitWindow: rlWindow,
		ReaperStaleSec:  stale,
		ShutdownTimeout: time.Duration(shutdownSec) * time.Second,
		ReaperInterval:  30 * time.Second,
	}, nil
}

type Migrate struct {
	DatabaseURL string
}

func LoadMigrate() (Migrate, error) {
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return Migrate{}, fmt.Errorf("DATABASE_URL is required")
	}
	return Migrate{DatabaseURL: dbURL}, nil
}
