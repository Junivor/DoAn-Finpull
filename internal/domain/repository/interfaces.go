package repository

import (
	"context"
	"time"

	"FinPull/internal/domain/models"
)

type MarketStream interface {
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context) error
	Read(ctx context.Context) (<-chan *models.Trade, <-chan error)
	Reconnect(ctx context.Context) error
	Close() error
	IsConnected() bool
}

type Publisher interface {
	Publish(ctx context.Context, t *models.Trade) error
	PublishBatch(ctx context.Context, trades []*models.Trade) error
	Close() error
}

type Storage interface {
	Init(ctx context.Context) error // ensure tables, health checks
	Store(ctx context.Context, t *models.Trade) error
	StoreBatch(ctx context.Context, trades []*models.Trade) error
	Query(ctx context.Context, symbol string, from, to time.Time, limit int) ([]*models.Trade, error)
	Health(ctx context.Context) error // ping
	Close() error
}

type Metrics interface {
	RecordMessageSent(backend, symbol string)
	RecordError(kind string)
	RecordLastPrice(symbol string, price float64)
	RecordLatency(op string, seconds float64)
}
