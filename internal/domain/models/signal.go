package models

import "time"

type Regime struct {
	Symbol     string
	Timestamp  time.Time
	State      string    // "bull", "bear", "volatile", "quiet"
	Prob       []float64 // probabilities per state
	Confidence float64
}

type VolatilityForecast struct {
	Symbol    string
	Timestamp time.Time
	Horizon   string  // "5m", "30m"
	Forecast  float64 // sigma forecast
	Nowcast   float64 // realized volatility now
	Model     string  // "GARCH" | "LightGBM"
}

type MarketAnomaly struct {
	Symbol     string
	Timestamp  time.Time
	Type       string  // "shock_up", "shock_down", "vol_spike"
	Severity   float64 // z-score magnitude
	Return     float64
	Volatility float64
}

type EdgeScore struct {
	Symbol     string
	Timestamp  time.Time
	Horizon    string  // "15m"
	ProbaUp    float64 // probability of price going up
	Regime     string
	Sigma      float64 // volatility estimate
	Confidence float64
}

// Candle represents an OHLCV record for feature engineering and training.
type Candle struct {
	Bucket time.Time
	Symbol string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	OrgID  string
}
