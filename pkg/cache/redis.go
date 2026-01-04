package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Service using Redis.
type RedisCache struct {
	client *redis.Client
	prefix string
}

// NewRedisCache creates a Redis cache client.
func NewRedisCache(opts ...RedisOption) (*RedisCache, error) {
	cfg := &RedisConfig{
		Host:         "localhost",
		Port:         6379,
		DB:           0,
		PoolSize:     10,
		PoolTimeout:  30 * time.Second,
		MinIdleConns: 5,
		Prefix:       "finpull",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		PoolTimeout:  cfg.PoolTimeout,
		MinIdleConns: cfg.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisCache{
		client: client,
		prefix: cfg.Prefix,
	}, nil
}

// Client returns underlying redis client.
func (c *RedisCache) Client() *redis.Client {
	return c.client
}

// Close closes the Redis connection.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	key = c.wrapKey(key)

	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	default:
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}
	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	key = c.wrapKey(key)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return err
	}

	if strPtr, ok := dest.(*string); ok {
		*strPtr = string(data)
		return nil
	}

	return json.Unmarshal(data, dest)
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	keys = c.wrapKeys(keys...)
	return c.client.Unlink(ctx, keys...).Err()
}

func (c *RedisCache) DeleteByPattern(ctx context.Context, pattern string) error {
	pattern = c.wrapKey(pattern)

	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	return c.client.Unlink(ctx, keys...).Err()
}

func (c *RedisCache) Exists(ctx context.Context, keys ...string) (bool, error) {
	keys = c.wrapKeys(keys...)
	result, err := c.client.Exists(ctx, keys...).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	key = c.wrapKey(key)
	return c.client.Incr(ctx, key).Result()
}

func (c *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	key = c.wrapKey(key)
	return c.client.Expire(ctx, key, expiration).Result()
}

func (c *RedisCache) MSet(ctx context.Context, values map[string]interface{}, expiration time.Duration) error {
	if len(values) == 0 {
		return nil
	}

	pipe := c.client.TxPipeline()
	for key, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		key = c.wrapKey(key)
		pipe.Set(ctx, key, data, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) MGet(ctx context.Context, keys ...string) (map[string]string, error) {
	if len(keys) == 0 {
		return make(map[string]string), nil
	}

	wrappedKeys := c.wrapKeys(keys...)
	results, err := c.client.MGet(ctx, wrappedKeys...).Result()
	if err != nil {
		return nil, err
	}

	resultMap := make(map[string]string, len(keys))
	for i, key := range keys {
		if results[i] != nil {
			if val, ok := results[i].(string); ok {
				resultMap[key] = val
			}
		}
	}
	return resultMap, nil
}

func (c *RedisCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	key = c.wrapKey(key)
	return c.client.SetNX(ctx, key, "locked", ttl).Result()
}

func (c *RedisCache) Unlock(ctx context.Context, key string) error {
	key = c.wrapKey(key)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return err
	}
	return nil
}

func (c *RedisCache) wrapKey(key string) string {
	return fmt.Sprintf("%s:%s", c.prefix, key)
}

func (c *RedisCache) unwrapKey(key string) string {
	if strings.HasPrefix(key, c.prefix+":") {
		return strings.TrimPrefix(key, c.prefix+":")
	}
	return key
}

func (c *RedisCache) wrapKeys(keys ...string) []string {
	wrapped := make([]string, len(keys))
	for i, key := range keys {
		wrapped[i] = c.wrapKey(key)
	}
	return wrapped
}
