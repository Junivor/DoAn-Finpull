package analytics

import (
    "context"
    "fmt"
    "time"

    domsvc "FinPull/internal/domain/service"
    "FinPull/internal/domain/models"
    "FinPull/pkg/config"
)

type HTTPEdgeScorer struct { base *HTTPServiceBase }

func NewHTTPEdgeScorer(cfg *config.Config) *HTTPEdgeScorer { return &HTTPEdgeScorer{base: NewHTTPServiceBase(cfg)} }

type edgeReq struct {
    Symbol   string             `json:"symbol"`
    Features map[string]float64 `json:"features"`
    Horizon  string             `json:"horizon"`
}

type edgeResp struct {
    ProbaUp    float64 `json:"proba_up"`
    Regime     string  `json:"regime"`
    Sigma      float64 `json:"sigma"`
    Confidence float64 `json:"confidence"`
}

func (s *HTTPEdgeScorer) Predict(ctx context.Context, symbol string, features map[string]float64, horizon string) (models.EdgeScore, error) {
    var result models.EdgeScore
    var er edgeResp
    err := s.base.PostJSON(ctx, "/edge/predict", edgeReq{Symbol: symbol, Features: features, Horizon: horizon}, &er)
    if err != nil {
        return result, fmt.Errorf("post edge: %w", err)
    }
    result.Symbol = symbol
    result.Timestamp = time.Now()
    result.Horizon = horizon
    result.ProbaUp = er.ProbaUp
    result.Regime = er.Regime
    result.Sigma = er.Sigma
    result.Confidence = er.Confidence
    return result, nil
}

var _ domsvc.EdgeScorer = (*HTTPEdgeScorer)(nil)