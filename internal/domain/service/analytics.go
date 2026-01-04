package service

import (
	"context"

	"FinPull/internal/domain/models"
)

// RegimeDetector detects market regimes based on returns time series.
type RegimeDetector interface {
	Detect(ctx context.Context, symbol string, returns []float64) (models.Regime, error)
}

// VolatilityForecaster forecasts volatility for a given horizon using features.
type VolatilityForecaster interface {
	Forecast(ctx context.Context, symbol string, features map[string]float64, horizon string) (models.VolatilityForecast, error)
}

// AnomalyDetector detects anomalies using returns and volatility series.
type AnomalyDetector interface {
	Detect(ctx context.Context, symbol string, returns []float64, volSeries []float64) ([]models.MarketAnomaly, error)
}

// EdgeScorer predicts edge/probability using features for a horizon.
type EdgeScorer interface {
	Predict(ctx context.Context, symbol string, features map[string]float64, horizon string) (models.EdgeScore, error)
}
