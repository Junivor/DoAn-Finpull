package kafka

import "time"

// ProducerOption configures Producer.
type ProducerOption func(*ProducerConfig)

// ProducerConfig holds producer configuration.
type ProducerConfig struct {
	Brokers      []string
	RequiredAcks int
	Compression  string
	MaxAttempts  int
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
	BatchSize    int
	BatchBytes   int
	BatchTimeout time.Duration
	Async        bool
	HashByKey    bool
}

// WithBrokers sets Kafka brokers.
func WithBrokers(brokers []string) ProducerOption {
	return func(c *ProducerConfig) {
		c.Brokers = brokers
	}
}

// WithCompression sets compression type.
func WithCompression(compression string) ProducerOption {
	return func(c *ProducerConfig) {
		c.Compression = compression
	}
}

// WithRequiredAcks sets required acknowledgements (-1 = all).
func WithRequiredAcks(acks int) ProducerOption {
	return func(c *ProducerConfig) {
		c.RequiredAcks = acks
	}
}

// WithMaxAttempts sets max retry attempts by the writer.
func WithMaxAttempts(n int) ProducerOption {
	return func(c *ProducerConfig) {
		c.MaxAttempts = n
	}
}

// WithBatchSize sets batch size.
func WithBatchSize(size int) ProducerOption {
	return func(c *ProducerConfig) {
		c.BatchSize = size
	}
}

// WithBatchTimeout sets batch timeout.
func WithBatchTimeout(timeout time.Duration) ProducerOption {
	return func(c *ProducerConfig) {
		c.BatchTimeout = timeout
	}
}

// WithBatchBytes sets target aggregate batch bytes.
func WithBatchBytes(bytes int) ProducerOption {
	return func(c *ProducerConfig) {
		c.BatchBytes = bytes
	}
}

// WithTimeouts sets writer read/write timeouts.
func WithTimeouts(write, read time.Duration) ProducerOption {
	return func(c *ProducerConfig) {
		c.WriteTimeout = write
		c.ReadTimeout = read
	}
}

// WithAsync toggles async writes (fire-and-forget).
func WithAsync(async bool) ProducerOption {
	return func(c *ProducerConfig) {
		c.Async = async
	}
}

// WithHashByKey sets hash balancer for per-key (symbol) ordering.
func WithHashByKey(hash bool) ProducerOption {
	return func(c *ProducerConfig) {
		c.HashByKey = hash
	}
}
