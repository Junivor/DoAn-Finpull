package util

import (
    "strconv"
    "testing"
    "time"
)

func TestParseTimeRFC3339(t *testing.T) {
    s := "2024-10-10T10:10:10Z"
    got, ok := ParseTime(s)
    if !ok {
        t.Fatalf("expected ok")
    }
    if got.UTC().Format(time.RFC3339) != s {
        t.Fatalf("unexpected time %v", got)
    }
}

func TestParseTimeUnix(t *testing.T) {
    ts := time.Date(2024, 10, 10, 10, 10, 10, 0, time.UTC).Unix()
    got, ok := ParseTime(strconv.FormatInt(ts, 10))
    if !ok {
        t.Fatalf("expected ok")
    }
    if got.Unix() != ts {
        t.Fatalf("unexpected unix %v", got.Unix())
    }
}

func TestParseTimeDefault(t *testing.T) {
    def := time.Date(2024, 10, 10, 10, 10, 10, 0, time.UTC)
    got := ParseTimeDefault("", def)
    if !got.Equal(def) {
        t.Fatalf("expected default")
    }
}