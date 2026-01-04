package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	cli *redis.Client
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func NewRedisCache(cfg RedisConfig) *RedisCache {
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB})
	return &RedisCache{cli: rdb}
}

func (r *RedisCache) GetBytes(key string) ([]byte, bool, error) {
    b, err := r.cli.Get(context.Background(), key).Bytes()
    if err != nil {
        if err == redis.Nil {
            return nil, false, nil
        }
        return nil, false, err
    }
    return b, true, nil
}

func (r *RedisCache) SetBytes(key string, value []byte, ttl time.Duration) error {
    if err := r.cli.Set(context.Background(), key, value, ttl).Err(); err != nil {
        return err
    }
    return nil
}
