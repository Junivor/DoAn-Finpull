package usecase

import (
	"context"
	"time"

	"FinPull/internal/domain/models"
	domrepo "FinPull/internal/domain/repository"
	domsvc "FinPull/internal/domain/service"
	"FinPull/internal/services/features"
)

type SignalAggregator struct {
	store   domrepo.FeatureStore
	regime  domsvc.RegimeDetector
	vol     domsvc.VolatilityForecaster
	anomaly domsvc.AnomalyDetector
	edge    domsvc.EdgeScorer
}

func NewSignalAggregator(store domrepo.FeatureStore, regime domsvc.RegimeDetector, vol domsvc.VolatilityForecaster, anomaly domsvc.AnomalyDetector, edge domsvc.EdgeScorer) *SignalAggregator {
	return &SignalAggregator{store: store, regime: regime, vol: vol, anomaly: anomaly, edge: edge}
}

func (a *SignalAggregator) LatestRegime(ctx context.Context, symbol string, n int, tf domrepo.Timeframe) (models.Regime, error) {
	cs, err := a.store.GetLatestNCandles(ctx, symbol, n, tf)
	if err != nil {
		return models.Regime{}, err
	}
	rets := features.ComputeLogReturns(cs)
	// Note: we intentionally keep naming stable; returns computed as log returns
	return a.regime.Detect(ctx, symbol, rets)
}

func (a *SignalAggregator) VolForecast(ctx context.Context, symbol string, horizon string, n int, tf domrepo.Timeframe) (models.VolatilityForecast, error) {
	cs, err := a.store.GetLatestNCandles(ctx, symbol, n, tf)
	if err != nil {
		return models.VolatilityForecast{}, err
	}
	rets := features.ComputeLogReturns(cs)
	feats := map[string]float64{}
	// realized volatility nowcast as a feature input
	bpY := features.BarsPerYearForTF(string(tf))
	feats["nowcast_sigma"] = features.RealizedVolatility(rets, min(60, len(rets)), bpY)
	return a.vol.Forecast(ctx, symbol, feats, horizon)
}

func (a *SignalAggregator) Anomalies(ctx context.Context, symbol string, n int, tf domrepo.Timeframe) ([]models.MarketAnomaly, error) {
	cs, err := a.store.GetLatestNCandles(ctx, symbol, n, tf)
	if err != nil {
		return nil, err
	}
	rets := features.ComputeLogReturns(cs)
	bpY := features.BarsPerYearForTF(string(tf))
	vols := make([]float64, 0, len(rets))
	for i := range rets {
		w := min(60, i+1)
		vols = append(vols, features.RealizedVolatility(rets[:i+1], w, bpY))
	}
	return a.anomaly.Detect(ctx, symbol, rets, vols)
}

func (a *SignalAggregator) Edge(ctx context.Context, symbol string, horizon string, n int, tf domrepo.Timeframe) (models.EdgeScore, error) {
	cs, err := a.store.GetLatestNCandles(ctx, symbol, n, tf)
	if err != nil {
		return models.EdgeScore{}, err
	}
	rets := features.ComputeLogReturns(cs)
	bpY := features.BarsPerYearForTF(string(tf))
	feats := map[string]float64{
		"ret_1":     lastOrZero(rets),
		"sigma_now": features.RealizedVolatility(rets, min(60, len(rets)), bpY),
	}
	return a.edge.Predict(ctx, symbol, feats, horizon)
}

func lastOrZero(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	return xs[len(xs)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = time.Now
