package analytics

import (
    "context"
    "fmt"
    "time"

    domsvc "FinPull/internal/domain/service"
    "FinPull/internal/domain/models"
    "FinPull/pkg/config"
)

type HTTPVolatilityForecaster struct { base *HTTPServiceBase }

func NewHTTPVolatilityForecaster(cfg *config.Config) *HTTPVolatilityForecaster { return &HTTPVolatilityForecaster{base: NewHTTPServiceBase(cfg)} }

type volReq struct {
    Symbol   string             `json:"symbol"`
    Features map[string]float64 `json:"features"`
    Horizon  string             `json:"horizon"`
}

type volResp struct {
    Forecast float64 `json:"forecast"`
    Nowcast  float64 `json:"nowcast"`
    Model    string  `json:"model"`
}

func (f *HTTPVolatilityForecaster) Forecast(ctx context.Context, symbol string, features map[string]float64, horizon string) (models.VolatilityForecast, error) {
    var result models.VolatilityForecast
    var vr volResp
    err := f.base.PostJSON(ctx, "/vol/forecast", volReq{Symbol: symbol, Features: features, Horizon: horizon}, &vr)
    if err != nil {
        return result, fmt.Errorf("post vol: %w", err)
    }
    result.Symbol = symbol
    result.Timestamp = time.Now()
    result.Horizon = horizon
    result.Forecast = vr.Forecast
    result.Nowcast = vr.Nowcast
    result.Model = vr.Model
    return result, nil
}

var _ domsvc.VolatilityForecaster = (*HTTPVolatilityForecaster)(nil)