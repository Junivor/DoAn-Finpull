package analytics

import (
    "context"
    "fmt"
    "time"

    domsvc "FinPull/internal/domain/service"
    "FinPull/internal/domain/models"
    "FinPull/pkg/config"
)

type HTTPRegimeDetector struct { base *HTTPServiceBase }

func NewHTTPRegimeDetector(cfg *config.Config) *HTTPRegimeDetector { return &HTTPRegimeDetector{base: NewHTTPServiceBase(cfg)} }

type regimeRequest struct {
    Symbol  string    `json:"symbol"`
    Returns []float64 `json:"returns"`
}

type regimeResponse struct {
    State      string    `json:"state"`
    Prob       []float64 `json:"prob"`
    Confidence float64   `json:"confidence"`
}

func (d *HTTPRegimeDetector) Detect(ctx context.Context, symbol string, returns []float64) (models.Regime, error) {
    var result models.Regime
    var rr regimeResponse
    err := d.base.PostJSONWithRetry(ctx, "/regime/detect", regimeRequest{Symbol: symbol, Returns: returns}, &rr, 3)
    if err != nil {
        return result, fmt.Errorf("post regime: %w", err)
    }
    result.Symbol = symbol
    result.Timestamp = time.Now()
    result.State = rr.State
    result.Prob = rr.Prob
    result.Confidence = rr.Confidence
    return result, nil
}

var _ domsvc.RegimeDetector = (*HTTPRegimeDetector)(nil)