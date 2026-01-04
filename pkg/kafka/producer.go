package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
)

// Producer wraps Kafka writer.
type Producer struct {
	writer *kafka.Writer
	comp   string
}

// NewProducer creates a new Kafka producer.
func NewProducer(opts ...ProducerOption) (*Producer, error) {
	cfg := &ProducerConfig{
		RequiredAcks: -1,
		Compression:  "gzip",
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		BatchSize:    100,
		BatchBytes:   1048576,
		BatchTimeout: 1 * time.Second,
		Async:        false,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers are required")
	}

	bal := kafka.Balancer(&kafka.LeastBytes{})
	if cfg.HashByKey {
		bal = &kafka.Hash{}
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     bal,
		RequiredAcks: kafka.RequiredAcks(cfg.RequiredAcks),
		Compression:  parseCompression(cfg.Compression),
		MaxAttempts:  cfg.MaxAttempts,
		WriteTimeout: cfg.WriteTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		BatchSize:    cfg.BatchSize,
		BatchBytes:   int64(cfg.BatchBytes),
		BatchTimeout: cfg.BatchTimeout,
		Async:        cfg.Async,
	}

	initProducerMetricsOnce()
	return &Producer{writer: writer, comp: cfg.Compression}, nil
}

// Publish sends a message to the specified topic.
func (p *Producer) Publish(ctx context.Context, topic string, key []byte, value interface{}) error {
	start := time.Now()
	var v []byte
	switch val := value.(type) {
	case []byte:
		v = val
	case string:
		v = []byte(val)
	default:
		var err error
		v, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal value: %w", err)
		}
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   key,
		Value: v,
		Time:  time.Now(),
	}

	err := p.writer.WriteMessages(ctx, msg)
	observeProducerMetrics(topic, p.comp, int64(len(v)), 1, time.Since(start), err)
	if err != nil {
		return err
	}
	return nil
}

// PublishBatch sends multiple messages to the specified topic.
func (p *Producer) PublishBatch(ctx context.Context, topic string, messages []Message) error {
	if len(messages) == 0 {
		return nil
	}

	start := time.Now()
	msgs := make([]kafka.Message, 0, len(messages))
	var totalBytes int64
	for _, m := range messages {
		var v []byte
		switch val := m.Value.(type) {
		case []byte:
			v = val
		case string:
			v = []byte(val)
		default:
			var err error
			v, err = json.Marshal(m.Value)
			if err != nil {
				return fmt.Errorf("marshal value: %w", err)
			}
		}

		msgs = append(msgs, kafka.Message{
			Topic: topic,
			Key:   m.Key,
			Value: v,
			Time:  time.Now(),
		})
		totalBytes += int64(len(v))
	}

	err := p.writer.WriteMessages(ctx, msgs...)
	observeProducerMetrics(topic, p.comp, totalBytes, len(messages), time.Since(start), err)
	if err != nil {
		return err
	}
	return nil
}

// Close closes the producer.
func (p *Producer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

// Message represents a Kafka message.
type Message struct {
	Key   []byte
	Value interface{}
}

func parseCompression(s string) kafka.Compression {
	switch s {
	case "gzip":
		return kafka.Gzip
	case "snappy":
		return kafka.Snappy
	case "lz4":
		return kafka.Lz4
	case "zstd":
		return kafka.Zstd
	default:
		return kafka.Gzip
	}
}

var (
	producerMsgsTotal   *prometheus.CounterVec
	producerErrsTotal   *prometheus.CounterVec
	producerBytesTotal  *prometheus.CounterVec
	producerLatencyHist *prometheus.HistogramVec
	producerOnce        = make(chan struct{}, 1)
)

func initProducerMetricsOnce() {
	select {
	case producerOnce <- struct{}{}:
		producerMsgsTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "finpull_kafka_producer_messages_total",
				Help: "Total messages published to Kafka",
			},
			[]string{"topic", "compression", "result"},
		)
		producerErrsTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "finpull_kafka_producer_errors_total",
				Help: "Total producer errors",
			},
			[]string{"topic"},
		)
		producerBytesTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "finpull_kafka_producer_bytes_total",
				Help: "Total payload bytes published",
			},
			[]string{"topic", "compression"},
		)
		producerLatencyHist = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "finpull_kafka_producer_publish_seconds",
				Help:    "Publish latency",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"topic"},
		)
	default:
		// already initialized
	}
}

func observeProducerMetrics(topic, comp string, bytes int64, count int, dur time.Duration, err error) {
	if producerMsgsTotal == nil {
		return
	}
	result := "ok"
	if err != nil {
		result = "error"
		producerErrsTotal.WithLabelValues(topic).Inc()
	}
	producerMsgsTotal.WithLabelValues(topic, comp, result).Add(float64(count))
	producerBytesTotal.WithLabelValues(topic, comp).Add(float64(bytes))
	producerLatencyHist.WithLabelValues(topic).Observe(dur.Seconds())
}
