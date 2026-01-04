package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryItem stores cached value with expiration.
type MemoryItem struct {
	Value    interface{}
	ExpireAt time.Time
}

// IsExpired checks if item has expired.
func (m *MemoryItem) IsExpired() bool {
	return time.Now().After(m.ExpireAt)
}

// MemoryCache implements Service using in-memory storage with LRU eviction.
type MemoryCache struct {
	data          map[string]*MemoryItem
	access        map[string]time.Time
	mutex         sync.RWMutex
	maxSize       int
	cleanupTicker *time.Ticker
}

// NewMemoryCache creates an in-memory cache.
func NewMemoryCache(opts ...MemoryOption) *MemoryCache {
	cfg := &MemoryConfig{
		MaxSize:         1000,
		CleanupInterval: 5 * time.Minute,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	mc := &MemoryCache{
		data:          make(map[string]*MemoryItem),
		access:        make(map[string]time.Time),
		maxSize:       cfg.MaxSize,
		cleanupTicker: time.NewTicker(cfg.CleanupInterval),
	}

	go mc.cleanupExpired()
	return mc
}

func (mc *MemoryCache) Set(_ context.Context, key string, value interface{}, expiration time.Duration) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if len(mc.data) >= mc.maxSize {
		mc.evictLRU()
	}

	expireAt := time.Now().Add(expiration)
	if expiration <= 0 {
		expireAt = time.Now().Add(7 * 24 * time.Hour) // default 7 days
	}

	mc.data[key] = &MemoryItem{
		Value:    value,
		ExpireAt: expireAt,
	}
	mc.access[key] = time.Now()
	return nil
}

func (mc *MemoryCache) Get(_ context.Context, key string, dest interface{}) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	item, exists := mc.data[key]
	if !exists || item.IsExpired() {
		if exists {
			delete(mc.data, key)
			delete(mc.access, key)
		}
		return ErrCacheMiss
	}

	mc.access[key] = time.Now()

	// Assign value to dest
	if strPtr, ok := dest.(*string); ok {
		if str, ok := item.Value.(string); ok {
			*strPtr = str
			return nil
		}
	}

	// For other types, simple assignment
	*dest.(*interface{}) = item.Value
	return nil
}

func (mc *MemoryCache) Delete(_ context.Context, keys ...string) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	for _, key := range keys {
		delete(mc.data, key)
		delete(mc.access, key)
	}
	return nil
}

func (mc *MemoryCache) DeleteByPattern(_ context.Context, _ string) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.data = make(map[string]*MemoryItem)
	mc.access = make(map[string]time.Time)
	return nil
}

func (mc *MemoryCache) Exists(_ context.Context, keys ...string) (bool, error) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	for _, key := range keys {
		if item, ok := mc.data[key]; ok && !item.IsExpired() {
			return true, nil
		}
	}
	return false, nil
}

func (mc *MemoryCache) Increment(_ context.Context, key string) (int64, error) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	item, exists := mc.data[key]
	if !exists {
		mc.data[key] = &MemoryItem{Value: int64(1), ExpireAt: time.Now().Add(7 * 24 * time.Hour)}
		return 1, nil
	}

	if val, ok := item.Value.(int64); ok {
		newVal := val + 1
		item.Value = newVal
		return newVal, nil
	}

	return 0, fmt.Errorf("value is not int64")
}

func (mc *MemoryCache) Expire(_ context.Context, key string, expiration time.Duration) (bool, error) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if item, ok := mc.data[key]; ok {
		item.ExpireAt = time.Now().Add(expiration)
		return true, nil
	}
	return false, nil
}

func (mc *MemoryCache) MSet(ctx context.Context, values map[string]interface{}, expiration time.Duration) error {
	for key, value := range values {
		if err := mc.Set(ctx, key, value, expiration); err != nil {
			return err
		}
	}
	return nil
}

func (mc *MemoryCache) MGet(_ context.Context, keys ...string) (map[string]string, error) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	results := make(map[string]string)
	for _, key := range keys {
		if item, ok := mc.data[key]; ok && !item.IsExpired() {
			if str, ok := item.Value.(string); ok {
				results[key] = str
			}
		}
	}
	return results, nil
}

func (mc *MemoryCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if item, ok := mc.data[key]; ok && !item.IsExpired() {
		return false, nil
	}

	mc.data[key] = &MemoryItem{Value: "locked", ExpireAt: time.Now().Add(ttl)}
	return true, nil
}

func (mc *MemoryCache) Unlock(ctx context.Context, key string) error {
	return mc.Delete(ctx, key)
}

func (mc *MemoryCache) evictLRU() {
	if len(mc.data) == 0 {
		return
	}

	var oldestKey string
	oldestTime := time.Now()

	for key, accessTime := range mc.access {
		if accessTime.Before(oldestTime) {
			oldestTime = accessTime
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(mc.data, oldestKey)
		delete(mc.access, oldestKey)
	}
}

func (mc *MemoryCache) cleanupExpired() {
	for range mc.cleanupTicker.C {
		mc.mutex.Lock()
		now := time.Now()
		expiredKeys := make([]string, 0)

		for key, item := range mc.data {
			if now.After(item.ExpireAt) {
				expiredKeys = append(expiredKeys, key)
			}
		}

		for _, key := range expiredKeys {
			delete(mc.data, key)
			delete(mc.access, key)
		}
		mc.mutex.Unlock()
	}
}

// Close stops the cleanup ticker.
func (mc *MemoryCache) Close() error {
	if mc.cleanupTicker != nil {
		mc.cleanupTicker.Stop()
	}
	return nil
}
