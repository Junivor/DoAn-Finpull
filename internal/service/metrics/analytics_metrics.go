package metrics

import (
    "sync"

    "github.com/prometheus/client_golang/prometheus"
)

var (
    once sync.Once

    AnalyticsLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "finpull",
            Subsystem: "analytics",
            Name:      "latency_seconds",
            Help:      "Latency of analytics endpoints",
            Buckets:   prometheus.DefBuckets,
        },
        []string{"endpoint"},
    )

    AnalyticsErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "finpull",
            Subsystem: "analytics",
            Name:      "errors_total",
            Help:      "Errors by analytics endpoint",
        },
        []string{"endpoint"},
    )
)

func Register() {
    once.Do(func() {
        prometheus.MustRegister(AnalyticsLatency, AnalyticsErrors)
    })
}


