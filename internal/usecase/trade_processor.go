package usecase

import (
	"context"
	"fmt"
	"time"

	"FinPull/internal/domain/models"
	drepo "FinPull/internal/domain/repository"
)

// TradeProcessor processes trade data and routes to appropriate backend.
type TradeProcessor struct {
	pub     drepo.Publisher
	store   drepo.Storage
	metrics drepo.Metrics
	backend string
	batchSz int
	batchTO time.Duration
}

// NewTradeProcessor creates a new TradeProcessor instance.
func NewTradeProcessor(
	pub drepo.Publisher,
	store drepo.Storage,
	metrics drepo.Metrics,
	backend string,
	batchSz int,
	batchTO time.Duration,
) *TradeProcessor {
	return &TradeProcessor{
		pub:     pub,
		store:   store,
		metrics: metrics,
		backend: backend,
		batchSz: batchSz,
		batchTO: batchTO,
	}
}

// Process processes a single trade and routes it to the configured backend.
func (p *TradeProcessor) Process(ctx context.Context, t *models.Trade) error {
	if t == nil {
		return fmt.Errorf("trade is nil")
	}

	fmt.Println("HITS PROCESSOR:", t)
	//fmt.Println(p.backend)
	start := time.Now()
	var err error

	switch p.backend {
	case "kafka":
		err = p.pub.Publish(ctx, t)
	case "clickhouse":
		err = p.store.Store(ctx, t)
	default:
		err = fmt.Errorf("unknown backend: %s", p.backend)
	}

	if err != nil {
		p.metrics.RecordError("process")
		return fmt.Errorf("process trade: %w", err)
	}

	p.metrics.RecordMessageSent(p.backend, t.Symbol)
	p.metrics.RecordLatency("process", time.Since(start).Seconds())

	return nil
}

// ProcessBatch processes multiple trades in a batch.
func (p *TradeProcessor) ProcessBatch(ctx context.Context, trades []*models.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	start := time.Now()
	var err error

	switch p.backend {
	case "kafka":
		err = p.pub.PublishBatch(ctx, trades)
	case "clickhouse":
		err = p.store.StoreBatch(ctx, trades)
	default:
		err = fmt.Errorf("unknown backend: %s", p.backend)
	}

	if err != nil {
		p.metrics.RecordError("process_batch")
		return fmt.Errorf("process batch: %w", err)
	}

	for _, t := range trades {
		p.metrics.RecordMessageSent(p.backend, t.Symbol)
	}
	p.metrics.RecordLatency("process_batch", time.Since(start).Seconds())

	return nil
}

// Close closes underlying resources if available.
func (p *TradeProcessor) Close() {
	if p.pub != nil {
		_ = p.pub.Close()
	}
	if p.store != nil {
		_ = p.store.Close()
	}
}
