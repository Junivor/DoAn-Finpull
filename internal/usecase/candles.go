package usecase

import (
	"context"
	"fmt"
	"time"

	"FinPull/internal/domain/models"
	domrepo "FinPull/internal/domain/repository"
)

// CandlesUseCase provides business logic for retrieving candles.
type CandlesUseCase struct {
	store domrepo.FeatureStore
}

func NewCandlesUseCase(store domrepo.FeatureStore) *CandlesUseCase {
	return &CandlesUseCase{store: store}
}

type GetCandlesParams struct {
	Symbol    string
	From      time.Time
	To        time.Time
	Timeframe domrepo.Timeframe
	Limit     int
}

type GetCandlesResult struct {
	Symbol    string
	Timeframe string
	From      time.Time
	To        time.Time
	Count     int
	Candles   []models.Candle
}

func (uc *CandlesUseCase) GetCandles(ctx context.Context, p GetCandlesParams) (*GetCandlesResult, error) {
	if p.Symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}
	if p.From.After(p.To) {
		return nil, fmt.Errorf("from must be <= to")
	}
	if p.Limit <= 0 {
		p.Limit = 10000
	}
	if p.Limit > 50000 {
		p.Limit = 50000
	}

	candles, err := uc.store.GetCandles(ctx, p.Symbol, p.From, p.To, p.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("get candles: %w", err)
	}
	if len(candles) > p.Limit {
		candles = candles[:p.Limit]
	}

	return &GetCandlesResult{
		Symbol:    p.Symbol,
		Timeframe: string(p.Timeframe),
		From:      p.From,
		To:        p.To,
		Count:     len(candles),
		Candles:   candles,
	}, nil
}
