package analytics

import (
    "context"
    "fmt"
    "time"

    "FinPull/pkg/config"
    xhttp "FinPull/pkg/http"
)

type Anomaly struct {
    TSIndex  int     `json:"ts_index"`
    Type     string  `json:"type"`
    Severity float64 `json:"severity"`
}

type HTTPAnomalyDetector struct {
    baseURL string
    client  *xhttp.Client
}

func NewHTTPAnomalyDetector(cfg *config.Config) *HTTPAnomalyDetector {
    timeout := cfg.Analytics.Timeout
    if timeout <= 0 {
        timeout = 3 * time.Second
    }
    return &HTTPAnomalyDetector{
        baseURL: cfg.Analytics.PythonServiceURL,
        client:  xhttp.NewClient(xhttp.WithTimeout(timeout)),
    }
}

type anomalyReq struct {
    Symbol  string    `json:"symbol"`
    Returns []float64 `json:"returns"`
    Vols    []float64 `json:"vols"`
}

type anomalyResp struct {
    Anomalies []Anomaly `json:"anomalies"`
}

func (d *HTTPAnomalyDetector) Detect(ctx context.Context, symbol string, returns, vols []float64) ([]Anomaly, error) {
    var ar anomalyResp
    err := d.client.SendAndParse(ctx, &xhttp.RequestOptions{
        Method: xhttp.MethodPost,
        URL:    d.baseURL + "/anomaly/detect",
        Headers: map[string]string{"Content-Type": "application/json"},
        Body:   anomalyReq{Symbol: symbol, Returns: returns, Vols: vols},
    }, &ar)
    if err != nil {
        return nil, fmt.Errorf("post anomaly: %w", err)
    }
    return ar.Anomalies, nil
}