package kafka

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
)

// MessageHandler handles messages from a specific topic.
type MessageHandler interface {
	Topic() string
	Handle(context.Context, []byte) error
}

// ConsumerOption configures Consumer.
type ConsumerOption func(*ConsumerConfig)

// ConsumerConfig holds consumer configuration.
type ConsumerConfig struct {
	Brokers         []string
	GroupID         string
	AutoOffsetReset string
	WorkerCount     int
	BufferSize      int
	RetryMax        int
	BackoffMin      time.Duration
	BackoffMax      time.Duration
	DLQTopic        string
	MinBytes        int
	MaxBytes        int
}

// WithConsumerBrokers sets Kafka brokers.
func WithConsumerBrokers(brokers []string) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.Brokers = brokers
	}
}

// WithConsumerGroupID sets consumer group ID.
func WithConsumerGroupID(groupID string) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.GroupID = groupID
	}
}

// WithConsumerAutoOffsetReset sets auto offset reset strategy.
func WithConsumerAutoOffsetReset(autoOffsetReset string) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.AutoOffsetReset = autoOffsetReset
	}
}

// WithConsumerWorkers sets number of worker goroutines.
func WithConsumerWorkers(count int) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.WorkerCount = count
	}
}

// WithConsumerRetry configures retry attempts and backoff range.
func WithConsumerRetry(max int, backoffMin, backoffMax time.Duration) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.RetryMax = max
		c.BackoffMin = backoffMin
		c.BackoffMax = backoffMax
	}
}

// WithConsumerDLQ sets a Kafka topic name for DLQ.
func WithConsumerDLQ(topic string) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.DLQTopic = topic
	}
}

// WithConsumerFetch sets fetch min/max bytes.
func WithConsumerFetch(minBytes, maxBytes int) ConsumerOption {
	return func(c *ConsumerConfig) {
		c.MinBytes = minBytes
		c.MaxBytes = maxBytes
	}
}

// WithConsumerBufferSize sets the internal channel buffer size.
func WithConsumerBufferSize(n int) ConsumerOption {
	return func(c *ConsumerConfig) {
		if n > 0 {
			c.BufferSize = n
		}
	}
}

// Consumer wraps Kafka reader with worker pool.
type Consumer struct {
	cfg       *ConsumerConfig
	readers   map[string]*kafka.Reader
	handlers  map[string]MessageHandler
	stopChan  chan struct{}
	wg        sync.WaitGroup
	stopOnce  sync.Once
	msgChan   chan *message
	dlq       *kafka.Writer
	partLocks map[string]map[int]*sync.Mutex
	hook      ConsumerHook
}

type message struct {
	topic string
	data  []byte
	km    kafka.Message
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(opts ...ConsumerOption) (*Consumer, error) {
	cfg := &ConsumerConfig{
		GroupID:         "default",
		AutoOffsetReset: "earliest",
		WorkerCount:     1,
		BufferSize:      10,
		RetryMax:        3,
		BackoffMin:      50 * time.Millisecond,
		BackoffMax:      2 * time.Second,
		MinBytes:        10e3, // 10KB
		MaxBytes:        10e6, // 10MB
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("brokers are required")
	}

	c := &Consumer{
		cfg:       cfg,
		readers:   make(map[string]*kafka.Reader),
		handlers:  make(map[string]MessageHandler),
		stopChan:  make(chan struct{}),
		msgChan:   make(chan *message, cfg.BufferSize),
		partLocks: make(map[string]map[int]*sync.Mutex),
		hook:      NoopHook{},
	}

	initConsumerMetricsOnce()

	if cfg.DLQTopic != "" {
		c.dlq = &kafka.Writer{Addr: kafka.TCP(cfg.Brokers...), Balancer: &kafka.LeastBytes{}}
	}

	return c, nil
}

// RegisterHandler registers a message handler for a specific topic.
func (c *Consumer) RegisterHandler(handler MessageHandler) {
	topic := handler.Topic()
	if _, ok := c.handlers[topic]; ok {
		log.Printf("warn: handler already registered for topic %s", topic)
	} else {
		c.handlers[topic] = handler
	}
}

// Start starts the Kafka consumer and workers.
func (c *Consumer) Start() error {
	// Create readers for each registered topic
	for topic, handler := range c.handlers {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  c.cfg.Brokers,
			Topic:    topic,
			GroupID:  c.cfg.GroupID,
			MinBytes: c.cfg.MinBytes,
			MaxBytes: c.cfg.MaxBytes,
		})
		c.readers[topic] = reader
		log.Printf("kafka consumer: registered topic=%s", handler.Topic())
	}

	// Start worker pool
	for i := 0; i < c.cfg.WorkerCount; i++ {
		c.wg.Add(1)
		go c.messageWorker()
	}
	log.Printf("kafka consumer: started workers=%d", c.cfg.WorkerCount)

	// Start readers for each topic
	for topic, reader := range c.readers {
		c.wg.Add(1)
		go c.consumeMessages(topic, reader)
	}

	log.Printf("kafka consumer: started successfully")
	return nil
}

// Stop stops the Kafka consumer gracefully.
func (c *Consumer) Stop(ctx context.Context) error {
	var stopErr error

	c.stopOnce.Do(func() {
		log.Println("kafka consumer: stopping...")

		// Signal goroutines to stop
		close(c.stopChan)

		// Close message channel to stop workers
		close(c.msgChan)

		// Wait for all goroutines to finish with context timeout
		stopErr = c.waitForWg(ctx)

		// Close all readers
		for topic, reader := range c.readers {
			if err := reader.Close(); err != nil {
				log.Printf("error closing reader for topic %s: %v", topic, err)
			}
		}

		if c.dlq != nil {
			if err := c.dlq.Close(); err != nil {
				log.Printf("error closing dlq writer: %v", err)
			}
		}

		if stopErr == nil {
			log.Println("kafka consumer: stopped successfully")
		}
	})

	return stopErr
}

func (c *Consumer) waitForWg(ctx context.Context) error {
	doneChan := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for consumer to stop: %w", ctx.Err())
	case <-doneChan:
		return nil
	}
}

func (c *Consumer) consumeMessages(topic string, reader *kafka.Reader) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			msg, err := reader.ReadMessage(ctx)
			cancel()

			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					log.Printf("error reading message from topic %s: %v", topic, err)
				}
				continue
			}

			// Send message to worker pool with backpressure (avoid drops)
			for {
				select {
				case c.msgChan <- &message{topic: topic, data: msg.Value, km: msg}:
					// Message sent to worker
					// update queue metrics after enqueue
					if consumerQueueDepth != nil {
						consumerQueueDepth.WithLabelValues(topic).Set(float64(len(c.msgChan)))
					}
					if consumerQueueFullness != nil {
						consumerQueueFullness.WithLabelValues(topic).Set(float64(len(c.msgChan)) / float64(cap(c.msgChan)))
					}
					goto sent
				case <-c.stopChan:
					return
				default:
					// Apply adaptive backpressure
					full := float64(len(c.msgChan)) / float64(cap(c.msgChan))
					if consumerQueueFullness != nil {
						consumerQueueFullness.WithLabelValues(topic).Set(full)
					}
					if full > 0.8 {
						time.Sleep(10 * time.Millisecond)
					} else {
						runtime.Gosched()
					}
				}
			}
		sent:
		}
	}
}

// messageWorker processes messages from the channel.
func (c *Consumer) messageWorker() {
	defer c.wg.Done()

	for msg := range c.msgChan {
		if handler, exists := c.handlers[msg.topic]; exists {
			// Add panic recovery to prevent worker goroutine crashes
			start := time.Now()
			func() {
				//fmt.Println("RECEIVED WORKER START")
				defer func() {
					if r := recover(); r != nil {
						log.Printf("panic in message handler for topic %s: %v", handler.Topic(), r)
					}
				}()
				// Ensure max in-flight=1 per (topic, partition)
				pl := c.getPartitionLock(msg.topic, msg.km.Partition)
				pl.Lock()
				defer pl.Unlock()

				var err error
				attempts := 0
				for {
					attempts++
					// apply hook before handle
					hctx, hmsg, hdata, berr := c.hook.BeforeHandle(context.Background(), msg.topic, msg.km, msg.data)
					if berr != nil {
						err = berr
						break
					}

					err = handler.Handle(hctx, hdata)
					// after handle hook
					c.hook.AfterHandle(hctx, msg.topic, hmsg, hdata, err)
					if err == nil || attempts > c.cfg.RetryMax {
						break
					}
					// per-attempt error hook before backoff
					c.hook.OnError(hctx, msg.topic, hmsg, hdata, err)
					// backoff with jitter
					sleep := backoffWithJitter(c.cfg.BackoffMin, c.cfg.BackoffMax, attempts)
					select {
					case <-time.After(sleep):
					case <-c.stopChan:
						return
					}
				}
				if err != nil {
					// error hook
					c.hook.OnError(context.Background(), msg.topic, msg.km, msg.data, err)
					log.Printf("error handling message from topic %s after %d attempts: %v", handler.Topic(), attempts-1, err)
					// DLQ publish
					if c.dlq != nil && c.cfg.DLQTopic != "" {
						if dlqErr := c.dlq.WriteMessages(context.Background(), kafka.Message{
							Topic:   c.cfg.DLQTopic,
							Value:   msg.data,
							Time:    time.Now(),
							Headers: []kafka.Header{{Key: "source_topic", Value: []byte(handler.Topic())}},
						}); dlqErr != nil {
							log.Printf("error writing to DLQ topic %s: %v", c.cfg.DLQTopic, dlqErr)
						}
					}
				}

				// Commit offset on success or after DLQ to avoid poison loops
				if err == nil || (c.dlq != nil && c.cfg.DLQTopic != "") {
					if reader := c.readers[msg.topic]; reader != nil {
						_ = c.commitWithRetry(reader, msg.km, 3)
					}
				}
				// record handling latency
				if consumerHandleLatency != nil {
					consumerHandleLatency.WithLabelValues(msg.topic).Observe(time.Since(start).Seconds())
				}
			}()
		}
	}
}

// commitWithRetry commits a single message offset with bounded retries.
func (c *Consumer) commitWithRetry(reader *kafka.Reader, km kafka.Message, max int) error {
	if max <= 0 {
		max = 1
	}
	var err error
	for attempt := 1; attempt <= max; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = reader.CommitMessages(ctx, km)
		cancel()
		if err == nil {
			return nil
		}
		sleep := backoffWithJitter(50*time.Millisecond, 500*time.Millisecond, attempt)
		time.Sleep(sleep)
	}
	log.Printf("error committing message after %d attempts: %v", max, err)
	return err
}

func (c *Consumer) getPartitionLock(topic string, partition int) *sync.Mutex {
	// fast path
	if m, ok := c.partLocks[topic]; ok {
		if l, ok2 := m[partition]; ok2 {
			return l
		}
	}
	// create lazily
	if _, ok := c.partLocks[topic]; !ok {
		c.partLocks[topic] = make(map[int]*sync.Mutex)
	}
	if _, ok := c.partLocks[topic][partition]; !ok {
		c.partLocks[topic][partition] = &sync.Mutex{}
	}
	return c.partLocks[topic][partition]
}

func backoffWithJitter(min, max time.Duration, attempt int) time.Duration {
	if min <= 0 {
		min = 50 * time.Millisecond
	}
	if max < min {
		max = min
	}
	// exponential backoff base
	exp := min * time.Duration(1<<uint(attempt-1))
	if exp > max {
		exp = max
	}
	// jitter up to 50%
	jitter := time.Duration(rand.Int63n(int64(exp) / 2))
	return exp - jitter
}

// Consumer metrics
var (
	consumerQueueDepth    *prometheus.GaugeVec
	consumerQueueFullness *prometheus.GaugeVec
	consumerHandleLatency *prometheus.HistogramVec
	consumerOnce          = make(chan struct{}, 1)
	consumerRegisterer    prometheus.Registerer
)

// SetConsumerMetricsRegisterer sets a custom Prometheus registerer for consumer metrics (useful for testing).
func SetConsumerMetricsRegisterer(reg prometheus.Registerer) { consumerRegisterer = reg }

func initConsumerMetricsOnce() {
	select {
	case consumerOnce <- struct{}{}:
		if consumerRegisterer != nil {
			consumerQueueDepth = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{Name: "finpull_kafka_consumer_queue_depth", Help: "Number of messages waiting in consumer queue"},
				[]string{"topic"},
			)
			consumerQueueFullness = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{Name: "finpull_kafka_consumer_queue_fullness", Help: "Queue utilization ratio (len/cap)"},
				[]string{"topic"},
			)
			consumerHandleLatency = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{Name: "finpull_kafka_consumer_handle_seconds", Help: "Handling time per message"},
				[]string{"topic"},
			)
			consumerRegisterer.MustRegister(consumerQueueDepth, consumerQueueFullness, consumerHandleLatency)
		} else {
			consumerQueueDepth = promauto.NewGaugeVec(
				prometheus.GaugeOpts{Name: "finpull_kafka_consumer_queue_depth", Help: "Number of messages waiting in consumer queue"},
				[]string{"topic"},
			)
			consumerQueueFullness = promauto.NewGaugeVec(
				prometheus.GaugeOpts{Name: "finpull_kafka_consumer_queue_fullness", Help: "Queue utilization ratio (len/cap)"},
				[]string{"topic"},
			)
			consumerHandleLatency = promauto.NewHistogramVec(
				prometheus.HistogramOpts{Name: "finpull_kafka_consumer_handle_seconds", Help: "Handling time per message"},
				[]string{"topic"},
			)
		}
	default:
		// already initialized
	}
}

// WithConsumerHook sets a hook implementation for lifecycle events.
func (c *Consumer) WithConsumerHook(h ConsumerHook) {
	if h != nil {
		c.hook = h
	}
}
