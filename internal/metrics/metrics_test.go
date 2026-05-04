package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestProducerMetricsRegister(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewProducer(reg)
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
	}
}

func TestConsumerMetricsRegister(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewConsumer(reg)
	if _, err := reg.Gather(); err != nil {
		t.Fatal(err)
	}
}
