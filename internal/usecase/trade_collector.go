package usecase

import (
	"FinPull/internal/domain/models"
	drepo "FinPull/internal/domain/repository"
	mid "FinPull/internal/middleware"
	"context"
)

// TradeCollector collects trades from market stream and processes them.
type TradeCollector struct {
	stream  drepo.MarketStream
	proc    *TradeProcessor
	metrics drepo.Metrics
	pipe    *mid.RealtimePipeline
}

// NewTradeCollector creates a new TradeCollector instance.
func NewTradeCollector(stream drepo.MarketStream, proc *TradeProcessor, metrics drepo.Metrics, pipe *mid.RealtimePipeline) *TradeCollector {
	return &TradeCollector{stream: stream, proc: proc, metrics: metrics, pipe: pipe}
}

// IsConnected returns true if the market stream is connected.
func (c *TradeCollector) IsConnected() bool {
	return c.stream.IsConnected()
}

func (c *TradeCollector) Start(ctx context.Context) error {
	if err := c.stream.Connect(ctx); err != nil {
		return err
	}
	if err := c.stream.Subscribe(ctx); err != nil {
		return err
	}
	if c.pipe != nil {
		c.pipe.Start(ctx)
	}
	trCh, errCh := c.stream.Read(ctx)
	go c.consume(ctx, trCh, errCh)
	return nil
}

func (c *TradeCollector) consume(ctx context.Context, trCh <-chan *models.Trade, errCh <-chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				c.metrics.RecordError("stream")
				_ = c.stream.Reconnect(ctx)
			}
		case t := <-trCh:
			if t == nil {
				continue
			}
			if c.pipe != nil {
				_ = c.pipe.Process(ctx, t)
			} else {
				_ = c.proc.Process(ctx, t)
			}
			c.metrics.RecordLastPrice(t.Symbol, t.Price)
		}
	}
}

func (c *TradeCollector) Stop() error { return c.stream.Close() }

// Processor returns the underlying TradeProcessor for lifecycle management.
func (c *TradeCollector) Processor() *TradeProcessor { return c.proc }

// Shutdown stops pipeline and closes stream.
func (c *TradeCollector) Shutdown(ctx context.Context) error {
	if c.pipe != nil {
		c.pipe.Stop()
	}
	return c.stream.Close()
}
