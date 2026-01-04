package util

import "strconv"

// ParseIntDefault parses string to int or returns default if empty/invalid.
func ParseIntDefault(s string, def int) int {
    if s == "" {
        return def
    }
    v, err := strconv.Atoi(s)
    if err != nil {
        return def
    }
    return v
}