package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	applogger "FinPull/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"route", "method", "status", "class"},
	)

	httpInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_in_flight_requests",
			Help: "Current number of in-flight HTTP requests",
		},
		[]string{"route", "method"},
	)

	httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{200, 500, 1_000, 2_000, 5_000, 10_000, 50_000, 100_000, 500_000, 1_000_000},
		},
		[]string{"route", "method", "status", "class"},
	)

	regOnce sync.Once
)

// Metrics is a net/http middleware that records request metrics with low cardinality labels.
// Note: Prefer templated route paths (e.g., "/api/edge") instead of raw URLs to control cardinality.
func Metrics(l *applogger.Logger, slowThreshold time.Duration) func(http.Handler) http.Handler {
	regOnce.Do(func() {
		prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpInFlight, httpResponseSize)
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := routeLabel(r)
			method := r.Method

			httpInFlight.WithLabelValues(route, method).Inc()
			start := time.Now()

			rw := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			status := strconv.Itoa(rw.status)
			class := statusClass(rw.status)
			dur := time.Since(start).Seconds()

			httpRequestsTotal.WithLabelValues(route, method, status).Inc()
			httpRequestDuration.WithLabelValues(route, method, status, class).Observe(dur)
			httpResponseSize.WithLabelValues(route, method, status, class).Observe(float64(rw.written))
			httpInFlight.WithLabelValues(route, method).Dec()

			// Structured logging: errors and slow requests
			if l != nil {
				duration := time.Duration(dur * float64(time.Second))
				// Log 5xx as errors
				if rw.status >= 500 {
					l.Error("http request failed",
						applogger.String("route", route),
						applogger.String("method", method),
						applogger.String("status", status),
						applogger.Duration("duration_ms", duration),
						applogger.Int("bytes", rw.written),
					)
					return
				}
				// Log slow requests as warnings
				if slowThreshold > 0 && duration >= slowThreshold {
					l.Warn("http request slow",
						applogger.String("route", route),
						applogger.String("method", method),
						applogger.String("status", status),
						applogger.Duration("duration_ms", duration),
						applogger.Int("bytes", rw.written),
					)
				}
			}
		})
	}
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status  int
	written int
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += n
	return n, err
}

// routeLabel attempts to use a normalized route path to keep label cardinality low.
// If a framework/mux sets a route template in context, prefer that; otherwise fallback to URL path.
func routeLabel(r *http.Request) string {
	if v := r.Context().Value("route"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return r.URL.Path
}

func statusClass(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "1xx"
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}
