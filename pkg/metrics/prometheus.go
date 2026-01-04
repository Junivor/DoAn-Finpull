package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Recorder implements domain.repository.Metrics using Prometheus.
type Recorder struct {
	messagesSent *prometheus.CounterVec
	errorsTotal  *prometheus.CounterVec
	lastPrice    *prometheus.GaugeVec
	latency      *prometheus.HistogramVec
}

// New creates a new Prometheus metrics recorder.
func New() *Recorder {
	return &Recorder{
		messagesSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "finpull_messages_sent_total",
				Help: "Total number of messages sent to backend",
			},
			[]string{"backend", "symbol"},
		),
		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "finpull_errors_total",
				Help: "Total number of errors encountered",
			},
			[]string{"type"},
		),
		lastPrice: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "finpull_last_price",
				Help: "Last recorded price for a symbol",
			},
			[]string{"symbol"},
		),
		latency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "finpull_operation_duration_seconds",
				Help:    "Duration of operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
	}
}

// RecordMessageSent records a message sent to a backend.
func (r *Recorder) RecordMessageSent(backend, symbol string) {
	r.messagesSent.WithLabelValues(backend, symbol).Inc()
}

// RecordError records an error occurrence.
func (r *Recorder) RecordError(kind string) {
	r.errorsTotal.WithLabelValues(kind).Inc()
}

// RecordLastPrice records the last price for a symbol.
func (r *Recorder) RecordLastPrice(symbol string, price float64) {
	r.lastPrice.WithLabelValues(symbol).Set(price)
}

// RecordLatency records operation latency in seconds.
func (r *Recorder) RecordLatency(op string, seconds float64) {
	r.latency.WithLabelValues(op).Observe(seconds)
}
