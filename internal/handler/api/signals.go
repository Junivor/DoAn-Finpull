package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	domrepo "FinPull/internal/domain/repository"
	icache "FinPull/internal/service/cache"
	"FinPull/internal/service/metrics"
	"FinPull/internal/service/ratelimit"
	"FinPull/internal/usecase"
	applogger "FinPull/pkg/logger"
)

type SignalsHandler struct {
	agg   *usecase.SignalAggregator
	cache icache.BytesCache
	rl    *ratelimit.Limiter
	l     *applogger.Logger
}

func NewSignalsHandler(agg *usecase.SignalAggregator) *SignalsHandler {
	metrics.Register()
	return &SignalsHandler{agg: agg, rl: ratelimit.New()}
}

func (h *SignalsHandler) SetCache(c icache.BytesCache) { h.cache = c }

// SetLogger injects a structured logger.
func (h *SignalsHandler) SetLogger(l *applogger.Logger) { h.l = l }

func (h *SignalsHandler) Regime() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		endpoint := "regime"
		defer func() { metrics.AnalyticsLatency.WithLabelValues(endpoint).Observe(time.Since(start).Seconds()) }()

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			if h.l != nil {
				h.l.Warn("signals.regime missing symbol")
			}
			http.Error(w, "symbol required", http.StatusBadRequest)
			return
		}
		n := parseInt(r.URL.Query().Get("n"), 600)
		tf := domrepo.Timeframe(r.URL.Query().Get("tf"))
		if tf == "" {
			tf = domrepo.TF1m
		}
		if !h.rl.Allow(r.RemoteAddr+":regime", 5, 2) {
			if h.l != nil {
				h.l.Warn("signals.regime rate_limited", applogger.String("remote", r.RemoteAddr))
			}
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		cacheKey := "regime:" + symbol + ":" + string(tf)
		if h.cache != nil {
			if b, ok, err := h.cache.GetBytes(cacheKey); err != nil {
				if h.l != nil {
					h.l.Warn("signals.regime cache_get_error", applogger.Error(err))
				}
			} else if ok {
				w.Header().Set("Content-Type", "application/json")
				if h.l != nil {
					h.l.Debug("signals.regime cache_hit", applogger.String("key", cacheKey))
				}
				if _, err := w.Write(b); err != nil && h.l != nil {
					h.l.Warn("signals.regime write_error", applogger.Error(err))
				}
				return
			}
			if h.l != nil {
				h.l.Debug("signals.regime cache_miss", applogger.String("key", cacheKey))
			}
		}
		res, err := h.agg.LatestRegime(r.Context(), symbol, n, tf)
		if err != nil {
			metrics.AnalyticsErrors.WithLabelValues(endpoint).Inc()
			if h.l != nil {
				h.l.Error("signals.regime error", applogger.Error(err))
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(res)
		if err != nil {
			if h.l != nil {
				h.l.Error("signals.regime marshal_error", applogger.Error(err))
			}
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if h.cache != nil {
			if err := h.cache.SetBytes(cacheKey, b, 30*time.Second); err != nil && h.l != nil {
				h.l.Warn("signals.regime cache_set_error", applogger.Error(err))
			}
		}
		if _, err := w.Write(b); err != nil && h.l != nil {
			h.l.Warn("signals.regime write_error", applogger.Error(err))
		}
	}
}

func (h *SignalsHandler) Vol() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		endpoint := "vol"
		defer func() { metrics.AnalyticsLatency.WithLabelValues(endpoint).Observe(time.Since(start).Seconds()) }()

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			if h.l != nil {
				h.l.Warn("signals.vol missing symbol")
			}
			http.Error(w, "symbol required", http.StatusBadRequest)
			return
		}
		horizon := r.URL.Query().Get("horizon")
		if horizon == "" {
			horizon = "5m"
		}
		n := parseInt(r.URL.Query().Get("n"), 600)
		tf := domrepo.Timeframe(r.URL.Query().Get("tf"))
		if tf == "" {
			tf = domrepo.TF1m
		}
		if !h.rl.Allow(r.RemoteAddr+":vol", 5, 2) {
			if h.l != nil {
				h.l.Warn("signals.vol rate_limited", applogger.String("remote", r.RemoteAddr))
			}
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		cacheKey := "vol:" + symbol + ":" + horizon + ":" + string(tf)
		if h.cache != nil {
			if b, ok, err := h.cache.GetBytes(cacheKey); err != nil {
				if h.l != nil {
					h.l.Warn("signals.vol cache_get_error", applogger.Error(err))
				}
			} else if ok {
				w.Header().Set("Content-Type", "application/json")
				if h.l != nil {
					h.l.Debug("signals.vol cache_hit", applogger.String("key", cacheKey))
				}
				if _, err := w.Write(b); err != nil && h.l != nil {
					h.l.Warn("signals.vol write_error", applogger.Error(err))
				}
				return
			}
			if h.l != nil {
				h.l.Debug("signals.vol cache_miss", applogger.String("key", cacheKey))
			}
		}
		res, err := h.agg.VolForecast(r.Context(), symbol, horizon, n, tf)
		if err != nil {
			metrics.AnalyticsErrors.WithLabelValues(endpoint).Inc()
			if h.l != nil {
				h.l.Error("signals.vol error", applogger.Error(err))
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(res)
		if err != nil {
			if h.l != nil {
				h.l.Error("signals.vol marshal_error", applogger.Error(err))
			}
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if h.cache != nil {
			if err := h.cache.SetBytes(cacheKey, b, 30*time.Second); err != nil && h.l != nil {
				h.l.Warn("signals.vol cache_set_error", applogger.Error(err))
			}
		}
		if _, err := w.Write(b); err != nil && h.l != nil {
			h.l.Warn("signals.vol write_error", applogger.Error(err))
		}
	}
}

func (h *SignalsHandler) Anomaly() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		endpoint := "anomaly"
		defer func() { metrics.AnalyticsLatency.WithLabelValues(endpoint).Observe(time.Since(start).Seconds()) }()

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			if h.l != nil {
				h.l.Warn("signals.anomaly missing symbol")
			}
			http.Error(w, "symbol required", http.StatusBadRequest)
			return
		}
		n := parseInt(r.URL.Query().Get("n"), 1200)
		tf := domrepo.Timeframe(r.URL.Query().Get("tf"))
		if tf == "" {
			tf = domrepo.TF1m
		}
		if !h.rl.Allow(r.RemoteAddr+":anom", 3, 1) {
			if h.l != nil {
				h.l.Warn("signals.anomaly rate_limited", applogger.String("remote", r.RemoteAddr))
			}
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		cacheKey := "anom:" + symbol + ":" + string(tf)
		if h.cache != nil {
			if b, ok, err := h.cache.GetBytes(cacheKey); err != nil {
				if h.l != nil {
					h.l.Warn("signals.anomaly cache_get_error", applogger.Error(err))
				}
			} else if ok {
				w.Header().Set("Content-Type", "application/json")
				if h.l != nil {
					h.l.Debug("signals.anomaly cache_hit", applogger.String("key", cacheKey))
				}
				if _, err := w.Write(b); err != nil && h.l != nil {
					h.l.Warn("signals.anomaly write_error", applogger.Error(err))
				}
				return
			}
			if h.l != nil {
				h.l.Debug("signals.anomaly cache_miss", applogger.String("key", cacheKey))
			}
		}
		res, err := h.agg.Anomalies(r.Context(), symbol, n, tf)
		if err != nil {
			metrics.AnalyticsErrors.WithLabelValues(endpoint).Inc()
			if h.l != nil {
				h.l.Error("signals.anomaly error", applogger.Error(err))
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(res)
		if err != nil {
			if h.l != nil {
				h.l.Error("signals.anomaly marshal_error", applogger.Error(err))
			}
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if h.cache != nil {
			if err := h.cache.SetBytes(cacheKey, b, 30*time.Second); err != nil && h.l != nil {
				h.l.Warn("signals.anomaly cache_set_error", applogger.Error(err))
			}
		}
		if _, err := w.Write(b); err != nil && h.l != nil {
			h.l.Warn("signals.anomaly write_error", applogger.Error(err))
		}
	}
}

func (h *SignalsHandler) Edge() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		endpoint := "edge"
		defer func() { metrics.AnalyticsLatency.WithLabelValues(endpoint).Observe(time.Since(start).Seconds()) }()

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			if h.l != nil {
				h.l.Warn("signals.edge missing symbol")
			}
			http.Error(w, "symbol required", http.StatusBadRequest)
			return
		}
		horizon := r.URL.Query().Get("horizon")
		if horizon == "" {
			horizon = "15m"
		}
		n := parseInt(r.URL.Query().Get("n"), 600)
		tf := domrepo.Timeframe(r.URL.Query().Get("tf"))
		if tf == "" {
			tf = domrepo.TF1m
		}
		if !h.rl.Allow(r.RemoteAddr+":edge", 5, 2) {
			if h.l != nil {
				h.l.Warn("signals.edge rate_limited", applogger.String("remote", r.RemoteAddr))
			}
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		cacheKey := "edge:" + symbol + ":" + horizon + ":" + string(tf)
		if h.cache != nil {
			if b, ok, err := h.cache.GetBytes(cacheKey); err != nil {
				if h.l != nil {
					h.l.Warn("signals.edge cache_get_error", applogger.Error(err))
				}
			} else if ok {
				w.Header().Set("Content-Type", "application/json")
				if h.l != nil {
					h.l.Debug("signals.edge cache_hit", applogger.String("key", cacheKey))
				}
				if _, err := w.Write(b); err != nil && h.l != nil {
					h.l.Warn("signals.edge write_error", applogger.Error(err))
				}
				return
			}
			if h.l != nil {
				h.l.Debug("signals.edge cache_miss", applogger.String("key", cacheKey))
			}
		}
		res, err := h.agg.Edge(r.Context(), symbol, horizon, n, tf)
		if err != nil {
			metrics.AnalyticsErrors.WithLabelValues(endpoint).Inc()
			if h.l != nil {
				h.l.Error("signals.edge error", applogger.Error(err))
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(res)
		if err != nil {
			if h.l != nil {
				h.l.Error("signals.edge marshal_error", applogger.Error(err))
			}
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		if h.cache != nil {
			if err := h.cache.SetBytes(cacheKey, b, 60*time.Second); err != nil && h.l != nil {
				h.l.Warn("signals.edge cache_set_error", applogger.Error(err))
			}
		}
		if _, err := w.Write(b); err != nil && h.l != nil {
			h.l.Warn("signals.edge write_error", applogger.Error(err))
		}
	}
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
