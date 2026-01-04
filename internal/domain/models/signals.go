package models

import "time"

// AggregateSignals represents a consolidated view of all analytics signals.
// Note: no transport (json/http) concerns here.
type AggregateSignals struct {
	Symbol     string
	Horizon    string
	Timestamp  time.Time
	Regime     *Regime
	Volatility *VolatilityForecast
	Anomalies  []MarketAnomaly
	Edge       *EdgeScore
	Errors     map[string]string
}
