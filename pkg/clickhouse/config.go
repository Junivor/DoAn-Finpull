package clickhouse

import "time"

// ClientOption configures Client.
type ClientOption func(*ClientConfig)

// ClientConfig holds ClickHouse configuration.
type ClientConfig struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    UseHTTP         bool
    AsyncInsert     bool
    WaitForAsync    bool
    MaxExecTime     time.Duration
}

// WithHost sets database host.
func WithHost(host string) ClientOption {
	return func(c *ClientConfig) {
		c.Host = host
	}
}

// WithPort sets database port.
func WithPort(port int) ClientOption {
	return func(c *ClientConfig) {
		c.Port = port
	}
}

// WithDatabase sets database name.
func WithDatabase(database string) ClientOption {
	return func(c *ClientConfig) {
		c.Database = database
	}
}

// WithCredentials sets username and password.
func WithCredentials(user, password string) ClientOption {
	return func(c *ClientConfig) {
		c.User = user
		c.Password = password
	}
}

// WithMaxConnections sets max open and idle connections.
func WithMaxConnections(maxOpen, maxIdle int) ClientOption {
	return func(c *ClientConfig) {
		c.MaxOpenConns = maxOpen
		c.MaxIdleConns = maxIdle
	}
}

// WithTimeouts sets dial/read/write timeouts.
func WithTimeouts(dial, read, write time.Duration) ClientOption {
    return func(c *ClientConfig) {
        c.DialTimeout = dial
        c.ReadTimeout = read
        c.WriteTimeout = write
    }
}

// WithHTTP enables HTTP protocol instead of native.
func WithHTTP(useHTTP bool) ClientOption {
    return func(c *ClientConfig) {
        c.UseHTTP = useHTTP
    }
}

// WithAsyncInsert configures async_insert and wait behavior.
func WithAsyncInsert(enabled, wait bool) ClientOption {
    return func(c *ClientConfig) {
        c.AsyncInsert = enabled
        c.WaitForAsync = wait
    }
}

// WithMaxExecutionTime sets max_execution_time per query.
func WithMaxExecutionTime(d time.Duration) ClientOption {
    return func(c *ClientConfig) {
        c.MaxExecTime = d
    }
}
