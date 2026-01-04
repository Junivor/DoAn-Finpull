package cache

import "time"

// RedisOption configures Redis cache.
type RedisOption func(*RedisConfig)

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	PoolTimeout  time.Duration
	MinIdleConns int
	Prefix       string
}

// WithRedisHost sets Redis host.
func WithRedisHost(host string) RedisOption {
	return func(c *RedisConfig) {
		c.Host = host
	}
}

// WithRedisPort sets Redis port.
func WithRedisPort(port int) RedisOption {
	return func(c *RedisConfig) {
		c.Port = port
	}
}

// WithRedisPassword sets Redis password.
func WithRedisPassword(password string) RedisOption {
	return func(c *RedisConfig) {
		c.Password = password
	}
}

// WithRedisDB sets Redis database number.
func WithRedisDB(db int) RedisOption {
	return func(c *RedisConfig) {
		c.DB = db
	}
}

// WithRedisPool sets connection pool settings.
func WithRedisPool(poolSize, minIdleConns int, timeout time.Duration) RedisOption {
	return func(c *RedisConfig) {
		c.PoolSize = poolSize
		c.MinIdleConns = minIdleConns
		c.PoolTimeout = timeout
	}
}

// WithRedisPrefix sets key prefix.
func WithRedisPrefix(prefix string) RedisOption {
	return func(c *RedisConfig) {
		c.Prefix = prefix
	}
}

// MemoryOption configures Memory cache.
type MemoryOption func(*MemoryConfig)

// MemoryConfig holds memory cache configuration.
type MemoryConfig struct {
	MaxSize         int
	CleanupInterval time.Duration
}

// WithMemoryMaxSize sets max cache size.
func WithMemoryMaxSize(size int) MemoryOption {
	return func(c *MemoryConfig) {
		c.MaxSize = size
	}
}

// WithMemoryCleanup sets cleanup interval.
func WithMemoryCleanup(interval time.Duration) MemoryOption {
	return func(c *MemoryConfig) {
		c.CleanupInterval = interval
	}
}

// LayeredOption configures Layered cache.
type LayeredOption func(*LayeredConfig)

// LayeredConfig holds layered cache configuration.
type LayeredConfig struct {
	MemoryMaxSize int
}

// WithLayeredMemorySize sets L1 cache size.
func WithLayeredMemorySize(size int) LayeredOption {
	return func(c *LayeredConfig) {
		c.MemoryMaxSize = size
	}
}
