package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Environment string `yaml:"environment"`
	Server      struct {
		Port            int           `yaml:"port"`
		ReadTimeout     time.Duration `yaml:"read_timeout"`
		WriteTimeout    time.Duration `yaml:"write_timeout"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"server"`
	Metrics struct {
		Enabled bool   `yaml:"enabled"`
		Path    string `yaml:"path"`
	} `yaml:"metrics"`
	Backend struct {
		Type         string        `yaml:"type"`
		BatchSize    int           `yaml:"batch_size"`
		BatchTimeout time.Duration `yaml:"batch_timeout"`
	} `yaml:"backend"`
	Kafka struct {
		Brokers      []string `yaml:"brokers"`
		Topic        string   `yaml:"topic"`
		RequiredAcks int      `yaml:"required_acks"`
		Compression  string   `yaml:"compression"`
		Producer     struct {
			MaxAttempts  int           `yaml:"max_attempts"`
			Linger       time.Duration `yaml:"linger"`
			BatchBytes   int           `yaml:"batch_bytes"`
			BatchSize    int           `yaml:"batch_size"`
			WriteTimeout time.Duration `yaml:"write_timeout"`
			ReadTimeout  time.Duration `yaml:"read_timeout"`
			Async        bool          `yaml:"async"`
		} `yaml:"producer"`
		Consumer struct {
			GroupID    string        `yaml:"group_id"`
			Workers    int           `yaml:"workers"`
			BufferSize int           `yaml:"buffer_size"`
			RetryMax   int           `yaml:"retry_max"`
			BackoffMin time.Duration `yaml:"backoff_min"`
			BackoffMax time.Duration `yaml:"backoff_max"`
			DLQTopic   string        `yaml:"dlq_topic"`
			MinBytes   int           `yaml:"min_bytes"`
			MaxBytes   int           `yaml:"max_bytes"`
		} `yaml:"consumer"`
	} `yaml:"kafka"`
	ClickHouse struct {
		Host             string        `yaml:"host"`
		Port             int           `yaml:"port"`
		Database         string        `yaml:"database"`
		User             string        `yaml:"user"`
		Password         string        `yaml:"password"`
		UseHTTP          bool          `yaml:"use_http"`
		AsyncInsert      bool          `yaml:"async_insert"`
		WaitForAsync     bool          `yaml:"wait_for_async_insert"`
		DialTimeout      time.Duration `yaml:"dial_timeout"`
		ReadTimeout      time.Duration `yaml:"read_timeout"`
		WriteTimeout     time.Duration `yaml:"write_timeout"`
		MaxExecutionTime time.Duration `yaml:"max_execution_time"`
	} `yaml:"clickhouse"`
	Finnhub struct {
		APIKey         string        `yaml:"api_key"`
		WebSocketURL   string        `yaml:"websocket_url"`
		Symbols        []string      `yaml:"symbols"`
		ReconnectDelay time.Duration `yaml:"reconnect_delay"`
		PingInterval   time.Duration `yaml:"ping_interval"`
	} `yaml:"finnhub"`
	Analytics struct {
		PythonServiceURL string        `yaml:"python_service_url"`
		ModelDir         string        `yaml:"model_dir"`
		Timeout          time.Duration `yaml:"timeout"`
		CacheTTL         struct {
			Regime time.Duration `yaml:"regime"`
			Vol    time.Duration `yaml:"vol"`
			Anom   time.Duration `yaml:"anomaly"`
			Edge   time.Duration `yaml:"edge"`
		} `yaml:"cache_ttl"`
		Redis struct {
			Enabled  bool   `yaml:"enabled"`
			Addr     string `yaml:"addr"`
			Password string `yaml:"password"`
			DB       int    `yaml:"db"`
		} `yaml:"redis"`
	} `yaml:"analytics"`
}

// Load reads and parses a YAML configuration file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Validate required fields
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &c, nil
}

// LoadWithEnv loads config from YAML and overrides with environment variables.
func LoadWithEnv(path string) (*Config, error) {
	c, err := Load(path)
	if err != nil {
		return nil, err
	}

	// Override with environment variables
	if v := os.Getenv("FINNHUB_API_KEY"); v != "" {
		c.Finnhub.APIKey = v
	}
	if v := os.Getenv("SYMBOLS"); v != "" {
		c.Finnhub.Symbols = strings.Split(v, ",")
	}
	if v := os.Getenv("BACKEND"); v != "" {
		c.Backend.Type = v
	}
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		c.Kafka.Brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("KAFKA_TOPIC"); v != "" {
		c.Kafka.Topic = v
	}

	return c, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Environment == "" {
		return fmt.Errorf("environment is required")
	}
	if c.Backend.Type == "" {
		return fmt.Errorf("backend.type is required")
	}
	if c.Backend.Type != "kafka" && c.Backend.Type != "clickhouse" {
		return fmt.Errorf("backend.type must be 'kafka' or 'clickhouse', got '%s'", c.Backend.Type)
	}
	if len(c.Finnhub.Symbols) == 0 {
		return fmt.Errorf("finnhub.symbols cannot be empty")
	}
	if c.Finnhub.APIKey == "" {
		return fmt.Errorf("finnhub.api_key is required")
	}
	return nil
}
