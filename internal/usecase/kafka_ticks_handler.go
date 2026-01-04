package usecase

import (
	"context"
	"encoding/json"
	"time"

	"FinPull/internal/domain/models"
	domrepo "FinPull/internal/domain/repository"
	pkgkafka "FinPull/pkg/kafka"
)

// KafkaTicksHandler consumes Kafka messages and writes to storage.
type KafkaTicksHandler struct {
	topic   string
	storage domrepo.Storage
	metrics domrepo.Metrics
}

func NewKafkaTicksHandler(topic string, storage domrepo.Storage, metrics domrepo.Metrics) *KafkaTicksHandler {
	return &KafkaTicksHandler{topic: topic, storage: storage, metrics: metrics}
}

func (h *KafkaTicksHandler) Topic() string { return h.topic }

// incoming message schema: {symbol, t, c, v}
func (h *KafkaTicksHandler) Handle(ctx context.Context, b []byte) error {
	var m struct {
		Symbol string  `json:"symbol"`
		T      int64   `json:"t"`
		C      float64 `json:"c"`
		V      float64 `json:"v"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		h.metrics.RecordError("consumer_unmarshal")
		return err
	}
	if m.T > 1e11 { // ms
		m.T = m.T / 1000
	}
	// E2E latency from event time to now (approx)
	h.metrics.RecordLatency("ingest_e2e_seconds", time.Since(time.Unix(m.T, 0)).Seconds())

	start := time.Now()
	err := h.storage.Store(ctx, &models.Trade{
		Symbol:    m.Symbol,
		Timestamp: m.T,
		Price:     m.C,
		Volume:    m.V,
	})
	h.metrics.RecordLatency("ch_insert_seconds", time.Since(start).Seconds())
	if err != nil {
		h.metrics.RecordError("consumer_store")
		return err
	}
	h.metrics.RecordMessageSent("clickhouse", m.Symbol)

	// Approx MV lag to VN bucket boundary (MV completion not checked)
	vnBucket := time.Unix(m.T, 0).In(time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)).Truncate(time.Second)
	h.metrics.RecordLatency("mv_lag_seconds", time.Since(vnBucket).Seconds())
	return nil
}

var _ pkgkafka.MessageHandler = (*KafkaTicksHandler)(nil)
