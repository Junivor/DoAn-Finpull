package analytics

import (
    "context"

    "FinPull/internal/domain/models"
    domsvc "FinPull/internal/domain/service"
    "FinPull/pkg/config"
)

// DomainAnomalyAdapter adapts HTTPAnomalyDetector to the domain AnomalyDetector interface.
type DomainAnomalyAdapter struct {
    impl *HTTPAnomalyDetector
}

func NewDomainAnomalyAdapter(cfg *config.Config) domsvc.AnomalyDetector {
    return &DomainAnomalyAdapter{impl: NewHTTPAnomalyDetector(cfg)}
}

func (a *DomainAnomalyAdapter) Detect(ctx context.Context, symbol string, returns []float64, volSeries []float64) ([]models.MarketAnomaly, error) {
    anns, err := a.impl.Detect(ctx, symbol, returns, volSeries)
    if err != nil {
        return nil, err
    }
    out := make([]models.MarketAnomaly, 0, len(anns))
    for _, an := range anns {
        out = append(out, models.MarketAnomaly{
            Symbol:   symbol,
            Type:     an.Type,
            Severity: an.Severity,
            // Timestamp, Return, Volatility are not provided by HTTP service; left zero-value
        })
    }
    return out, nil
}

// Ensure adapter implements domain interface
var _ domsvc.AnomalyDetector = (*DomainAnomalyAdapter)(nil)