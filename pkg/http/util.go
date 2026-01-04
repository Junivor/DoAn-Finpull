package http

import (
    "time"

    xutil "FinPull/pkg/util"
)

// ParseIntDefault parses string to int or returns default if empty/invalid.
func ParseIntDefault(s string, def int) int { return xutil.ParseIntDefault(s, def) }

// ParseTime tries RFC3339, RFC3339Nano, and unix seconds. Returns (t, true) if any worked.
func ParseTime(s string) (time.Time, bool) { return xutil.ParseTime(s) }

// ParseTimeDefault parses time or returns default if empty/invalid.
func ParseTimeDefault(s string, def time.Time) time.Time { return xutil.ParseTimeDefault(s, def) }
