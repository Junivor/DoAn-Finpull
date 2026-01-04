package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// Client manages ClickHouse connection pool.
type Client struct {
	db *sql.DB
}

// NewClient creates a ClickHouse client with connection pool.
func NewClient(opts ...ClientOption) (*Client, error) {
	cfg := &ClientConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("host is required")
	}

	dsn := buildDSN(*cfg)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
        _ = db.Close() // best-effort close
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	return &Client{db: db}, nil
}

// DB returns *sql.DB for direct use.
func (c *Client) DB() *sql.DB {
	return c.db
}

// Health performs health check.
func (c *Client) Health(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Close closes connection pool.
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// InitSchema ensures database and tables exist (idempotent).
func (c *Client) InitSchema(ctx context.Context, stmts []string) error {
	for _, stmt := range stmts {
		if _, err := c.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func buildDSN(cfg ClientConfig) string {
	scheme := "clickhouse://"
	if cfg.UseHTTP {
		scheme = "clickhouse+http://"
	}
	dsn := fmt.Sprintf("%s%s:%s@%s:%d/%s",
		scheme, cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	// helper to add query params
	add := func(first bool, key string, val any) string {
		sep := "&"
		if first {
			sep = "?"
		}
		return fmt.Sprintf("%s%s=%v", sep, key, val)
	}

	first := true
	if cfg.DialTimeout > 0 {
		dsn += add(first, "dial_timeout", cfg.DialTimeout)
		first = false
	}
	if cfg.ReadTimeout > 0 {
		dsn += add(first, "read_timeout", cfg.ReadTimeout)
		first = false
	}
	// Note: write_timeout is not supported as a server setting on some versions;
	// keep it client-side only (do not append to DSN).
	if cfg.MaxExecTime > 0 {
		// seconds granularity is typical for max_execution_time
		dsn += add(first, "max_execution_time", int(cfg.MaxExecTime.Seconds()))
		first = false
	}
	if cfg.AsyncInsert {
		dsn += add(first, "async_insert", 1)
		first = false
		if cfg.WaitForAsync {
			dsn += add(first, "wait_for_async_insert", 1)
			first = false
		}
	}
	return dsn
}
