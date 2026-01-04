package cache

import (
	"context"
	"time"
)

// LayeredCache implements two-level cache (L1: Memory, L2: Redis).
type LayeredCache struct {
	memCache   *MemoryCache
	redisCache *RedisCache
}

// NewLayeredCache creates a layered cache with memory and Redis.
func NewLayeredCache(redisCache *RedisCache, opts ...LayeredOption) *LayeredCache {
	cfg := &LayeredConfig{
		MemoryMaxSize: 1000,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &LayeredCache{
		memCache:   NewMemoryCache(WithMemoryMaxSize(cfg.MemoryMaxSize)),
		redisCache: redisCache,
	}
}

func (lc *LayeredCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	// Write-through: Redis first, then memory
	if err := lc.redisCache.Set(ctx, key, value, expiration); err != nil {
		return err
	}
	_ = lc.memCache.Set(ctx, key, value, expiration)
	return nil
}

func (lc *LayeredCache) Get(ctx context.Context, key string, dest interface{}) error {
	// L1: Try memory first
	if err := lc.memCache.Get(ctx, key, dest); err == nil {
		return nil
	}

	// L2: Try Redis
	if err := lc.redisCache.Get(ctx, key, dest); err != nil {
		return err
	}

	// Store in memory for next time
	_ = lc.memCache.Set(ctx, key, dest, 0)
	return nil
}

func (lc *LayeredCache) Delete(ctx context.Context, keys ...string) error {
	_ = lc.memCache.Delete(ctx, keys...)
	return lc.redisCache.Delete(ctx, keys...)
}

func (lc *LayeredCache) DeleteByPattern(ctx context.Context, pattern string) error {
	_ = lc.memCache.DeleteByPattern(ctx, pattern)
	return lc.redisCache.DeleteByPattern(ctx, pattern)
}

func (lc *LayeredCache) Exists(ctx context.Context, keys ...string) (bool, error) {
	return lc.redisCache.Exists(ctx, keys...)
}

func (lc *LayeredCache) Increment(ctx context.Context, key string) (int64, error) {
	return lc.redisCache.Increment(ctx, key)
}

func (lc *LayeredCache) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return lc.redisCache.Expire(ctx, key, expiration)
}

func (lc *LayeredCache) MSet(ctx context.Context, values map[string]interface{}, expiration time.Duration) error {
	return lc.redisCache.MSet(ctx, values, expiration)
}

func (lc *LayeredCache) MGet(ctx context.Context, keys ...string) (map[string]string, error) {
	return lc.redisCache.MGet(ctx, keys...)
}

func (lc *LayeredCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return lc.redisCache.TryLock(ctx, key, ttl)
}

func (lc *LayeredCache) Unlock(ctx context.Context, key string) error {
	return lc.redisCache.Unlock(ctx, key)
}

// Close closes both cache layers.
func (lc *LayeredCache) Close() error {
	_ = lc.memCache.Close()
	return lc.redisCache.Close()
}
