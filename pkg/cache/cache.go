package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrCacheMiss = errors.New("cache: key not found")
)

// Service defines cache operations interface.
type Service interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, keys ...string) error
	DeleteByPattern(ctx context.Context, pattern string) error
	Exists(ctx context.Context, keys ...string) (bool, error)
	Increment(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	MSet(ctx context.Context, values map[string]interface{}, expiration time.Duration) error
	MGet(ctx context.Context, keys ...string) (map[string]string, error)
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
}

// MGetTyped retrieves multiple keys and unmarshals to typed map.
func MGetTyped[T any](ctx context.Context, c Service, keys ...string) (map[string]T, error) {
	if len(keys) == 0 {
		return make(map[string]T), nil
	}

	rawResults, err := c.MGet(ctx, keys...)
	if err != nil {
		return nil, err
	}

	typedResults := make(map[string]T, len(rawResults))
	for key, rawValue := range rawResults {
		var obj T
		if err := json.Unmarshal([]byte(rawValue), &obj); err != nil {
			continue // Skip invalid JSON
		}
		typedResults[key] = obj
	}

	return typedResults, nil
}
