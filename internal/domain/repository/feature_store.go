package repository

import (
	"context"
	"time"

	"FinPull/internal/domain/models"
)

// Timeframe represents candle resolution buckets.
type Timeframe string

const (
	TF1s Timeframe = "1s"
	TF1m Timeframe = "1m"
	TF5m Timeframe = "5m"
)

// FeatureStore provides read-only access to candles/features for analytics.
type FeatureStore interface {
	GetCandles(ctx context.Context, symbol string, from, to time.Time, tf Timeframe) ([]models.Candle, error)
	GetLatestNCandles(ctx context.Context, symbol string, n int, tf Timeframe) ([]models.Candle, error)
}
