package cache

import "time"

// BytesCache is a minimal cache API storing raw bytes with TTL.
type BytesCache interface {
	GetBytes(key string) (b []byte, ok bool, err error)
	SetBytes(key string, value []byte, ttl time.Duration) error
}
