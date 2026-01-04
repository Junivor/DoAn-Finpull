package analytics

import (
    "context"
    "fmt"
    "time"

    "FinPull/pkg/config"
    xhttp "FinPull/pkg/http"
)

// HTTPServiceBase provides a DRY foundation for analytics HTTP clients.
// It centralizes client construction and JSON POST request handling.
type HTTPServiceBase struct {
    baseURL string
    client  *xhttp.Client
}

// NewHTTPServiceBase builds an HTTP client with timeout and base URL from config.
func NewHTTPServiceBase(cfg *config.Config) *HTTPServiceBase {
    timeout := cfg.Analytics.Timeout
    if timeout <= 0 {
        timeout = 3 * 1000000000 // 3s in nanoseconds to avoid import time here; but we rely on xhttp.WithTimeout
    }
    return &HTTPServiceBase{
        baseURL: cfg.Analytics.PythonServiceURL,
        client:  xhttp.NewClient(xhttp.WithTimeout(timeout)),
    }
}

// PostJSON posts the given payload to `path` under baseURL and decodes JSON into dest.
func (b *HTTPServiceBase) PostJSON(ctx context.Context, path string, payload interface{}, dest interface{}) error {
    if b.client == nil || b.baseURL == "" {
        return fmt.Errorf("analytics http client not initialized")
    }
    err := b.client.SendAndParse(ctx, &xhttp.RequestOptions{
        Method: xhttp.MethodPost,
        URL:    b.baseURL + path,
        Headers: map[string]string{
            "Content-Type": "application/json",
        },
        Body: payload,
    }, dest)
    if err != nil {
        return fmt.Errorf("post %s: %w", path, err)
    }
    return nil
}

// PostJSONWithRetry posts JSON with up to `attempts` retries for transient errors.
func (b *HTTPServiceBase) PostJSONWithRetry(ctx context.Context, path string, payload interface{}, dest interface{}, attempts int) error {
    if attempts <= 1 {
        return b.PostJSON(ctx, path, payload, dest)
    }
    var err error
    for i := 1; i <= attempts; i++ {
        err = b.PostJSON(ctx, path, payload, dest)
        if err == nil {
            return nil
        }
        // simple backoff
        select {
        case <-time.After(time.Duration(i) * 50 * time.Millisecond):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return err
}