package repository

// IsValidTimeframe returns true if tf is a supported timeframe.
func IsValidTimeframe(tf Timeframe) bool {
	switch tf {
	case TF1s, TF1m, TF5m:
		return true
	default:
		return false
	}
}

// DefaultTimeframe returns the default timeframe.
func DefaultTimeframe() Timeframe { return TF1m }

// NormalizeTimeframe converts raw string to a valid timeframe (or default).
func NormalizeTimeframe(s string) Timeframe {
	if s == "" {
		return DefaultTimeframe()
	}
	tf := Timeframe(s)
	if IsValidTimeframe(tf) {
		return tf
	}
	return DefaultTimeframe()
}
