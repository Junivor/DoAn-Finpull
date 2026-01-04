package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"FinPull/internal/domain/models"
	domrepo "FinPull/internal/domain/repository"
)

// SignalsAggregateUseCase aggregates signals using SignalAggregator.
type SignalsAggregateUseCase struct {
	agg     *SignalAggregator
	timeout time.Duration
}

func NewSignalsAggregateUseCase(agg *SignalAggregator) *SignalsAggregateUseCase {
	return &SignalsAggregateUseCase{agg: agg, timeout: 10 * time.Second}
}

type GetSignalsParams struct {
	Symbol    string
	Horizon   string
	N         int
	Timeframe domrepo.Timeframe
}

func (uc *SignalsAggregateUseCase) GetSignals(ctx context.Context, p GetSignalsParams) (*models.AggregateSignals, error) {
	if p.Symbol == "" {
		return nil, fmt.Errorf("symbol required")
	}
	if p.N <= 0 {
		p.N = 600
	}
	if p.Horizon == "" {
		p.Horizon = "5m"
	}

	// Overall timeout
	ctx, cancel := context.WithTimeout(ctx, uc.timeout)
	defer cancel()

	res := &models.AggregateSignals{
		Symbol:    p.Symbol,
		Horizon:   p.Horizon,
		Timestamp: time.Now(),
		Errors:    map[string]string{},
	}

	type item struct {
		name string
		val  interface{}
		err  error
	}
	ch := make(chan item, 4)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := uc.agg.LatestRegime(ctx, p.Symbol, p.N, p.Timeframe)
		ch <- item{"regime", v, err}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := uc.agg.VolForecast(ctx, p.Symbol, p.Horizon, p.N, p.Timeframe)
		ch <- item{"volatility", v, err}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := uc.agg.Anomalies(ctx, p.Symbol, p.N, p.Timeframe)
		ch <- item{"anomalies", v, err}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		v, err := uc.agg.Edge(ctx, p.Symbol, p.Horizon, p.N, p.Timeframe)
		ch <- item{"edge", v, err}
	}()

	go func() { wg.Wait(); close(ch) }()

	for it := range ch {
		if it.err != nil {
			res.Errors[it.name] = it.err.Error()
			continue
		}
		switch it.name {
		case "regime":
			v := it.val.(models.Regime)
			res.Regime = &v
		case "volatility":
			v := it.val.(models.VolatilityForecast)
			res.Volatility = &v
		case "anomalies":
			v := it.val.([]models.MarketAnomaly)
			res.Anomalies = v
		case "edge":
			v := it.val.(models.EdgeScore)
			res.Edge = &v
		}
	}

	if len(res.Errors) == 0 {
		res.Errors = nil
	}
	return res, nil
}
