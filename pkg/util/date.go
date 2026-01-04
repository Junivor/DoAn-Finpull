package util

import (
    "strconv"
    "time"
)

// ParseTime tries RFC3339, RFC3339Nano, and unix seconds. Returns (t, true) if any worked.
func ParseTime(s string) (time.Time, bool) {
    if s == "" {
        return time.Time{}, false
    }
    if t, err := time.Parse(time.RFC3339, s); err == nil {
        return t, true
    }
    if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
        return t, true
    }
    if ts, err := strconv.ParseInt(s, 10, 64); err == nil && ts > 0 {
        return time.Unix(ts, 0), true
    }
    return time.Time{}, false
}

// ParseTimeDefault parses time or returns default if empty/invalid.
func ParseTimeDefault(s string, def time.Time) time.Time {
    if t, ok := ParseTime(s); ok {
        return t
    }
    return def
}

// AlignFromTo rounds the time range to boundaries for the timeframe.
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

// No extra helpers here; use strconv where needed.