package cache

import (
	"sync"
	"time"
)

type entry struct {
	v   any
	exp time.Time
}

type TTLCache struct {
	mu sync.RWMutex
	m  map[string]entry
}

func NewTTLCache() *TTLCache {
	return &TTLCache{m: make(map[string]entry)}
}

func (c *TTLCache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.exp.IsZero() && time.Now().After(e.exp) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}
	return e.v, true
}

func (c *TTLCache) Set(key string, v any, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.m[key] = entry{v: v, exp: exp}
	c.mu.Unlock()
}

// Implement BytesCache
func (c *TTLCache) GetBytes(key string) ([]byte, bool, error) {
	if v, ok := c.Get(key); ok {
		if b, ok2 := v.([]byte); ok2 {
			return b, true, nil
		}
		return nil, false, nil
	}
	return nil, false, nil
}

func (c *TTLCache) SetBytes(key string, value []byte, ttl time.Duration) error {
	c.Set(key, value, ttl)
	return nil
}
