package features

import (
    "math"
    "time"

    "FinPull/internal/domain/models"
)

// ComputeLogReturns computes log returns r_t = ln(C_t / C_{t-1}).
// It returns a slice of length len(candles)-1, or nil if insufficient data.
func ComputeLogReturns(candles []models.Candle) []float64 {
    if len(candles) < 2 {
        return nil
    }
    out := make([]float64, 0, len(candles)-1)
    for i := 1; i < len(candles); i++ {
        prev := candles[i-1].Close
        cur := candles[i].Close
        if prev <= 0 || cur <= 0 {
            out = append(out, 0)
            continue
        }
        out = append(out, math.Log(cur/prev))
    }
    return out
}

// RealizedVolatility computes annualized realized volatility over a rolling window
// using the provided number of bars per year. Returns the latest window sigma.
func RealizedVolatility(logReturns []float64, window int, barsPerYear float64) float64 {
    if window <= 1 || len(logReturns) < window {
        return 0
    }
    sum := 0.0
    sum2 := 0.0
    for i := len(logReturns) - window; i < len(logReturns); i++ {
        r := logReturns[i]
        sum += r
        sum2 += r * r
    }
    n := float64(window)
    mean := sum / n
    variance := (sum2 - n*mean*mean) / (n - 1)
    if variance < 0 {
        variance = 0
    }
    // annualize
    return math.Sqrt(variance * barsPerYear)
}

// BarsPerYearForTF returns the approximate number of bars per year for a timeframe.
func BarsPerYearForTF(tf string) float64 {
    switch tf {
    case "1s":
        return 365 * 24 * 60 * 60
    case "1m":
        return 365 * 24 * 60
    case "5m":
        return 365 * 24 * 12
    default:
        return 365 * 24 * 60
    }
}

// AlignFromTo rounds time range to candle boundaries based on timeframe.
func AlignFromTo(from, to time.Time, tf string) (time.Time, time.Time) {
    switch tf {
    case "1s":
        from = from.Truncate(time.Second)
        to = to.Truncate(time.Second)
    case "1m":
        from = from.Truncate(time.Minute)
        to = to.Truncate(time.Minute)
    case "5m":
        d := time.Duration(5) * time.Minute
        from = from.Truncate(d)
        to = to.Truncate(d)
    default:
        from = from.Truncate(time.Minute)
        to = to.Truncate(time.Minute)
    }
    return from, to
}